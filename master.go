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

// keyMapMaster defines the key bindings available to the Scrum Master,
// including reveal, clear, disconnect, quit, and timer controls.
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

// masterView is the Bubble Tea model for the Scrum Master interface, displaying
// connected players, voting status, timer countdown, and voting statistics.
type masterView struct {
	revealed bool
	timer    *time.Timer
	endTime  time.Time
	duration time.Duration
	keys     keyMapMaster
	help     help.Model
}

// timerExpiredMsg is sent when the voting timer reaches zero, triggering
// automatic reveal of all player votes.
type timerExpiredMsg struct{}

// tickMsg and tickEvery moved to main.go for shared access

// newMasterView creates and initializes a new Scrum Master view with default
// settings. The help panel is expanded by default to show all available commands.
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

// Init initializes the master view and starts the periodic tick command
// for UI updates. Implements the tea.Model interface.
func (m masterView) Init() tea.Cmd {
	return tea.Batch(tickEvery())
}

// startTimer returns a Bubble Tea command that waits for the specified duration
// and then sends a timerExpiredMsg to trigger automatic vote reveal.
func startTimer(duration time.Duration) tea.Cmd {
	return func() tea.Msg {
		time.Sleep(duration)
		return timerExpiredMsg{}
	}
}

// quitPlayers disconnects all connected player sessions by resetting their
// terminals and closing their SSH connections, then clears the players map.
func quitPlayers() {
	for _, player := range state.players {
		// Reset terminal before closing session
		resetTerminal(player.session)
		player.session.Close()
	}
	state.players = make(map[string]*playerState)
}

// clearPlayerState resets the game state for a new voting round by clearing
// the revealed flag and resetting all player selections and points.
func clearPlayerState() {
	state.mu.Lock()
	state.revealed = false
	for _, player := range state.players {
		player.points = ""
		player.selected = false
	}
	state.mu.Unlock()
}

// Update handles all incoming messages for the master view including keyboard
// input for reveal/clear/disconnect/quit actions, timer key presses, window
// resize events, and timer expiration. Implements the tea.Model interface.
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

// View renders the Scrum Master dashboard showing the timer (if active),
// list of connected players with their voting status, voting progress,
// and statistics when votes are revealed. Implements the tea.Model interface.
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
