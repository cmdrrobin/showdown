package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	bm "github.com/charmbracelet/wish/bubbletea"
	"github.com/charmbracelet/wish/logging"
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
	// set Story Points options for each team player
	pointOptions = []string{"0.5", "1", "2", "3", "5", "8", "10", "?"}
)

// Default Scrum Poker handler
func pokerHandler(s ssh.Session) (tea.Model, []tea.ProgramOption) {
	_, _, active := s.Pty()
	if !active {
		wish.Fatalln(s, "no active terminal, sipping")
		return nil, nil
	}

	// Set Scrum Master connection view when there is none.
	// The initial (first) connection is always for the Scrum Master
	if state.masterConn == nil {
		state.masterConn = s
		m := newMasterView()
		return m, []tea.ProgramOption{tea.WithAltScreen()}
	}

	// Setup Player connection view
	// TODO: better naming functions
	return initialNameInputView(s), []tea.ProgramOption{tea.WithAltScreen()}
}

func main() {
	port := flag.Int("p", 23234, "SSH server port")
	flag.Parse()
	portStr := fmt.Sprintf("%d", *port)

	host, err := os.Hostname()
	if err != nil {
		log.Error("couldn't determine hostname: %v", err)
	}

	// create SSH server
	s, err := wish.NewServer(
		wish.WithAddress(net.JoinHostPort(host, portStr)),
		wish.WithHostKeyPath(".ssh/scrumpoker_ed25519"),
		wish.WithMiddleware(
			bm.Middleware(pokerHandler),
			logging.Middleware(),
		),
	)
	if err != nil {
		log.Error("Could not start server", "error", err)
	}

	// Open SSH listerner and serve SSH. Make it possible to stop the service
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	log.Info("Starting Scrum Poker server", "host", host, "port", portStr)
	go func() {
		if err = s.ListenAndServe(); err != nil && !errors.Is(err, ssh.ErrServerClosed) {
			log.Error("Could not start server", "error", err)
			done <- nil
		}
	}()

	<-done
	log.Info("Stopping Scrum Poker server")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer func() { cancel() }()
	if err := s.Shutdown(ctx); err != nil && !errors.Is(err, ssh.ErrServerClosed) {
		log.Error("Could not stop server", "error", err)
	}
}
