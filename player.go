package main

import (
	"fmt"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/ssh"
)

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

func (p playerView) Init() tea.Cmd {
	return nil
}

func (p playerView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "q" {
			state.mu.Lock()
			delete(state.players, p.name)
			state.mu.Unlock()
			return p, tea.Quit
		}
	}

	var cmd tea.Cmd
	p.list, cmd = p.list.Update(msg)

	if item, ok := p.list.SelectedItem().(PointItem); ok {
		state.mu.Lock()
		if player, exists := state.players[p.name]; exists {
			player.points = item.value
			player.selected = true
		}
		state.mu.Unlock()
		p.selected = item.value
	}

	return p, cmd
}

func (p playerView) View() string {
	s := fmt.Sprintf("🎲 Scrum Poker - Player: %s\n\n", p.name)
	s += p.list.View() + "\n\n"

	if p.selected != "" {
		s += fmt.Sprintf("Selected: %s\n", p.selected)
	}

	s += "\nPress q to quit"
	return lipgloss.NewStyle().Padding(1).Render(s)
}

func initPlayerView(playerName string, session ssh.Session) (tea.Model, tea.Cmd) {
	items := make([]list.Item, len(pointOptions))
	for i, p := range pointOptions {
		items[i] = PointItem{value: p}
	}

	l := list.New(items, list.NewDefaultDelegate(), 20, 20)
	l.Title = "Select Points"
	l.SetShowTitle(true)
	l.SetFilteringEnabled(false)
	l.Styles.Title = lipgloss.NewStyle().
		Background(lipgloss.Color("62")).
		Foreground(lipgloss.Color("230")).
		Padding(0, 1)

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
	ti.Placeholder = "Enter your name"
	ti.Focus()
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
	var s string
	s += "Welcome to Scrum Poker!\n\n"
	s += v.textInput.View() + "\n\n"
	s += "Press Enter to continue\n"
	if v.err != nil {
		s += "\nError: " + v.err.Error() + "\n"
	}
	return lipgloss.NewStyle().Padding(1).Render(s)
}
