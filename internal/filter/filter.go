// Package filter provides content filtering functionality to exclude station IDs,
// advertisements, and other non-music content from track listings using string
// matching and regex patterns.
package filter

import (
	"fmt"
	"log"
	"regexp"
	"strings"

	"github.com/nowwaveradio/mixcloud-updater/internal/config"
	"github.com/nowwaveradio/mixcloud-updater/internal/cue"
)

// AIDEV-TODO: Implement string-based filtering for excluded artists/titles
// AIDEV-TODO: Add regex pattern matching capabilities
// AIDEV-TODO: Integrate with configuration for filter rules
// AIDEV-NOTE: Radio stations often have predictable naming patterns for IDs/ads

// Filter represents the content filtering system
type Filter struct {
	excludedArtists       []string         // Case-insensitive string matches for artists
	excludedTitles        []string         // Case-insensitive string matches for titles
	excludedArtistRegex   []*regexp.Regexp // Compiled regex patterns for artists
	excludedTitleRegex    []*regexp.Regexp // Compiled regex patterns for titles
}

// FilterStats holds statistics about filtering operations
type FilterStats struct {
	TracksProcessed int            // Total tracks processed
	TracksFiltered  int            // Total tracks filtered out
	FilterReasons   map[string]int // Count of each filter reason
}

// NewFilterStats creates a new FilterStats instance
func NewFilterStats() *FilterStats {
	return &FilterStats{
		FilterReasons: make(map[string]int),
	}
}

// FilterResult represents the result of filtering a track
type FilterResult struct {
	ShouldInclude bool   // Whether the track should be included
	Reason        string // Reason for filtering (if filtered)
	MatchedValue  string // The value that matched the filter
}

// AIDEV-NOTE: Common radio station patterns that typically need filtering
var defaultStationPatterns = []string{
	`(?i)station.?id`,           // Station ID variations
	`(?i)now.?wave.?radio`,      // Station name variations
	`(?i)sweepers?`,             // Station sweepers/jingles
	`(?i)bumpers?`,              // Station bumpers
	`(?i)commercial`,            // Commercial breaks
	`(?i)advertisement`,         // Advertisement content
	`(?i)\b(ad|ads)\b`,          // Simple ad references
	`(?i)promo(tion)?`,          // Station promos
	`(?i)ident(ification)?`,     // Station idents
}

// NewFilter creates a new Filter instance from configuration
// AIDEV-NOTE: Compiles regex patterns at initialization for better performance
func NewFilter(cfg *config.Config) (*Filter, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	filter := &Filter{
		excludedArtists: make([]string, 0),
		excludedTitles:  make([]string, 0),
	}

	// Process excluded artists (convert to lowercase for case-insensitive matching)
	for _, artist := range cfg.Filtering.ExcludedArtists {
		if strings.TrimSpace(artist) != "" {
			filter.excludedArtists = append(filter.excludedArtists, strings.ToLower(strings.TrimSpace(artist)))
		}
	}

	// Process excluded titles (convert to lowercase for case-insensitive matching)
	for _, title := range cfg.Filtering.ExcludedTitles {
		if strings.TrimSpace(title) != "" {
			filter.excludedTitles = append(filter.excludedTitles, strings.ToLower(strings.TrimSpace(title)))
		}
	}

	// Compile artist regex patterns
	filter.excludedArtistRegex = make([]*regexp.Regexp, 0)
	for _, pattern := range cfg.Filtering.ExcludedArtistPatterns {
		if strings.TrimSpace(pattern) != "" {
			compiled, err := regexp.Compile(pattern)
			if err != nil {
				return nil, fmt.Errorf("invalid artist regex pattern '%s': %w", pattern, err)
			}
			filter.excludedArtistRegex = append(filter.excludedArtistRegex, compiled)
		}
	}

	// Compile title regex patterns
	filter.excludedTitleRegex = make([]*regexp.Regexp, 0)
	for _, pattern := range cfg.Filtering.ExcludedTitlePatterns {
		if strings.TrimSpace(pattern) != "" {
			compiled, err := regexp.Compile(pattern)
			if err != nil {
				return nil, fmt.Errorf("invalid title regex pattern '%s': %w", pattern, err)
			}
			filter.excludedTitleRegex = append(filter.excludedTitleRegex, compiled)
		}
	}

	// Add default station patterns to artist regex if no custom patterns are provided
	// AIDEV-NOTE: This helps catch common radio station content automatically
	if len(filter.excludedArtistRegex) == 0 && len(cfg.Filtering.ExcludedArtistPatterns) == 0 {
		for _, pattern := range defaultStationPatterns {
			compiled, err := regexp.Compile(pattern)
			if err != nil {
				// AIDEV-NOTE: Skip invalid default patterns rather than failing
				continue
			}
			filter.excludedArtistRegex = append(filter.excludedArtistRegex, compiled)
		}
	}

	return filter, nil
}

