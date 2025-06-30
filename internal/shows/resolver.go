// Package shows provides show resolution and lookup functionality for the
// unified config-driven architecture. It handles alias resolution and 
// show configuration management.
package shows

import (
	"fmt"
	"strings"

	"github.com/nowwaveradio/mixcloud-updater/internal/config"
)

// Resolver handles show alias resolution and lookup operations
type Resolver struct {
	config    *config.Config
	aliasMap  map[string]string // Maps aliases and names to show keys
	showKeys  []string          // Ordered list of show keys for iteration
}

// NewResolver creates a new show resolver from the given configuration
func NewResolver(cfg *config.Config) (*Resolver, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	resolver := &Resolver{
		config:   cfg,
		aliasMap: make(map[string]string),
		showKeys: make([]string, 0, len(cfg.Shows)),
	}

	// Build alias mapping and validate for conflicts
	if err := resolver.buildAliasMap(); err != nil {
		return nil, fmt.Errorf("building alias map: %w", err)
	}

	return resolver, nil
}

// buildAliasMap constructs the alias-to-showkey mapping and validates for conflicts
func (r *Resolver) buildAliasMap() error {
	conflictCheck := make(map[string][]string) // Track which shows claim each alias

	for showKey, showConfig := range r.config.Shows {
		r.showKeys = append(r.showKeys, showKey)
		
		// Add primary show key (case-insensitive)
		normalizedKey := strings.ToLower(showKey)
		r.aliasMap[normalizedKey] = showKey
		conflictCheck[normalizedKey] = append(conflictCheck[normalizedKey], showKey)

		// Add each alias (case-insensitive)
		for _, alias := range showConfig.Aliases {
			normalizedAlias := strings.ToLower(alias)
			r.aliasMap[normalizedAlias] = showKey
			conflictCheck[normalizedAlias] = append(conflictCheck[normalizedAlias], showKey)
		}
	}

	// Check for conflicts (same alias used by multiple shows)
	for alias, shows := range conflictCheck {
		if len(shows) > 1 {
			return fmt.Errorf("alias conflict: '%s' is used by multiple shows: %v", alias, shows)
		}
	}

	return nil
}

// FindShowConfig resolves a show name or alias to its configuration
// Returns nil if the show is not found
func (r *Resolver) FindShowConfig(nameOrAlias string) *config.ShowConfig {
	// Handle empty input
	if strings.TrimSpace(nameOrAlias) == "" {
		return nil
	}

	// Normalize the input for case-insensitive matching
	normalized := strings.ToLower(strings.TrimSpace(nameOrAlias))
	
	// Look up in alias map
	if showKey, exists := r.aliasMap[normalized]; exists {
		if showConfig, found := r.config.Shows[showKey]; found {
			return &showConfig
		}
	}

	return nil
}

// FindShowKey resolves a show name or alias to its primary key
// Returns empty string if the show is not found
func (r *Resolver) FindShowKey(nameOrAlias string) string {
	// Handle empty input
	if strings.TrimSpace(nameOrAlias) == "" {
		return ""
	}

	// Normalize the input for case-insensitive matching
	normalized := strings.ToLower(strings.TrimSpace(nameOrAlias))
	
	// Look up in alias map
	if showKey, exists := r.aliasMap[normalized]; exists {
		return showKey
	}

	return ""
}

// ListShows returns all show keys in the configuration
func (r *Resolver) ListShows() []string {
	// Return a copy to prevent external modification
	result := make([]string, len(r.showKeys))
	copy(result, r.showKeys)
	return result
}

// ListEnabledShows returns all enabled show keys, optionally sorted by priority
func (r *Resolver) ListEnabledShows(sortByPriority bool) []string {
	var enabled []string
	
	for _, showKey := range r.showKeys {
		if showConfig, exists := r.config.Shows[showKey]; exists && showConfig.Enabled {
			enabled = append(enabled, showKey)
		}
	}

	if sortByPriority {
		enabled = r.sortByPriority(enabled)
	}

	return enabled
}

// sortByPriority sorts show keys by their priority (higher priority first)
func (r *Resolver) sortByPriority(showKeys []string) []string {
	// Create a copy and sort it
	sorted := make([]string, len(showKeys))
	copy(sorted, showKeys)

	// Simple bubble sort by priority (higher first)
	for i := 0; i < len(sorted)-1; i++ {
		for j := 0; j < len(sorted)-i-1; j++ {
			config1 := r.config.Shows[sorted[j]]
			config2 := r.config.Shows[sorted[j+1]]
			
			// Higher priority should come first
			if config1.Priority < config2.Priority {
				sorted[j], sorted[j+1] = sorted[j+1], sorted[j]
			}
		}
	}

	return sorted
}

// GetShowAliases returns all aliases for a given show key
func (r *Resolver) GetShowAliases(showKey string) []string {
	showConfig, exists := r.config.Shows[showKey]
	if !exists {
		return []string{}
	}

	// Return a copy to prevent external modification
	aliases := make([]string, len(showConfig.Aliases))
	copy(aliases, showConfig.Aliases)
	return aliases
}

// ValidateShows performs validation on all show configurations
func (r *Resolver) ValidateShows() error {
	var errors []string

	for showKey, showConfig := range r.config.Shows {
		// Validate that at least one CUE file source is configured
		if showConfig.CueFilePattern == "" && showConfig.CueFileMapping == "" {
			errors = append(errors, fmt.Sprintf("show '%s': no CUE file source configured (cue_file_pattern or cue_file_mapping required)", showKey))
		}

		// Validate that show name pattern is provided
		if strings.TrimSpace(showConfig.ShowNamePattern) == "" {
			errors = append(errors, fmt.Sprintf("show '%s': show_name_pattern is required", showKey))
		}

		// Validate priority is non-negative
		if showConfig.Priority < 0 {
			errors = append(errors, fmt.Sprintf("show '%s': priority must be non-negative, got %d", showKey, showConfig.Priority))
		}

		// Validate template configuration
		if showConfig.TemplateName != "" && showConfig.CustomTemplate != "" {
			errors = append(errors, fmt.Sprintf("show '%s': cannot specify both template and custom_template", showKey))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("show validation failed: %s", strings.Join(errors, "; "))
	}

	return nil
}