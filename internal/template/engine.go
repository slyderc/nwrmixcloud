// Package template provides a flexible templating system for tracklist formatting
// using Go's text/template package with custom functions and data structures.
package template

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"
	"time"

	"github.com/nowwaveradio/mixcloud-updater/internal/config"
	"github.com/nowwaveradio/mixcloud-updater/internal/cue"
	"github.com/nowwaveradio/mixcloud-updater/internal/filter"
)

// TemplateFormatter provides template-based tracklist formatting
type TemplateFormatter struct {
	templates map[string]*template.Template
	config    *config.Config
}

// TemplateData represents the data structure passed to templates for execution
type TemplateData struct {
	ShowTitle    string           `json:"show_title"`
	ShowDate     string           `json:"show_date"`
	TrackCount   int              `json:"track_count"`
	Tracks       []FormattedTrack `json:"tracks"`
	StationName  string           `json:"station_name"`
	Custom       map[string]interface{} `json:"custom"` // user-defined variables
}

// FormattedTrack represents a single track for template processing
type FormattedTrack struct {
	Index     int    `json:"index"`
	StartTime string `json:"start_time"`
	Artist    string `json:"artist"`
	Title     string `json:"title"`
	Genre     string `json:"genre"`
	Duration  string `json:"duration"`
}

// NewTemplateFormatter creates a new TemplateFormatter
func NewTemplateFormatter(cfg *config.Config) *TemplateFormatter {
	return &TemplateFormatter{
		templates: make(map[string]*template.Template),
		config:    cfg,
	}
}

// LoadTemplates parses template definitions from config and registers custom functions
func (tf *TemplateFormatter) LoadTemplates() error {
	if tf.config == nil {
		return fmt.Errorf("config is nil")
	}

	// Clear existing templates
	tf.templates = make(map[string]*template.Template)

	// Create custom function map
	funcMap := template.FuncMap{
		"repeat": strings.Repeat,
		"upper":  strings.ToUpper,
		"lower":  strings.ToLower,
		"title":  strings.Title,
		"truncate": func(s string, n int) string {
			if len(s) <= n {
				return s
			}
			return s[:n] + "..."
		},
		"printf": fmt.Sprintf,
		"join": func(sep string, items []string) string {
			return strings.Join(items, sep)
		},
		"add": func(a, b int) int {
			return a + b
		},
		"sub": func(a, b int) int {
			return a - b
		},
	}

	// Load each template from config
	for name, templateConfig := range tf.config.Templates.Templates {
		if err := tf.loadSingleTemplate(name, templateConfig, funcMap); err != nil {
			return fmt.Errorf("loading template %s: %w", name, err)
		}
	}

	return nil
}

// loadSingleTemplate loads and parses a single template configuration
func (tf *TemplateFormatter) loadSingleTemplate(name string, templateConfig config.TemplateConfig, funcMap template.FuncMap) error {
	// Create combined template text
	var templateText strings.Builder
	
	// Add header template
	if templateConfig.Header != "" {
		templateText.WriteString("{{define \"header\"}}")
		templateText.WriteString(templateConfig.Header)
		templateText.WriteString("{{end}}")
	}
	
	// Add track template (required)
	if templateConfig.Track == "" {
		return fmt.Errorf("track template is required")
	}
	templateText.WriteString("{{define \"track\"}}")
	templateText.WriteString(templateConfig.Track)
	templateText.WriteString("{{end}}")
	
	// Add footer template
	if templateConfig.Footer != "" {
		templateText.WriteString("{{define \"footer\"}}")
		templateText.WriteString(templateConfig.Footer)
		templateText.WriteString("{{end}}")
	}

	// Parse the combined template
	tmpl, err := template.New(name).Funcs(funcMap).Parse(templateText.String())
	if err != nil {
		return fmt.Errorf("parsing template: %w", err)
	}

	tf.templates[name] = tmpl
	return nil
}

