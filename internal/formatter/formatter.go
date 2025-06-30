// Package formatter provides tracklist formatting functionality to convert filtered
// CUE track data into properly formatted Mixcloud descriptions with smart truncation
// and character limit handling.
package formatter

import (
	"fmt"
	"strings"

	"github.com/nowwaveradio/mixcloud-updater/internal/config"
	"github.com/nowwaveradio/mixcloud-updater/internal/constants"
	"github.com/nowwaveradio/mixcloud-updater/internal/cue"
	"github.com/nowwaveradio/mixcloud-updater/internal/filter"
	"github.com/nowwaveradio/mixcloud-updater/internal/template"
)

// AIDEV-NOTE: Mixcloud has a 1000 character limit for descriptions
// AIDEV-TODO: Implement smart truncation that preserves complete track entries
// AIDEV-TODO: Add configurable formatting templates

// FormatterInterface defines the contract for tracklist formatting
type FormatterInterface interface {
	FormatTracklist(tracks []cue.Track, filter *filter.Filter) string
}

// Formatter handles conversion of filtered CUE tracks into formatted tracklists
type Formatter struct {
	maxLength         int                       // Character limit for Mixcloud descriptions
	templateFormatter *template.TemplateFormatter // Template-based formatter (nil if not configured)
	config           *config.Config            // Configuration for template access
}

// FormatOptions provides configuration for formatting behavior
type FormatOptions struct {
	MaxLength       int    // Maximum character limit (default: constants.MixcloudDescriptionLimit)
	TruncationText  string // Text to append when truncated (default: "... and more")
	LineFormat      string // Format template for each line (default: auto)
	IncludeNumbers  bool   // Whether to include track numbers
}

// DefaultFormatOptions returns the default formatting configuration
func DefaultFormatOptions() FormatOptions {
	return FormatOptions{
		MaxLength:      constants.MixcloudDescriptionLimit,
		TruncationText: "... and more",
		LineFormat:     "", // Will use default auto format
		IncludeNumbers: false,
	}
}

// NewFormatter creates a new Formatter instance with default settings
func NewFormatter() *Formatter {
	return &Formatter{
		maxLength: constants.MixcloudDescriptionLimit,
		config:    nil,
	}
}

// NewFormatterWithConfig creates a new Formatter instance with template support
func NewFormatterWithConfig(cfg *config.Config) *Formatter {
	formatter := &Formatter{
		maxLength: constants.MixcloudDescriptionLimit,
		config:    cfg,
	}
	
	// Initialize template formatter if templates are configured
	if cfg != nil && len(cfg.Templates.Config) > 0 {
		templateFormatter := template.NewTemplateFormatter(cfg)
		if err := templateFormatter.LoadTemplates(); err == nil {
			formatter.templateFormatter = templateFormatter
		}
		// If template loading fails, we'll fall back to classic formatting
	}
	
	return formatter
}

// NewFormatterWithOptions creates a new Formatter with custom options
func NewFormatterWithOptions(options FormatOptions) *Formatter {
	maxLen := options.MaxLength
	if maxLen <= 0 {
		maxLen = constants.MixcloudDescriptionLimit // Use default if invalid
	}
	
	return &Formatter{
		maxLength: maxLen,
	}
}

// GetMaxLength returns the current character limit setting
func (f *Formatter) GetMaxLength() int {
	return f.maxLength
}

// SetMaxLength updates the character limit (useful for testing)
func (f *Formatter) SetMaxLength(length int) {
	if length > 0 {
		f.maxLength = length
	}
}

// FormatTracklist converts filtered CUE tracks into a formatted tracklist string
// AIDEV-NOTE: Main entry point for formatting - applies filtering and builds tracklist
func (f *Formatter) FormatTracklist(tracks []cue.Track, trackFilter *filter.Filter) string {
	// Handle edge cases
	if tracks == nil || len(tracks) == 0 {
		return ""
	}
	
	// Check if template formatting is available and configured
	if f.templateFormatter != nil && f.config != nil {
		// Use template-based formatting with default template
		defaultTemplate := f.templateFormatter.GetDefaultTemplateName()
		if f.templateFormatter.HasTemplate(defaultTemplate) {
			// Apply filtering first
			filteredTracks := f.applyFilter(tracks, trackFilter)
			
			// Use template formatting
			result, err := f.templateFormatter.FormatWithTemplate(defaultTemplate, filteredTracks, trackFilter, nil)
			if err == nil {
				return result
			}
			// Fall through to classic formatting on error
		}
	}
	
	// Fall back to classic formatting
	return f.formatClassic(tracks, trackFilter)
}

