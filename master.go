package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/ssh"
)

var (
	state = &gameState{
		players: make(map[string]*playerState),
	}
	timerDurations = map[string]time.Duration{
		"1": time.Minute,
		"2": 2 * time.Minute,
		"3": 3 * time.Minute,
		"5": 5 * time.Minute,
	}
)

type playerState struct {
	points   string
	session  ssh.Session
	selected bool
}

type masterView struct {
	revealed bool
	timer    *time.Timer
	endTime  time.Time
	duration time.Duration
}

// Add timer control messages
type (
	timerStartMsg   struct{}
	timerExpiredMsg struct{}
)

type tickMsg time.Time

func tickEvery() tea.Cmd {
	return tea.Every(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m masterView) Init() tea.Cmd {
	return tea.Batch(tickEvery())
}

func startTimer(duration time.Duration) tea.Cmd {
	return func() tea.Msg {
		time.Sleep(duration)
		return timerExpiredMsg{}
	}
}

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

func quitPlayers() {
	for _, player := range state.players {
		player.session.Close()
	}
	state.players = make(map[string]*playerState)
}

func (m masterView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			quitPlayers()
			state.masterConn = nil
			return m, tea.Quit
		case "r":
			state.mu.Lock()
			state.revealed = true
			state.mu.Unlock()
			return m, tickEvery()
		case "c":
			state.mu.Lock()
			state.revealed = false
			for _, player := range state.players {
				player.points = ""
				player.selected = false
			}
			state.mu.Unlock()
			m.timer = nil
			return m, tickEvery()
		case "d":
			state.mu.Lock()
			quitPlayers()
			state.mu.Unlock()
			m.timer = nil
			return m, tickEvery()
		case "1", "2", "3", "5":
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

func (m masterView) View() string {
	state.mu.RLock()
	defer state.mu.RUnlock()

	labelStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(catppuccinMauve)).
		PaddingRight(2)

	countStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(catppuccinPeach)).
		PaddingRight(1)

	percentStyle := lipgloss.NewStyle().
		Italic(true).
		Foreground(lipgloss.Color(catppuccinSky))

	var s strings.Builder
	s.WriteString("🎲 Scrum Poker Master View\n\n")

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

	s.WriteString(fmt.Sprintf("Connected Players: %d\n\n", len(state.players)))

	if len(state.players) == 0 {
		s.WriteString("Waiting for players to join...\n")
	} else {
		// Sort players by name for consistent display
		names := make([]string, 0, len(state.players))
		for name := range state.players {
			names = append(names, name)
		}
		sort.Strings(names)

		// Calculate voting progress
		voted := 0
		var points []string
		for _, player := range state.players {
			if player.selected {
				voted++
				points = append(points, player.points)
			}
		}

		s.WriteString(fmt.Sprintf("Voting Progress: %d/%d\n\n", voted, len(state.players)))

		// Display statistics when points are revealed
		if state.revealed && voted > 0 {
			avg, median, distribution := calculateStatistics(points)

			p := progress.New(
				progress.WithScaledGradient(catppuccinMaroon, catppuccinLavender),
				progress.WithWidth(50),
			)

			s.WriteString("📊 Voting Statistics:\n")
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
		}

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
	}

	s.WriteString(timerStyle("\nTimer Commands: (1) min, (2) min, (3) min or (5) min\n"))
	s.WriteString("\n")
	s.WriteString(helpStyle("r = reveal, c = clear, d = disconnect all, q = quit"))

	return lipgloss.NewStyle().Padding(1).Render(s.String())
}