// FormatWithTemplate executes a template with track data while respecting character limits
func (tf *TemplateFormatter) FormatWithTemplate(templateName string, tracks []cue.Track, fltr *filter.Filter, metadata map[string]interface{}) (string, error) {
	// Check if template exists
	tmpl, exists := tf.templates[templateName]
	if !exists {
		return "", fmt.Errorf("template %s not found", templateName)
	}

	// Build template data
	templateData := tf.buildTemplateData(tracks, metadata)

	var result strings.Builder

	// Execute header template if it exists
	if tmpl.Lookup("header") != nil {
		var headerBuf bytes.Buffer
		if err := tmpl.ExecuteTemplate(&headerBuf, "header", templateData); err != nil {
			return "", fmt.Errorf("executing header template: %w", err)
		}
		result.WriteString(headerBuf.String())
	}

	// Execute track template for each track with smart truncation
	trackTemplate := tmpl.Lookup("track")
	if trackTemplate == nil {
		return "", fmt.Errorf("track template not found")
	}

	const maxLength = 1000 // Mixcloud's character limit
	currentLength := result.Len()

	// Pre-calculate footer size to reserve space
	var footerOutput string
	footerLength := 0
	if footerTmpl := tmpl.Lookup("footer"); footerTmpl != nil {
		var footerBuf bytes.Buffer
		if err := footerTmpl.Execute(&footerBuf, templateData); err != nil {
			return "", fmt.Errorf("executing footer template: %w", err)
		}
		footerOutput = footerBuf.String()
		footerLength = len(footerOutput)
	}

	// Reserve space for footer and potential truncation message
	const truncationMargin = 50 // Space for "... and more tracks" type messages
	availableLength := maxLength - currentLength - footerLength - truncationMargin

	var trackOutputs []string
	totalTrackLength := 0

	// Generate all track outputs and calculate total length
	for _, track := range templateData.Tracks {
		var trackBuf bytes.Buffer
		if err := tmpl.ExecuteTemplate(&trackBuf, "track", track); err != nil {
			return "", fmt.Errorf("executing track template: %w", err)
		}

		trackOutput := trackBuf.String()
		
		// Check if adding this track would exceed available space
		if totalTrackLength+len(trackOutput) > availableLength {
			// Try smart truncation - find the last complete line
			if len(trackOutputs) > 0 {
				// Add truncation indicator if we had to skip tracks
				skippedCount := len(templateData.Tracks) - len(trackOutputs)
				if skippedCount > 0 {
					truncationMsg := fmt.Sprintf("... and %d more tracks\n", skippedCount)
					trackOutputs = append(trackOutputs, truncationMsg)
				}
			}
			break
		}

		trackOutputs = append(trackOutputs, trackOutput)
		totalTrackLength += len(trackOutput)
	}

	// Write all accepted track outputs
	for _, trackOutput := range trackOutputs {
		result.WriteString(trackOutput)
	}

	// Add footer if there's still space
	if footerOutput != "" && result.Len()+len(footerOutput) <= maxLength {
		result.WriteString(footerOutput)
	}

	return result.String(), nil
}

// buildTemplateData converts tracks and metadata into TemplateData structure
func (tf *TemplateFormatter) buildTemplateData(tracks []cue.Track, metadata map[string]interface{}) TemplateData {
	formattedTracks := make([]FormattedTrack, len(tracks))
	
	for i, track := range tracks {
		formattedTracks[i] = FormattedTrack{
			Index:     i + 1,
			StartTime: track.StartTime,
			Artist:    track.Artist,
			Title:     track.Title,
			Genre:     track.Genre,
			Duration:  "", // TODO: Calculate duration if available
		}
	}

	// Extract show title and date from metadata or use defaults
	showTitle := "Radio Show"
	if title, ok := metadata["show_title"].(string); ok && title != "" {
		showTitle = title
	}

	showDate := time.Now().Format("January 2, 2006")
	if date, ok := metadata["show_date"].(string); ok && date != "" {
		showDate = date
	}

	stationName := ""
	if tf.config != nil {
		stationName = tf.config.Station.Name
	}

	// Extract custom variables from metadata
	custom := make(map[string]interface{})
	if metadata != nil {
		for key, value := range metadata {
			if key != "show_title" && key != "show_date" {
				custom[key] = value
			}
		}
	}

	return TemplateData{
		ShowTitle:   showTitle,
		ShowDate:    showDate,
		TrackCount:  len(tracks),
		Tracks:      formattedTracks,
		StationName: stationName,
		Custom:      custom,
	}
}

// ValidateTemplate checks template syntax and required variables
func (tf *TemplateFormatter) ValidateTemplate(name string) error {
	tmpl, exists := tf.templates[name]
	if !exists {
		return fmt.Errorf("template %s not found", name)
	}

	// Create test data to validate template execution
	testData := TemplateData{
		ShowTitle:   "Test Show",
		ShowDate:    "Test Date",
		TrackCount:  1,
		StationName: "Test Station",
		Tracks: []FormattedTrack{
			{
				Index:     1,
				StartTime: "00:00",
				Artist:    "Test Artist",
				Title:     "Test Title",
				Genre:     "Test Genre",
				Duration:  "3:30",
			},
		},
		Custom: map[string]interface{}{
			"test": "value",
		},
	}

	// Try to execute each template component
	if headerTmpl := tmpl.Lookup("header"); headerTmpl != nil {
		var buf bytes.Buffer
		if err := headerTmpl.Execute(&buf, testData); err != nil {
			return fmt.Errorf("header template validation failed: %w", err)
		}
	}

	if trackTmpl := tmpl.Lookup("track"); trackTmpl != nil {
		var buf bytes.Buffer
		if err := trackTmpl.Execute(&buf, testData.Tracks[0]); err != nil {
			return fmt.Errorf("track template validation failed: %w", err)
		}
	} else {
		return fmt.Errorf("track template is required")
	}

	if footerTmpl := tmpl.Lookup("footer"); footerTmpl != nil {
		var buf bytes.Buffer
		if err := footerTmpl.Execute(&buf, testData); err != nil {
			return fmt.Errorf("footer template validation failed: %w", err)
		}
	}

	return nil
}

// ListTemplates returns the names of all loaded templates
func (tf *TemplateFormatter) ListTemplates() []string {
	names := make([]string, 0, len(tf.templates))
	for name := range tf.templates {
		names = append(names, name)
	}
	return names
}

// HasTemplate checks if a template with the given name exists
func (tf *TemplateFormatter) HasTemplate(name string) bool {
	_, exists := tf.templates[name]
	return exists
}

// GetDefaultTemplateName returns the configured default template name
func (tf *TemplateFormatter) GetDefaultTemplateName() string {
	if tf.config != nil && tf.config.Templates.Default != "" {
		return tf.config.Templates.Default
	}
	return "classic" // fallback to classic formatting
}