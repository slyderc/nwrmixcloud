package formatter

import (
	"strings"
	"testing"

	"github.com/nowwaveradio/mixcloud-updater/internal/config"
	"github.com/nowwaveradio/mixcloud-updater/internal/cue"
	"github.com/nowwaveradio/mixcloud-updater/internal/filter"
)

func TestNewFormatter(t *testing.T) {
	formatter := NewFormatter()
	if formatter == nil {
		t.Fatal("NewFormatter returned nil")
	}
	
	if formatter.GetMaxLength() != 1000 {
		t.Errorf("Expected max length 1000, got %d", formatter.GetMaxLength())
	}
}

func TestNewFormatterWithOptions(t *testing.T) {
	options := FormatOptions{
		MaxLength: 500,
	}
	
	formatter := NewFormatterWithOptions(options)
	if formatter.GetMaxLength() != 500 {
		t.Errorf("Expected max length 500, got %d", formatter.GetMaxLength())
	}
}

func TestFormatTrackLine(t *testing.T) {
	formatter := NewFormatter()
	
	// Test normal track
	track := &cue.Track{
		Index:     1,
		StartTime: "03:45",
		Artist:    "Test Artist",
		Title:     "Test Title",
	}
	
	result := formatter.formatTrackLine(track)
	expected := `03:45 - "Test Title" by Test Artist`
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestFormatTrackLineWithQuotes(t *testing.T) {
	formatter := NewFormatter()
	
	// Test track with quotes in title
	track := &cue.Track{
		Index:     1,
		StartTime: "03:45",
		Artist:    "Test Artist",
		Title:     `Song "Title" With Quotes`,
	}
	
	result := formatter.formatTrackLine(track)
	expected := `03:45 - "Song 'Title' With Quotes" by Test Artist`
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestFormatTrackLineEdgeCases(t *testing.T) {
	formatter := NewFormatter()
	
	tests := []struct {
		name     string
		track    *cue.Track
		expected string
	}{
		{
			name:     "nil track",
			track:    nil,
			expected: "",
		},
		{
			name:     "empty track",
			track:    &cue.Track{},
			expected: "",
		},
		{
			name: "missing start time",
			track: &cue.Track{
				Index:  1,
				Artist: "Artist",
				Title:  "Title",
			},
			expected: `00:00 - "Title" by Artist`,
		},
		{
			name: "missing title",
			track: &cue.Track{
				Index:     1,
				StartTime: "01:23",
				Artist:    "Artist",
			},
			expected: `01:23 - "(Unknown Title)" by Artist`,
		},
		{
			name: "missing artist",
			track: &cue.Track{
				Index:     1,
				StartTime: "01:23",
				Title:     "Title",
			},
			expected: `01:23 - "Title" by (Unknown Artist)`,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatter.formatTrackLine(tt.track)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestFormatTracklistWithoutFilter(t *testing.T) {
	formatter := NewFormatter()
	
	tracks := []cue.Track{
		{
			Index:     1,
			StartTime: "00:00",
			Artist:    "Artist 1",
			Title:     "Title 1",
		},
		{
			Index:     2,
			StartTime: "03:45",
			Artist:    "Artist 2",
			Title:     "Title 2",
		},
	}
	
	result := formatter.FormatTracklist(tracks, nil)
	expected := `00:00 - "Title 1" by Artist 1
03:45 - "Title 2" by Artist 2`
	
	if result != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, result)
	}
}

func TestFormatTracklistWithFilter(t *testing.T) {
	// Create a basic config for the filter
	cfg := &config.Config{}
	cfg.Filtering.ExcludedArtists = []string{"Station ID"}
	
	trackFilter, err := filter.NewFilter(cfg)
	if err != nil {
		t.Fatalf("Failed to create filter: %v", err)
	}
	
	formatter := NewFormatter()
	
	tracks := []cue.Track{
		{
			Index:     1,
			StartTime: "00:00",
			Artist:    "Good Artist",
			Title:     "Good Song",
		},
		{
			Index:     2,
			StartTime: "03:45",
			Artist:    "Station ID", // This should be filtered out
			Title:     "Station Identification",
		},
		{
			Index:     3,
			StartTime: "04:00",
			Artist:    "Another Artist",
			Title:     "Another Song",
		},
	}
	
	result := formatter.FormatTracklist(tracks, trackFilter)
	expected := `00:00 - "Good Song" by Good Artist
04:00 - "Another Song" by Another Artist`
	
	if result != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, result)
	}
}

func TestTruncateSmartly(t *testing.T) {
	formatter := NewFormatter()
	formatter.SetMaxLength(100) // Set a small limit for testing
	
	// Create a long tracklist that exceeds the limit
	longTracklist := `00:00 - "Very Long Song Title That Goes On And On" by Very Long Artist Name
03:45 - "Another Long Song Title" by Another Long Artist Name
07:30 - "Yet Another Song" by Yet Another Artist`
	
	result := formatter.truncateSmartly(longTracklist)
	
	// Should end with truncation text
	if !strings.HasSuffix(result, "... and more") {
		t.Errorf("Expected result to end with truncation text, got: %s", result)
	}
	
	// Should not exceed max length
	if len(result) > formatter.GetMaxLength() {
		t.Errorf("Result length %d exceeds max length %d", len(result), formatter.GetMaxLength())
	}
	
	// Should not contain partial lines
	lines := strings.Split(result, "\n")
	lastLine := lines[len(lines)-1]
	if lastLine != "... and more" {
		// Check that the second-to-last line is a complete track line
		if len(lines) >= 2 {
			secondToLast := lines[len(lines)-2]
			if !strings.Contains(secondToLast, " - ") || !strings.Contains(secondToLast, " by ") {
				t.Errorf("Second-to-last line appears to be partial: %s", secondToLast)
			}
		}
	}
}

func TestGetFormattedTrackCount(t *testing.T) {
	formatter := NewFormatter()
	
	// Create a basic config for the filter
	cfg := &config.Config{}
	cfg.Filtering.ExcludedArtists = []string{"Station ID"}
	
	trackFilter, err := filter.NewFilter(cfg)
	if err != nil {
		t.Fatalf("Failed to create filter: %v", err)
	}
	
	tracks := []cue.Track{
		{Index: 1, Artist: "Good Artist", Title: "Good Song"},
		{Index: 2, Artist: "Station ID", Title: "Station Identification"}, // Filtered out
		{Index: 3, Artist: "Another Artist", Title: "Another Song"},
		{Index: 4}, // Empty track, should be filtered out
	}
	
	// Test with filter
	count := formatter.GetFormattedTrackCount(tracks, trackFilter)
	if count != 2 {
		t.Errorf("Expected 2 tracks after filtering, got %d", count)
	}
	
	// Test without filter (should still exclude empty tracks)
	count = formatter.GetFormattedTrackCount(tracks, nil)
	if count != 3 {
		t.Errorf("Expected 3 tracks without filtering, got %d", count)
	}
}

func TestEstimateTracklistLength(t *testing.T) {
	formatter := NewFormatter()
	
	tracks := []cue.Track{
		{
			Index:     1,
			StartTime: "00:00",
			Artist:    "Artist",
			Title:     "Title",
		},
	}
	
	estimated := formatter.EstimateTracklistLength(tracks, nil)
	actual := len(formatter.FormatTracklist(tracks, nil))
	
	// The estimate should be reasonably close to the actual length
	diff := estimated - actual
	if diff < -5 || diff > 5 { // Allow some variance
		t.Errorf("Estimate %d is too far from actual %d (diff: %d)", estimated, actual, diff)
	}
}

func TestEdgeCases(t *testing.T) {
	formatter := NewFormatter()
	
	// Test with nil tracks
	result := formatter.FormatTracklist(nil, nil)
	if result != "" {
		t.Errorf("Expected empty string for nil tracks, got %q", result)
	}
	
	// Test with empty tracks slice
	result = formatter.FormatTracklist([]cue.Track{}, nil)
	if result != "" {
		t.Errorf("Expected empty string for empty tracks slice, got %q", result)
	}
	
	// Test with all empty tracks
	emptyTracks := []cue.Track{
		{}, // Empty track
		{}, // Another empty track
	}
	result = formatter.FormatTracklist(emptyTracks, nil)
	if result != "" {
		t.Errorf("Expected empty string for all empty tracks, got %q", result)
	}
}

// Template Integration Tests

func TestNewFormatterWithConfig(t *testing.T) {
	cfg := &config.Config{
		Templates: struct {
			Default   string                    `toml:"default"`
			Templates map[string]config.TemplateConfig `toml:"templates"`
		}{
			Default: "simple",
			Templates: map[string]config.TemplateConfig{
				"simple": {
					Header: "Playlist:\n",
					Track:  "{{.StartTime}} - {{.Title}} by {{.Artist}}\n",
					Footer: "Total: {{.TrackCount}} tracks",
				},
			},
		},
	}

	formatter := NewFormatterWithConfig(cfg)
	if formatter == nil {
		t.Fatal("NewFormatterWithConfig returned nil")
	}

	if !formatter.HasTemplateSupport() {
		t.Error("Formatter should have template support")
	}

	if !formatter.HasTemplate("simple") {
		t.Error("Formatter should have 'simple' template")
	}

	if formatter.GetDefaultTemplateName() != "simple" {
		t.Errorf("Expected default template 'simple', got %s", formatter.GetDefaultTemplateName())
	}
}

func TestNewFormatterWithConfigNoTemplates(t *testing.T) {
	cfg := &config.Config{}

	formatter := NewFormatterWithConfig(cfg)
	if formatter == nil {
		t.Fatal("NewFormatterWithConfig returned nil")
	}

	if formatter.HasTemplateSupport() {
		t.Error("Formatter should not have template support when no templates configured")
	}

	templates := formatter.ListAvailableTemplates()
	if len(templates) != 0 {
		t.Errorf("Expected 0 templates, got %d", len(templates))
	}
}

func TestFormatTracklistWithTemplate(t *testing.T) {
	cfg := &config.Config{
		Station: struct {
			Name             string `toml:"name"`
			MixcloudUsername string `toml:"mixcloud_username"`
		}{
			Name: "Test Station",
		},
		Templates: struct {
			Default   string                    `toml:"default"`
			Templates map[string]config.TemplateConfig `toml:"templates"`
		}{
			Default: "test",
			Templates: map[string]config.TemplateConfig{
				"test": {
					Header: "Show Tracklist:\n",
					Track:  "{{.Index}}. {{.StartTime}} - {{.Title}} by {{.Artist}}\n",
					Footer: "Broadcast by {{.StationName}}",
				},
			},
		},
	}

	formatter := NewFormatterWithConfig(cfg)
	
	tracks := []cue.Track{
		{StartTime: "00:00", Artist: "Artist One", Title: "Song One"},
		{StartTime: "03:30", Artist: "Artist Two", Title: "Song Two"},
	}

	filter, err := filter.NewFilter(&config.Config{})
	if err != nil {
		t.Fatalf("NewFilter failed: %v", err)
	}

	// Test default template via FormatTracklist
	result := formatter.FormatTracklist(tracks, filter)
	
	if !strings.Contains(result, "Show Tracklist:") {
		t.Error("Result should contain header")
	}
	if !strings.Contains(result, "1. 00:00 - Song One by Artist One") {
		t.Error("Result should contain formatted track 1")
	}
	if !strings.Contains(result, "2. 03:30 - Song Two by Artist Two") {
		t.Error("Result should contain formatted track 2")
	}
	if !strings.Contains(result, "Broadcast by Test Station") {
		t.Error("Result should contain footer")
	}
}

func TestFormatTracklistFallbackToClassic(t *testing.T) {
	// Test with no template configuration
	formatter := NewFormatter()
	
	tracks := []cue.Track{
		{StartTime: "00:00", Artist: "Artist One", Title: "Song One"},
	}

	filter, err := filter.NewFilter(&config.Config{})
	if err != nil {
		t.Fatalf("NewFilter failed: %v", err)
	}

	result := formatter.FormatTracklist(tracks, filter)
	expected := `00:00 - "Song One" by Artist One`
	if result != expected {
		t.Errorf("Classic fallback mismatch.\nExpected: %q\nGot: %q", expected, result)
	}
}

func TestTemplateHelperMethods(t *testing.T) {
	cfg := &config.Config{
		Templates: struct {
			Default   string                    `toml:"default"`
			Templates map[string]config.TemplateConfig `toml:"templates"`
		}{
			Default: "test",
			Templates: map[string]config.TemplateConfig{
				"test": {Track: "{{.Title}}\n"},
				"another": {Track: "{{.Artist}}\n"},
			},
		},
	}

	formatter := NewFormatterWithConfig(cfg)

	// Test ListAvailableTemplates
	templates := formatter.ListAvailableTemplates()
	if len(templates) != 2 {
		t.Errorf("Expected 2 templates, got %d", len(templates))
	}

	// Test HasTemplate
	if !formatter.HasTemplate("test") {
		t.Error("Should have 'test' template")
	}
	if !formatter.HasTemplate("another") {
		t.Error("Should have 'another' template")
	}
	if formatter.HasTemplate("nonexistent") {
		t.Error("Should not have 'nonexistent' template")
	}

	// Test ValidateTemplate
	err := formatter.ValidateTemplate("test")
	if err != nil {
		t.Errorf("Template validation should pass: %v", err)
	}

	err = formatter.ValidateTemplate("nonexistent")
	if err == nil {
		t.Error("Validation should fail for nonexistent template")
	}
}

func TestTemplateHelperMethodsWithoutSupport(t *testing.T) {
	formatter := NewFormatter() // No template support

	// Test methods without template support
	if formatter.HasTemplateSupport() {
		t.Error("Should not have template support")
	}

	templates := formatter.ListAvailableTemplates()
	if len(templates) != 0 {
		t.Errorf("Expected 0 templates, got %d", len(templates))
	}

	if formatter.HasTemplate("any") {
		t.Error("Should not have any templates")
	}

	if formatter.GetDefaultTemplateName() != "classic" {
		t.Error("Should return 'classic' as default when no template support")
	}

	err := formatter.ValidateTemplate("any")
	if err == nil {
		t.Error("Validation should fail when no template support")
	}
}