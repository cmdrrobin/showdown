package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/ssh"
)

// set Story Points options for each team player
var pointOptions = []string{"0.5", "1", "2", "3", "5", "8", "10", "?"}

type PointItem struct {
	value string
}

func (i PointItem) FilterValue() string { return i.value }
func (i PointItem) Title() string       { return i.value }
func (i PointItem) Description() string { return "" }

type playerView struct {
	name     string
	list     list.Model
	selected string
}

// Name input view
type nameInputView struct {
	textInput textinput.Model
	err       error
	session   ssh.Session
}

type delegateKeyMap struct {
	choose key.Binding
}

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

func (p playerView) Init() tea.Cmd {
	return nil
}

func (p playerView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		selectedValue string
		cmd           tea.Cmd
	)

	p.list, cmd = p.list.Update(msg)

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
			state.mu.Lock()
			if player, exists := state.players[p.name]; exists {
				player.points = selectedValue
				player.selected = true
			}
			state.mu.Unlock()
			p.selected = selectedValue
		}
	}

	return p, cmd
}

func (p playerView) View() string {
	var s strings.Builder
	s.WriteString(fmt.Sprintf("🎲 Showdown - Player: %s\n\n", p.name))
	s.WriteString(p.list.View() + "\n\n")

	if p.selected != "" {
		s.WriteString(fmt.Sprintf("Selected: %s\n", p.selected))
	}

	s.WriteString("\nPress q to quit")
	return lipgloss.NewStyle().Padding(1).Render(s.String())
}

// A function to add additional help to key bindings
// TODO: need to find out if this is _really_ required and not a better simpler way
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

func (v nameInputView) Init() tea.Cmd {
	return textinput.Blink
}

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
	}

	v.textInput, cmd = v.textInput.Update(msg)
	return v, cmd
}

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
