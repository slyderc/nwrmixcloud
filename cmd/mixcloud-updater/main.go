package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nowwaveradio/mixcloud-updater/internal/config"
	"github.com/nowwaveradio/mixcloud-updater/internal/mixcloud"
	"github.com/nowwaveradio/mixcloud-updater/internal/processor"
	"github.com/nowwaveradio/mixcloud-updater/internal/shows"
)

const version = "1.0.0"

var (
	configFile  = flag.String("config", "config.toml", "Path to the configuration file")
	showAlias   = flag.String("show", "", "Process specific show by name/alias (optional)")
	templateName = flag.String("template", "", "Template name to use for formatting (optional)")
	dryRun      = flag.Bool("dry-run", false, "Preview changes without updating Mixcloud")
	showVersion = flag.Bool("version", false, "Show version information")
	help        = flag.Bool("help", false, "Show help information")
	listShows   = flag.Bool("list-shows", false, "List available shows and their aliases")
	listTemplates = flag.Bool("list-templates", false, "List available templates")
)

func init() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Mixcloud Updater v%s\n\n", version)
		fmt.Fprintf(os.Stderr, "Automatically updates Mixcloud show descriptions with formatted tracklists from CUE files.\n")
		fmt.Fprintf(os.Stderr, "Uses a unified config-driven architecture for batch and single-show processing.\n\n")
		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "  %s [config.toml]                    # Process all enabled shows\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s [OPTIONS] [config.toml]          # Process with options\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  # Process all enabled shows from config\n")
		fmt.Fprintf(os.Stderr, "  %s config.toml\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\n  # Process specific show by alias\n")
		fmt.Fprintf(os.Stderr, "  %s -show nnw config.toml\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -show \"newer-new-wave\" config.toml\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\n  # Preview without updating\n")
		fmt.Fprintf(os.Stderr, "  %s -dry-run config.toml\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -show sounds-like -dry-run config.toml\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\n  # List available shows and templates\n")
		fmt.Fprintf(os.Stderr, "  %s -list-shows config.toml\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -list-templates config.toml\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\n  # Use specific template override\n")
		fmt.Fprintf(os.Stderr, "  %s -show morning -template detailed config.toml\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\n  # Automation with cron (process all shows)\n")
		fmt.Fprintf(os.Stderr, "  0 */2 * * * /path/to/mixcloud-updater /path/to/config.toml\n")
	}
}

// validateArguments performs comprehensive validation of command-line arguments
func validateArguments(configFilePath string) error {
	// Validate and check config file
	if err := validateConfigFile(configFilePath); err != nil {
		return fmt.Errorf("config file validation failed: %w", err)
	}

	// Validate show alias format if provided
	if *showAlias != "" {
		if err := validateShowAlias(*showAlias); err != nil {
			return fmt.Errorf("show alias validation failed: %w", err)
		}
	}

	return nil
}

// validateShowAlias checks if the show alias format is valid  
func validateShowAlias(alias string) error {
	// Check if show alias is not just whitespace
	trimmed := strings.TrimSpace(alias)
	if trimmed == "" {
		return fmt.Errorf("show alias cannot be empty or just whitespace")
	}

	// Check reasonable length limits
	if len(trimmed) < 1 {
		return fmt.Errorf("show alias too short (minimum 1 character): %q", trimmed)
	}

	if len(trimmed) > 50 {
		return fmt.Errorf("show alias too long (maximum 50 characters): %q", trimmed)
	}

	// Update the global variable to use the trimmed version
	*showAlias = trimmed

	return nil
}


// validateConfigFile checks if the config file exists or can be created
func validateConfigFile(filePath string) error {
	// Clean the path
	cleanPath := filepath.Clean(filePath)
	
	// Check if file exists
	info, err := os.Stat(cleanPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Config file doesn't exist - check if directory is writable for creating default
			dir := filepath.Dir(cleanPath)
			if dirInfo, dirErr := os.Stat(dir); dirErr != nil {
				return fmt.Errorf("config file directory does not exist: %s", dir)
			} else if !dirInfo.IsDir() {
				return fmt.Errorf("config file directory path is not a directory: %s", dir)
			}
			// Directory exists, we can create config file later if needed
			return nil
		}
		return fmt.Errorf("cannot access config file: %w", err)
	}

	// Config file exists - check if it's readable
	if info.IsDir() {
		return fmt.Errorf("config file path is a directory, not a file: %s", cleanPath)
	}

	file, err := os.Open(cleanPath)
	if err != nil {
		return fmt.Errorf("config file is not readable: %w", err)
	}
	file.Close()

	return nil
}


