package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nowwaveradio/mixcloud-updater/internal/config"
	"github.com/nowwaveradio/mixcloud-updater/internal/cue"
	"github.com/nowwaveradio/mixcloud-updater/internal/filter"
	"github.com/nowwaveradio/mixcloud-updater/internal/formatter"
	"github.com/nowwaveradio/mixcloud-updater/internal/mixcloud"
)

const version = "1.0.0"

var (
	cueFile    = flag.String("cue-file", "", "Path to the CUE file to parse (required)")
	configFile = flag.String("config", "config.toml", "Path to the configuration file")
	showName   = flag.String("show-name", "", "Name of the show to update on Mixcloud (required)")
	dryRun     = flag.Bool("dry-run", false, "Preview changes without updating Mixcloud")
	showVersion = flag.Bool("version", false, "Show version information")
	help       = flag.Bool("help", false, "Show help information")
)

func init() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Mixcloud Updater v%s\n\n", version)
		fmt.Fprintf(os.Stderr, "Automatically updates Mixcloud show descriptions with formatted tracklists from CUE files.\n\n")
		fmt.Fprintf(os.Stderr, "Usage: %s [OPTIONS]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Required flags:\n")
		fmt.Fprintf(os.Stderr, "  -cue-file string\n        Path to the CUE file to parse\n")
		fmt.Fprintf(os.Stderr, "  -show-name string\n        Name of the show to update on Mixcloud\n\n")
		fmt.Fprintf(os.Stderr, "Optional flags:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  %s -cue-file MYR04137.cue -show-name \"The Newer New Wave Show\"\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -cue-file MYR04137.cue -show-name \"The Newer New Wave Show\" -dry-run\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -cue-file MYR04137.cue -show-name \"The Newer New Wave Show\" -config custom.toml\n", os.Args[0])
	}
}

// validateArguments performs comprehensive validation of command-line arguments and file accessibility
func validateArguments() error {
	// Check required arguments
	if *cueFile == "" {
		return fmt.Errorf("--cue-file is required")
	}

	if *showName == "" {
		return fmt.Errorf("--show-name is required")
	}

	// Validate and check CUE file
	if err := validateCueFile(*cueFile); err != nil {
		return fmt.Errorf("CUE file validation failed: %w", err)
	}

	// Validate and check config file
	if err := validateConfigFile(*configFile); err != nil {
		return fmt.Errorf("config file validation failed: %w", err)
	}

	// Validate show name format
	if err := validateShowName(*showName); err != nil {
		return fmt.Errorf("show name validation failed: %w", err)
	}

	return nil
}

// validateCueFile checks if the CUE file exists and is readable
func validateCueFile(filePath string) error {
	// Clean and resolve the path to handle platform differences
	cleanPath := filepath.Clean(filePath)
	
	// Check if file exists
	info, err := os.Stat(cleanPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("CUE file does not exist: %s", cleanPath)
		}
		return fmt.Errorf("cannot access CUE file: %w", err)
	}

	// Check if it's a regular file (not a directory)
	if info.IsDir() {
		return fmt.Errorf("CUE file path is a directory, not a file: %s", cleanPath)
	}

	// Check if file is readable
	file, err := os.Open(cleanPath)
	if err != nil {
		return fmt.Errorf("CUE file is not readable: %w", err)
	}
	file.Close()

	// Check file extension (case-insensitive)
	ext := strings.ToLower(filepath.Ext(cleanPath))
	if ext != ".cue" {
		return fmt.Errorf("file does not have .cue extension: %s (got %s)", cleanPath, ext)
	}

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

// validateShowName checks if the show name format is valid
func validateShowName(name string) error {
	// Check if show name is not just whitespace
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return fmt.Errorf("show name cannot be empty or just whitespace")
	}

	// Check reasonable length limits
	if len(trimmed) < 2 {
		return fmt.Errorf("show name too short (minimum 2 characters): %q", trimmed)
	}

	if len(trimmed) > 100 {
		return fmt.Errorf("show name too long (maximum 100 characters): %q", trimmed)
	}

	// Update the global variable to use the trimmed version
	*showName = trimmed

	return nil
}

