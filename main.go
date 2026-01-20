package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"regexp"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unicode"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	"github.com/charmbracelet/wish/bubbletea"
	"github.com/charmbracelet/wish/logging"
	gossh "golang.org/x/crypto/ssh"
)

const (
	shaLen = 7

	// Connection and resource limits
	maxConnections      = 100
	maxConnectionsPerIP = 10
	maxPlayers          = 15
	sessionTimeout      = 30 * time.Minute

	// Player name validation
	minNameLength = 2
	maxNameLength = 20
)

// validNameRegex allows only alphanumeric characters, spaces, hyphens, and underscores
var validNameRegex = regexp.MustCompile(`^[a-zA-Z0-9 _-]+$`)

// Build-time variables set by -ldflags
var (
	// Version contains the application Version number. It's set via ldflags
	// when building.
	Version = "dev"

	// CommitSHA contains the SHA of the commit that this application was built
	// against. It's set via ldflags when building.
	CommitSHA = "unknown"
)

// Connection tracking for DoS protection
var (
	connectionCount atomic.Int32
	connectionsByIP sync.Map // map[string]*atomic.Int32
	reservedNames   = []string{"master", "admin", "system", "server", "scrum", "poker"}
)

// getConfigPath returns an absolute path for configuration files.
// It uses the current working directory as the base to ensure consistent
// path resolution regardless of how the application is started.
func getConfigPath(filename string) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get working directory: %w", err)
	}
	return fmt.Sprintf("%s/.ssh/%s", cwd, filename), nil
}

// validatePlayerName performs comprehensive validation on player names to prevent
// injection attacks, control character exploits, and ensure usability.
func validatePlayerName(name string) error {
	// Trim whitespace
	name = strings.TrimSpace(name)

	// Check length
	if len(name) < minNameLength {
		return fmt.Errorf("name must be at least %d characters", minNameLength)
	}
	if len(name) > maxNameLength {
		return fmt.Errorf("name must be at most %d characters", maxNameLength)
	}

	// Check for valid characters only
	if !validNameRegex.MatchString(name) {
		return fmt.Errorf("name contains invalid characters (use only letters, numbers, spaces, - or _)")
	}

	// Check for control characters and ANSI escapes
	for _, r := range name {
		if unicode.IsControl(r) {
			return fmt.Errorf("name contains control characters")
		}
	}

	// Prevent reserved names
	lowerName := strings.ToLower(name)
	for _, reserved := range reservedNames {
		if lowerName == reserved {
			return fmt.Errorf("name '%s' is reserved", name)
		}
	}

	return nil
}

// resetTerminal sends ANSI escape sequences to reset the terminal state for a
// given SSH session. It exits alternate screen buffer, shows the cursor, resets
// terminal to initial state, and clears the screen.
func resetTerminal(s ssh.Session) {
	// Exit alternate screen buffer
	s.Write([]byte("\033[?1049l"))
	// Show cursor
	s.Write([]byte("\033[?25h"))
	// Reset terminal to initial state
	s.Write([]byte("\033c"))
	// Clear screen and move cursor to home
	s.Write([]byte("\033[2J\033[H"))
}

// Catppuccin Mocha colors
const (
	catppuccinMauve    = "#cba6f7"
	catppuccinMaroon   = "#eba0ac"
	catppuccinPeach    = "#fab387"
	catppuccinSky      = "#89dceb"
	catppuccinBlue     = "#89b4fa"
	catppuccinLavender = "#b4befe"
	catppuccinCrust    = "#11111b"
	catppuccinOverlay1 = "#7f849c"
)

// Shared styles for statistics display
var (
	labelStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(catppuccinMauve)).
			PaddingRight(2)

	countStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(catppuccinPeach)).
			PaddingRight(1)

	percentStyle = lipgloss.NewStyle().
			Italic(true).
			Foreground(lipgloss.Color(catppuccinSky))
)

// tickMsg represents a periodic tick message used for UI updates.
type tickMsg time.Time

