package shows

import (
	"testing"

	"github.com/nowwaveradio/mixcloud-updater/internal/config"
)

func TestNewResolver(t *testing.T) {
	cfg := &config.Config{
		Shows: map[string]config.ShowConfig{
			"test-show": {
				ShowNamePattern: "Test Show",
				Aliases:         []string{"test", "ts"},
				Enabled:         true,
				Priority:        1,
			},
		},
	}

	resolver, err := NewResolver(cfg)
	if err != nil {
		t.Fatalf("NewResolver() error = %v", err)
	}

	if resolver == nil {
		t.Fatal("NewResolver() returned nil resolver")
	}

	if len(resolver.showKeys) != 1 {
		t.Errorf("Expected 1 show key, got %d", len(resolver.showKeys))
	}

	if len(resolver.aliasMap) != 3 { // show key + 2 aliases
		t.Errorf("Expected 3 alias mappings, got %d", len(resolver.aliasMap))
	}
}

func TestNewResolverNilConfig(t *testing.T) {
	_, err := NewResolver(nil)
	if err == nil {
		t.Error("NewResolver() should return error for nil config")
	}
}

func TestFindShowConfig(t *testing.T) {
	cfg := &config.Config{
		Shows: map[string]config.ShowConfig{
			"newer-new-wave": {
				ShowNamePattern: "The Newer New Wave Show - {date}",
				Aliases:         []string{"nnw", "new-wave"},
				TemplateName:    "default",
				Enabled:         true,
				Priority:        1,
			},
			"sounds-like": {
				ShowNamePattern: "Sounds Like - Jeri-Rig & V-Dub - {date}",
				Aliases:         []string{"sl", "sounds"},
				CustomTemplate:  "custom template",
				Enabled:         true,
				Priority:        2,
			},
		},
	}

	resolver, err := NewResolver(cfg)
	if err != nil {
		t.Fatalf("NewResolver() error = %v", err)
	}

	tests := []struct {
		name      string
		input     string
		wantFound bool
		wantKey   string
	}{
		{"exact show key", "newer-new-wave", true, "newer-new-wave"},
		{"exact show key case insensitive", "NEWER-NEW-WAVE", true, "newer-new-wave"},
		{"alias exact", "nnw", true, "newer-new-wave"},
		{"alias case insensitive", "NNW", true, "newer-new-wave"},
		{"second show by alias", "sl", true, "sounds-like"},
		{"second show by key", "sounds-like", true, "sounds-like"},
		{"non-existent show", "non-existent", false, ""},
		{"empty string", "", false, ""},
		{"whitespace only", "   ", false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := resolver.FindShowConfig(tt.input)
			
			if tt.wantFound {
				if result == nil {
					t.Errorf("FindShowConfig(%q) = nil, want non-nil", tt.input)
					return
				}
				
				// Verify we got the right show by checking a unique field
				expectedConfig := cfg.Shows[tt.wantKey]
				if result.ShowNamePattern != expectedConfig.ShowNamePattern {
					t.Errorf("FindShowConfig(%q) returned wrong show. Got pattern %q, want %q", 
						tt.input, result.ShowNamePattern, expectedConfig.ShowNamePattern)
				}
			} else {
				if result != nil {
					t.Errorf("FindShowConfig(%q) = %v, want nil", tt.input, result)
				}
			}
		})
	}
}

