package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/ssh"
)

// pointOptions defines the available story point values that players can select
// during voting, following a modified Fibonacci sequence plus a "?" for uncertainty.
var pointOptions = []string{"0.5", "1", "2", "3", "5", "8", "10", "?"}

// PointItem represents a selectable story point value in the player's list.
// It implements the list.Item interface for use with Bubble Tea's list component.
type PointItem struct {
	value string
}

// FilterValue returns the value used for filtering in the list (implements list.Item).
func (i PointItem) FilterValue() string { return i.value }

// Title returns the display title for this point item (implements list.Item).
func (i PointItem) Title() string { return i.value }

// Description returns an empty string as point items have no description (implements list.Item).
func (i PointItem) Description() string { return "" }

// playerView is the Bubble Tea model for the player voting interface, displaying
// the point selection list and the player's current selection status.
type playerView struct {
	name     string
	list     list.Model
	selected string
}

// nameInputView is the Bubble Tea model for the initial name entry screen
// where players enter their display name before joining the voting session.
type nameInputView struct {
	textInput textinput.Model
	err       error
	session   ssh.Session
}

// delegateKeyMap defines the key bindings for the list delegate,
// specifically the enter key for choosing a point value.
type delegateKeyMap struct {
	choose key.Binding
}

// newDelegateKeyMap creates a new key map for the list delegate with
// the enter key bound to the choose action.
func newDelegateKeyMap() *delegateKeyMap {
	return &delegateKeyMap{
		choose: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "choose"),
		),
	}
}

// Additional short help entries. This satisfies the help.KeyMap interface and
// is entirely optional.
func (d delegateKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{
		d.choose,
	}
}

// Additional full help entries. This satisfies the help.KeyMap interface and
// is entirely optional.
func (d delegateKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{
			d.choose,
		},
	}
}

// Init initializes the player view and starts the periodic tick command
// for UI updates. Implements the tea.Model interface.
func (p playerView) Init() tea.Cmd {
	return tickEvery()
}

// Update handles incoming messages for the player view including keyboard
// navigation, point selection with enter, quit commands, and tick updates.
// Selection is disabled once votes are revealed. Implements the tea.Model interface.
func (p playerView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		selectedValue string
		cmd           tea.Cmd
	)

	// Only update list if scores aren't revealed
	state.mu.RLock()
	revealed := state.revealed
	state.mu.RUnlock()

	if !revealed {
		p.list, cmd = p.list.Update(msg)
	}

	if item, ok := p.list.SelectedItem().(PointItem); ok {
		selectedValue = item.value
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			state.mu.Lock()
			delete(state.players, p.name)
			state.mu.Unlock()
			return p, tea.Quit
		case "enter":
			// Only allow selection if scores aren't revealed
			if !revealed {
				state.mu.Lock()
				if player, exists := state.players[p.name]; exists {
					player.points = selectedValue
					player.selected = true
				}
				state.mu.Unlock()
				p.selected = selectedValue
			}
		}
	case tickMsg:
		return p, tickEvery()
	}

	return p, cmd
}

// showResults renders the voting results panel displaying all player votes
// and statistics after the Scrum Master reveals the votes.
func (p playerView) showResults() string {
	state.mu.RLock()
	defer state.mu.RUnlock()

	var s strings.Builder
	s.WriteString("📊 Voting Results:\n\n")

	// Show all players and their votes
	names := make([]string, 0, len(state.players))
	for name := range state.players {
		names = append(names, name)
	}
	sort.Strings(names)

	s.WriteString("Player Votes:\n")
	var points []string
	voted := 0

	for _, name := range names {
		player := state.players[name]
		if player.selected {
			fmt.Fprintf(&s, "• %s: %s\n", name, player.points)
			points = append(points, player.points)
			voted++
		} else {
			fmt.Fprintf(&s, "• %s: no vote\n", name)
		}
	}

	// Show statistics if there are votes
	if voted > 0 {
		s.WriteString(showFinalVotes(points, voted))
	}

	return s.String()
}