// tickEvery returns a Bubble Tea command that sends a tickMsg every second.
// This enables periodic UI refreshes for both master and player views.
func tickEvery() tea.Cmd {
	return tea.Every(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// calculateStatistics computes voting statistics from a slice of point values.
// It returns the average (for numeric values), median, and a distribution map
// showing how many times each point value was selected.
func calculateStatistics(points []string) (float64, string, map[string]int) {
	var numericPoints []float64
	distribution := make(map[string]int)

	// Calculate distribution and collect numeric points
	for _, p := range points {
		distribution[p]++
		if num, err := strconv.ParseFloat(p, 64); err == nil {
			numericPoints = append(numericPoints, num)
		}
	}

	// Calculate average
	var average float64
	if len(numericPoints) > 0 {
		sum := 0.0
		for _, num := range numericPoints {
			sum += num
		}
		average = sum / float64(len(numericPoints))
	}

	// Calculate median
	var median string
	if len(numericPoints) > 0 {
		sort.Float64s(numericPoints)
		mid := len(numericPoints) / 2
		if len(numericPoints)%2 == 0 {
			median = fmt.Sprintf("%.1f", (numericPoints[mid-1]+numericPoints[mid])/2)
		} else {
			median = fmt.Sprintf("%.1f", numericPoints[mid])
		}
	} else {
		median = "N/A"
	}

	return average, median, distribution
}

// showFinalVotes renders a formatted string displaying voting statistics including
// average, median, and a visual distribution with progress bars for each point value.
// It takes the list of voted points and total vote count as parameters.
func showFinalVotes(points []string, voted int) string {
	var s strings.Builder

	avg, median, distribution := calculateStatistics(points)

	p := progress.New(
		progress.WithScaledGradient(catppuccinMaroon, catppuccinLavender),
		progress.WithWidth(50),
	)

	s.WriteString("\n📊 Voting Statistics:\n")
	if avg > 0 {
		fmt.Fprintf(&s, "Average: %.1f\n", avg)
	}
	fmt.Fprintf(&s, "Median: %s\n", median)

	s.WriteString("Distribution:\n")
	// Sort point values for consistent display
	pointValues := make([]string, 0, len(distribution))
	for p := range distribution {
		pointValues = append(pointValues, p)
	}
	sort.Strings(pointValues)

	for _, pointVal := range pointValues {
		count := distribution[pointVal]
		percentage := float64(count) / float64(voted)

		label := labelStyle.Render(pointVal + ":")
		votes := countStyle.Render(fmt.Sprintf("%d votes", count))
		percent := percentStyle.Render(fmt.Sprintf("(%.1f%%)", percentage*100))

		// Add the point value and vote count
		fmt.Fprintf(&s, "%s %s %s\n", label, votes, percent)

		// Add the progress bar
		s.WriteString(p.ViewAs(percentage))
		s.WriteString("\n\n")
	}

	return s.String()
}

// gameState holds the shared state for a Scrum Poker session, including all
// connected players, reveal status, and the master connection reference.
type gameState struct {
	players    map[string]*playerState
	revealed   bool
	mu         sync.RWMutex
	masterConn ssh.Session
}

// playerState holds the state for an individual player including their selected
// points, SSH session reference, and whether they have made a selection.
type playerState struct {
	points   string
	session  ssh.Session
	selected bool
}

// some variables for both master and player
var (
	// some style colouring
	focusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(catppuccinMauve))
	helpStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color(catppuccinOverlay1)).Render
)

// checkAuthorizedKey validates whether the SSH session's public key matches
// any key in the .ssh/showdown_keys file. Returns true if the key is authorized,
// which grants Scrum Master privileges to the connecting user.
func checkAuthorizedKey(s ssh.Session) bool {
	pubKey := s.PublicKey()
	if pubKey == nil {
		log.Warn("No public key found!")
		return false
	}

	authorizedKeysPath, err := getConfigPath("showdown_keys")
	if err != nil {
		log.Error("failed to resolve authorized_keys path", "error", err)
		return false
	}

	authorizedKeys, err := os.ReadFile(authorizedKeysPath)
	if err != nil {
		log.Error("failed to read authorized_keys", "error", err, "path", authorizedKeysPath)
		return false
	}

	// Parse all keys in the file, not just the first one
	for len(authorizedKeys) > 0 {
		parsedKey, _, _, rest, err := ssh.ParseAuthorizedKey(authorizedKeys)
		if err != nil {
			// Stop on parse error
			break
		}

		if ssh.KeysEqual(parsedKey, pubKey) {
			return true
		}

		authorizedKeys = rest
	}

	return false
}

// pokerHandler is the main Bubble Tea handler for SSH connections. It determines
// whether to show the Scrum Master view (for authorized keys when no master exists)
// or the player name input view for regular participants.
func pokerHandler(s ssh.Session) (tea.Model, []tea.ProgramOption) {
	_, _, active := s.Pty()
	if !active {
		wish.Fatalln(s, "no active terminal, skipping")
		return nil, nil
	}

	// Check if the connection has valid authorized key
	if checkAuthorizedKey(s) {
		// Set Scrum Master connection view when there is none (thread-safe).
		state.mu.Lock()
		if state.masterConn == nil {
			state.masterConn = s
			state.mu.Unlock()
			log.Info("Scrum Master connected", "user", s.User())
			return newMasterView(), []tea.ProgramOption{tea.WithAltScreen()}
		}
		state.mu.Unlock()

		// If master already exists, deny connection
		wish.Fatalln(s, "Scrum Master is already connected")
		return nil, nil
	}

	// Setup Player connection view
	// TODO: better naming functions
	return initialNameInputView(s), []tea.ProgramOption{tea.WithAltScreen()}
}

