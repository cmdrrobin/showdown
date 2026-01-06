package main

import (
	"testing"

	"github.com/charmbracelet/bubbles/key"
)

// TestClearPlayerState tests the state reset functionality
func TestClearPlayerState(t *testing.T) {
	// Setup initial state
	state.mu.Lock()
	state.revealed = true
	state.players["player1"] = &playerState{
		points:   "5",
		selected: true,
	}
	state.players["player2"] = &playerState{
		points:   "8",
		selected: true,
	}
	state.mu.Unlock()

	// Execute clear
	clearPlayerState()

	// Verify state is cleared
	state.mu.RLock()
	defer state.mu.RUnlock()

	if state.revealed {
		t.Errorf("clearPlayerState() revealed = true, want false")
	}

	for name, player := range state.players {
		if player.points != "" {
			t.Errorf("clearPlayerState() player %s points = %s, want empty", name, player.points)
		}
		if player.selected {
			t.Errorf("clearPlayerState() player %s selected = true, want false", name)
		}
	}
}

// TestKeyMapMasterBindings tests that all master key bindings are properly configured
func TestKeyMapMasterBindings(t *testing.T) {
	tests := []struct {
		name    string
		binding key.Binding
		keys    []string
	}{
		{
			name:    "reveal binding",
			binding: keysMaster.Reveal,
			keys:    []string{"r"},
		},
		{
			name:    "clear binding",
			binding: keysMaster.Clear,
			keys:    []string{"c"},
		},
		{
			name:    "disconnect binding",
			binding: keysMaster.Disconnect,
			keys:    []string{"d"},
		},
		{
			name:    "quit binding",
			binding: keysMaster.Quit,
			keys:    []string{"q", "esc", "ctrl+c"},
		},
		{
			name:    "one minute timer binding",
			binding: keysMaster.One,
			keys:    []string{"1"},
		},
		{
			name:    "three minute timer binding",
			binding: keysMaster.Three,
			keys:    []string{"3"},
		},
		{
			name:    "six minute timer binding",
			binding: keysMaster.Six,
			keys:    []string{"6"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !tt.binding.Enabled() {
				t.Errorf("%s is not enabled", tt.name)
			}

			// Verify keys are set
			bindingKeys := tt.binding.Keys()
			if len(bindingKeys) == 0 {
				t.Errorf("%s has no keys configured", tt.name)
			}
		})
	}
}

// TestKeyMapMasterShortHelp tests the short help display
func TestKeyMapMasterShortHelp(t *testing.T) {
	shortHelp := keysMaster.ShortHelp()

	expectedCount := 7 // One, Three, Six, Reveal, Clear, Disconnect, Quit
	if len(shortHelp) != expectedCount {
		t.Errorf("ShortHelp() returned %d bindings, want %d", len(shortHelp), expectedCount)
	}
}

// TestKeyMapMasterFullHelp tests the full help display
func TestKeyMapMasterFullHelp(t *testing.T) {
	fullHelp := keysMaster.FullHelp()

	if len(fullHelp) != 2 {
		t.Errorf("FullHelp() returned %d groups, want 2", len(fullHelp))
	}

	// First group should have 3 timer keys
	if len(fullHelp[0]) != 3 {
		t.Errorf("FullHelp() first group has %d bindings, want 3", len(fullHelp[0]))
	}

	// Second group should have 4 action keys
	if len(fullHelp[1]) != 4 {
		t.Errorf("FullHelp() second group has %d bindings, want 4", len(fullHelp[1]))
	}
}

// TestNewMasterView tests master view initialization
func TestNewMasterView(t *testing.T) {
	m := newMasterView()

	if m.revealed {
		t.Errorf("newMasterView() revealed = true, want false")
	}

	if !m.help.ShowAll {
		t.Errorf("newMasterView() help.ShowAll = false, want true")
	}

	if m.keys.Reveal.Keys()[0] != "r" {
		t.Errorf("newMasterView() keys not properly initialized")
	}
}

// TestGameStateInitialization verifies the game state is properly initialized
func TestGameStateInitialization(t *testing.T) {
	if state == nil {
		t.Fatal("state is nil")
	}

	if state.players == nil {
		t.Fatal("state.players is nil")
	}
}
