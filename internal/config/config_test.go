package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTemplateConfigParsing(t *testing.T) {
	tests := []struct {
		name     string
		tomlData string
		expected struct {
			Default   string
			Templates map[string]TemplateConfig
		}
	}{
		{
			name: "basic template configuration",
			tomlData: `
[templates]
default = "minimal"

[templates.templates.minimal]
header = "Tracklist:\n"
track = "{{.StartTime}} - {{.Title}} by {{.Artist}}\n"
footer = "\nTotal tracks: {{.TrackCount}}"

[templates.templates.detailed]
header = "{{.ShowTitle}} - {{.ShowDate}}\n{{repeat \"-\" 50}}\n"
track = "{{printf \"%02d\" .Index}}. [{{.StartTime}}] {{.Artist}} - {{.Title}}{{if .Genre}} ({{.Genre}}){{end}}\n"
footer = "\n{{repeat \"-\" 50}}\nBroadcast by {{.StationName}} | {{.TrackCount}} tracks played"
`,
			expected: struct {
				Default   string
				Templates map[string]TemplateConfig
			}{
				Default: "minimal",
				Templates: map[string]TemplateConfig{
					"minimal": {
						Header: "Tracklist:\n",
						Track:  "{{.StartTime}} - {{.Title}} by {{.Artist}}\n",
						Footer: "\nTotal tracks: {{.TrackCount}}",
					},
					"detailed": {
						Header: "{{.ShowTitle}} - {{.ShowDate}}\n{{repeat \"-\" 50}}\n",
						Track:  "{{printf \"%02d\" .Index}}. [{{.StartTime}}] {{.Artist}} - {{.Title}}{{if .Genre}} ({{.Genre}}){{end}}\n",
						Footer: "\n{{repeat \"-\" 50}}\nBroadcast by {{.StationName}} | {{.TrackCount}} tracks played",
					},
				},
			},
		},
		{
			name: "template with missing sections",
			tomlData: `
[templates]
default = "simple"

[templates.templates.simple]
track = "{{.StartTime}} - {{.Title}} by {{.Artist}}\n"
`,
			expected: struct {
				Default   string
				Templates map[string]TemplateConfig
			}{
				Default: "simple",
				Templates: map[string]TemplateConfig{
					"simple": {
						Header: "",
						Track:  "{{.StartTime}} - {{.Title}} by {{.Artist}}\n",
						Footer: "",
					},
				},
			},
		},
		{
			name: "no templates section",
			tomlData: `
[station]
name = "Test Station"
`,
			expected: struct {
				Default   string
				Templates map[string]TemplateConfig
			}{
				Default:   "classic",
				Templates: map[string]TemplateConfig{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary config file
			tmpFile := createTempConfigFile(t, tt.tomlData)
			defer os.Remove(tmpFile)

			// Load config
			config, err := LoadConfig(tmpFile)
			if err != nil {
				t.Fatalf("LoadConfig() error = %v", err)
			}

			// Verify template configuration
			if config.Templates.Default != tt.expected.Default {
				t.Errorf("Templates.Default = %v, want %v", config.Templates.Default, tt.expected.Default)
			}

			if len(config.Templates.Templates) != len(tt.expected.Templates) {
				t.Errorf("len(Templates.Templates) = %v, want %v", len(config.Templates.Templates), len(tt.expected.Templates))
			}

			for name, expectedTemplate := range tt.expected.Templates {
				actualTemplate, exists := config.Templates.Templates[name]
				if !exists {
					t.Errorf("Template %s not found in config", name)
					continue
				}

				if actualTemplate.Header != expectedTemplate.Header {
					t.Errorf("Template %s Header = %v, want %v", name, actualTemplate.Header, expectedTemplate.Header)
				}
				if actualTemplate.Track != expectedTemplate.Track {
					t.Errorf("Template %s Track = %v, want %v", name, actualTemplate.Track, expectedTemplate.Track)
				}
				if actualTemplate.Footer != expectedTemplate.Footer {
					t.Errorf("Template %s Footer = %v, want %v", name, actualTemplate.Footer, expectedTemplate.Footer)
				}
			}
		})
	}
}

func TestDefaultConfigTemplates(t *testing.T) {
	config := DefaultConfig()
	
	if config.Templates.Default != "classic" {
		t.Errorf("Default Templates.Default = %v, want 'classic'", config.Templates.Default)
	}
	
	if config.Templates.Templates == nil {
		t.Error("Default Templates.Templates should not be nil")
	}
	
	if len(config.Templates.Templates) != 0 {
		t.Errorf("Default Templates.Templates should be empty, got %d templates", len(config.Templates.Templates))
	}
}

func TestMergeTemplatesWithDefaults(t *testing.T) {
	defaults := DefaultConfig()
	
	loaded := &Config{
		Templates: struct {
			Default   string                    `toml:"default"`
			Templates map[string]TemplateConfig `toml:"templates"`
		}{
			Default: "custom",
			Templates: map[string]TemplateConfig{
				"custom": {
					Header: "Custom Header",
					Track:  "Custom Track: {{.Title}}",
					Footer: "Custom Footer",
				},
			},
		},
	}
	
	result := mergeWithDefaults(loaded, defaults)
	
	if result.Templates.Default != "custom" {
		t.Errorf("Merged Templates.Default = %v, want 'custom'", result.Templates.Default)
	}
	
	if len(result.Templates.Templates) != 1 {
		t.Errorf("Merged Templates.Templates should have 1 template, got %d", len(result.Templates.Templates))
	}
	
	customTemplate, exists := result.Templates.Templates["custom"]
	if !exists {
		t.Error("Custom template should exist after merge")
	} else {
		if customTemplate.Header != "Custom Header" {
			t.Errorf("Custom template Header = %v, want 'Custom Header'", customTemplate.Header)
		}
		if customTemplate.Track != "Custom Track: {{.Title}}" {
			t.Errorf("Custom template Track = %v, want 'Custom Track: {{.Title}}'", customTemplate.Track)
		}
		if customTemplate.Footer != "Custom Footer" {
			t.Errorf("Custom template Footer = %v, want 'Custom Footer'", customTemplate.Footer)
		}
	}
}

func TestShowConfigParsing(t *testing.T) {
	tests := []struct {
		name     string
		tomlData string
		expected map[string]ShowConfig
	}{
		{
			name: "complete show configuration",
			tomlData: `
[shows.newer-new-wave]
cue_file_pattern = "MYR4*.cue"
show_name_pattern = "The Newer New Wave Show - {date}"
aliases = ["nnw", "new-wave"]
template = "default"
date_extraction = "MYR4(\\d{4})"
date_format = "01/02/2006"
enabled = true
priority = 1

[shows.sounds-like]
cue_file_mapping = "latest-sounds-like.cue"
show_name_pattern = "Sounds Like - Jeri-Rig & V-Dub - {date}"
aliases = ["sl", "sounds"]
custom_template = "Morning Show Playlist:\n{{range .Tracks}}{{.Time}} {{.Title}}{{end}}"
enabled = true
priority = 2
`,
			expected: map[string]ShowConfig{
				"newer-new-wave": {
					CueFilePattern:  "MYR4*.cue",
					ShowNamePattern: "The Newer New Wave Show - {date}",
					Aliases:         []string{"nnw", "new-wave"},
					TemplateName:    "default",
					DateExtraction:  "MYR4(\\d{4})",
					DateFormat:      "01/02/2006",
					Enabled:         true,
					Priority:        1,
				},
				"sounds-like": {
					CueFileMapping:  "latest-sounds-like.cue",
					ShowNamePattern: "Sounds Like - Jeri-Rig & V-Dub - {date}",
					Aliases:         []string{"sl", "sounds"},
					CustomTemplate:  "Morning Show Playlist:\n{{range .Tracks}}{{.Time}} {{.Title}}{{end}}",
					Enabled:         true,
					Priority:        2,
				},
			},
		},
		{
			name: "minimal show configuration",
			tomlData: `
[shows.simple-show]
show_name_pattern = "Simple Show"
enabled = true
`,
			expected: map[string]ShowConfig{
				"simple-show": {
					ShowNamePattern: "Simple Show",
					Enabled:         true,
					Priority:        0, // Default value
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpFile := createTempConfigFile(t, tt.tomlData)
			defer os.Remove(tmpFile)

			config, err := LoadConfig(tmpFile)
			if err != nil {
				t.Fatalf("LoadConfig() error = %v", err)
			}

			if len(config.Shows) != len(tt.expected) {
				t.Errorf("len(Shows) = %v, want %v", len(config.Shows), len(tt.expected))
			}

			for name, expectedShow := range tt.expected {
				actualShow, exists := config.Shows[name]
				if !exists {
					t.Errorf("Show %s not found in config", name)
					continue
				}

				if actualShow.CueFilePattern != expectedShow.CueFilePattern {
					t.Errorf("Show %s CueFilePattern = %v, want %v", name, actualShow.CueFilePattern, expectedShow.CueFilePattern)
				}
				if actualShow.CueFileMapping != expectedShow.CueFileMapping {
					t.Errorf("Show %s CueFileMapping = %v, want %v", name, actualShow.CueFileMapping, expectedShow.CueFileMapping)
				}
				if actualShow.ShowNamePattern != expectedShow.ShowNamePattern {
					t.Errorf("Show %s ShowNamePattern = %v, want %v", name, actualShow.ShowNamePattern, expectedShow.ShowNamePattern)
				}
				if len(actualShow.Aliases) != len(expectedShow.Aliases) {
					t.Errorf("Show %s Aliases length = %v, want %v", name, len(actualShow.Aliases), len(expectedShow.Aliases))
				}
				if actualShow.TemplateName != expectedShow.TemplateName {
					t.Errorf("Show %s TemplateName = %v, want %v", name, actualShow.TemplateName, expectedShow.TemplateName)
				}
				if actualShow.CustomTemplate != expectedShow.CustomTemplate {
					t.Errorf("Show %s CustomTemplate = %v, want %v", name, actualShow.CustomTemplate, expectedShow.CustomTemplate)
				}
				if actualShow.DateExtraction != expectedShow.DateExtraction {
					t.Errorf("Show %s DateExtraction = %v, want %v", name, actualShow.DateExtraction, expectedShow.DateExtraction)
				}
				if actualShow.DateFormat != expectedShow.DateFormat {
					t.Errorf("Show %s DateFormat = %v, want %v", name, actualShow.DateFormat, expectedShow.DateFormat)
				}
				if actualShow.Enabled != expectedShow.Enabled {
					t.Errorf("Show %s Enabled = %v, want %v", name, actualShow.Enabled, expectedShow.Enabled)
				}
				if actualShow.Priority != expectedShow.Priority {
					t.Errorf("Show %s Priority = %v, want %v", name, actualShow.Priority, expectedShow.Priority)
				}
			}
		})
	}
}

func TestProcessingConfigParsing(t *testing.T) {
	tests := []struct {
		name     string
		tomlData string
		expected struct {
			CueFileDirectory string
			AutoProcess      bool
			BatchSize        int
		}
	}{
		{
			name: "complete processing configuration",
			tomlData: `
[processing]
cue_file_directory = "` + filepath.Join("radio", "playout", "logs") + `"
auto_process = true
batch_size = 10
`,
			expected: struct {
				CueFileDirectory string
				AutoProcess      bool
				BatchSize        int
			}{
				CueFileDirectory: filepath.Join("radio", "playout", "logs"),
				AutoProcess:      true,
				BatchSize:        10,
			},
		},
		{
			name: "minimal processing configuration",
			tomlData: `
[processing]
cue_file_directory = "` + filepath.Join("path", "to", "cue") + `"
`,
			expected: struct {
				CueFileDirectory string
				AutoProcess      bool
				BatchSize        int
			}{
				CueFileDirectory: filepath.Join("path", "to", "cue"),
				AutoProcess:      false, // Default
				BatchSize:        5,     // Default
			},
		},
		{
			name: "no processing section",
			tomlData: `
[station]
name = "Test Station"
`,
			expected: struct {
				CueFileDirectory string
				AutoProcess      bool
				BatchSize        int
			}{
				CueFileDirectory: ".", // Default
				AutoProcess:      false,
				BatchSize:        5,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpFile := createTempConfigFile(t, tt.tomlData)
			defer os.Remove(tmpFile)

			config, err := LoadConfig(tmpFile)
			if err != nil {
				t.Fatalf("LoadConfig() error = %v", err)
			}

			if config.Processing.CueFileDirectory != tt.expected.CueFileDirectory {
				t.Errorf("Processing.CueFileDirectory = %v, want %v", config.Processing.CueFileDirectory, tt.expected.CueFileDirectory)
			}
			if config.Processing.AutoProcess != tt.expected.AutoProcess {
				t.Errorf("Processing.AutoProcess = %v, want %v", config.Processing.AutoProcess, tt.expected.AutoProcess)
			}
			if config.Processing.BatchSize != tt.expected.BatchSize {
				t.Errorf("Processing.BatchSize = %v, want %v", config.Processing.BatchSize, tt.expected.BatchSize)
			}
		})
	}
}

func TestUnifiedConfigParsing(t *testing.T) {
	tomlData := `
[station]
name = "Example FM"
mixcloud_username = "examplefm"

[oauth]
client_id = "test-client-id"
client_secret = "test-client-secret"

[processing]
cue_file_directory = "` + filepath.Join("radio", "playout", "logs") + `"
auto_process = true
batch_size = 5

[templates.templates.default]
header = "Today's tracklist:"
track = "{{.Time}} - {{.Title}} by {{.Artist}}"
footer = "Thanks for listening!"

[shows.newer-new-wave]
cue_file_pattern = "MYR4*.cue"
show_name_pattern = "The Newer New Wave Show - {date}"
aliases = ["nnw", "new-wave"]
template = "default"
date_extraction = "MYR4(\\d{4})"
date_format = "01/02/2006"
enabled = true
priority = 1

[shows.sounds-like]
cue_file_mapping = "latest-sounds-like.cue"
show_name_pattern = "Sounds Like - Jeri-Rig & V-Dub - {date}"
aliases = ["sl", "sounds"]
custom_template = "Morning Show Playlist:\\n{{range .Tracks}}{{.Time}} {{.Title}}{{end}}"
enabled = true
priority = 2
`

	tmpFile := createTempConfigFile(t, tomlData)
	defer os.Remove(tmpFile)

	config, err := LoadConfig(tmpFile)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	// Verify all sections are loaded correctly
	if config.Station.Name != "Example FM" {
		t.Errorf("Station.Name = %v, want 'Example FM'", config.Station.Name)
	}

	expectedPath := filepath.Join("radio", "playout", "logs")
	if config.Processing.CueFileDirectory != expectedPath {
		t.Errorf("Processing.CueFileDirectory = %v, want %v", config.Processing.CueFileDirectory, expectedPath)
	}

	if len(config.Shows) != 2 {
		t.Errorf("len(Shows) = %v, want 2", len(config.Shows))
	}

	if len(config.Templates.Templates) != 1 {
		t.Errorf("len(Templates.Templates) = %v, want 1", len(config.Templates.Templates))
	}
}

func TestDefaultConfigShowsAndProcessing(t *testing.T) {
	config := DefaultConfig()

	if config.Shows == nil {
		t.Error("Default Shows should not be nil")
	}

	if len(config.Shows) != 0 {
		t.Errorf("Default Shows should be empty, got %d shows", len(config.Shows))
	}

	if config.Processing.CueFileDirectory != "." {
		t.Errorf("Default Processing.CueFileDirectory = %v, want '.'", config.Processing.CueFileDirectory)
	}

	if config.Processing.AutoProcess != false {
		t.Errorf("Default Processing.AutoProcess = %v, want false", config.Processing.AutoProcess)
	}

	if config.Processing.BatchSize != 5 {
		t.Errorf("Default Processing.BatchSize = %v, want 5", config.Processing.BatchSize)
	}
}

func TestEnvironmentOverridesProcessing(t *testing.T) {
	// Set environment variables
	customPath := filepath.Join("custom", "path")
	os.Setenv("NWRMIXCLOUD_PROCESSING_CUE_FILE_DIRECTORY", customPath)
	os.Setenv("NWRMIXCLOUD_PROCESSING_AUTO_PROCESS", "true")
	defer func() {
		os.Unsetenv("NWRMIXCLOUD_PROCESSING_CUE_FILE_DIRECTORY")
		os.Unsetenv("NWRMIXCLOUD_PROCESSING_AUTO_PROCESS")
	}()

	config := DefaultConfig()
	config.ApplyEnvironmentOverrides()

	if config.Processing.CueFileDirectory != customPath {
		t.Errorf("Processing.CueFileDirectory = %v, want %v", config.Processing.CueFileDirectory, customPath)
	}

	if config.Processing.AutoProcess != true {
		t.Errorf("Processing.AutoProcess = %v, want true", config.Processing.AutoProcess)
	}
}

// Helper function to create temporary config files for testing
func createTempConfigFile(t *testing.T, content string) string {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "config.toml")
	
	err := os.WriteFile(tmpFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create temp config file: %v", err)
	}
	
	return tmpFile
}