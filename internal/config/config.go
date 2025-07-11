// Package config provides configuration management for the Mixcloud updater.
// It handles loading TOML configuration files, validating settings, and managing
// OAuth credentials and filtering rules.
package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
	
	"github.com/BurntSushi/toml"
	"github.com/nowwaveradio/mixcloud-updater/internal/constants"
	"github.com/nowwaveradio/mixcloud-updater/internal/errorutil"
	"github.com/nowwaveradio/mixcloud-updater/internal/logger"
)

// AIDEV-TODO: Implement TOML parsing with BurntSushi/toml library
// AIDEV-TODO: Add validation for required fields (OAuth keys, station settings)
// AIDEV-TODO: Support environment variable overrides for sensitive data

// Config represents the main configuration structure
type Config struct {
	Station struct {
		Name             string `toml:"name"`
		MixcloudUsername string `toml:"mixcloud_username"`
	} `toml:"station"`
	
	OAuth struct {
		ClientID     string `toml:"client_id"`
		ClientSecret string `toml:"client_secret"`
		AccessToken  string `toml:"access_token"`
		RefreshToken string `toml:"refresh_token"`
	} `toml:"oauth"`
	
	Filtering struct {
		ExcludedArtists       []string `toml:"excluded_artists"`
		ExcludedTitles        []string `toml:"excluded_titles"`
		ExcludedArtistPatterns []string `toml:"excluded_artist_patterns"`
		ExcludedTitlePatterns  []string `toml:"excluded_title_patterns"`
	} `toml:"filtering"`
	
	Paths struct {
		CueFileDirectory string `toml:"cue_file_directory"`
	} `toml:"paths"`
	
	Templates struct {
		Default string                    `toml:"default"`
		Config  map[string]TemplateConfig `toml:"config"`
	} `toml:"templates"`
	
	Shows map[string]ShowConfig `toml:"shows"`
	
	Processing struct {
		CueFileDirectory string `toml:"cue_file_directory"`
		AutoProcess      bool   `toml:"auto_process"`
		BatchSize        int    `toml:"batch_size"`
	} `toml:"processing"`
	
	Logging logger.Config `toml:"logging"`
}

// TemplateConfig represents a template configuration for tracklist formatting
type TemplateConfig struct {
	Header string `toml:"header"`
	Track  string `toml:"track"`
	Footer string `toml:"footer"`
}

// ShowConfig represents configuration for a specific show
type ShowConfig struct {
	// CUE file mapping
	CueFilePattern string `toml:"cue_file_pattern"` // e.g., "MYR*.cue"
	CueFileMapping string `toml:"cue_file_mapping"` // e.g., "latest.cue" or specific file
	
	// Show identification
	ShowNamePattern string   `toml:"show_name_pattern"` // e.g., "Sounds Like - {date}"
	Aliases         []string `toml:"aliases"`           // e.g., ["sounds-like", "sl"]
	
	// Template overrides
	TemplateName   string `toml:"template"`        // Reference to templates section
	CustomTemplate string `toml:"custom_template"` // Inline template override
	
	// Date/time handling
	DateFormat     string `toml:"date_format"`     // Format for show title generation
	
	// Processing options
	Enabled  bool `toml:"enabled"`
	Priority int  `toml:"priority"`
}

// ConfigError represents configuration-related errors
type ConfigError struct {
	Field   string
	Message string
}

func (e ConfigError) Error() string {
	if e.Field != "" {
		return "config." + e.Field + ": " + e.Message
	}
	return e.Message
}

// AIDEV-NOTE: Error types help with specific error handling and better user feedback
var (
	ErrFileNotFound   = errors.New("configuration file not found")
	ErrInvalidFormat  = errors.New("invalid configuration file format")
	ErrMissingField   = errors.New("required field is missing or empty")
	ErrInvalidPath    = errors.New("specified path does not exist or is not accessible")
)

