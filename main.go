package main

import (
	"context"
	"errors"
	"flag"
	"net"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	"github.com/charmbracelet/wish/bubbletea"
	"github.com/charmbracelet/wish/logging"
	gossh "golang.org/x/crypto/ssh"
)

// Catppuccin Mocha colors
const (
	catppuccinRosewater = "#f5e0dc"
	catppuccinFlamingo  = "#f2cdcd"
	catppuccinPink      = "#f5c2e7"
	catppuccinMauve     = "#cba6f7"
	catppuccinRed       = "#f38ba8"
	catppuccinMaroon    = "#eba0ac"
	catppuccinPeach     = "#fab387"
	catppuccinYellow    = "#f9e2af"
	catppuccinGreen     = "#a6e3a1"
	catppuccinTeal      = "#94e2d5"
	catppuccinSky       = "#89dceb"
	catppuccinSapphire  = "#74c7ec"
	catppuccinBlue      = "#89b4fa"
	catppuccinLavender  = "#b4befe"
	catppuccinText      = "#cdd6f4"
	catppuccinCrust     = "#11111b"
	catppuccinSubtext0  = "#a6adc8"
	catppuccinOverlay1  = "#7f849c"
)

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
	timerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(catppuccinSubtext0)).Render
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
	log.Info("Starting Showdown server", "host", host, "port", *port)
	go func() {
		if err = s.ListenAndServe(); err != nil && !errors.Is(err, ssh.ErrServerClosed) {
			log.Error("Could not start server", "error", err)
			done <- nil
		}
	}()

	<-done
	log.Info("Stopping Showdown server")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer func() { cancel() }()
	if err := s.Shutdown(ctx); err != nil && !errors.Is(err, ssh.ErrServerClosed) {
		log.Error("Could not stop server", "error", err)
	}
}