// FormatTracklistWithTemplate formats tracks using a specific template
func (f *Formatter) FormatTracklistWithTemplate(tracks []cue.Track, trackFilter *filter.Filter, templateName string, metadata map[string]interface{}) string {
	// Handle edge cases
	if tracks == nil || len(tracks) == 0 {
		return ""
	}
	
	// Check if template formatting is available
	if f.templateFormatter == nil || !f.templateFormatter.HasTemplate(templateName) {
		// Fall back to classic formatting
		return f.formatClassic(tracks, trackFilter)
	}
	
	// Apply filtering first
	filteredTracks := f.applyFilter(tracks, trackFilter)
	
	// Use template formatting
	result, err := f.templateFormatter.FormatWithTemplate(templateName, filteredTracks, trackFilter, metadata)
	if err != nil {
		// Fall back to classic formatting on error
		return f.formatClassic(tracks, trackFilter)
	}
	
	return result
}

// FormatTracklistWithShowConfig formats tracks using template selection based on show configuration
func (f *Formatter) FormatTracklistWithShowConfig(tracks []cue.Track, trackFilter *filter.Filter, showCfg *config.ShowConfig, metadata map[string]interface{}) string {
	// Handle edge cases
	if tracks == nil || len(tracks) == 0 {
		return ""
	}
	
	// Check if template formatting is available
	if f.templateFormatter == nil {
		// Fall back to classic formatting
		return f.formatClassic(tracks, trackFilter)
	}
	
	// Apply filtering first
	filteredTracks := f.applyFilter(tracks, trackFilter)
	
	// Use show-specific template formatting
	result, err := f.templateFormatter.FormatWithShowConfig(filteredTracks, showCfg, metadata)
	if err != nil {
		// Fall back to classic formatting on error (including when "classic" is requested)
		return f.formatClassic(tracks, trackFilter)
	}
	
	return result
}

// applyFilter applies the filter to tracks and returns filtered results
func (f *Formatter) applyFilter(tracks []cue.Track, trackFilter *filter.Filter) []cue.Track {
	if trackFilter == nil {
		// No filter, return all non-empty tracks
		var result []cue.Track
		for _, track := range tracks {
			if !track.IsEmpty() {
				result = append(result, track)
			}
		}
		return result
	}
	
	// Apply filter
	var result []cue.Track
	for _, track := range tracks {
		if trackFilter.ShouldIncludeTrack(&track) && !track.IsEmpty() {
			result = append(result, track)
		}
	}
	return result
}

// formatClassic implements the original classic formatting logic
func (f *Formatter) formatClassic(tracks []cue.Track, trackFilter *filter.Filter) string {
	if trackFilter == nil {
		// If no filter provided, format all tracks
		return f.formatAllTracks(tracks)
	}

	// Apply filtering and build formatted lines
	var lines []string
	filteredCount := 0
	
	for _, track := range tracks {
		// Apply filter to determine if track should be included
		if trackFilter.ShouldIncludeTrack(&track) {
			// Format the track into a line
			line := f.formatTrackLine(&track)
			if line != "" { // Only add non-empty lines
				lines = append(lines, line)
				filteredCount++
			}
		}
	}
	
	// Join lines with newlines to create tracklist
	tracklist := strings.Join(lines, "\n")
	
	// Apply truncation if necessary
	if len(tracklist) > f.maxLength {
		tracklist = f.truncateSmartly(tracklist)
	}
	
	return tracklist
}