// GetStats returns current filtering statistics
func (f *Filter) GetStats() *FilterStats {
	return &FilterStats{
		FilterReasons: make(map[string]int),
	}
}

// isExcludedByString checks if an artist or title is excluded by string matching
// AIDEV-NOTE: Uses case-insensitive exact matching for precision
func (f *Filter) isExcludedByString(artist, title string) (bool, string, string) {
	// Convert inputs to lowercase for comparison
	artistLower := strings.ToLower(strings.TrimSpace(artist))
	titleLower := strings.ToLower(strings.TrimSpace(title))

	// Check artist against excluded artists list
	for _, excluded := range f.excludedArtists {
		if artistLower == excluded {
			return true, "excluded_artist", excluded
		}
	}

	// Check title against excluded titles list
	for _, excluded := range f.excludedTitles {
		if titleLower == excluded {
			return true, "excluded_title", excluded
		}
	}

	return false, "", ""
}

// isExcludedByStringContains checks if an artist or title contains excluded strings
// AIDEV-NOTE: Uses substring matching for broader filtering capabilities
func (f *Filter) isExcludedByStringContains(artist, title string) (bool, string, string) {
	// Convert inputs to lowercase for comparison
	artistLower := strings.ToLower(strings.TrimSpace(artist))
	titleLower := strings.ToLower(strings.TrimSpace(title))

	// Check if artist contains any excluded artist strings
	for _, excluded := range f.excludedArtists {
		if strings.Contains(artistLower, excluded) {
			return true, "excluded_artist_contains", excluded
		}
	}

	// Check if title contains any excluded title strings
	for _, excluded := range f.excludedTitles {
		if strings.Contains(titleLower, excluded) {
			return true, "excluded_title_contains", excluded
		}
	}

	return false, "", ""
}

// containsAnyExcludedString checks if a string contains any excluded substring
// AIDEV-NOTE: Helper for checking if any part of artist/title matches exclusions
func containsAnyExcludedString(input string, exclusions []string) (bool, string) {
	inputLower := strings.ToLower(strings.TrimSpace(input))
	
	for _, excluded := range exclusions {
		if strings.Contains(inputLower, excluded) {
			return true, excluded
		}
	}
	
	return false, ""
}

// isExcludedByRegex checks if an artist or title matches any regex patterns
// AIDEV-NOTE: Uses compiled regex patterns for efficient matching
func (f *Filter) isExcludedByRegex(artist, title string) (bool, string, string) {
	// Check artist against excluded artist regex patterns
	if len(f.excludedArtistRegex) > 0 {
		for _, pattern := range f.excludedArtistRegex {
			if pattern != nil && pattern.MatchString(artist) {
				return true, "excluded_artist_regex", pattern.String()
			}
		}
	}

	// Check title against excluded title regex patterns  
	if len(f.excludedTitleRegex) > 0 {
		for _, pattern := range f.excludedTitleRegex {
			if pattern != nil && pattern.MatchString(title) {
				return true, "excluded_title_regex", pattern.String()
			}
		}
	}

	return false, "", ""
}

// matchesAnyRegexPattern checks if input matches any of the provided regex patterns
// AIDEV-NOTE: Helper for testing string against multiple regex patterns safely
func matchesAnyRegexPattern(input string, patterns []*regexp.Regexp) (bool, string) {
	if len(patterns) == 0 {
		return false, ""
	}

	for _, pattern := range patterns {
		if pattern != nil && pattern.MatchString(input) {
			return true, pattern.String()
		}
	}

	return false, ""
}

