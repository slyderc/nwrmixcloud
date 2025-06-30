package processor

import (
	"testing"
	"time"

	"github.com/nowwaveradio/mixcloud-updater/internal/config"
)

func TestConvertDateFormatToGoLayout(t *testing.T) {
	sp := &ShowProcessor{}
	
	tests := []struct {
		name       string
		userFormat string
		expected   string
	}{
		{
			name:       "M/D/YYYY format",
			userFormat: "M/D/YYYY",
			expected:   "1/2/2006",
		},
		{
			name:       "MM/DD/YYYY format",
			userFormat: "MM/DD/YYYY", 
			expected:   "01/02/2006",
		},
		{
			name:       "D-M-YY format",
			userFormat: "D-M-YY",
			expected:   "2-1-06",
		},
		{
			name:       "YYYY.MM.DD format",
			userFormat: "YYYY.MM.DD",
			expected:   "2006.01.02",
		},
		{
			name:       "Mixed separators",
			userFormat: "M/DD-YYYY",
			expected:   "1/02-2006",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sp.convertDateFormatToGoLayout(tt.userFormat)
			if result != tt.expected {
				t.Errorf("convertDateFormatToGoLayout(%q) = %q, want %q", 
					tt.userFormat, result, tt.expected)
			}
		})
	}
}


func TestGenerateShowName(t *testing.T) {
	sp := &ShowProcessor{
		config: &config.Config{
			Station: struct {
				Name             string `toml:"name"`
				MixcloudUsername string `toml:"mixcloud_username"`
			}{
				Name: "Test Station",
			},
		},
	}
	
	tests := []struct {
		name         string
		showCfg      *config.ShowConfig
		cueFile      string
		dateOverride string
		expectedDate string
		expectError  bool
	}{
		{
			name: "Date override provided",
			showCfg: &config.ShowConfig{
				ShowNamePattern: "Test Show - {date}",
				DateFormat:      "M/D/YYYY",
			},
			cueFile:      "static_file.cue",
			dateOverride: "6/28/2025",
			expectedDate: "6/28/2025",
			expectError:  false,
		},
		{
			name: "No override, use current date",
			showCfg: &config.ShowConfig{
				ShowNamePattern: "Test Show - {date}",
				DateFormat:      "M/D/YYYY",
			},
			cueFile:      "static_file.cue",
			dateOverride: "",
			expectedDate: time.Now().Format("1/2/2006"), // Current date in M/D/YYYY format
			expectError:  false,
		},
		{
			name: "Date override provided",
			showCfg: &config.ShowConfig{
				ShowNamePattern: "Test Show - {date}",
				DateFormat:      "M/D/YYYY",
			},
			cueFile:      "any_file.cue",
			dateOverride: "6/25/2025",
			expectedDate: "6/25/2025",
			expectError:  false,
		},
		{
			name: "No show name pattern",
			showCfg: &config.ShowConfig{
				ShowNamePattern: "",
			},
			cueFile:      "test.cue",
			dateOverride: "",
			expectedDate: "",
			expectError:  true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := sp.generateShowName(tt.showCfg, tt.cueFile, tt.dateOverride)
			
			if tt.expectError {
				if err == nil {
					t.Errorf("generateShowName() expected error, got nil")
				}
				return
			}
			
			if err != nil {
				t.Errorf("generateShowName() unexpected error: %v", err)
				return
			}
			
			expectedShowName := "Test Show - " + tt.expectedDate
			if result != expectedShowName {
				t.Errorf("generateShowName() = %q, want %q", result, expectedShowName)
			}
		})
	}
}

func TestDateHandlingFallbackLogic(t *testing.T) {
	sp := &ShowProcessor{
		config: &config.Config{
			Station: struct {
				Name             string `toml:"name"`
				MixcloudUsername string `toml:"mixcloud_username"`
			}{
				Name: "Test Station",
			},
		},
	}
	
	// Test the simplified fallback logic
	showCfg := &config.ShowConfig{
		ShowNamePattern: "Test Show - {date}",
		DateFormat:      "M/D/YYYY",
	}
	
	t.Run("Priority 1: Date override (highest priority)", func(t *testing.T) {
		// Override should be used when provided
		result, err := sp.generateShowName(showCfg, "any_file.cue", "6/28/2025")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if result != "Test Show - 6/28/2025" {
			t.Errorf("Expected override date, got %q", result)
		}
	})
	
	t.Run("Priority 2: Current date fallback", func(t *testing.T) {
		// No override, should use current date
		result, err := sp.generateShowName(showCfg, "any_file.cue", "")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		
		expectedDate := time.Now().Format("1/2/2006") // M/D/YYYY format
		expectedResult := "Test Show - " + expectedDate
		if result != expectedResult {
			t.Errorf("Expected current date fallback %q, got %q", expectedResult, result)
		}
	})
}