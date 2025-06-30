package processor

import (
	"strings"
	"testing"

	"github.com/nowwaveradio/mixcloud-updater/internal/config"
)

func TestNewShowProcessor(t *testing.T) {
	cfg := &config.Config{
		Station: struct {
			Name             string `toml:"name"`
			MixcloudUsername string `toml:"mixcloud_username"`
		}{
			Name:             "Test Station",
			MixcloudUsername: "testuser",
		},
		OAuth: struct {
			ClientID     string `toml:"client_id"`
			ClientSecret string `toml:"client_secret"`
			AccessToken  string `toml:"access_token"`
			RefreshToken string `toml:"refresh_token"`
		}{
			ClientID:     "test-client-id",
			ClientSecret: "test-client-secret",
			AccessToken:  "test-access-token",
		},
		Processing: struct {
			CueFileDirectory string `toml:"cue_file_directory"`
			AutoProcess      bool   `toml:"auto_process"`
			BatchSize        int    `toml:"batch_size"`
		}{
			CueFileDirectory: ".",
			AutoProcess:      false,
			BatchSize:        3,
		},
		Shows: map[string]config.ShowConfig{
			"test-show": {
				CueFilePattern:  "test*.cue",
				ShowNamePattern: "Test Show - {date}",
				Aliases:         []string{"test", "ts"},
				TemplateName:    "default",
				Enabled:         true,
				Priority:        1,
			},
		},
		Templates: struct {
			Default string                    `toml:"default"`
			Config  map[string]config.TemplateConfig `toml:"config"`
		}{
			Default: "default",
			Config: map[string]config.TemplateConfig{
				"default": {
					Header: "Playlist:",
					Track:  "{{.StartTime}} - {{.Title}} by {{.Artist}}",
					Footer: "End",
				},
			},
		},
	}

	processor, err := NewShowProcessor(cfg, "test-config.toml")
	if err != nil {
		t.Fatalf("NewShowProcessor() error = %v", err)
	}

	if processor == nil {
		t.Fatal("NewShowProcessor() returned nil processor")
	}

	if processor.config != cfg {
		t.Error("Config not properly set")
	}

	if processor.resolver == nil {
		t.Error("Resolver not initialized")
	}

	if processor.cueResolver == nil {
		t.Error("CueResolver not initialized")
	}

	if processor.formatter == nil {
		t.Error("Formatter not initialized")
	}
}

func TestNewShowProcessorNilConfig(t *testing.T) {
	_, err := NewShowProcessor(nil, "test-config.toml")
	if err == nil {
		t.Error("NewShowProcessor() should return error for nil config")
	}
}

func TestNewShowProcessorInvalidShows(t *testing.T) {
	cfg := &config.Config{
		Station: struct {
			Name             string `toml:"name"`
			MixcloudUsername string `toml:"mixcloud_username"`
		}{
			Name:             "Test Station",
			MixcloudUsername: "testuser",
		},
		OAuth: struct {
			ClientID     string `toml:"client_id"`
			ClientSecret string `toml:"client_secret"`
			AccessToken  string `toml:"access_token"`
			RefreshToken string `toml:"refresh_token"`
		}{
			ClientID:     "test-client-id",
			ClientSecret: "test-client-secret",
			AccessToken:  "test-access-token",
		},
		Shows: map[string]config.ShowConfig{
			"invalid-show": {
				// Missing required fields - should cause validation error
				ShowNamePattern: "", // Required field missing
				Enabled:         true,
			},
		},
	}

	_, err := NewShowProcessor(cfg, "test-config.toml")
	if err == nil {
		t.Error("NewShowProcessor() should return error for invalid show configuration")
	}
}

func TestGenerateShowNameLegacy(t *testing.T) {
	cfg := &config.Config{
		Station: struct {
			Name             string `toml:"name"`
			MixcloudUsername string `toml:"mixcloud_username"`
		}{
			Name: "Test Station",
		},
	}

	processor := &ShowProcessor{
		config: cfg,
	}

	tests := []struct {
		name      string
		showCfg   *config.ShowConfig
		cueFile   string
		expected  string
		wantError bool
	}{
		{
			name: "simple pattern without date",
			showCfg: &config.ShowConfig{
				ShowNamePattern: "Test Show",
			},
			cueFile:  "test.cue",
			expected: "Test Show",
		},
		{
			name: "pattern with station placeholder",
			showCfg: &config.ShowConfig{
				ShowNamePattern: "{station} Weekly Show",
			},
			cueFile:  "test.cue",
			expected: "Test Station Weekly Show",
		},
		{
			name: "pattern with current date",
			showCfg: &config.ShowConfig{
				ShowNamePattern: "Show - {date}",
			},
			cueFile:  "any_file.cue",
			expected: "Show -", // Will use current date, but test just checks prefix
		},
		{
			name: "empty pattern",
			showCfg: &config.ShowConfig{
				ShowNamePattern: "",
			},
			cueFile:   "test.cue",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := processor.generateShowName(tt.showCfg, tt.cueFile, "")

			if tt.wantError {
				if err == nil {
					t.Error("generateShowName() should return error")
				}
			} else {
				if err != nil {
					t.Errorf("generateShowName() error = %v", err)
				}
				if tt.expected == "Show -" {
					// Special case for current date - just check prefix
					if !strings.HasPrefix(result, tt.expected) {
						t.Errorf("generateShowName() = %v, want prefix %v", result, tt.expected)
					}
				} else if result != tt.expected {
					t.Errorf("generateShowName() = %v, want %v", result, tt.expected)
				}
			}
		})
	}
}