// loadConfiguration loads and validates the configuration file
func loadConfiguration(configPath string) (*config.Config, error) {
	cleanPath := filepath.Clean(configPath)
	
	// Check if config file exists
	if _, err := os.Stat(cleanPath); err != nil {
		if os.IsNotExist(err) {
			// Config file doesn't exist - try to create a default one
			fmt.Printf("Config file not found: %s\n", cleanPath)
			fmt.Printf("Creating default configuration file...\n")
			
			defaultCfg := config.DefaultConfig()
			if err := config.SaveConfig(defaultCfg, cleanPath); err != nil {
				return nil, fmt.Errorf("failed to create default config file: %w", err)
			}
			
			fmt.Printf("Default configuration created at: %s\n", cleanPath)
			fmt.Printf("Please edit this file with your Mixcloud OAuth credentials before running again.\n")
			return nil, fmt.Errorf("configuration file created - please update with your credentials and run again")
		}
		return nil, fmt.Errorf("cannot access config file: %w", err)
	}
	
	// Load the configuration
	cfg, err := config.LoadConfig(cleanPath)
	if err != nil {
		// Provide specific error messages for common config issues
		if strings.Contains(err.Error(), "toml") {
			return nil, fmt.Errorf("invalid TOML format in config file %s: %w", cleanPath, err)
		}
		return nil, fmt.Errorf("failed to load config from %s: %w", cleanPath, err)
	}
	
	// Apply environment variable overrides
	cfg.ApplyEnvironmentOverrides()
	
	// Validate the loaded configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}
	
	// Log some configuration details (without sensitive info)
	fmt.Printf("Station: %s\n", cfg.Station.Name)
	fmt.Printf("Mixcloud Username: %s\n", cfg.Station.MixcloudUsername)
	hasTokens := cfg.OAuth.AccessToken != "" && cfg.OAuth.RefreshToken != ""
	fmt.Printf("OAuth Tokens: %s\n", func() string {
		if hasTokens {
			return "configured"
		}
		return "missing - OAuth flow required"
	}())
	
	return cfg, nil
}

// WorkflowResult contains the results of workflow execution for reporting
type WorkflowResult struct {
	ParsedTracks    int
	FilteredTracks  int
	ExcludedTracks  int
	FormattedLength int
	ShowURL         string
	DryRun          bool
	Success         bool
	TotalDuration   time.Duration
	StepDurations   map[string]time.Duration
}