// formatAllTracks formats all tracks without filtering (helper method)
// AIDEV-NOTE: Used when no filter is provided
func (f *Formatter) formatAllTracks(tracks []cue.Track) string {
	var lines []string
	
	for _, track := range tracks {
		// Skip empty tracks
		if track.IsEmpty() {
			continue
		}
		
		line := f.formatTrackLine(&track)
		if line != "" {
			lines = append(lines, line)
		}
	}
	
	// Join with newlines
	tracklist := strings.Join(lines, "\n")
	
	// Apply truncation if necessary
	if len(tracklist) > f.maxLength {
		tracklist = f.truncateSmartly(tracklist)
	}
	
	return tracklist
}

// formatTrackLine formats a single track into the specified string format
// AIDEV-NOTE: Implements the format: MM:SS - "Track Title" by Artist Name
func (f *Formatter) formatTrackLine(track *cue.Track) string {
	if track == nil || track.IsEmpty() {
		return ""
	}

	// Handle missing or empty fields gracefully
	startTime := strings.TrimSpace(track.StartTime)
	title := strings.TrimSpace(track.Title)
	artist := strings.TrimSpace(track.Artist)

	// If no start time, use placeholder
	if startTime == "" {
		startTime = "00:00"
	}

	// Handle missing title
	if title == "" {
		title = "(Unknown Title)"
	}

	// Handle missing artist
	if artist == "" {
		artist = "(Unknown Artist)"
	}

	// Apply proper escaping for quotes in titles
	// AIDEV-NOTE: Escape existing quotes to prevent formatting issues
	escapedTitle := f.escapeQuotes(title)

	// Format: MM:SS - "Track Title" by Artist Name
	formatted := fmt.Sprintf(`%s - "%s" by %s`, startTime, escapedTitle, artist)

	return formatted
}

// escapeQuotes handles quote escaping in track titles
// AIDEV-NOTE: Prevents formatting issues when titles contain quotes
func (f *Formatter) escapeQuotes(text string) string {
	// Replace any existing double quotes with escaped quotes or remove them
	// For simplicity, we'll replace them with single quotes to maintain readability
	escaped := strings.ReplaceAll(text, `"`, `'`)
	return escaped
}

// GetFormattedTrackCount returns the number of tracks that would be included after filtering
// AIDEV-NOTE: Useful for statistics and validation
func (f *Formatter) GetFormattedTrackCount(tracks []cue.Track, trackFilter *filter.Filter) int {
	if tracks == nil || len(tracks) == 0 {
		return 0
	}

	if trackFilter == nil {
		// Count non-empty tracks
		count := 0
		for _, track := range tracks {
			if !track.IsEmpty() {
				count++
			}
		}
		return count
	}

	// Count filtered tracks
	count := 0
	for _, track := range tracks {
		if trackFilter.ShouldIncludeTrack(&track) && !track.IsEmpty() {
			count++
		}
	}

	return count
}

// truncateSmartly truncates a tracklist at line boundaries while preserving formatting
// AIDEV-NOTE: Implements smart truncation that cuts at complete track entries, not mid-line
func (f *Formatter) truncateSmartly(tracklist string) string {
	if len(tracklist) <= f.maxLength {
		return tracklist // No truncation needed
	}

	// Default truncation text
	truncationText := "... and more"
	
	// Account for the truncation text in our length calculation
	// We need room for the truncation text plus a newline
	availableLength := f.maxLength - len(truncationText) - 1 // -1 for newline
	
	// Handle edge case where truncation text itself is too long
	if availableLength <= 0 {
		// If even the truncation text won't fit, just return a simple truncated version
		if f.maxLength <= len(truncationText) {
			return truncationText[:f.maxLength]
		}
		return truncationText
	}

	// Split tracklist into lines
	lines := strings.Split(tracklist, "\n")
	if len(lines) == 0 {
		return ""
	}

	// Handle case where even the first line is too long
	if len(lines[0]) > availableLength {
		// If the first track line itself exceeds the available length,
		// we need to decide whether to show a partial track or just the truncation text
		// For formatting integrity, we'll show just the truncation text
		return truncationText
	}

	// Build the result by adding complete lines until we run out of space
	var result strings.Builder
	totalLength := 0
	
	for i, line := range lines {
		// Calculate length if we add this line
		newLength := totalLength
		if i > 0 {
			newLength += 1 // Add 1 for the newline character
		}
		newLength += len(line)
		
		// Check if adding this line would exceed our available length
		if newLength > availableLength {
			// Can't fit this line, stop here
			break
		}
		
		// Add the line
		if i > 0 {
			result.WriteString("\n")
		}
		result.WriteString(line)
		totalLength = newLength
	}
	
	// Add truncation text
	if result.Len() > 0 {
		result.WriteString("\n")
	}
	result.WriteString(truncationText)
	
	return result.String()
}