// LoadConfig reads and parses a TOML configuration file
// AIDEV-NOTE: Uses BurntSushi/toml for parsing - handles most TOML format edge cases
func LoadConfig(filepath string) (*Config, error) {
	// Validate file exists and is readable
	if err := errorutil.ValidateFileReadable(filepath, "loading config"); err != nil {
		if fileErr, ok := err.(*errorutil.FileOpError); ok && strings.Contains(fileErr.Error(), "not found") {
			return nil, fmt.Errorf("%w: %s", ErrFileNotFound, filepath)
		}
		return nil, err
	}

	// Read the configuration file
	data, err := os.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", filepath, err)
	}

	// Parse TOML into Config struct
	var loadedConfig Config
	if err := toml.Unmarshal(data, &loadedConfig); err != nil {
		return nil, fmt.Errorf("%w: %s - %v", ErrInvalidFormat, filepath, err)
	}

	// Merge loaded config with defaults
	defaults := DefaultConfig()
	config := mergeWithDefaults(&loadedConfig, defaults)

	// Apply environment variable overrides
	config.ApplyEnvironmentOverrides()

	// AIDEV-TODO: Add validation logic after config is loaded

	return config, nil
}

// Validate checks that all required configuration fields are present and valid
// AIDEV-NOTE: Validation helps catch configuration issues early rather than failing at runtime
func (c *Config) Validate() error {
	return errorutil.ValidateConfig("main", func(vb *errorutil.ValidationBuilder) *errorutil.ValidationBuilder {
		return vb.
			// Validate Station fields
			RequiredString("station.name", c.Station.Name).
			RequiredString("station.mixcloud_username", c.Station.MixcloudUsername).
			// Validate OAuth fields
			RequiredString("oauth.client_id", c.OAuth.ClientID).
			RequiredString("oauth.client_secret", c.OAuth.ClientSecret).
			// Validate Paths fields
			RequiredString("paths.cue_file_directory", c.Paths.CueFileDirectory).
			// Custom validation for directory existence
			Custom("paths.cue_file_directory", c.Paths.CueFileDirectory, func(value interface{}) bool {
				if dirPath, ok := value.(string); ok && !errorutil.IsEmptyString(dirPath) {
					return errorutil.ValidateDirectory(dirPath, "config validation", false) == nil
				}
				return true // Skip validation if empty (handled by RequiredString)
			}, "directory does not exist or is not accessible")
			// AIDEV-NOTE: OAuth AccessToken and RefreshToken are optional during validation
	})
}

// DefaultConfig returns a Config struct with sensible default values
// AIDEV-NOTE: Defaults help ensure the application works with minimal configuration
func DefaultConfig() *Config {
	return &Config{
		Station: struct {
			Name             string `toml:"name"`
			MixcloudUsername string `toml:"mixcloud_username"`
		}{
			Name:             "",
			MixcloudUsername: "",
		},
		OAuth: struct {
			ClientID     string `toml:"client_id"`
			ClientSecret string `toml:"client_secret"`
			AccessToken  string `toml:"access_token"`
			RefreshToken string `toml:"refresh_token"`
		}{
			ClientID:     "",
			ClientSecret: "",
			AccessToken:  "",
			RefreshToken: "",
		},
		Filtering: struct {
			ExcludedArtists       []string `toml:"excluded_artists"`
			ExcludedTitles        []string `toml:"excluded_titles"`
			ExcludedArtistPatterns []string `toml:"excluded_artist_patterns"`
			ExcludedTitlePatterns  []string `toml:"excluded_title_patterns"`
		}{
			ExcludedArtists:       []string{},
			ExcludedTitles:        []string{},
			ExcludedArtistPatterns: []string{},
			ExcludedTitlePatterns:  []string{},
		},
		Paths: struct {
			CueFileDirectory string `toml:"cue_file_directory"`
		}{
			CueFileDirectory: ".", // Default to current directory
		},
		Templates: struct {
			Default string                    `toml:"default"`
			Config  map[string]TemplateConfig `toml:"config"`
		}{
			Default: "classic", // Use existing hardcoded format as default
			Config:  make(map[string]TemplateConfig),
		},
		Shows: make(map[string]ShowConfig),
		Processing: struct {
			CueFileDirectory string `toml:"cue_file_directory"`
			AutoProcess      bool   `toml:"auto_process"`
			BatchSize        int    `toml:"batch_size"`
		}{
			CueFileDirectory: ".", // Default to current directory
			AutoProcess:      false,
			BatchSize:        constants.DefaultBatchSize,
		},
		Logging: logger.Config{
			Enabled:         true,
			Directory:       "logs",
			FilenamePattern: "mixcloud-updater-%Y%m%d.log",
			Level:           "info",
			MaxFiles:        constants.DefaultMaxLogFiles,
			MaxSizeMB:       constants.DefaultMaxLogSizeMB,
			ConsoleOutput:   true,
		},
	}
}

