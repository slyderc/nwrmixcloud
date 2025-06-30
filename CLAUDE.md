# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Go application that parses CUE sheet files (used by radio automation software like Myriad) and automatically updates Mixcloud show descriptions with formatted tracklists. Uses a unified config-driven architecture supporting batch and single-show processing with flexible templating.

## Development Commands

```bash
# Build and test
go build ./cmd/mixcloud-updater
go test ./...
go test ./internal/config -v     # Test specific package

# Platform builds
make build-windows
make build-macos 
make build-all

# Run application
./mixcloud-updater config.toml              # Process all enabled shows
./mixcloud-updater -show "alias" config.toml # Process specific show
./mixcloud-updater -dry-run config.toml     # Preview without updating
```

## Architecture Overview

### Core Pipeline Flow (Per Show)
1. **Resolve** show configuration → Find show by name/alias with CUE file detection
2. **Parse** CUE file → Extract track metadata (timing, artist, title)
3. **Filter** tracks → Remove station IDs, commercials, unwanted content  
4. **Format** tracklist → Generate Mixcloud-compatible description with templates
5. **Upload** → Update Mixcloud show description via OAuth API with retry logic

### Key Features
- **Config-Driven**: TOML configuration with shows, templates, and processing options
- **Show Management**: Aliases, CUE file patterns, template selection, priority ordering
- **Template System**: Go text/template with custom functions and smart truncation
- **Batch Processing**: Process multiple shows with structured logging and error handling
- **Retry Logic**: Exponential backoff for transient API failures

## Package Structure

**`cmd/mixcloud-updater`** - CLI entry point with unified interface

**`internal/config`** - Configuration management
- TOML parsing with shows, templates, processing sections
- Environment variable overrides and validation
- OAuth credentials handling

**`internal/shows`** - Show resolution and management  
- Alias-based lookup with case-insensitive matching
- CUE file pattern matching and latest file detection
- Show validation and conflict detection

**`internal/processor`** - Core orchestration engine
- ShowProcessor coordinates complete workflow
- Batch and single-show processing modes
- Structured logging with contextual information
- Retry logic with exponential backoff

**`internal/template`** - Template engine
- Go text/template with custom functions (upper, lower, truncate, repeat)
- Show-specific template selection hierarchy
- Smart truncation with header/footer preservation

**`internal/cue`** - CUE file parsing
- Myriad-specific format handling with UTF-8 BOM support
- INDEX, PERFORMER, TITLE command parsing
- MM:SS:FF to MM:SS time conversion

**`internal/filter`** - Content filtering
- Multi-layer filtering: genre, exact match, substring, regex
- Default patterns for radio station content
- Configurable exclusion lists

**`internal/formatter`** - Tracklist formatting
- Classic format: `MM:SS - "Track Title" by Artist Name`
- Template-based formatting with show configuration
- Smart truncation at line boundaries (1000 char Mixcloud limit)

**`internal/mixcloud`** - Mixcloud API integration
- OAuth 2.0 with automatic token refresh
- Rate limiting with exponential backoff
- Multipart form uploads for description updates

## Key Data Structures

```go
// Track metadata flow
cue.Track {
    Index: 1,
    StartTime: "03:45", 
    Artist: "Artist Name",
    Title: "Song Title",
    Genre: "Synthpop"
}

// Show configuration
config.ShowConfig {
    CueFilePattern:  "MYR*.cue",
    ShowNamePattern: "Show - {date}",
    Aliases:         []string{"alias1", "alias2"},
    TemplateName:    "detailed",
    Enabled:         true,
    Priority:        1
}

// Processing result with comprehensive reporting
processor.ProcessingResult {
    ShowKey:         string,
    Success:         bool,
    Error:           error,
    ParsedTracks:    int,
    FilteredTracks:  int,
    FormattedLength: int,
    Duration:        time.Duration
}
```

## Development Patterns

### Error Handling
Standard Go error handling with wrapped errors:
```go
if err := parser.processLine(line); err != nil {
    return nil, fmt.Errorf("parsing error in '%s': %w", filename, err)
}
```

### Filtering System Precedence
1. Genre-based exclusions (most specific)
2. Exact string matches
3. Substring contains matching  
4. Regex pattern matching

### Template Selection Hierarchy
1. Custom inline template (`custom_template`)
2. Named template reference (`template_name`) 
3. Default template from config
4. Classic fallback

### OAuth Integration
Automatic token refresh handled transparently:
```go
client, err := mixcloud.NewClient(cfg, "config.toml")
show, err := client.GetShow(showURL)  // May trigger token refresh
```

### Structured Logging
Uses slog for production monitoring:
```go
sp.logger.Info("Processing show",
    slog.String("show_key", showKey),
    slog.Bool("dry_run", dryRun),
    slog.Int("track_count", trackCount))
```

## Common Development Tasks

### Adding New Filter Rules
1. Update `internal/config/config.go` struct
2. Add filter logic in `internal/filter/filter.go`
3. Update `isExcludedBy*` methods
4. Add test cases in `filter_test.go`

### Extending CUE Parser
1. Add command type to `internal/cue/parser.go`
2. Implement parsing logic in `parseLine()` method
3. Add handler method in `trackParser` struct
4. Add validation and test cases

### Adding Template Functions
1. Define function in `internal/template/engine.go`
2. Add to `funcMap` in template creation
3. Update template tests
4. Document in template examples

### Modifying Show Processing
1. Update `ShowProcessor.processingleShow()` in `internal/processor/show_processor.go`
2. Add structured logging for new steps
3. Update `ProcessingResult` struct if needed
4. Add integration tests

## Testing Strategy

- **Unit Tests**: Table-driven tests for each package
- **Integration Tests**: Sample CUE files and mock dependencies  
- **Boundary Tests**: Character limits and edge cases
- **Config Tests**: Cross-platform path handling with `filepath.Join`

## Dependencies

- `github.com/BurntSushi/toml` - TOML configuration parsing
- `golang.org/x/oauth2` - OAuth 2.0 authentication
- `golang.org/x/text` - Unicode normalization

Minimal external dependencies, uses Go standard library extensively.