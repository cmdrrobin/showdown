package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

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

const shaLen = 7

// Build-time variables set by -ldflags
var (
	// Version contains the application Version number. It's set via ldflags
	// when building.
	Version = "dev"

	// CommitSHA contains the SHA of the commit that this application was built
	// against. It's set via ldflags when building.
	CommitSHA = "unknown"
)

// resetTerminal sends ANSI escape sequences to reset terminal state
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

// Shared types and functions for tick updates
type tickMsg time.Time

func tickEvery() tea.Cmd {
	return tea.Every(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// Shared functions for statistics calculation and display
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

// Give an overview to all players what the scores are
func showFinalVotes(points []string, voted int) string {
	var s strings.Builder

	avg, median, distribution := calculateStatistics(points)

	p := progress.New(
		progress.WithScaledGradient(catppuccinMaroon, catppuccinLavender),
		progress.WithWidth(50),
	)

	s.WriteString("\n📊 Voting Statistics:\n")
	if avg > 0 {
		s.WriteString(fmt.Sprintf("Average: %.1f\n", avg))
	}
	s.WriteString(fmt.Sprintf("Median: %s\n", median))

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
		s.WriteString(fmt.Sprintf("%s %s %s\n", label, votes, percent))

		// Add the progress bar
		s.WriteString(p.ViewAs(percentage))
		s.WriteString("\n\n")
	}

	return s.String()
}

// Information about game state for Scrum Master view
type gameState struct {
	players    map[string]*playerState
	revealed   bool
	mu         sync.RWMutex
	masterConn ssh.Session
}

// Information about a player
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

// Validate SSH public key with defined authorized keys
func checkAuthorizedKey(s ssh.Session) bool {
	pubKey := s.PublicKey()
	if pubKey == nil {
		log.Warn("No public key found!")
		return false
	}

	authorizedKeys, err := os.ReadFile(".ssh/showdown_keys")
	if err != nil {
		log.Error("failed to read authorized_keys", "error", err)
		return false
	}

	parsedKeys, _, _, _, err := ssh.ParseAuthorizedKey(authorizedKeys)
	if err != nil {
		log.Error("failed to parse authorized_keys", "error", err)
		return false
	}

	return ssh.KeysEqual(parsedKeys, pubKey)
}

// Default Scrum Poker handler
func pokerHandler(s ssh.Session) (tea.Model, []tea.ProgramOption) {
	_, _, active := s.Pty()
	if !active {
		wish.Fatalln(s, "no active terminal, sipping")
		return nil, nil
	}

	// Check if the connection has valid authorized key
	if checkAuthorizedKey(s) {
		// Set Scrum Master connection view when there is none.
		if state.masterConn == nil {
			state.masterConn = s
			return newMasterView(), []tea.ProgramOption{tea.WithAltScreen()}
		}
	}

	// Setup Player connection view
	// TODO: better naming functions
	return initialNameInputView(s), []tea.ProgramOption{tea.WithAltScreen()}
}

// Middleware to handle session closure and reset masterConn if needed
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

	// create SSH server
	s, err := wish.NewServer(
		wish.WithAddress(net.JoinHostPort(host, portStr)),
		wish.WithHostKeyPath(".ssh/showdown_ed25519"),
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