func TestFindShowKey(t *testing.T) {
	cfg := &config.Config{
		Shows: map[string]config.ShowConfig{
			"test-show": {
				ShowNamePattern: "Test Show",
				Aliases:         []string{"test", "ts"},
				Enabled:         true,
			},
		},
	}

	resolver, err := NewResolver(cfg)
	if err != nil {
		t.Fatalf("NewResolver() error = %v", err)
	}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"exact key", "test-show", "test-show"},
		{"case insensitive key", "TEST-SHOW", "test-show"},
		{"alias", "test", "test-show"},
		{"case insensitive alias", "TS", "test-show"},
		{"non-existent", "non-existent", ""},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := resolver.FindShowKey(tt.input)
			if result != tt.expected {
				t.Errorf("FindShowKey(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestListShows(t *testing.T) {
	cfg := &config.Config{
		Shows: map[string]config.ShowConfig{
			"show-a": {ShowNamePattern: "Show A", Enabled: true},
			"show-b": {ShowNamePattern: "Show B", Enabled: false},
			"show-c": {ShowNamePattern: "Show C", Enabled: true},
		},
	}

	resolver, err := NewResolver(cfg)
	if err != nil {
		t.Fatalf("NewResolver() error = %v", err)
	}

	shows := resolver.ListShows()
	if len(shows) != 3 {
		t.Errorf("ListShows() returned %d shows, want 3", len(shows))
	}

	// Test that modifying the returned slice doesn't affect internal state
	originalLen := len(resolver.showKeys)
	shows[0] = "modified"
	if len(resolver.showKeys) != originalLen || resolver.showKeys[0] == "modified" {
		t.Error("ListShows() should return a copy, not the original slice")
	}
}

func TestListEnabledShows(t *testing.T) {
	cfg := &config.Config{
		Shows: map[string]config.ShowConfig{
			"high-priority": {ShowNamePattern: "High Priority", Enabled: true, Priority: 10},
			"disabled-show": {ShowNamePattern: "Disabled", Enabled: false, Priority: 5},
			"low-priority":  {ShowNamePattern: "Low Priority", Enabled: true, Priority: 1},
			"med-priority":  {ShowNamePattern: "Med Priority", Enabled: true, Priority: 5},
		},
	}

	resolver, err := NewResolver(cfg)
	if err != nil {
		t.Fatalf("NewResolver() error = %v", err)
	}

	// Test without sorting
	enabled := resolver.ListEnabledShows(false)
	if len(enabled) != 3 { // Should exclude disabled-show
		t.Errorf("ListEnabledShows(false) returned %d shows, want 3", len(enabled))
	}

	// Test with priority sorting
	sorted := resolver.ListEnabledShows(true)
	if len(sorted) != 3 {
		t.Errorf("ListEnabledShows(true) returned %d shows, want 3", len(sorted))
	}

	// Verify priority ordering (higher first)
	expectedOrder := []string{"high-priority", "med-priority", "low-priority"}
	for i, expected := range expectedOrder {
		if i >= len(sorted) || sorted[i] != expected {
			t.Errorf("Priority ordering incorrect. Got %v, want %v", sorted, expectedOrder)
			break
		}
	}
}

func TestGetShowAliases(t *testing.T) {
	cfg := &config.Config{
		Shows: map[string]config.ShowConfig{
			"test-show": {
				ShowNamePattern: "Test Show",
				Aliases:         []string{"test", "ts", "testing"},
				Enabled:         true,
			},
			"no-alias-show": {
				ShowNamePattern: "No Alias Show",
				Aliases:         []string{},
				Enabled:         true,
			},
		},
	}

	resolver, err := NewResolver(cfg)
	if err != nil {
		t.Fatalf("NewResolver() error = %v", err)
	}

	// Test show with aliases
	aliases := resolver.GetShowAliases("test-show")
	expectedAliases := []string{"test", "ts", "testing"}
	if len(aliases) != len(expectedAliases) {
		t.Errorf("GetShowAliases() returned %d aliases, want %d", len(aliases), len(expectedAliases))
	}

	// Test show without aliases
	noAliases := resolver.GetShowAliases("no-alias-show")
	if len(noAliases) != 0 {
		t.Errorf("GetShowAliases() for show without aliases returned %d, want 0", len(noAliases))
	}

	// Test non-existent show
	nonExistent := resolver.GetShowAliases("non-existent")
	if len(nonExistent) != 0 {
		t.Errorf("GetShowAliases() for non-existent show returned %d, want 0", len(nonExistent))
	}

	// Test that returned slice is a copy
	aliases[0] = "modified"
	originalAliases := resolver.GetShowAliases("test-show")
	if originalAliases[0] == "modified" {
		t.Error("GetShowAliases() should return a copy, not the original slice")
	}
}

func TestAliasConflicts(t *testing.T) {
	cfg := &config.Config{
		Shows: map[string]config.ShowConfig{
			"show-a": {
				ShowNamePattern: "Show A",
				Aliases:         []string{"conflict", "unique-a"},
				Enabled:         true,
			},
			"show-b": {
				ShowNamePattern: "Show B",
				Aliases:         []string{"conflict", "unique-b"}, // Conflict with show-a
				Enabled:         true,
			},
		},
	}

	_, err := NewResolver(cfg)
	if err == nil {
		t.Error("NewResolver() should return error for conflicting aliases")
	}

	if !contains(err.Error(), "alias conflict") {
		t.Errorf("Error should mention alias conflict, got: %v", err)
	}
}

func TestValidateShows(t *testing.T) {
	tests := []struct {
		name      string
		shows     map[string]config.ShowConfig
		wantError bool
		errorText string
	}{
		{
			name: "valid shows",
			shows: map[string]config.ShowConfig{
				"valid-show": {
					CueFilePattern:  "*.cue",
					ShowNamePattern: "Valid Show",
					TemplateName:    "default",
					Priority:        1,
					Enabled:         true,
				},
			},
			wantError: false,
		},
		{
			name: "missing cue file source",
			shows: map[string]config.ShowConfig{
				"invalid-show": {
					ShowNamePattern: "Invalid Show",
					Priority:        1,
					Enabled:         true,
				},
			},
			wantError: true,
			errorText: "no CUE file source configured",
		},
		{
			name: "missing show name pattern",
			shows: map[string]config.ShowConfig{
				"invalid-show": {
					CueFilePattern: "*.cue",
					Priority:       1,
					Enabled:        true,
				},
			},
			wantError: true,
			errorText: "show_name_pattern is required",
		},
		{
			name: "negative priority",
			shows: map[string]config.ShowConfig{
				"invalid-show": {
					CueFilePattern:  "*.cue",
					ShowNamePattern: "Invalid Show",
					Priority:        -1,
					Enabled:         true,
				},
			},
			wantError: true,
			errorText: "priority must be non-negative",
		},
		{
			name: "both template and custom template",
			shows: map[string]config.ShowConfig{
				"invalid-show": {
					CueFilePattern:  "*.cue",
					ShowNamePattern: "Invalid Show",
					TemplateName:    "default",
					CustomTemplate:  "custom",
					Priority:        1,
					Enabled:         true,
				},
			},
			wantError: true,
			errorText: "cannot specify both template and custom_template",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{Shows: tt.shows}
			resolver, err := NewResolver(cfg)
			if err != nil {
				t.Fatalf("NewResolver() error = %v", err)
			}

			err = resolver.ValidateShows()
			if tt.wantError {
				if err == nil {
					t.Error("ValidateShows() should return error")
				} else if !contains(err.Error(), tt.errorText) {
					t.Errorf("Error should contain %q, got: %v", tt.errorText, err)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateShows() unexpected error = %v", err)
				}
			}
		})
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