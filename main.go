package main

import (
	"flag"
	"fmt"
	"log"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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

var (
	// some style colouring
	focusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(catppuccinMauve))
	timerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(catppuccinSubtext0)).Render
	helpStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color(catppuccinOverlay1)).Render
	// set Story Points options for each team player
	pointOptions = []string{"1", "2", "3", "5", "8", "13", "21", "?"}
)

type gameState struct {
	players    map[string]*playerState
	revealed   bool
	mu         sync.RWMutex
	masterConn ssh.Session
}

func pokerHandler(s ssh.Session) (tea.Model, []tea.ProgramOption) {
	_, _, active := s.Pty()
	if !active {
		wish.Fatalln(s, "no active terminal, sipping")
		return nil, nil
	}

	if state.masterConn == nil {
		state.masterConn = s
		m := masterView{
			revealed: false,
		}
		return m, []tea.ProgramOption{tea.WithAltScreen()}
	}

	return initialNameInputView(s), []tea.ProgramOption{tea.WithAltScreen()}
}

func main() {
	port := flag.Int("p", 23234, "SSH server port")
	flag.Parse()

	s, err := wish.NewServer(
		wish.WithAddress(fmt.Sprintf(":%d", *port)),
		wish.WithHostKeyPath(".ssh/term_info_ed25519"),
		wish.WithMiddleware(
			logging.Middleware(),
			bm.Middleware(pokerHandler),
		),
	)
	if err != nil {
		log.Fatalln(err)
	}

	fmt.Printf("Starting SSH server on :%d\n", *port)
	fmt.Printf("Connect with: ssh -t localhost -p %d\n", *port)
	err = s.ListenAndServe()
	if err != nil {
		log.Fatalln(err)
	}
}