// View renders the player interface showing either the point selection list
// (during voting) or the results panel (after reveal). Implements the tea.Model interface.
func (p playerView) View() string {
	var s strings.Builder
	fmt.Fprintf(&s, "🎲 Showdown - Player: %s\n\n", p.name)

	state.mu.RLock()
	revealed := state.revealed
	state.mu.RUnlock()

	if revealed {
		s.WriteString(p.showResults())
	} else {
		s.WriteString(p.list.View() + "\n\n")
		if p.selected != "" {
			fmt.Fprintf(&s, "Selected: %s\n", p.selected)
		}
	}

	s.WriteString("\nPress q to quit")
	return lipgloss.NewStyle().Padding(1).Render(s.String())
}

// additionalDelegateKeys creates a list delegate with custom key bindings for
// the help display. It configures the delegate's help functions to show the
// choose key binding in both short and full help views.
func additionalDelegateKeys(keys *delegateKeyMap) list.DefaultDelegate {
	d := list.NewDefaultDelegate()

	help := []key.Binding{keys.choose}

	d.ShortHelpFunc = func() []key.Binding {
		return help
	}

	d.FullHelpFunc = func() [][]key.Binding {
		return [][]key.Binding{help}
	}

	return d
}

// initPlayerView creates and initializes a new player view with the point
// selection list and registers the player in the global game state.
func initPlayerView(playerName string, session ssh.Session) (tea.Model, tea.Cmd) {
	items := make([]list.Item, len(pointOptions))
	for i, p := range pointOptions {
		items[i] = PointItem{value: p}
	}

	selectedColor := lipgloss.Color(catppuccinMauve)
	d := additionalDelegateKeys(newDelegateKeyMap())
	d.Styles.SelectedTitle = d.Styles.SelectedTitle.Foreground(selectedColor).BorderLeftForeground(selectedColor)
	d.Styles.SelectedDesc = d.Styles.SelectedTitle
	l := list.New(items, d, 20, 20)
	l.Title = "Select Points"
	l.SetShowTitle(true)
	l.SetFilteringEnabled(false) // no filtering needed
	// styling of the list title
	l.Styles.Title = lipgloss.NewStyle().
		Background(lipgloss.Color(catppuccinSky)).
		Foreground(lipgloss.Color(catppuccinCrust)).
		Bold(true).
		Padding(0, 1)
	// * styling of the number of items in a list (* item(s))
	l.Styles.StatusBar = lipgloss.NewStyle().
		Foreground(lipgloss.Color(catppuccinBlue))

	p := playerView{
		name: playerName,
		list: l,
	}

	state.mu.Lock()
	state.players[playerName] = &playerState{
		session: session,
	}
	state.mu.Unlock()

	return p, nil
}

// initialNameInputView creates the name input form for new players joining
// the session, with styled text input and a 30-character limit.
func initialNameInputView(session ssh.Session) nameInputView {
	ti := textinput.New()
	ti.Cursor.Style = focusStyle
	ti.Placeholder = "Enter your name"
	ti.Focus()
	ti.PromptStyle = focusStyle
	ti.TextStyle = focusStyle
	ti.CharLimit = 30
	ti.Width = 30

	return nameInputView{
		textInput: ti,
		session:   session,
	}
}

// Init initializes the name input view with cursor blink animation and
// periodic tick commands. Implements the tea.Model interface.
func (v nameInputView) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, tickEvery())
}

// Update handles keyboard input for the name entry form including validation
// for empty names and duplicate names. Transitions to playerView on successful
// entry. Implements the tea.Model interface.
func (v nameInputView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			name := v.textInput.Value()
			if name == "" {
				v.err = fmt.Errorf("name cannot be empty")
				return v, nil
			}
			state.mu.RLock()
			_, exists := state.players[name]
			state.mu.RUnlock()
			if exists {
				v.err = fmt.Errorf("name already taken")
				return v, nil
			}
			return initPlayerView(name, v.session)
		case tea.KeyCtrlC:
			return v, tea.Quit
		}
	case tickMsg:
		return v, tickEvery()
	}

	v.textInput, cmd = v.textInput.Update(msg)
	return v, cmd
}

// View renders the welcome screen with the name input field, help text,
// and any validation error messages. Implements the tea.Model interface.
func (v nameInputView) View() string {
	var s strings.Builder
	s.WriteString("Welcome to Showdown!\n\n")
	s.WriteString(v.textInput.View() + "\n\n")
	s.WriteString(helpStyle("Press Enter to continue\n"))
	if v.err != nil {
		s.WriteString("\nError: " + v.err.Error() + "\n")
	}
	return lipgloss.NewStyle().Padding(1).Render(s.String())
}
