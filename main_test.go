package main

import (
	"strings"
	"testing"
)

// TestCalculateStatistics tests the statistics calculation function with various inputs
func TestCalculateStatistics(t *testing.T) {
	tests := []struct {
		name             string
		points           []string
		wantAverage      float64
		wantMedian       string
		wantDistribution map[string]int
	}{
		{
			name:             "empty points slice",
			points:           []string{},
			wantAverage:      0,
			wantMedian:       "N/A",
			wantDistribution: map[string]int{},
		},
		{
			name:             "single numeric value",
			points:           []string{"5"},
			wantAverage:      5.0,
			wantMedian:       "5.0",
			wantDistribution: map[string]int{"5": 1},
		},
		{
			name:             "multiple numeric values - odd count",
			points:           []string{"1", "2", "3", "5", "8"},
			wantAverage:      3.8,
			wantMedian:       "3.0",
			wantDistribution: map[string]int{"1": 1, "2": 1, "3": 1, "5": 1, "8": 1},
		},
		{
			name:             "multiple numeric values - even count",
			points:           []string{"2", "3", "5", "8"},
			wantAverage:      4.5,
			wantMedian:       "4.0",
			wantDistribution: map[string]int{"2": 1, "3": 1, "5": 1, "8": 1},
		},
		{
			name:             "mixed numeric and non-numeric values",
			points:           []string{"1", "2", "?", "5", "?"},
			wantAverage:      2.666666666666667,
			wantMedian:       "2.0",
			wantDistribution: map[string]int{"1": 1, "2": 1, "?": 2, "5": 1},
		},
		{
			name:             "all non-numeric values",
			points:           []string{"?", "?", "?"},
			wantAverage:      0,
			wantMedian:       "N/A",
			wantDistribution: map[string]int{"?": 3},
		},
		{
			name:             "duplicate values",
			points:           []string{"5", "5", "5", "8", "8"},
			wantAverage:      6.2,
			wantMedian:       "5.0",
			wantDistribution: map[string]int{"5": 3, "8": 2},
		},
		{
			name:             "decimal values",
			points:           []string{"0.5", "1", "2", "3"},
			wantAverage:      1.625,
			wantMedian:       "1.5",
			wantDistribution: map[string]int{"0.5": 1, "1": 1, "2": 1, "3": 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotAverage, gotMedian, gotDistribution := calculateStatistics(tt.points)

			// Check average (with floating point tolerance)
			if diff := gotAverage - tt.wantAverage; diff < -0.0001 || diff > 0.0001 {
				t.Errorf("calculateStatistics() average = %v, want %v", gotAverage, tt.wantAverage)
			}

			// Check median
			if gotMedian != tt.wantMedian {
				t.Errorf("calculateStatistics() median = %v, want %v", gotMedian, tt.wantMedian)
			}

			// Check distribution
			if len(gotDistribution) != len(tt.wantDistribution) {
				t.Errorf("calculateStatistics() distribution length = %v, want %v", len(gotDistribution), len(tt.wantDistribution))
			}
			for key, wantCount := range tt.wantDistribution {
				if gotCount, ok := gotDistribution[key]; !ok || gotCount != wantCount {
					t.Errorf("calculateStatistics() distribution[%s] = %v, want %v", key, gotCount, wantCount)
				}
			}
		})
	}
}

// TestShowFinalVotes tests the final votes display function
func TestShowFinalVotes(t *testing.T) {
	tests := []struct {
		name       string
		points     []string
		voted      int
		wantSubstr []string // substrings that should appear in output
	}{
		{
			name:   "single vote",
			points: []string{"5"},
			voted:  1,
			wantSubstr: []string{
				"Voting Statistics",
				"Average: 5.0",
				"Median: 5.0",
				"Distribution:",
				"5:",
				"1 votes",
				"100.0%",
			},
		},
		{
			name:   "multiple votes with distribution",
			points: []string{"3", "5", "5", "8"},
			voted:  4,
			wantSubstr: []string{
				"Voting Statistics",
				"Average:",
				"Median:",
				"Distribution:",
				"3:",
				"5:",
				"8:",
			},
		},
		{
			name:   "votes with non-numeric values",
			points: []string{"5", "?", "5"},
			voted:  3,
			wantSubstr: []string{
				"Voting Statistics",
				"Median:",
				"Distribution:",
				"?:",
				"5:",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := showFinalVotes(tt.points, tt.voted)

			for _, substr := range tt.wantSubstr {
				if !strings.Contains(got, substr) {
					t.Errorf("showFinalVotes() output missing substring %q\nGot: %s", substr, got)
				}
			}
		})
	}
}

// TestTimerDurations verifies the timer duration mapping
func TestTimerDurations(t *testing.T) {
	tests := []struct {
		key      string
		wantSecs int
	}{
		{"1", 15},
		{"3", 30},
		{"6", 60},
	}

	for _, tt := range tests {
		t.Run("key_"+tt.key, func(t *testing.T) {
			duration, exists := timerDurations[tt.key]
			if !exists {
				t.Errorf("timerDurations[%s] does not exist", tt.key)
			}
			if int(duration.Seconds()) != tt.wantSecs {
				t.Errorf("timerDurations[%s] = %d seconds, want %d seconds", tt.key, int(duration.Seconds()), tt.wantSecs)
			}
		})
	}
}

// TestPointOptions verifies the available point options
func TestPointOptions(t *testing.T) {
	expectedOptions := []string{"0.5", "1", "2", "3", "5", "8", "10", "?"}

	if len(pointOptions) != len(expectedOptions) {
		t.Errorf("pointOptions length = %d, want %d", len(pointOptions), len(expectedOptions))
	}

	for i, expected := range expectedOptions {
		if i >= len(pointOptions) {
			t.Errorf("pointOptions[%d] missing, want %s", i, expected)
			continue
		}
		if pointOptions[i] != expected {
			t.Errorf("pointOptions[%d] = %s, want %s", i, pointOptions[i], expected)
		}
	}
}