// executeWorkflow orchestrates the main application workflow
func executeWorkflow(cfg *config.Config, cueFilePath, showName string, dryRun bool) error {
	startTime := time.Now()
	stepDurations := make(map[string]time.Duration)
	result := &WorkflowResult{
		DryRun:        dryRun,
		StepDurations: stepDurations,
	}
	// Step 1: Parse CUE file
	fmt.Printf("Step 1: Parsing CUE file...\n")
	stepStart := time.Now()
	cueSheet, err := cue.ParseCueFile(cueFilePath)
	if err != nil {
		return fmt.Errorf("failed to parse CUE file: %w", err)
	}
	stepDurations["parse"] = time.Since(stepStart)
	result.ParsedTracks = len(cueSheet.Tracks)
	fmt.Printf("✓ Parsed %d tracks from CUE file (%.2fs)\n\n", len(cueSheet.Tracks), stepDurations["parse"].Seconds())

	// Step 2: Initialize content filter
	fmt.Printf("Step 2: Initializing content filter...\n")
	stepStart = time.Now()
	trackFilter, err := filter.NewFilter(cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize content filter: %w", err)
	}
	stepDurations["filter_init"] = time.Since(stepStart)
	fmt.Printf("✓ Content filter initialized (%.2fs)\n\n", stepDurations["filter_init"].Seconds())

	// Step 3: Filter tracks
	fmt.Printf("Step 3: Filtering tracks...\n")
	stepStart = time.Now()
	var filteredTracks []cue.Track
	originalCount := len(cueSheet.Tracks)
	
	for _, track := range cueSheet.Tracks {
		if trackFilter.ShouldIncludeTrack(&track) {
			filteredTracks = append(filteredTracks, track)
		}
	}
	
	filteredCount := len(filteredTracks)
	excludedCount := originalCount - filteredCount
	stepDurations["filter"] = time.Since(stepStart)
	result.FilteredTracks = filteredCount
	result.ExcludedTracks = excludedCount
	fmt.Printf("✓ Filtered tracks: %d included, %d excluded (%.2fs)\n\n", filteredCount, excludedCount, stepDurations["filter"].Seconds())

	if filteredCount == 0 {
		return fmt.Errorf("no tracks remaining after filtering - check your filter configuration")
	}

	// Step 4: Format tracklist
	fmt.Printf("Step 4: Formatting tracklist...\n")
	stepStart = time.Now()
	trackFormatter := formatter.NewFormatter()
	formattedTracklist := trackFormatter.FormatTracklist(filteredTracks, trackFilter)
	stepDurations["format"] = time.Since(stepStart)
	result.FormattedLength = len(formattedTracklist)
	fmt.Printf("✓ Tracklist formatted (%d characters, %.2fs)\n\n", len(formattedTracklist), stepDurations["format"].Seconds())

	// Step 5: Initialize Mixcloud client
	fmt.Printf("Step 5: Initializing Mixcloud client...\n")
	stepStart = time.Now()
	mixcloudClient, err := mixcloud.NewClient(cfg, filepath.Clean(*configFile))
	if err != nil {
		return fmt.Errorf("failed to initialize Mixcloud client: %w", err)
	}
	stepDurations["client_init"] = time.Since(stepStart)
	fmt.Printf("✓ Mixcloud client initialized (%.2fs)\n\n", stepDurations["client_init"].Seconds())

	// Step 6: Generate show URL and get show info
	fmt.Printf("Step 6: Locating Mixcloud show...\n")
	stepStart = time.Now()
	showURL := mixcloud.GenerateShowURL(cfg.Station.MixcloudUsername, showName)
	result.ShowURL = showURL
	fmt.Printf("Generated show URL: %s\n", showURL)

	if !dryRun {
		// Verify the show exists
		show, err := mixcloudClient.GetShow(showURL)
		if err != nil {
			return fmt.Errorf("failed to get show information: %w", err)
		}
		stepDurations["show_lookup"] = time.Since(stepStart)
		fmt.Printf("✓ Found show: %s (%.2fs)\n\n", show.Name, stepDurations["show_lookup"].Seconds())
	} else {
		stepDurations["show_lookup"] = time.Since(stepStart)
		fmt.Printf("✓ Dry run mode - skipping show verification (%.2fs)\n\n", stepDurations["show_lookup"].Seconds())
	}

	// Step 7: Update show description
	fmt.Printf("Step 7: Updating show description...\n")
	stepStart = time.Now()
	if dryRun {
		fmt.Printf("DRY RUN MODE - Would update show with:\n")
		fmt.Printf("─────────────────────────────────────────\n")
		fmt.Printf("%s\n", formattedTracklist)
		fmt.Printf("─────────────────────────────────────────\n")
		stepDurations["update"] = time.Since(stepStart)
		fmt.Printf("✓ Dry run completed - no changes made (%.2fs)\n", stepDurations["update"].Seconds())
	} else {
		err := mixcloudClient.UpdateShowDescription(showURL, formattedTracklist)
		if err != nil {
			return fmt.Errorf("failed to update show description: %w", err)
		}
		stepDurations["update"] = time.Since(stepStart)
		fmt.Printf("✓ Show description updated successfully (%.2fs)\n", stepDurations["update"].Seconds())
	}

	// Calculate total duration and mark as successful
	result.TotalDuration = time.Since(startTime)
	result.Success = true

	// Print final summary
	printWorkflowSummary(result)

	return nil
}

