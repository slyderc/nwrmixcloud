// Package processor provides the core show processing orchestration for the
// unified config-driven architecture. It coordinates show resolution, CUE file
// detection, parsing, filtering, formatting, and Mixcloud API interactions.
package processor

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/nowwaveradio/mixcloud-updater/internal/config"
	"github.com/nowwaveradio/mixcloud-updater/internal/cue"
	"github.com/nowwaveradio/mixcloud-updater/internal/filter"
	"github.com/nowwaveradio/mixcloud-updater/internal/formatter"
	"github.com/nowwaveradio/mixcloud-updater/internal/logger"
	"github.com/nowwaveradio/mixcloud-updater/internal/mixcloud"
	"github.com/nowwaveradio/mixcloud-updater/internal/shows"
)

// ShowProcessor orchestrates the complete workflow for processing shows
type ShowProcessor struct {
	config       *config.Config
	configPath   string
	resolver     *shows.Resolver
	cueResolver  *shows.CueResolver
	filter       *filter.Filter
	formatter    *formatter.Formatter
	mixcloud     *mixcloud.Client
	logger       *slog.Logger
}

// ProcessingResult contains the results of processing a single show
type ProcessingResult struct {
	ShowKey         string
	ShowName        string
	CueFile         string
	ParsedTracks    int
	FilteredTracks  int
	ExcludedTracks  int
	FormattedLength int
	ShowURL         string
	Template        string
	DryRun          bool
	Success         bool
	Error           error
	Duration        time.Duration
}

// BatchResult contains the results of batch processing multiple shows
type BatchResult struct {
	TotalShows      int
	ProcessedShows  int
	SuccessfulShows int
	FailedShows     int
	SkippedShows    int
	Results         []ProcessingResult
	TotalDuration   time.Duration
}

// NewShowProcessor creates a new ShowProcessor with all dependencies initialized
func NewShowProcessor(cfg *config.Config, configPath string) (*ShowProcessor, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	// Initialize show resolver
	resolver, err := shows.NewResolver(cfg)
	if err != nil {
		return nil, fmt.Errorf("initializing show resolver: %w", err)
	}

	// Validate show configurations
	if err := resolver.ValidateShows(); err != nil {
		return nil, fmt.Errorf("show validation failed: %w", err)
	}

	// Initialize CUE resolver with processing directory
	baseDir := cfg.Processing.CueFileDirectory
	if baseDir == "" {
		baseDir = cfg.Paths.CueFileDirectory // Fallback to legacy path config
	}
	if baseDir == "" {
		baseDir = "." // Final fallback
	}
	cueResolver := shows.NewCueResolver(baseDir)

	// Initialize content filter
	trackFilter, err := filter.NewFilter(cfg)
	if err != nil {
		return nil, fmt.Errorf("initializing content filter: %w", err)
	}

	// Initialize formatter with template support
	trackFormatter := formatter.NewFormatterWithConfig(cfg)

	// Initialize Mixcloud client
	mixcloudClient, err := mixcloud.NewClient(cfg, configPath)
	if err != nil {
		return nil, fmt.Errorf("initializing Mixcloud client: %w", err)
	}

	// Use the global file logger
	log := logger.Get()

	return &ShowProcessor{
		config:      cfg,
		configPath:  configPath,
		resolver:    resolver,
		cueResolver: cueResolver,
		filter:      trackFilter,
		formatter:   trackFormatter,
		mixcloud:    mixcloudClient,
		logger:      log.Logger, // Use the underlying slog.Logger
	}, nil
}

// ProcessShow processes a single show by name or alias
func (sp *ShowProcessor) ProcessShow(nameOrAlias string, templateOverride string, dateOverride string, dryRun bool) error {
	startTime := time.Now()
	
	fmt.Printf("Processing show: %s\n", nameOrAlias)
	fmt.Printf("================\n\n")

	// Find show configuration
	showCfg := sp.resolver.FindShowConfig(nameOrAlias)
	if showCfg == nil {
		return fmt.Errorf("show not found: %s", nameOrAlias)
	}

	showKey := sp.resolver.FindShowKey(nameOrAlias)
	
	// Check if show is enabled
	if !showCfg.Enabled {
		fmt.Printf("⚠️  Show '%s' is disabled in configuration\n", showKey)
		fmt.Printf("Set enabled = true in config to process this show\n")
		return nil
	}

	// Process the show
	result := sp.processingleShow(showKey, showCfg, templateOverride, dateOverride, dryRun)
	result.Duration = time.Since(startTime)

	// Print results
	sp.printSingleResult(result)

	if result.Error != nil {
		return result.Error
	}

	return nil
}