func TestProcessShowValidation(t *testing.T) {
	// Create temporary directory for test
	tmpDir := t.TempDir()

	cfg := &config.Config{
		Station: struct {
			Name             string `toml:"name"`
			MixcloudUsername string `toml:"mixcloud_username"`
		}{
			Name:             "Test Station",
			MixcloudUsername: "testuser",
		},
		OAuth: struct {
			ClientID     string `toml:"client_id"`
			ClientSecret string `toml:"client_secret"`
			AccessToken  string `toml:"access_token"`
			RefreshToken string `toml:"refresh_token"`
		}{
			ClientID:     "test-client-id",
			ClientSecret: "test-client-secret",
			AccessToken:  "test-access-token",
		},
		Processing: struct {
			CueFileDirectory string `toml:"cue_file_directory"`
			AutoProcess      bool   `toml:"auto_process"`
			BatchSize        int    `toml:"batch_size"`
		}{
			CueFileDirectory: tmpDir,
		},
		Shows: map[string]config.ShowConfig{
			"enabled-show": {
				CueFileMapping:  "test.cue",
				ShowNamePattern: "Enabled Show",
				Enabled:         true,
				Priority:        1,
			},
			"disabled-show": {
				CueFileMapping:  "test.cue",
				ShowNamePattern: "Disabled Show",
				Enabled:         false,
				Priority:        2,
			},
		},
	}

	processor, err := NewShowProcessor(cfg, "test-config.toml")
	if err != nil {
		t.Fatalf("NewShowProcessor() error = %v", err)
	}

	// Test non-existent show
	err = processor.ProcessShow("non-existent", "", "", true)
	if err == nil {
		t.Error("ProcessShow() should return error for non-existent show")
	}

	// Test disabled show (should not error, but should skip)
	err = processor.ProcessShow("disabled-show", "", "", true)
	if err != nil {
		t.Errorf("ProcessShow() unexpected error for disabled show = %v", err)
	}
}

func TestProcessAllShowsEmptyConfig(t *testing.T) {
	cfg := &config.Config{
		Station: struct {
			Name             string `toml:"name"`
			MixcloudUsername string `toml:"mixcloud_username"`
		}{
			Name:             "Test Station",
			MixcloudUsername: "testuser",
		},
		OAuth: struct {
			ClientID     string `toml:"client_id"`
			ClientSecret string `toml:"client_secret"`
			AccessToken  string `toml:"access_token"`
			RefreshToken string `toml:"refresh_token"`
		}{
			ClientID:     "test-client-id",
			ClientSecret: "test-client-secret",
			AccessToken:  "test-access-token",
		},
		Shows: map[string]config.ShowConfig{}, // No shows configured
	}

	processor, err := NewShowProcessor(cfg, "test-config.toml")
	if err != nil {
		t.Fatalf("NewShowProcessor() error = %v", err)
	}

	// Should not error when no shows are configured
	err = processor.ProcessAllShows(true)
	if err != nil {
		t.Errorf("ProcessAllShows() unexpected error = %v", err)
	}
}

func TestProcessAllShowsWithDisabledShows(t *testing.T) {
	cfg := &config.Config{
		Station: struct {
			Name             string `toml:"name"`
			MixcloudUsername string `toml:"mixcloud_username"`
		}{
			Name:             "Test Station",
			MixcloudUsername: "testuser",
		},
		OAuth: struct {
			ClientID     string `toml:"client_id"`
			ClientSecret string `toml:"client_secret"`
			AccessToken  string `toml:"access_token"`
			RefreshToken string `toml:"refresh_token"`
		}{
			ClientID:     "test-client-id",
			ClientSecret: "test-client-secret",
			AccessToken:  "test-access-token",
		},
		Shows: map[string]config.ShowConfig{
			"disabled-show1": {
				CueFileMapping:  "test1.cue",
				ShowNamePattern: "Disabled Show 1",
				Enabled:         false,
				Priority:        1,
			},
			"disabled-show2": {
				CueFileMapping:  "test2.cue",
				ShowNamePattern: "Disabled Show 2",
				Enabled:         false,
				Priority:        2,
			},
		},
	}

	processor, err := NewShowProcessor(cfg, "test-config.toml")
	if err != nil {
		t.Fatalf("NewShowProcessor() error = %v", err)
	}

	// Should not error when all shows are disabled
	err = processor.ProcessAllShows(true)
	if err != nil {
		t.Errorf("ProcessAllShows() unexpected error = %v", err)
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && 
		   (len(substr) == 0 || 
		    s == substr || 
		    (len(s) > len(substr) && (s[:len(substr)] == substr || 
		     s[len(s)-len(substr):] == substr || 
		     containsInMiddle(s, substr))))
}

func containsInMiddle(s, substr string) bool {
	for i := 1; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}