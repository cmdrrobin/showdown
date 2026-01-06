package main

import (
	"testing"
)

// TestPointItem tests the PointItem interface implementation
func TestPointItem(t *testing.T) {
	item := PointItem{value: "5"}

	if item.FilterValue() != "5" {
		t.Errorf("PointItem.FilterValue() = %s, want 5", item.FilterValue())
	}

	if item.Title() != "5" {
		t.Errorf("PointItem.Title() = %s, want 5", item.Title())
	}

	if item.Description() != "" {
		t.Errorf("PointItem.Description() = %s, want empty", item.Description())
	}
}

// TestPointItemCreation tests creating PointItems from pointOptions
func TestPointItemCreation(t *testing.T) {
	for _, pointValue := range pointOptions {
		item := PointItem{value: pointValue}
		if item.Title() != pointValue {
			t.Errorf("PointItem{value: %s}.Title() = %s, want %s", pointValue, item.Title(), pointValue)
		}
	}
}

// TestDelegateKeyMap tests the delegate key map configuration
func TestDelegateKeyMap(t *testing.T) {
	keys := newDelegateKeyMap()

	if keys == nil {
		t.Fatal("newDelegateKeyMap() returned nil")
	}

	// Verify choose binding exists and is enabled
	if !keys.choose.Enabled() {
		t.Errorf("choose binding is not enabled")
	}

	// Verify choose binding has the correct key
	chooseKeys := keys.choose.Keys()
	if len(chooseKeys) == 0 {
		t.Fatal("choose binding has no keys")
	}
	if chooseKeys[0] != "enter" {
		t.Errorf("choose binding key = %s, want enter", chooseKeys[0])
	}
}

// TestDelegateKeyMapShortHelp tests the short help for delegate
func TestDelegateKeyMapShortHelp(t *testing.T) {
	keys := newDelegateKeyMap()
	shortHelp := keys.ShortHelp()

	if len(shortHelp) != 1 {
		t.Errorf("ShortHelp() returned %d bindings, want 1", len(shortHelp))
	}
}

// TestDelegateKeyMapFullHelp tests the full help for delegate
func TestDelegateKeyMapFullHelp(t *testing.T) {
	keys := newDelegateKeyMap()
	fullHelp := keys.FullHelp()

	if len(fullHelp) != 1 {
		t.Errorf("FullHelp() returned %d groups, want 1", len(fullHelp))
	}

	if len(fullHelp[0]) != 1 {
		t.Errorf("FullHelp()[0] has %d bindings, want 1", len(fullHelp[0]))
	}
}

// TestPlayerStateManagement tests adding and removing players from state
func TestPlayerStateManagement(t *testing.T) {
	// Clear any existing players
	state.mu.Lock()
	state.players = make(map[string]*playerState)
	state.mu.Unlock()

	// Add a player
	testPlayerName := "test_player"
	state.mu.Lock()
	state.players[testPlayerName] = &playerState{
		points:   "",
		selected: false,
	}
	state.mu.Unlock()

	// Verify player was added
	state.mu.RLock()
	player, exists := state.players[testPlayerName]
	state.mu.RUnlock()

	if !exists {
		t.Fatalf("player %s was not added to state", testPlayerName)
	}

	if player.points != "" {
		t.Errorf("new player points = %s, want empty", player.points)
	}

	if player.selected {
		t.Errorf("new player selected = true, want false")
	}

	// Test player selection
	state.mu.Lock()
	player.points = "5"
	player.selected = true
	state.mu.Unlock()

	state.mu.RLock()
	if !player.selected {
		t.Errorf("player.selected = false, want true")
	}
	if player.points != "5" {
		t.Errorf("player.points = %s, want 5", player.points)
	}
	state.mu.RUnlock()

	// Remove player
	state.mu.Lock()
	delete(state.players, testPlayerName)
	state.mu.Unlock()

	// Verify player was removed
	state.mu.RLock()
	_, exists = state.players[testPlayerName]
	state.mu.RUnlock()

	if exists {
		t.Errorf("player %s still exists after deletion", testPlayerName)
	}
}

// TestPlayerVotingScenario tests a complete voting scenario
func TestPlayerVotingScenario(t *testing.T) {
	// Clear and setup
	state.mu.Lock()
	state.players = make(map[string]*playerState)
	state.revealed = false
	state.mu.Unlock()

	// Add multiple players
	players := []string{"alice", "bob", "charlie"}
	votes := []string{"3", "5", "5"}

	for i, name := range players {
		state.mu.Lock()
		state.players[name] = &playerState{
			points:   votes[i],
			selected: true,
		}
		state.mu.Unlock()
	}

	// Verify all players voted
	state.mu.RLock()
	votedCount := 0
	for _, player := range state.players {
		if player.selected {
			votedCount++
		}
	}
	state.mu.RUnlock()

	if votedCount != len(players) {
		t.Errorf("voted count = %d, want %d", votedCount, len(players))
	}

	// Reveal votes
	state.mu.Lock()
	state.revealed = true
	state.mu.Unlock()

	// Verify revealed state
	state.mu.RLock()
	if !state.revealed {
		t.Errorf("state.revealed = false, want true")
	}
	state.mu.RUnlock()

	// Clear state for next round
	clearPlayerState()

	// Verify state was cleared
	state.mu.RLock()
	if state.revealed {
		t.Errorf("after clear: state.revealed = true, want false")
	}
	for name, player := range state.players {
		if player.selected {
			t.Errorf("after clear: player %s still selected", name)
		}
		if player.points != "" {
			t.Errorf("after clear: player %s points = %s, want empty", name, player.points)
		}
	}
	state.mu.RUnlock()
}

// TestNameValidation tests player name validation scenarios
func TestNameValidation(t *testing.T) {
	// Clear players
	state.mu.Lock()
	state.players = make(map[string]*playerState)
	state.mu.Unlock()

	tests := []struct {
		name       string
		playerName string
		wantExists bool
	}{
		{
			name:       "new player name should not exist",
			playerName: "newplayer",
			wantExists: false,
		},
		{
			name:       "empty name should not exist",
			playerName: "",
			wantExists: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state.mu.RLock()
			_, exists := state.players[tt.playerName]
			state.mu.RUnlock()

			if exists != tt.wantExists {
				t.Errorf("player %s exists = %v, want %v", tt.playerName, exists, tt.wantExists)
			}
		})
	}

	// Test duplicate name scenario
	duplicateName := "duplicate"
	state.mu.Lock()
	state.players[duplicateName] = &playerState{}
	state.mu.Unlock()

	state.mu.RLock()
	_, exists := state.players[duplicateName]
	state.mu.RUnlock()

	if !exists {
		t.Errorf("duplicate player name should exist in state")
	}
}