// ProcessAllShows processes all enabled shows in priority order
func (sp *ShowProcessor) ProcessAllShows(dryRun bool) error {
	startTime := time.Now()

	enabledShows := sp.resolver.ListEnabledShows(true) // sorted by priority
	
	if len(enabledShows) == 0 {
		fmt.Printf("No enabled shows found in configuration.\n")
		fmt.Printf("Add show configurations with enabled = true to process shows.\n")
		return nil
	}

	fmt.Printf("Processing %d enabled shows\n", len(enabledShows))
	fmt.Printf("============================\n\n")

	batchResult := &BatchResult{
		TotalShows:    len(enabledShows),
		Results:       make([]ProcessingResult, 0, len(enabledShows)),
		TotalDuration: 0,
	}

	// Process shows according to batch size
	batchSize := sp.config.Processing.BatchSize

	sp.logger.Info("Starting batch processing",
		slog.Int("total_shows", len(enabledShows)),
		slog.Int("batch_size", batchSize))
	if batchSize <= 0 {
		batchSize = 5 // Default batch size
	}

	for i := 0; i < len(enabledShows); i += batchSize {
		end := i + batchSize
		if end > len(enabledShows) {
			end = len(enabledShows)
		}

		batch := enabledShows[i:end]
		fmt.Printf("Processing batch %d/%d (%d shows)\n", 
			(i/batchSize)+1, (len(enabledShows)+batchSize-1)/batchSize, len(batch))
		fmt.Printf("──────────────────────────────────────\n")

		for _, showKey := range batch {
			showCfg := sp.config.Shows[showKey]
			result := sp.processingleShow(showKey, &showCfg, "", "", dryRun)
			
			batchResult.Results = append(batchResult.Results, result)
			batchResult.ProcessedShows++

			if result.Error != nil {
				batchResult.FailedShows++
				fmt.Printf("❌ Failed: %s - %v\n\n", showKey, result.Error)
			} else if result.Success {
				batchResult.SuccessfulShows++
				fmt.Printf("✅ Success: %s\n\n", showKey)
			} else {
				batchResult.SkippedShows++
				fmt.Printf("⏭️  Skipped: %s\n\n", showKey)
			}
		}
	}

	batchResult.TotalDuration = time.Since(startTime)

	// Log batch completion
	sp.logger.Info("Batch processing completed",
		slog.Int("total_shows", batchResult.TotalShows),
		slog.Int("successful", batchResult.SuccessfulShows),
		slog.Int("failed", batchResult.FailedShows),
		slog.Int("skipped", batchResult.SkippedShows),
		slog.Duration("total_duration", batchResult.TotalDuration))

	// Print batch summary
	sp.printBatchSummary(batchResult)

	// Return error if any shows failed (but continue processing)
	if batchResult.FailedShows > 0 {
		return fmt.Errorf("%d of %d shows failed", batchResult.FailedShows, batchResult.TotalShows)
	}

	return nil
}