// isGenreExcluded checks if a track's genre should be filtered out
// AIDEV-NOTE: Special handling for genre-based filtering (e.g., "Sweepers")
func (f *Filter) isGenreExcluded(genre string) (bool, string) {
	if strings.TrimSpace(genre) == "" {
		return false, ""
	}

	genreLower := strings.ToLower(strings.TrimSpace(genre))
	
	// Check for common station content genres
	stationGenres := []string{
		"sweepers", "sweeper", "bumpers", "bumper", 
		"station id", "commercial", "advertisement", 
		"promo", "ident", "jingle",
	}
	
	for _, excluded := range stationGenres {
		if strings.Contains(genreLower, excluded) {
			return true, excluded
		}
	}
	
	return false, ""
}

// ShouldIncludeTrack determines if a track should be included in the final tracklist
// AIDEV-NOTE: Main entry point for filtering - combines all filtering methods
func (f *Filter) ShouldIncludeTrack(track *cue.Track) bool {
	// Handle nil track gracefully
	if track == nil {
		log.Printf("[FILTER] Skipping nil track")
		return false
	}

	// Handle empty track gracefully
	if track.IsEmpty() {
		log.Printf("[FILTER] Skipping empty track (index %d)", track.Index)
		return false
	}

	// Check genre-based exclusions first (most specific)
	if excluded, reason := f.isGenreExcluded(track.Genre); excluded {
		log.Printf("[FILTER] Excluding track %d (%s - %s): genre contains '%s'", 
			track.Index, track.Artist, track.Title, reason)
		return false
	}

	// Check string-based exclusions (exact matches)
	if excluded, filterType, matchedValue := f.isExcludedByString(track.Artist, track.Title); excluded {
		log.Printf("[FILTER] Excluding track %d (%s - %s): %s matched '%s'", 
			track.Index, track.Artist, track.Title, filterType, matchedValue)
		return false
	}

	// Check string-based exclusions (substring matches)
	if excluded, filterType, matchedValue := f.isExcludedByStringContains(track.Artist, track.Title); excluded {
		log.Printf("[FILTER] Excluding track %d (%s - %s): %s contains '%s'", 
			track.Index, track.Artist, track.Title, filterType, matchedValue)
		return false
	}

	// Check regex-based exclusions
	if excluded, filterType, pattern := f.isExcludedByRegex(track.Artist, track.Title); excluded {
		log.Printf("[FILTER] Excluding track %d (%s - %s): %s matched pattern '%s'", 
			track.Index, track.Artist, track.Title, filterType, pattern)
		return false
	}

	// Track passed all filters - include it
	return true
}

// FilterTrack returns detailed information about why a track was filtered
// AIDEV-NOTE: Alternative to ShouldIncludeTrack that provides detailed results
func (f *Filter) FilterTrack(track *cue.Track) FilterResult {
	// Handle nil track gracefully
	if track == nil {
		return FilterResult{
			ShouldInclude: false,
			Reason:        "nil_track",
			MatchedValue:  "",
		}
	}

	// Handle empty track gracefully
	if track.IsEmpty() {
		return FilterResult{
			ShouldInclude: false,
			Reason:        "empty_track",
			MatchedValue:  fmt.Sprintf("index_%d", track.Index),
		}
	}

	// Check genre-based exclusions first
	if excluded, reason := f.isGenreExcluded(track.Genre); excluded {
		return FilterResult{
			ShouldInclude: false,
			Reason:        "excluded_genre",
			MatchedValue:  reason,
		}
	}

	// Check string-based exclusions (exact matches)
	if excluded, filterType, matchedValue := f.isExcludedByString(track.Artist, track.Title); excluded {
		return FilterResult{
			ShouldInclude: false,
			Reason:        filterType,
			MatchedValue:  matchedValue,
		}
	}

	// Check string-based exclusions (substring matches)
	if excluded, filterType, matchedValue := f.isExcludedByStringContains(track.Artist, track.Title); excluded {
		return FilterResult{
			ShouldInclude: false,
			Reason:        filterType,
			MatchedValue:  matchedValue,
		}
	}

	// Check regex-based exclusions
	if excluded, filterType, pattern := f.isExcludedByRegex(track.Artist, track.Title); excluded {
		return FilterResult{
			ShouldInclude: false,
			Reason:        filterType,
			MatchedValue:  pattern,
		}
	}

	// Track passed all filters
	return FilterResult{
		ShouldInclude: true,
		Reason:        "",
		MatchedValue:  "",
	}
}