// mergeWithDefaults takes a loaded config and merges it with default values
// AIDEV-NOTE: Only non-zero values from loaded config override defaults
func mergeWithDefaults(loaded, defaults *Config) *Config {
	result := *defaults // Start with defaults

	// Merge Station values
	if loaded.Station.Name != "" {
		result.Station.Name = loaded.Station.Name
	}
	if loaded.Station.MixcloudUsername != "" {
		result.Station.MixcloudUsername = loaded.Station.MixcloudUsername
	}

	// Merge OAuth values
	if loaded.OAuth.ClientID != "" {
		result.OAuth.ClientID = loaded.OAuth.ClientID
	}
	if loaded.OAuth.ClientSecret != "" {
		result.OAuth.ClientSecret = loaded.OAuth.ClientSecret
	}
	if loaded.OAuth.AccessToken != "" {
		result.OAuth.AccessToken = loaded.OAuth.AccessToken
	}
	if loaded.OAuth.RefreshToken != "" {
		result.OAuth.RefreshToken = loaded.OAuth.RefreshToken
	}

	// Merge Filtering values (preserve non-empty slices)
	if len(loaded.Filtering.ExcludedArtists) > 0 {
		result.Filtering.ExcludedArtists = loaded.Filtering.ExcludedArtists
	}
	if len(loaded.Filtering.ExcludedTitles) > 0 {
		result.Filtering.ExcludedTitles = loaded.Filtering.ExcludedTitles
	}
	if len(loaded.Filtering.ExcludedArtistPatterns) > 0 {
		result.Filtering.ExcludedArtistPatterns = loaded.Filtering.ExcludedArtistPatterns
	}
	if len(loaded.Filtering.ExcludedTitlePatterns) > 0 {
		result.Filtering.ExcludedTitlePatterns = loaded.Filtering.ExcludedTitlePatterns
	}

	// Merge Paths values
	if loaded.Paths.CueFileDirectory != "" {
		result.Paths.CueFileDirectory = loaded.Paths.CueFileDirectory
	}

	// Merge Templates values
	if loaded.Templates.Default != "" {
		result.Templates.Default = loaded.Templates.Default
	}
	if len(loaded.Templates.Config) > 0 {
		if result.Templates.Config == nil {
			result.Templates.Config = make(map[string]TemplateConfig)
		}
		for name, template := range loaded.Templates.Config {
			result.Templates.Config[name] = template
		}
	}

	// Merge Shows values
	if len(loaded.Shows) > 0 {
		if result.Shows == nil {
			result.Shows = make(map[string]ShowConfig)
		}
		for name, show := range loaded.Shows {
			result.Shows[name] = show
		}
	}

	// Merge Processing values
	if loaded.Processing.CueFileDirectory != "" {
		result.Processing.CueFileDirectory = loaded.Processing.CueFileDirectory
	}
	if loaded.Processing.AutoProcess {
		result.Processing.AutoProcess = loaded.Processing.AutoProcess
	}
	if loaded.Processing.BatchSize > 0 {
		result.Processing.BatchSize = loaded.Processing.BatchSize
	}

	// Merge Logging values
	if loaded.Logging.Directory != "" {
		result.Logging.Directory = loaded.Logging.Directory
	}
	if loaded.Logging.FilenamePattern != "" {
		result.Logging.FilenamePattern = loaded.Logging.FilenamePattern
	}
	if loaded.Logging.Level != "" {
		result.Logging.Level = loaded.Logging.Level
	}
	if loaded.Logging.MaxFiles > 0 {
		result.Logging.MaxFiles = loaded.Logging.MaxFiles
	}
	if loaded.Logging.MaxSizeMB > 0 {
		result.Logging.MaxSizeMB = loaded.Logging.MaxSizeMB
	}
	// Handle boolean fields explicitly (since false is a valid value)
	if loaded.Logging.Enabled != result.Logging.Enabled {
		result.Logging.Enabled = loaded.Logging.Enabled
	}
	if loaded.Logging.ConsoleOutput != result.Logging.ConsoleOutput {
		result.Logging.ConsoleOutput = loaded.Logging.ConsoleOutput
	}

	return &result
}