// printWorkflowSummary displays a comprehensive summary of the workflow execution
func printWorkflowSummary(result *WorkflowResult) {
	fmt.Printf("\n")
	fmt.Printf("═══════════════════════════════════════════\n")
	fmt.Printf("             WORKFLOW SUMMARY              \n")
	fmt.Printf("═══════════════════════════════════════════\n")
	
	// Execution status
	status := "✓ SUCCESS"
	if !result.Success {
		status = "✗ FAILED"
	}
	fmt.Printf("Status: %s\n", status)
	fmt.Printf("Mode: %s\n", func() string {
		if result.DryRun {
			return "DRY RUN (no changes made)"
		}
		return "LIVE UPDATE"
	}())
	
	fmt.Printf("\n")
	
	// Track statistics
	fmt.Printf("Track Processing:\n")
	fmt.Printf("  • Parsed from CUE: %d tracks\n", result.ParsedTracks)
	fmt.Printf("  • Included after filtering: %d tracks\n", result.FilteredTracks)
	fmt.Printf("  • Excluded by filters: %d tracks\n", result.ExcludedTracks)
	if result.ParsedTracks > 0 {
		inclusion_rate := float64(result.FilteredTracks) / float64(result.ParsedTracks) * 100
		fmt.Printf("  • Inclusion rate: %.1f%%\n", inclusion_rate)
	}
	
	fmt.Printf("\n")
	
	// Output information
	fmt.Printf("Output:\n")
	fmt.Printf("  • Formatted tracklist: %d characters\n", result.FormattedLength)
	if result.ShowURL != "" {
		fmt.Printf("  • Target show URL: %s\n", result.ShowURL)
	}
	
	fmt.Printf("\n")
	
	// Performance timing
	fmt.Printf("Performance Timing:\n")
	fmt.Printf("  • Total execution time: %.2fs\n", result.TotalDuration.Seconds())
	fmt.Printf("  • Breakdown by step:\n")
	
	stepNames := map[string]string{
		"parse":       "CUE file parsing",
		"filter_init": "Filter initialization",
		"filter":      "Track filtering",
		"format":      "Tracklist formatting",
		"client_init": "Mixcloud client init",
		"show_lookup": "Show verification",
		"update":      "Description update",
	}
	
	for step, duration := range result.StepDurations {
		if name, exists := stepNames[step]; exists {
			percentage := duration.Seconds() / result.TotalDuration.Seconds() * 100
			fmt.Printf("    - %s: %.2fs (%.1f%%)\n", name, duration.Seconds(), percentage)
		}
	}
	
	fmt.Printf("\n")
	
	// Final message
	if result.DryRun {
		fmt.Printf("💡 This was a dry run. To apply changes, run again without --dry-run\n")
	} else if result.Success {
		fmt.Printf("🎉 Show description successfully updated on Mixcloud!\n")
	}
	
	fmt.Printf("═══════════════════════════════════════════\n")
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

	// Print banner
	fmt.Printf("Mixcloud Updater v%s\n", version)
	fmt.Printf("=================================\n\n")

	// Validate required arguments
	if err := validateArguments(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n\n", err)
		flag.Usage()
		os.Exit(1)
	}

	// Print configuration
	fmt.Printf("Configuration:\n")
	fmt.Printf("  CUE File: %s\n", *cueFile)
	fmt.Printf("  Show Name: %s\n", *showName)
	fmt.Printf("  Config: %s\n", *configFile)
	fmt.Printf("  Dry Run: %t\n\n", *dryRun)

	// Load configuration from file
	fmt.Printf("Loading configuration...\n")
	cfg, err := loadConfiguration(*configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading configuration: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Configuration loaded successfully.\n\n")

	// Execute the main workflow
	if err := executeWorkflow(cfg, *cueFile, *showName, *dryRun); err != nil {
		fmt.Fprintf(os.Stderr, "Workflow execution failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✓ Done!")
}