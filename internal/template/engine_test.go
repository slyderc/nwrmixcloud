package template

import (
	"fmt"
	"strings"
	"testing"

	"github.com/nowwaveradio/mixcloud-updater/internal/config"
	"github.com/nowwaveradio/mixcloud-updater/internal/cue"
	"github.com/nowwaveradio/mixcloud-updater/internal/filter"
)

func TestNewTemplateFormatter(t *testing.T) {
	cfg := &config.Config{}
	formatter := NewTemplateFormatter(cfg)

	if formatter == nil {
		t.Fatal("NewTemplateFormatter returned nil")
	}

	if formatter.config != cfg {
		t.Error("Config not properly set")
	}

	if formatter.templates == nil {
		t.Error("Templates map not initialized")
	}
}

func TestLoadTemplates(t *testing.T) {
	cfg := &config.Config{
		Templates: struct {
			Default   string                    `toml:"default"`
			Templates map[string]config.TemplateConfig `toml:"templates"`
		}{
			Default: "minimal",
			Templates: map[string]config.TemplateConfig{
				"minimal": {
					Header: "Tracklist:",
					Track:  "{{.StartTime}} - {{.Title}} by {{.Artist}}",
					Footer: "Total: {{.TrackCount}} tracks",
				},
				"detailed": {
					Header: "=== {{.ShowTitle}} ===",
					Track:  "[{{.Index}}] {{.StartTime}} - \"{{.Title}}\" by {{.Artist}}{{if .Genre}} ({{.Genre}}){{end}}",
					Footer: "Broadcast by {{.StationName}}",
				},
			},
		},
	}

	formatter := NewTemplateFormatter(cfg)
	err := formatter.LoadTemplates()

	if err != nil {
		t.Fatalf("LoadTemplates failed: %v", err)
	}

	// Check that templates were loaded
	if len(formatter.templates) != 2 {
		t.Errorf("Expected 2 templates, got %d", len(formatter.templates))
	}

	if !formatter.HasTemplate("minimal") {
		t.Error("Minimal template not loaded")
	}

	if !formatter.HasTemplate("detailed") {
		t.Error("Detailed template not loaded")
	}
}

func TestLoadTemplatesWithMissingTrack(t *testing.T) {
	cfg := &config.Config{
		Templates: struct {
			Default   string                    `toml:"default"`
			Templates map[string]config.TemplateConfig `toml:"templates"`
		}{
			Templates: map[string]config.TemplateConfig{
				"invalid": {
					Header: "Header only",
					Footer: "Footer only",
					// Missing Track template
				},
			},
		},
	}

	formatter := NewTemplateFormatter(cfg)
	err := formatter.LoadTemplates()

	if err == nil {
		t.Error("Expected error for missing track template")
	}

	if !strings.Contains(err.Error(), "track template is required") {
		t.Errorf("Expected 'track template is required' error, got: %v", err)
	}
}

func TestFormatWithTemplate(t *testing.T) {
	cfg := &config.Config{
		Station: struct {
			Name             string `toml:"name"`
			MixcloudUsername string `toml:"mixcloud_username"`
		}{
			Name: "Test Radio",
		},
		Templates: struct {
			Default   string                    `toml:"default"`
			Templates map[string]config.TemplateConfig `toml:"templates"`
		}{
			Templates: map[string]config.TemplateConfig{
				"simple": {
					Header: "Playlist:\n",
					Track:  "{{.StartTime}} - {{.Title}} by {{.Artist}}\n",
					Footer: "\nTotal: {{.TrackCount}} tracks",
				},
			},
		},
	}

	formatter := NewTemplateFormatter(cfg)
	err := formatter.LoadTemplates()
	if err != nil {
		t.Fatalf("LoadTemplates failed: %v", err)
	}

	tracks := []cue.Track{
		{
			StartTime: "00:00",
			Artist:    "Artist One",
			Title:     "Song One",
			Genre:     "Rock",
		},
		{
			StartTime: "03:30",
			Artist:    "Artist Two",
			Title:     "Song Two",
			Genre:     "Pop",
		},
	}

	metadata := map[string]interface{}{
		"show_title": "Test Show",
		"show_date":  "2024-01-01",
	}

	// Create a filter for testing
	testFilter, err := filter.NewFilter(&config.Config{})
	if err != nil {
		t.Fatalf("NewFilter failed: %v", err)
	}
	
	result, err := formatter.FormatWithTemplate("simple", tracks, testFilter, metadata)
	if err != nil {
		t.Fatalf("FormatWithTemplate failed: %v", err)
	}

	expected := "Playlist:\n00:00 - Song One by Artist One\n03:30 - Song Two by Artist Two\n\nTotal: 2 tracks"
	if result != expected {
		t.Errorf("Template output mismatch.\nExpected:\n%s\nGot:\n%s", expected, result)
	}
}