// processingleShow handles the core processing logic for a single show
func (sp *ShowProcessor) processingleShow(showKey string, showCfg *config.ShowConfig, templateOverride string, dateOverride string, dryRun bool) ProcessingResult {
	result := ProcessingResult{
		ShowKey:  showKey,
		DryRun:   dryRun,
		Template: templateOverride,
	}

	// Log processing start
	sp.logger.Info("Processing show",
		slog.String("show_key", showKey),
		slog.Bool("dry_run", dryRun),
		slog.String("template_override", templateOverride))

	// Resolve CUE file
	cueFile, err := sp.cueResolver.ResolveCueFile(showCfg)
	if err != nil {
		sp.logger.Error("Failed to resolve CUE file",
			slog.String("show_key", showKey),
			slog.String("error", err.Error()))
		result.Error = fmt.Errorf("resolving CUE file: %w", err)
		return result
	}
	result.CueFile = cueFile
	sp.logger.Debug("CUE file resolved", slog.String("file", cueFile))

	// Validate CUE file
	if err := sp.cueResolver.ValidateCueFile(cueFile); err != nil {
		sp.logger.Error("CUE file validation failed",
			slog.String("show_key", showKey),
			slog.String("file", cueFile),
			slog.String("error", err.Error()))
		result.Error = fmt.Errorf("validating CUE file: %w", err)
		return result
	}

	// Parse CUE file
	cueSheet, err := cue.ParseCueFile(cueFile)
	if err != nil {
		sp.logger.Error("CUE file parsing failed",
			slog.String("show_key", showKey),
			slog.String("file", cueFile),
			slog.String("error", err.Error()))
		result.Error = fmt.Errorf("parsing CUE file: %w", err)
		return result
	}
	result.ParsedTracks = len(cueSheet.Tracks)

	sp.logger.Info("CUE file parsed successfully",
		slog.String("show_key", showKey),
		slog.Int("track_count", result.ParsedTracks))

	if result.ParsedTracks == 0 {
		sp.logger.Warn("No tracks found in CUE file",
			slog.String("show_key", showKey),
			slog.String("file", cueFile))
		result.Error = fmt.Errorf("no tracks found in CUE file")
		return result
	}

	// Filter tracks
	var filteredTracks []cue.Track
	for _, track := range cueSheet.Tracks {
		if sp.filter.ShouldIncludeTrack(&track) && !track.IsEmpty() {
			filteredTracks = append(filteredTracks, track)
		}
	}
	result.FilteredTracks = len(filteredTracks)
	result.ExcludedTracks = result.ParsedTracks - result.FilteredTracks

	sp.logger.Info("Track filtering completed",
		slog.String("show_key", showKey),
		slog.Int("included", result.FilteredTracks),
		slog.Int("excluded", result.ExcludedTracks))

	if result.FilteredTracks == 0 {
		sp.logger.Warn("No tracks remaining after filtering",
			slog.String("show_key", showKey))
		result.Error = fmt.Errorf("no tracks remaining after filtering")
		return result
	}

	// Generate show name with date substitution
	showName, err := sp.generateShowName(showCfg, cueFile, dateOverride)
	if err != nil {
		sp.logger.Error("Show name generation failed",
			slog.String("show_key", showKey),
			slog.String("error", err.Error()))
		result.Error = fmt.Errorf("generating show name: %w", err)
		return result
	}
	result.ShowName = showName
	sp.logger.Debug("Show name generated", slog.String("name", showName))

	// Generate show URL
	showURL := mixcloud.GenerateShowURL(sp.config.Station.MixcloudUsername, showName)
	result.ShowURL = showURL

	// Select and format with template
	var formattedTracklist string
	if templateOverride != "" {
		// Use template override
		metadata := map[string]interface{}{
			"show_title": showName,
			"show_date":  time.Now().Format("January 2, 2006"),
		}
		formattedTracklist = sp.formatter.FormatTracklistWithTemplate(filteredTracks, sp.filter, templateOverride, metadata)
		result.Template = templateOverride
	} else {
		// Use show-specific template selection
		metadata := map[string]interface{}{
			"show_title": showName,
			"show_date":  time.Now().Format("January 2, 2006"),
		}
		formattedTracklist = sp.formatter.FormatTracklistWithShowConfig(filteredTracks, sp.filter, showCfg, metadata)
		
		// Determine which template was used
		if selectedTemplate, err := sp.formatter.SelectTemplateForShow(showCfg); err == nil {
			result.Template = selectedTemplate
		} else {
			result.Template = "classic"
		}
	}

	result.FormattedLength = len(formattedTracklist)

	sp.logger.Info("Tracklist formatted",
		slog.String("show_key", showKey),
		slog.String("template", result.Template),
		slog.Int("length", result.FormattedLength))

	if formattedTracklist == "" {
		sp.logger.Error("Formatting produced empty result",
			slog.String("show_key", showKey))
		result.Error = fmt.Errorf("formatting produced empty result")
		return result
	}

	// Handle dry run
	if dryRun {
		fmt.Printf("DRY RUN - Would update %s:\n", showName)
		fmt.Printf("─────────────────────────────────────────\n")
		fmt.Printf("%s\n", formattedTracklist)
		fmt.Printf("─────────────────────────────────────────\n")
		result.Success = true
		return result
	}

	// Verify show exists on Mixcloud with retry logic
	sp.logger.Debug("Verifying show exists on Mixcloud", slog.String("url", showURL))
	_, err = sp.verifyShowWithRetry(showURL, 3)
	if err != nil {
		sp.logger.Error("Show verification failed",
			slog.String("show_key", showKey),
			slog.String("url", showURL),
			slog.String("error", err.Error()))
		result.Error = fmt.Errorf("verifying show exists: %w", err)
		return result
	}

	// Update show description with retry logic
	sp.logger.Info("Updating show description",
		slog.String("show_key", showKey),
		slog.String("url", showURL))
	err = sp.updateShowWithRetry(showURL, formattedTracklist, 3)
	if err != nil {
		sp.logger.Error("Show description update failed",
			slog.String("show_key", showKey),
			slog.String("url", showURL),
			slog.String("error", err.Error()))
		result.Error = fmt.Errorf("updating show description: %w", err)
		return result
	}

	sp.logger.Info("Show description updated successfully",
		slog.String("show_key", showKey),
		slog.String("url", showURL))
	result.Success = true
	return result
}