// loadConfiguration loads and validates the configuration file with automatic OAuth if needed
func loadConfiguration(configPath string) (*config.Config, error) {
	cleanPath := filepath.Clean(configPath)
	
	// Check if config file exists
	if _, err := os.Stat(cleanPath); err != nil {
		if os.IsNotExist(err) {
			// Config file doesn't exist - create a default one
			defaultCfg := config.DefaultConfig()
			if err := config.SaveConfig(defaultCfg, cleanPath); err != nil {
				return nil, fmt.Errorf("failed to create default config file: %w", err)
			}
			
			fmt.Printf("Created config file: %s\n", cleanPath)
			fmt.Printf("Please edit this file with your Mixcloud OAuth credentials, then run again.\n")
			return nil, fmt.Errorf("configuration file created")
		}
		return nil, fmt.Errorf("cannot access config file: %w", err)
	}
	
	// Load the configuration
	cfg, err := config.LoadConfig(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	
	// Apply environment variable overrides
	cfg.ApplyEnvironmentOverrides()
	
	// Check if we need OAuth authorization
	if needsAuthorization(cfg) {
		// Validate OAuth credentials are present
		if cfg.OAuth.ClientID == "" || cfg.OAuth.ClientSecret == "" {
			return nil, fmt.Errorf("OAuth client_id and client_secret must be configured in %s", cleanPath)
		}
		if cfg.Station.MixcloudUsername == "" {
			return nil, fmt.Errorf("station.mixcloud_username must be configured in %s", cleanPath)
		}
		
		fmt.Printf("🔑 OAuth authorization required - launching browser...\n")
		
		// Perform the OAuth flow
		err = mixcloud.AuthorizeAndSave(cfg, cleanPath)
		if err != nil {
			return nil, fmt.Errorf("authorization failed: %w", err)
		}
		
		// Reload the configuration with new tokens
		cfg, err = config.LoadConfig(cleanPath)
		if err != nil {
			return nil, fmt.Errorf("failed to reload config after authorization: %w", err)
		}
		cfg.ApplyEnvironmentOverrides()
	}
	
	// Validate the final configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}
	
	return cfg, nil
}


// needsAuthorization checks if OAuth authorization is needed
func needsAuthorization(cfg *config.Config) bool {
	if cfg == nil {
		return true
	}
	
	// Check if OAuth credentials are missing
	if cfg.OAuth.ClientID == "" || cfg.OAuth.ClientSecret == "" {
		return true
	}
	
	// Check if access token is missing
	// AIDEV-NOTE: Mixcloud doesn't provide refresh tokens, only access tokens
	if cfg.OAuth.AccessToken == "" {
		return true
	}
	
	return false
}




func main() {
	flag.Parse()

	// Handle help and version flags
	if *help {
		flag.Usage()
		os.Exit(0)
	}

	if *showVersion {
		fmt.Printf("Mixcloud Updater v%s\n", version)
		os.Exit(0)
	}

	// Determine config file path - support single positional argument
	configFilePath := *configFile
	if flag.NArg() == 1 {
		configFilePath = flag.Arg(0)
	} else if flag.NArg() > 1 {
		fmt.Fprintf(os.Stderr, "Error: Too many arguments. Expected at most one config file path.\n\n")
		flag.Usage()
		os.Exit(1)
	}

	// Print banner
	fmt.Printf("Mixcloud Updater v%s\n", version)
	fmt.Printf("=================================\n\n")

	// Validate arguments
	if err := validateArguments(configFilePath); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n\n", err)
		flag.Usage()
		os.Exit(1)
	}

	// Load configuration
	fmt.Printf("Loading configuration: %s\n", configFilePath)
	cfg, err := loadConfiguration(configFilePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Handle list operations
	if *listShows {
		if err := listAvailableShows(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "Error listing shows: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	if *listTemplates {
		if err := listAvailableTemplates(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "Error listing templates: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	// Create show processor  
	showProcessor, err := processor.NewShowProcessor(cfg, configFilePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing processor: %v\n", err)
		os.Exit(1)
	}

	// Execute processing based on arguments
	if *showAlias != "" {
		// Process specific show
		if err := showProcessor.ProcessShow(*showAlias, *templateName, *dryRun); err != nil {
			fmt.Fprintf(os.Stderr, "Error processing show: %v\n", err)
			handleAuthError(err)
			os.Exit(1)
		}
	} else {
		// Process all enabled shows
		if err := showProcessor.ProcessAllShows(*dryRun); err != nil {
			fmt.Fprintf(os.Stderr, "Error processing shows: %v\n", err)
			handleAuthError(err)
			os.Exit(1)
		}
	}

	fmt.Println("✓ Done!")
}

// handleAuthError provides helpful messages for authentication errors
func handleAuthError(err error) {
	errStr := err.Error()
	if strings.Contains(errStr, "authentication") || strings.Contains(errStr, "unauthorized") || 
	   strings.Contains(errStr, "token has expired") || strings.Contains(errStr, "OAuthException") {
		fmt.Fprintf(os.Stderr, "\nYour OAuth tokens have expired. Please run the command again to re-authenticate.\n")
	}
}

// listAvailableShows displays all configured shows and their aliases
func listAvailableShows(cfg *config.Config) error {
	resolver, err := shows.NewResolver(cfg)
	if err != nil {
		return fmt.Errorf("creating show resolver: %w", err)
	}

	allShows := resolver.ListShows()
	enabledShows := resolver.ListEnabledShows(true) // sorted by priority

	fmt.Printf("Configured Shows:\n")
	fmt.Printf("================\n\n")

	if len(allShows) == 0 {
		fmt.Printf("No shows configured in config file.\n")
		fmt.Printf("Add show configurations to the [shows] section.\n")
		return nil
	}

	for _, showKey := range allShows {
		showCfg := cfg.Shows[showKey]
		aliases := resolver.GetShowAliases(showKey)
		
		status := "disabled"
		priority := ""
		if showCfg.Enabled {
			status = "enabled"
			priority = fmt.Sprintf(" (priority: %d)", showCfg.Priority)
		}

		fmt.Printf("• %s [%s]%s\n", showKey, status, priority)
		fmt.Printf("  Pattern: %s | %s\n", showCfg.ShowNamePattern, 
			getSourceDescription(showCfg))
		
		if len(aliases) > 0 {
			fmt.Printf("  Aliases: %s\n", strings.Join(aliases, ", "))
		}
		fmt.Printf("\n")
	}

	if len(enabledShows) > 0 {
		fmt.Printf("Processing Order (enabled shows by priority):\n")
		for i, showKey := range enabledShows {
			fmt.Printf("%d. %s\n", i+1, showKey)
		}
	}

	return nil
}

// listAvailableTemplates displays all configured templates
func listAvailableTemplates(cfg *config.Config) error {
	fmt.Printf("Available Templates:\n")
	fmt.Printf("===================\n\n")

	if len(cfg.Templates.Templates) == 0 {
		fmt.Printf("No templates configured in config file.\n")
		fmt.Printf("Add template configurations to the [templates.templates] section.\n")
		return nil
	}

	defaultTemplate := cfg.Templates.Default
	if defaultTemplate == "" {
		defaultTemplate = "classic"
	}

	for name, templateCfg := range cfg.Templates.Templates {
		isDefault := ""
		if name == defaultTemplate {
			isDefault = " (default)"
		}

		fmt.Printf("• %s%s\n", name, isDefault)
		
		hasHeader := templateCfg.Header != ""
		hasFooter := templateCfg.Footer != ""
		
		fmt.Printf("  Structure: ")
		if hasHeader {
			fmt.Printf("Header + ")
		}
		fmt.Printf("Track")
		if hasFooter {
			fmt.Printf(" + Footer")
		}
		fmt.Printf("\n")
		
		fmt.Printf("  Track format: %s\n", 
			truncateForDisplay(templateCfg.Track, 60))
		fmt.Printf("\n")
	}

	fmt.Printf("Default template: %s\n", defaultTemplate)
	return nil
}

// getSourceDescription returns a human-readable description of the CUE file source
func getSourceDescription(showCfg config.ShowConfig) string {
	if showCfg.CueFileMapping != "" {
		return fmt.Sprintf("file: %s", showCfg.CueFileMapping)
	}
	if showCfg.CueFilePattern != "" {
		return fmt.Sprintf("pattern: %s", showCfg.CueFilePattern)
	}
	return "no source configured"
}

// truncateForDisplay truncates a string for display purposes
func truncateForDisplay(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}