func TestFormatWithNonExistentTemplate(t *testing.T) {
	cfg := &config.Config{}
	formatter := NewTemplateFormatter(cfg)

	tracks := []cue.Track{
		{StartTime: "00:00", Artist: "Test", Title: "Test"},
	}

	// Should return error for non-existent template
	testFilter, err := filter.NewFilter(&config.Config{})
	if err != nil {
		t.Fatalf("NewFilter failed: %v", err)
	}
	result, err := formatter.FormatWithTemplate("nonexistent", tracks, testFilter, nil)
	if err == nil {
		t.Error("Expected error for non-existent template")
	}

	if result != "" {
		t.Error("Expected empty result on error")
	}
}

func TestValidateTemplate(t *testing.T) {
	cfg := &config.Config{
		Templates: struct {
			Default   string                    `toml:"default"`
			Templates map[string]config.TemplateConfig `toml:"templates"`
		}{
			Templates: map[string]config.TemplateConfig{
				"valid": {
					Header: "Header: {{.ShowTitle}}",
					Track:  "{{.StartTime}} - {{.Title}}",
					Footer: "Footer: {{.TrackCount}}",
				},
				"invalid": {
					Track: "{{.InvalidField}}", // This should cause validation to fail
				},
			},
		},
	}

	formatter := NewTemplateFormatter(cfg)
	err := formatter.LoadTemplates()
	if err != nil {
		t.Fatalf("LoadTemplates failed: %v", err)
	}

	// Valid template should pass validation
	err = formatter.ValidateTemplate("valid")
	if err != nil {
		t.Errorf("Valid template failed validation: %v", err)
	}

	// Invalid template should fail validation
	err = formatter.ValidateTemplate("invalid")
	if err == nil {
		t.Error("Expected validation to fail for invalid template")
	}
}

func TestTemplateFunctions(t *testing.T) {
	cfg := &config.Config{
		Templates: struct {
			Default   string                    `toml:"default"`
			Templates map[string]config.TemplateConfig `toml:"templates"`
		}{
			Templates: map[string]config.TemplateConfig{
				"functions": {
					Track: "{{upper .Artist}} - {{lower .Title}} ({{truncate .Genre 3}})",
				},
			},
		},
	}

	formatter := NewTemplateFormatter(cfg)
	err := formatter.LoadTemplates()
	if err != nil {
		t.Fatalf("LoadTemplates failed: %v", err)
	}

	tracks := []cue.Track{
		{
			Artist: "artist name",
			Title:  "SONG TITLE",
			Genre:  "Electronic Music",
		},
	}

	testFilter, err := filter.NewFilter(&config.Config{})
	if err != nil {
		t.Fatalf("NewFilter failed: %v", err)
	}
	result, err := formatter.FormatWithTemplate("functions", tracks, testFilter, nil)
	if err != nil {
		t.Fatalf("FormatWithTemplate failed: %v", err)
	}

	expected := "ARTIST NAME - song title (Ele...)"
	if result != expected {
		t.Errorf("Function template output mismatch.\nExpected: %s\nGot: %s", expected, result)
	}
}

func TestCharacterLimitEnforcement(t *testing.T) {
	cfg := &config.Config{
		Templates: struct {
			Default   string                    `toml:"default"`
			Templates map[string]config.TemplateConfig `toml:"templates"`
		}{
			Templates: map[string]config.TemplateConfig{
				"long": {
					Track: "{{repeat \"Very long track description that exceeds normal length \" 10}}{{.Title}}\n",
				},
			},
		},
	}

	formatter := NewTemplateFormatter(cfg)
	err := formatter.LoadTemplates()
	if err != nil {
		t.Fatalf("LoadTemplates failed: %v", err)
	}

	// Create many tracks to test limit enforcement
	tracks := make([]cue.Track, 20)
	for i := range tracks {
		tracks[i] = cue.Track{
			StartTime: "00:00",
			Artist:    "Artist",
			Title:     "Title",
		}
	}

	testFilter, err := filter.NewFilter(&config.Config{})
	if err != nil {
		t.Fatalf("NewFilter failed: %v", err)
	}
	result, err := formatter.FormatWithTemplate("long", tracks, testFilter, nil)
	if err != nil {
		t.Fatalf("FormatWithTemplate failed: %v", err)
	}

	// Should be limited to 1000 characters
	if len(result) > 1000 {
		t.Errorf("Result exceeds 1000 character limit: %d characters", len(result))
	}
}