// ApplyEnvironmentOverrides checks for environment variables and overrides config values
// AIDEV-NOTE: Environment variables allow sensitive data to be kept out of config files
func (c *Config) ApplyEnvironmentOverrides() {
	// Station environment overrides
	if envVal := os.Getenv("NWRMIXCLOUD_STATION_NAME"); envVal != "" {
		c.Station.Name = envVal
	}
	if envVal := os.Getenv("NWRMIXCLOUD_STATION_MIXCLOUD_USERNAME"); envVal != "" {
		c.Station.MixcloudUsername = envVal
	}

	// OAuth environment overrides (most common use case)
	if envVal := os.Getenv("NWRMIXCLOUD_OAUTH_CLIENT_ID"); envVal != "" {
		c.OAuth.ClientID = envVal
	}
	if envVal := os.Getenv("NWRMIXCLOUD_OAUTH_CLIENT_SECRET"); envVal != "" {
		c.OAuth.ClientSecret = envVal
	}
	if envVal := os.Getenv("NWRMIXCLOUD_OAUTH_ACCESS_TOKEN"); envVal != "" {
		c.OAuth.AccessToken = envVal
	}
	if envVal := os.Getenv("NWRMIXCLOUD_OAUTH_REFRESH_TOKEN"); envVal != "" {
		c.OAuth.RefreshToken = envVal
	}

	// Paths environment overrides
	if envVal := os.Getenv("NWRMIXCLOUD_PATHS_CUE_FILE_DIRECTORY"); envVal != "" {
		c.Paths.CueFileDirectory = envVal
	}

	// Processing environment overrides
	if envVal := os.Getenv("NWRMIXCLOUD_PROCESSING_CUE_FILE_DIRECTORY"); envVal != "" {
		c.Processing.CueFileDirectory = envVal
	}
	if envVal := os.Getenv("NWRMIXCLOUD_PROCESSING_AUTO_PROCESS"); envVal == "true" {
		c.Processing.AutoProcess = true
	}

	// AIDEV-NOTE: Filtering arrays are typically not overridden via env vars due to complexity
	// Consider using a comma-separated format if needed:
	// if envVal := os.Getenv("NWRMIXCLOUD_FILTERING_EXCLUDED_ARTISTS"); envVal != "" {
	//     c.Filtering.ExcludedArtists = strings.Split(envVal, ",")
	// }
}

// SaveConfig writes a Config struct to a TOML file
// AIDEV-NOTE: Used primarily for persisting updated OAuth tokens
func SaveConfig(config *Config, filepath string) error {
	if config == nil {
		return fmt.Errorf("config cannot be nil")
	}

	// Marshal config to TOML format
	data, err := toml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config to TOML: %w", err)
	}

	// Write to file with appropriate permissions and directory creation
	return errorutil.SafeWriteFile(filepath, data, "saving config", true)
}