// EstimateTracklistLength estimates the final tracklist length without formatting
// AIDEV-NOTE: Useful for planning and validation before full formatting
func (f *Formatter) EstimateTracklistLength(tracks []cue.Track, trackFilter *filter.Filter) int {
	if tracks == nil || len(tracks) == 0 {
		return 0
	}

	totalLength := 0
	trackCount := 0

	for _, track := range tracks {
		// Check if track would be included
		shouldInclude := trackFilter == nil || trackFilter.ShouldIncludeTrack(&track)
		if !shouldInclude || track.IsEmpty() {
			continue
		}

		// Estimate line length
		// Format: MM:SS - "Track Title" by Artist Name
		startTime := track.StartTime
		if startTime == "" {
			startTime = "00:00"
		}
		
		title := track.Title
		if title == "" {
			title = "(Unknown Title)"
		}
		
		artist := track.Artist
		if artist == "" {
			artist = "(Unknown Artist)"
		}

		// Estimate: MM:SS - "Title" by Artist
		// 5 (time) + 3 (" - ") + 1 (") + len(title) + 1 (") + 4 (" by ") + len(artist)
		lineLength := 5 + 3 + 1 + len(title) + 1 + 4 + len(artist)
		
		if trackCount > 0 {
			totalLength += 1 // newline character
		}
		totalLength += lineLength
		trackCount++
	}

	return totalLength
}

// HasTemplateSupport returns true if the formatter has template support enabled
func (f *Formatter) HasTemplateSupport() bool {
	return f.templateFormatter != nil
}

// ListAvailableTemplates returns the names of all available templates
func (f *Formatter) ListAvailableTemplates() []string {
	if f.templateFormatter == nil {
		return []string{}
	}
	return f.templateFormatter.ListTemplates()
}

// HasTemplate checks if a specific template is available
func (f *Formatter) HasTemplate(templateName string) bool {
	if f.templateFormatter == nil {
		return false
	}
	return f.templateFormatter.HasTemplate(templateName)
}

// GetDefaultTemplateName returns the configured default template name
func (f *Formatter) GetDefaultTemplateName() string {
	if f.templateFormatter == nil {
		return "classic"
	}
	return f.templateFormatter.GetDefaultTemplateName()
}

// ValidateTemplate validates a template's syntax and execution
func (f *Formatter) ValidateTemplate(templateName string) error {
	if f.templateFormatter == nil {
		return fmt.Errorf("template support not enabled")
	}
	return f.templateFormatter.ValidateTemplate(templateName)
}

// SelectTemplateForShow determines which template to use for a given show configuration
func (f *Formatter) SelectTemplateForShow(showCfg *config.ShowConfig) (string, error) {
	if f.templateFormatter == nil {
		return "classic", nil
	}
	return f.templateFormatter.SelectTemplateForShow(showCfg)
}

// GetTemplateInfo returns information about a loaded template
func (f *Formatter) GetTemplateInfo(templateName string) (map[string]bool, error) {
	if f.templateFormatter == nil {
		return nil, fmt.Errorf("template support not enabled")
	}
	return f.templateFormatter.GetTemplateInfo(templateName)
}

// LoadCustomTemplate loads an inline custom template
func (f *Formatter) LoadCustomTemplate(name string, customTemplate string) error {
	if f.templateFormatter == nil {
		return fmt.Errorf("template support not enabled")
	}
	return f.templateFormatter.LoadCustomTemplate(name, customTemplate)
}