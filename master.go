package main

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type keyMapMaster struct {
	Reveal     key.Binding
	Clear      key.Binding
	Disconnect key.Binding
	Quit       key.Binding
	One        key.Binding
	Three      key.Binding
	Six        key.Binding
}

var (
	state = &gameState{
		players: make(map[string]*playerState),
	}

	timerDurations = map[string]time.Duration{
		"1": 15 * time.Second,
		"3": 30 * time.Second,
		"6": 60 * time.Second,
	}

	keysMaster = keyMapMaster{
		Reveal: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "reveal"),
		),
		Clear: key.NewBinding(
			key.WithKeys("c"),
			key.WithHelp("c", "clear score"),
		),
		Disconnect: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "disconnect players"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "esc", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
		One: key.NewBinding(
			key.WithKeys("1"),
			key.WithHelp("1", "15 seconds"),
		),
		Three: key.NewBinding(
			key.WithKeys("3"),
			key.WithHelp("3", "30 seconds"),
		),
		Six: key.NewBinding(
			key.WithKeys("6"),
			key.WithHelp("6", "60 seconds"),
		),
	}

	// Styles moved to main.go for shared access
)

type masterView struct {
	revealed bool
	timer    *time.Timer
	endTime  time.Time
	duration time.Duration
	keys     keyMapMaster
	help     help.Model
}

// Add timer control messages
type (
	timerExpiredMsg struct{}
)

// tickMsg and tickEvery moved to main.go for shared access

// set some default values for masterView and by default show help information
func newMasterView() masterView {
	m := masterView{
		revealed: false,
		keys:     keysMaster,
		help:     help.New(),
	}

	// default show full help information
	m.help.ShowAll = true
	return m
}

// ShortHelp returns keybindings to be shown in the mini help view. It's part
// of the key.Map interface.
func (k keyMapMaster) ShortHelp() []key.Binding {
	return []key.Binding{k.One, k.Three, k.Six, k.Reveal, k.Clear, k.Disconnect, k.Quit}
}

// FullHelp returns keybindings for the expanded help view. It's part of the
// key.Map interface.
func (k keyMapMaster) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.One, k.Three, k.Six},
		{k.Reveal, k.Clear, k.Disconnect, k.Quit},
	}
}

// tickEvery function moved to main.go

func (m masterView) Init() tea.Cmd {
	return tea.Batch(tickEvery())
}

func startTimer(duration time.Duration) tea.Cmd {
	return func() tea.Msg {
		time.Sleep(duration)
		return timerExpiredMsg{}
	}
}

// calculateStatistics function moved to main.go for shared access

// Quit all team player sessions
func quitPlayers() {
	for _, player := range state.players {
		player.session.Close()
	}
	state.players = make(map[string]*playerState)
}

// To clear revealing and points of players
func clearPlayerState() {
	state.mu.Lock()
	state.revealed = false
	for _, player := range state.players {
		player.points = ""
		player.selected = false
	}
	state.mu.Unlock()
}

func (m masterView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		// If we set a width on the help menu it can gracefully truncate
		// its view as needed.
		m.help.Width = msg.Width

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
			quitPlayers()
			state.masterConn = nil

			return m, tea.Quit
		case key.Matches(msg, m.keys.Reveal):
			state.mu.Lock()
			state.revealed = true
			state.mu.Unlock()

			return m, tickEvery()
		case key.Matches(msg, m.keys.Clear):
			clearPlayerState()
			m.timer = nil

			return m, tickEvery()
		case key.Matches(msg, m.keys.Disconnect):
			state.mu.Lock()
			quitPlayers()
			state.mu.Unlock()
			m.timer = nil

			return m, tickEvery()
		case key.Matches(msg, m.keys.One),
			key.Matches(msg, m.keys.Three),
			key.Matches(msg, m.keys.Six):
			clearPlayerState()
			// Start timer with selected duration
			duration := timerDurations[msg.String()]
			m.duration = duration
			m.endTime = time.Now().Add(duration)

			return m, tea.Batch(
				tickEvery(),
				startTimer(duration),
			)
		}
	case tickMsg:
		return m, tickEvery()
	case timerExpiredMsg:
		state.mu.Lock()
		state.revealed = true
		state.mu.Unlock()
		m.timer = nil
		return m, tickEvery()
	}
	return m, nil
}

// showFinalVotes function moved to main.go for shared access

func (m masterView) View() string {
	state.mu.RLock()
	defer state.mu.RUnlock()

	var s strings.Builder
	s.WriteString("🎲 Showdown - Scrum Master\n\n")

	// Show timer if active
	if !m.endTime.IsZero() {
		remaining := time.Until(m.endTime)
		if remaining > 0 {
			s.WriteString(fmt.Sprintf("⏱  Timer: %02d:%02d\n\n",
				int(remaining.Minutes()),
				int(remaining.Seconds())%60))
		} else {
			s.WriteString("⏱  Time's up!\n\n")
		}
	}

	if len(state.players) == 0 {
		s.WriteString("Waiting for players to join...\n")
	} else {
		s.WriteString(fmt.Sprintf("Connected Players: %d\n\n", len(state.players)))

		// Sort players by name for consistent display
		names := make([]string, 0, len(state.players))
		for name := range state.players {
			names = append(names, name)
		}
		sort.Strings(names)

		// Display players
		s.WriteString("Players:\n")
		for _, name := range names {
			player := state.players[name]
			if state.revealed {
				s.WriteString(fmt.Sprintf("• %s: %s\n", name, player.points))
			} else {
				if player.selected {
					s.WriteString(fmt.Sprintf("• %s: ✓\n", name))
				} else {
					s.WriteString(fmt.Sprintf("• %s: waiting...\n", name))
				}
			}
		}

		// Calculate voting progress
		voted := 0
		var points []string
		for _, player := range state.players {
			if player.selected {
				voted++
				points = append(points, player.points)
			}
		}

		// Display statistics when revealed key is pressed and votes are available
		if state.revealed && voted > 0 {
			s.WriteString(showFinalVotes(points, voted))
		} else {
			s.WriteString(fmt.Sprintf("\nVoting Progress: %d/%d\n\n", voted, len(state.players)))
		}
	}

	// show help menu
	s.WriteString(fmt.Sprintf("\n%s", m.help.View(m.keys)))

	return lipgloss.NewStyle().Padding(1).Render(s.String())
}
