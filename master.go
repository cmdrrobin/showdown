package main

import (
	"fmt"
	"sort"
	"strconv"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/ssh"
)

type gameState struct {
	players    map[string]*playerState
	revealed   bool
	mu         sync.RWMutex
	masterConn ssh.Session
}

type playerState struct {
	points   string
	session  ssh.Session
	selected bool
}

var (
	state = &gameState{
		players: make(map[string]*playerState),
	}
	pointOptions = []string{"1", "2", "3", "5", "8", "13", "21", "?"}
)

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

// Add timer durations
var timerDurations = map[string]time.Duration{
	"1": time.Minute,
	"2": 2 * time.Minute,
	"3": 3 * time.Minute,
	"5": 5 * time.Minute,
}

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

func (m masterView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
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
			for _, player := range state.players {
				player.session.Close()
			}
			state.players = make(map[string]*playerState)
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

	s := "🎲 Scrum Poker Master View\n\n"

	// Show timer if active
	if !m.endTime.IsZero() {
		remaining := time.Until(m.endTime)
		if remaining > 0 {
			s += fmt.Sprintf("⏱  Timer: %02d:%02d\n\n",
				int(remaining.Minutes()),
				int(remaining.Seconds())%60)
		} else {
			s += "⏱  Time's up!\n\n"
		}
	}

	s += fmt.Sprintf("Connected Players: %d\n\n", len(state.players))

	if len(state.players) == 0 {
		s += "Waiting for players to join...\n"
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

		s += fmt.Sprintf("Voting Progress: %d/%d\n\n", voted, len(state.players))

		// Display statistics when points are revealed
		if state.revealed && voted > 0 {
			avg, median, distribution := calculateStatistics(points)

			s += "📊 Voting Statistics:\n"
			if avg > 0 {
				s += fmt.Sprintf("Average: %.1f\n", avg)
			}
			s += fmt.Sprintf("Median: %s\n", median)

			s += "Distribution:\n"
			// Sort point values for consistent display
			pointValues := make([]string, 0, len(distribution))
			for p := range distribution {
				pointValues = append(pointValues, p)
			}
			sort.Strings(pointValues)

			for _, p := range pointValues {
				count := distribution[p]
				percentage := float64(count) / float64(voted) * 100
				s += fmt.Sprintf("%s: %d votes (%.1f%%)", p, count, percentage)
				// Add visual bar
				barLength := int(percentage / 10)
				s += " "
				for i := 0; i < barLength; i++ {
					s += "█"
				}
				s += "\n"
			}
			s += "\n"
		}

		// Display players
		s += "Players:\n"
		for _, name := range names {
			player := state.players[name]
			if state.revealed {
				s += fmt.Sprintf("• %s: %s\n", name, player.points)
			} else {
				if player.selected {
					s += fmt.Sprintf("• %s: ✓\n", name)
				} else {
					s += fmt.Sprintf("• %s: waiting...\n", name)
				}
			}
		}
	}

	s += "\nCommands:\n"
	s += "r: reveal points\n"
	s += "c: clear points\n"
	s += "d: disconnect all\n"
	s += "q: quit\n"
	s += "\nTimer Commands:\n"
	s += "1: start 1 min timer\n"
	s += "2: start 2 min timer\n"
	s += "3: start 3 min timer\n"
	s += "5: start 5 min timer\n"

	// Add timestamp for refresh indication
	s += fmt.Sprintf("\nLast updated: %s", time.Now().Format("15:04:05"))

	return lipgloss.NewStyle().Padding(1).Render(s)
}