// connectionLimitMiddleware enforces global and per-IP connection limits to prevent DoS attacks.
func connectionLimitMiddleware() wish.Middleware {
	return func(h ssh.Handler) ssh.Handler {
		return func(s ssh.Session) {
			// Global limit check
			if connectionCount.Load() >= maxConnections {
				wish.Fatalln(s, fmt.Sprintf("Server at capacity (%d connections), please try again later", maxConnections))
				return
			}
			connectionCount.Add(1)
			defer connectionCount.Add(-1)

			// Per-IP limit check
			clientIP := s.RemoteAddr().String()
			// Extract just the IP without port
			if host, _, err := net.SplitHostPort(clientIP); err == nil {
				clientIP = host
			}

			ipCountI, _ := connectionsByIP.LoadOrStore(clientIP, &atomic.Int32{})
			ipCount := ipCountI.(*atomic.Int32)

			if ipCount.Load() >= maxConnectionsPerIP {
				wish.Fatalln(s, "Too many connections from your IP address")
				return
			}
			ipCount.Add(1)
			defer ipCount.Add(-1)

			h(s)
		}
	}
}

// sessionTimeoutMiddleware enforces a maximum session duration to prevent resource leaks.
func sessionTimeoutMiddleware() wish.Middleware {
	return func(h ssh.Handler) ssh.Handler {
		return func(s ssh.Session) {
			ctx, cancel := context.WithTimeout(s.Context(), sessionTimeout)
			defer cancel()

			done := make(chan struct{})
			go func() {
				h(s)
				close(done)
			}()

			select {
			case <-done:
				// Session ended normally
			case <-ctx.Done():
				log.Info("Session timeout", "user", s.User(), "remote", s.RemoteAddr())
				resetTerminal(s)
				s.Close()
			}
		}
	}
}

// sessionCloseMiddleware returns a Wish middleware that handles SSH session cleanup.
// It resets the terminal state when sessions close and clears the master connection
// reference if the disconnecting session was the Scrum Master.
func sessionCloseMiddleware() wish.Middleware {
	return func(h ssh.Handler) ssh.Handler {
		return func(s ssh.Session) {
			// Run the handler (this blocks until session ends)
			h(s)

			// Reset terminal state before session closes
			resetTerminal(s)

			// After session ends, check if it was the master connection
			state.mu.Lock()
			defer state.mu.Unlock()
			if state.masterConn == s {
				state.masterConn = nil
				log.Info("Scrum Master disconnected, reset connection")
			}
		}
	}
}

// main is the application entry point. It initializes version information from
// build flags or runtime, parses command-line flags for port configuration,
// creates and starts the SSH server with authentication and middleware, and
// handles graceful shutdown on interrupt signals.
func main() {
	if Version == "" {
		if info, ok := debug.ReadBuildInfo(); ok && info.Main.Sum != "" {
			Version = info.Main.Version
		} else {
			Version = "unknown (built from source)"
		}
	}
	version := Version
	if len(CommitSHA) >= shaLen {
		version += " (" + CommitSHA[:shaLen] + ")"
	}

	// define flag for custom port
	port := flag.Int("p", 23234, "SSH server port")
	// Parse all declared flags
	flag.Parse()

	host, err := os.Hostname()
	if err != nil {
		log.Error("couldn't determine hostname: %v", err)
	}

	// Convert port into string
	portStr := strconv.Itoa(*port)

	// Get absolute path for host key
	hostKeyPath, err := getConfigPath("showdown_ed25519")
	if err != nil {
		log.Fatal("failed to resolve host key path", "error", err)
	}

	// create SSH server
	s, err := wish.NewServer(
		wish.WithAddress(net.JoinHostPort(host, portStr)),
		wish.WithHostKeyPath(hostKeyPath),
		wish.WithPublicKeyAuth(func(ctx ssh.Context, key ssh.PublicKey) bool {
			// Allow connections with any ed25519 key
			return key != nil && key.Type() == "ssh-ed25519"
		}),
		// Add keyboard-interactive auth that immediately succeeds without prompting
		// HACK(robin): need to allow normal players to join. For those who don't have a public key set
		wish.WithKeyboardInteractiveAuth(func(ctx ssh.Context, challenger gossh.KeyboardInteractiveChallenge) bool {
			// Authenticate immediately without any challenge
			return true
		}),
		wish.WithMiddleware(
			connectionLimitMiddleware(),
			sessionTimeoutMiddleware(),
			bubbletea.Middleware(pokerHandler),
			logging.Middleware(),
			sessionCloseMiddleware(),
		),
	)
	if err != nil {
		log.Error("Could not start server", "error", err)
	}

	// Open SSH listerner and serve SSH. Make it possible to stop the service
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	log.Info("Starting Showdown server", "host", host, "port", *port, "version", version)
	go func() {
		if err = s.ListenAndServe(); err != nil && !errors.Is(err, ssh.ErrServerClosed) {
			log.Error("Could not start server", "error", err)
			done <- nil
		}
	}()

	<-done
	log.Info("Stopping Showdown server")

	// Reset terminal for all active sessions before shutdown
	state.mu.RLock()
	if state.masterConn != nil {
		resetTerminal(state.masterConn)
	}
	for _, player := range state.players {
		if player.session != nil {
			resetTerminal(player.session)
		}
	}
	state.mu.RUnlock()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer func() { cancel() }()
	if err := s.Shutdown(ctx); err != nil && !errors.Is(err, ssh.ErrServerClosed) {
		log.Error("Could not stop server", "error", err)
	}
}