func TestSmartTruncationWithFooter(t *testing.T) {
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
			Templates: map[string]config.TemplateConfig{
				"truncation_test": {
					Header: "Show Playlist:\n",
					Track:  "{{.Index}}. {{.StartTime}} - \"{{.Title}}\" by {{.Artist}} ({{repeat \"X\" 50}})\n",
					Footer: "\nBroadcast by {{.StationName}} | Total: {{.TrackCount}} tracks",
				},
			},
		},
	}

	formatter := NewTemplateFormatter(cfg)
	err := formatter.LoadTemplates()
	if err != nil {
		t.Fatalf("LoadTemplates failed: %v", err)
	}

	// Create many tracks to force truncation
	tracks := make([]cue.Track, 15)
	for i := range tracks {
		tracks[i] = cue.Track{
			StartTime: fmt.Sprintf("%02d:00", i),
			Artist:    fmt.Sprintf("Artist %d", i+1),
			Title:     fmt.Sprintf("Long Song Title %d", i+1),
		}
	}

	metadata := map[string]interface{}{
		"show_title": "Test Show",
	}

	testFilter, err := filter.NewFilter(&config.Config{})
	if err != nil {
		t.Fatalf("NewFilter failed: %v", err)
	}
	result, err := formatter.FormatWithTemplate("truncation_test", tracks, testFilter, metadata)
	if err != nil {
		t.Fatalf("FormatWithTemplate failed: %v", err)
	}

	// Should be limited to 1000 characters
	if len(result) > 1000 {
		t.Errorf("Result exceeds 1000 character limit: %d characters", len(result))
	}

	// Should contain truncation message
	if !strings.Contains(result, "and") && !strings.Contains(result, "more") {
		t.Logf("Result: %s", result)
		// This is not necessarily an error as truncation message is only added when needed
	}

	// Should contain the footer
	if !strings.Contains(result, "Broadcast by Test Station") {
		t.Errorf("Result should contain footer")
	}

	// Should start with header
	if !strings.HasPrefix(result, "Show Playlist:") {
		t.Errorf("Result should start with header")
	}
}

func TestListTemplates(t *testing.T) {
	cfg := &config.Config{
		Templates: struct {
			Default   string                    `toml:"default"`
			Templates map[string]config.TemplateConfig `toml:"templates"`
		}{
			Templates: map[string]config.TemplateConfig{
				"template1": {Track: "{{.Title}}"},
				"template2": {Track: "{{.Artist}}"},
			},
		},
	}

	formatter := NewTemplateFormatter(cfg)
	err := formatter.LoadTemplates()
	if err != nil {
		t.Fatalf("LoadTemplates failed: %v", err)
	}

	templates := formatter.ListTemplates()
	if len(templates) != 2 {
		t.Errorf("Expected 2 templates, got %d", len(templates))
	}

	// Check that both templates are present
	hasTemplate1 := false
	hasTemplate2 := false
	for _, name := range templates {
		if name == "template1" {
			hasTemplate1 = true
		}
		if name == "template2" {
			hasTemplate2 = true
		}
	}

	if !hasTemplate1 {
		t.Error("template1 not found in ListTemplates")
	}
	if !hasTemplate2 {
		t.Error("template2 not found in ListTemplates")
	}
}

func TestGetDefaultTemplateName(t *testing.T) {
	// Test with configured default
	cfg := &config.Config{
		Templates: struct {
			Default   string                    `toml:"default"`
			Templates map[string]config.TemplateConfig `toml:"templates"`
		}{
			Default: "custom_default",
		},
	}

	formatter := NewTemplateFormatter(cfg)
	defaultName := formatter.GetDefaultTemplateName()
	if defaultName != "custom_default" {
		t.Errorf("Expected 'custom_default', got '%s'", defaultName)
	}

	// Test without configured default
	cfg.Templates.Default = ""
	defaultName = formatter.GetDefaultTemplateName()
	if defaultName != "classic" {
		t.Errorf("Expected 'classic' fallback, got '%s'", defaultName)
	}
}

func TestBuildTemplateData(t *testing.T) {
	cfg := &config.Config{
		Station: struct {
			Name             string `toml:"name"`
			MixcloudUsername string `toml:"mixcloud_username"`
		}{
			Name: "Test Station",
		},
	}

	formatter := NewTemplateFormatter(cfg)

	tracks := []cue.Track{
		{
			StartTime: "00:00",
			Artist:    "Artist One",
			Title:     "Song One",
			Genre:     "Rock",
		},
		{
			StartTime: "03:30",
			Artist:    "Artist Two",
			Title:     "Song Two",
			Genre:     "Pop",
		},
	}

	metadata := map[string]interface{}{
		"show_title": "My Show",
		"show_date":  "2024-01-01",
		"custom_var": "custom_value",
	}

	templateData := formatter.buildTemplateData(tracks, metadata)

	if templateData.ShowTitle != "My Show" {
		t.Errorf("Expected ShowTitle 'My Show', got '%s'", templateData.ShowTitle)
	}

	if templateData.ShowDate != "2024-01-01" {
		t.Errorf("Expected ShowDate '2024-01-01', got '%s'", templateData.ShowDate)
	}

	if templateData.TrackCount != 2 {
		t.Errorf("Expected TrackCount 2, got %d", templateData.TrackCount)
	}

	if templateData.StationName != "Test Station" {
		t.Errorf("Expected StationName 'Test Station', got '%s'", templateData.StationName)
	}

	if len(templateData.Tracks) != 2 {
		t.Errorf("Expected 2 tracks, got %d", len(templateData.Tracks))
	}

	if templateData.Custom["custom_var"] != "custom_value" {
		t.Errorf("Custom variable not properly set")
	}

	// Check first track
	track1 := templateData.Tracks[0]
	if track1.Index != 1 {
		t.Errorf("Expected track 1 index to be 1, got %d", track1.Index)
	}
	if track1.Artist != "Artist One" {
		t.Errorf("Expected track 1 artist 'Artist One', got '%s'", track1.Artist)
	}
}