// generateShowName generates the final show name with date substitution
func (sp *ShowProcessor) generateShowName(showCfg *config.ShowConfig, cueFile string, dateOverride string) (string, error) {
	showName := showCfg.ShowNamePattern
	if showName == "" {
		return "", fmt.Errorf("show_name_pattern is required")
	}

	// Date handling with simple priority:
	// 1. Command line date override (if provided)
	// 2. Current date (default)
	
	var finalDate string
	
	if dateOverride != "" {
		// Parse the date override and reformat according to show's date_format
		if showCfg.DateFormat != "" {
			// Try to parse the input date using various common formats
			parsedDate, err := sp.parseFlexibleDate(dateOverride)
			if err != nil {
				return "", fmt.Errorf("invalid date format '%s': %w", dateOverride, err)
			}
			goLayout := sp.convertDateFormatToGoLayout(showCfg.DateFormat)
			finalDate = parsedDate.Format(goLayout)
		} else {
			// No format specified, use the override as-is
			finalDate = dateOverride
		}
	} else {
		// Use current date with configured format
		if showCfg.DateFormat != "" {
			goLayout := sp.convertDateFormatToGoLayout(showCfg.DateFormat)
			finalDate = time.Now().Format(goLayout)
		} else {
			finalDate = time.Now().Format("01/02/2006")
		}
	}
	
	// Replace the {date} placeholder with the final date
	showName = strings.ReplaceAll(showName, "{date}", finalDate)

	// Replace other placeholders
	showName = strings.ReplaceAll(showName, "{station}", sp.config.Station.Name)

	return showName, nil
}


// convertDateFormatToGoLayout converts user-friendly date formats to Go time layouts
func (sp *ShowProcessor) convertDateFormatToGoLayout(userFormat string) string {
	// Replace user-friendly patterns with Go time reference patterns
	replacer := strings.NewReplacer(
		"YYYY", "2006",  // 4-digit year
		"YY", "06",      // 2-digit year
		"MM", "01",      // 2-digit month with leading zero
		"M", "1",        // 1-2 digit month without leading zero
		"DD", "02",      // 2-digit day with leading zero
		"D", "2",        // 1-2 digit day without leading zero
	)
	
	return replacer.Replace(userFormat)
}

// parseFlexibleDate attempts to parse a date string using various common formats
func (sp *ShowProcessor) parseFlexibleDate(dateStr string) (time.Time, error) {
	// Try various common date formats
	formats := []string{
		"1/2/2006",      // M/D/YYYY
		"01/02/2006",    // MM/DD/YYYY
		"2006-01-02",    // YYYY-MM-DD
		"2006_01_02",    // YYYY_MM_DD
		"20060102",      // YYYYMMDD
		"2-1-2006",      // D-M-YYYY
		"02-01-2006",    // DD-MM-YYYY
		"2006.01.02",    // YYYY.MM.DD
		"Jan 2, 2006",   // Mon D, YYYY
		"January 2, 2006", // Month D, YYYY
		"2/1/2006",      // D/M/YYYY
		"02/01/2006",    // DD/MM/YYYY
		"2006/01/02",    // YYYY/MM/DD
	}
	
	for _, format := range formats {
		if parsedDate, err := time.Parse(format, dateStr); err == nil {
			return parsedDate, nil
		}
	}
	
	return time.Time{}, fmt.Errorf("unable to parse date string '%s' - try formats like MM/DD/YYYY, M/D/YYYY, or YYYYMMDD", dateStr)
}

