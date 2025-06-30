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