// printSingleResult displays results for single show processing
func (sp *ShowProcessor) printSingleResult(result ProcessingResult) {
	fmt.Printf("\n")
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	
	if result.Error != nil {
		fmt.Printf("❌ Failed: %s\n", result.ShowKey)
		fmt.Printf("Error: %v\n", result.Error)
	} else if result.Success {
		fmt.Printf("✅ Success: %s\n", result.ShowKey)
		fmt.Printf("Show: %s\n", result.ShowName)
		fmt.Printf("URL: %s\n", result.ShowURL)
		fmt.Printf("Tracks: %d/%d included (%.0f%%)\n", 
			result.FilteredTracks, result.ParsedTracks,
			float64(result.FilteredTracks)/float64(result.ParsedTracks)*100)
		fmt.Printf("Template: %s\n", result.Template)
		fmt.Printf("Length: %d characters\n", result.FormattedLength)
	}
	
	fmt.Printf("Duration: %.1fs\n", result.Duration.Seconds())
	
	if result.DryRun {
		fmt.Printf("\nDry run complete. Use --dry-run=false to apply changes.\n")
	}
	
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
}

// printBatchSummary displays summary for batch processing
func (sp *ShowProcessor) printBatchSummary(result *BatchResult) {
	fmt.Printf("\n")
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Printf("Batch Processing Summary\n")
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Printf("Total Shows: %d\n", result.TotalShows)
	fmt.Printf("Successful: %d\n", result.SuccessfulShows)
	fmt.Printf("Failed: %d\n", result.FailedShows)
	fmt.Printf("Skipped: %d\n", result.SkippedShows)
	fmt.Printf("Duration: %.1fs\n", result.TotalDuration.Seconds())
	
	if result.FailedShows > 0 {
		fmt.Printf("\nFailed Shows:\n")
		for _, res := range result.Results {
			if res.Error != nil {
				fmt.Printf("• %s: %v\n", res.ShowKey, res.Error)
			}
		}
	}
	
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
}

// verifyShowWithRetry attempts to verify a show exists with exponential backoff retry
func (sp *ShowProcessor) verifyShowWithRetry(showURL string, maxRetries int) (interface{}, error) {
	for attempt := 1; attempt <= maxRetries; attempt++ {
		show, err := sp.mixcloud.GetShow(showURL)
		if err == nil {
			return show, nil
		}

		// Check if this is a retryable error
		if !sp.isRetryableError(err) {
			return nil, err
		}

		if attempt < maxRetries {
			backoffDuration := time.Duration(attempt*attempt) * time.Second
			sp.logger.Warn("Show verification failed, retrying",
				slog.String("url", showURL),
				slog.Int("attempt", attempt),
				slog.Int("max_retries", maxRetries),
				slog.Duration("backoff", backoffDuration),
				slog.String("error", err.Error()))
			time.Sleep(backoffDuration)
		}
	}

	return nil, fmt.Errorf("max retries (%d) exceeded", maxRetries)
}

// updateShowWithRetry attempts to update a show description with exponential backoff retry
func (sp *ShowProcessor) updateShowWithRetry(showURL, description string, maxRetries int) error {
	for attempt := 1; attempt <= maxRetries; attempt++ {
		err := sp.mixcloud.UpdateShowDescription(showURL, description)
		if err == nil {
			return nil
		}

		// Check if this is a retryable error
		if !sp.isRetryableError(err) {
			return err
		}

		if attempt < maxRetries {
			backoffDuration := time.Duration(attempt*attempt) * time.Second
			sp.logger.Warn("Show update failed, retrying",
				slog.String("url", showURL),
				slog.Int("attempt", attempt),
				slog.Int("max_retries", maxRetries),
				slog.Duration("backoff", backoffDuration),
				slog.String("error", err.Error()))
			time.Sleep(backoffDuration)
		}
	}

	return fmt.Errorf("max retries (%d) exceeded", maxRetries)
}

// isRetryableError determines if an error is worth retrying
func (sp *ShowProcessor) isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	errStr := strings.ToLower(err.Error())
	
	// Network-related errors that might be transient
	retryablePatterns := []string{
		"timeout",
		"connection refused",
		"temporary",
		"network is unreachable",
		"no such host",
		"502", "503", "504", // HTTP server errors
		"rate limit",
		"too many requests",
	}

	for _, pattern := range retryablePatterns {
		if strings.Contains(errStr, pattern) {
			return true
		}
	}

	// Don't retry authentication errors or client errors (4xx except 429)
	nonRetryablePatterns := []string{
		"unauthorized",
		"forbidden",
		"not found",
		"bad request",
		"401", "403", "404", "400",
	}

	for _, pattern := range nonRetryablePatterns {
		if strings.Contains(errStr, pattern) {
			return false
		}
	}

	// Default to not retrying unknown errors
	return false
}