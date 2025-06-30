# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a Go application that parses CUE sheet files (used by radio automation software like Myriad) and automatically updates Mixcloud show descriptions with formatted tracklists. The application filters out station IDs, commercials, and other non-music content before generating clean tracklists.

## Essential Commands

### Development Workflow
```bash
# Build the application
go build ./cmd/mixcloud-updater

# Run tests for all packages
go test ./...

# Run tests for specific package
go test ./internal/cue -v
go test ./internal/filter -v
go test ./internal/formatter -v
go test ./internal/mixcloud -v

# Build for different platforms
make build-windows    # Windows executable
make build-macos      # macOS executable
make build-all        # Both platforms
make clean            # Remove build artifacts

# Run the application
./mixcloud-updater -cue-file MYR04137.cue -show-name "The Newer New Wave Show" -config config.toml
./mixcloud-updater -cue-file MYR04137.cue -show-name "The Newer New Wave Show" -dry-run
```

### CLI Usage
```bash
# Required flags
-cue-file string     # Path to CUE file to parse
-show-name string    # Show name for Mixcloud URL matching

# Optional flags  
-config string       # Config file path (default: config.toml)
-dry-run            # Preview changes without updating Mixcloud
-help               # Show help information
-version            # Show version information
```

## Architecture Overview

The application follows a pipeline architecture with distinct processing stages:

### Core Pipeline Flow
1. **Parse** CUE file → Extract track metadata (timing, artist, title)
2. **Filter** tracks → Remove station IDs, commercials, unwanted content  
3. **Format** tracklist → Generate Mixcloud-compatible description
4. **Upload** → Update Mixcloud show description via OAuth API

### Package Structure

**`internal/cue`** - CUE file parsing
- Handles Myriad-specific CUE format variations
- Parses track metadata: INDEX, PERFORMER, TITLE commands
- Converts MM:SS:FF timing to MM:SS format
- Validates track data and handles malformed files

**`internal/filter`** - Content filtering  
- String-based and regex-based filtering rules
- Default patterns for common radio station content
- Genre-based filtering (e.g., "Sweepers", "Station ID")
- Configurable exclusion lists for artists and titles

**`internal/formatter`** - Tracklist formatting
- Generates format: `MM:SS - "Track Title" by Artist Name`
- Smart truncation at line boundaries (1000 char Mixcloud limit)
- Quote escaping and missing field handling
- Estimates final length before formatting

**`internal/mixcloud`** - Mixcloud API integration
- OAuth 2.0 authentication with automatic token refresh
- URL parsing and cloudcast key extraction
- Rate limiting with exponential backoff and jitter
- Multipart form uploads for description updates

**`internal/config`** - Configuration management
- TOML-based configuration files
- OAuth credentials and station settings
- Filtering rules and excluded content patterns

### Key Data Structures

**Track metadata flow:**
```go
// Raw CUE data
cue.Track {
    Index: 1,
    StartTime: "03:45", 
    Artist: "Artist Name",
    Title: "Song Title",
    Genre: "Synthpop"
}

// After filtering and formatting
"03:45 - \"Song Title\" by Artist Name"
```

**OAuth flow with automatic refresh:**
```go
// Client handles token lifecycle automatically
client := mixcloud.NewClient(config, "config.toml")
show, err := client.GetShow("https://mixcloud.com/user/show-name/")
err = client.UpdateShowDescription(showURL, formattedTracklist)
```

## Configuration

### config.toml Structure
```toml
[station]
name = "Station Name"
mixcloud_username = "your-username"

[oauth]
client_id = "your-client-id"
client_secret = "your-client-secret"
access_token = "auto-updated"
refresh_token = "auto-updated"

[filtering]
excluded_artists = ["Station ID", "Commercial"]
excluded_titles = ["Station Identification"]
excluded_artist_patterns = ["(?i)sweeper", "(?i)promo"]
excluded_title_patterns = ["(?i)advertisement"]

[paths]
cue_file_directory = "/path/to/cue/files"
```

### OAuth Setup
The application requires Mixcloud OAuth credentials. Tokens are automatically refreshed and persisted back to the config file during API operations.

## CUE File Format

The parser handles Myriad-generated CUE files with this structure:
```
PERFORMER "Show Host"
TITLE "Show Title"
FILE "audio.wav" WAV
  TRACK 01 AUDIO
    TITLE "Song Title"
    PERFORMER "Artist Name"
    GENRE "Genre"
    INDEX 01 MM:SS:FF
```

Key parsing features:
- Handles UTF-8 BOM from Windows-generated files
- Supports both album-level and track-level metadata
- Converts MM:SS:FF timing to MM:SS format (drops frame info)
- Processes REM commands for extended metadata

## Development Patterns

### Error Handling
The codebase uses Go's standard error handling with wrapped errors:
```go
if err := parser.processLine(line); err != nil {
    return nil, fmt.Errorf("parsing error in '%s': %w", filename, err)
}
```

### OAuth Integration
The Mixcloud client automatically handles token refresh:
```go
// Token refresh is transparent - just use the client
client, err := mixcloud.NewClient(cfg, "config.toml")
show, err := client.GetShow(showURL)  // May trigger token refresh
```

### Filtering System
Multi-layer filtering with clear precedence:
1. Genre-based exclusions (most specific)
2. Exact string matches
3. Substring contains matching  
4. Regex pattern matching

### Testing Strategy
- Unit tests for each package with table-driven tests
- Mock-based testing for external dependencies
- Integration tests with sample CUE files
- Boundary testing for character limits and edge cases

## Common Development Tasks

### Adding New Filter Rules
1. Update `internal/config/config.go` struct if needed
2. Add filter logic in `internal/filter/filter.go`
3. Update `isExcludedBy*` methods with new rule type
4. Add test cases in `filter_test.go`

### Extending CUE Parser
1. Add new command type to `CueCommand` enum in `internal/cue/parser.go`
2. Implement parsing logic in `parseLine()` method
3. Add handler method in `trackParser` struct
4. Add validation and test cases

### Modifying Output Format
1. Update `formatTrackLine()` in `internal/formatter/formatter.go`
2. Adjust `truncateSmartly()` if format affects length calculations
3. Update tests to match new format expectations

## Dependencies

- `github.com/BurntSushi/toml` - TOML configuration parsing
- `golang.org/x/oauth2` - OAuth 2.0 authentication
- `golang.org/x/text` - Unicode normalization for international characters

The application has minimal external dependencies and uses Go's standard library extensively.

## Production Automation Setup

### OAuth Setup

The application handles OAuth automatically:

1. **First run:**
   ```bash
   ./mixcloud-updater -cue-file show.cue -show-name "My Weekly Show"
   # Browser opens automatically for authorization
   # Tokens are saved to config.toml
   ```

2. **Subsequent runs:**
   ```bash
   # No re-authorization needed - tokens persist
   ./mixcloud-updater -cue-file show.cue -show-name "My Weekly Show"
   ```

### Automation Examples

**Cron Job (Run every 2 hours):**
```bash
# Add to crontab with: crontab -e
0 */2 * * * /path/to/mixcloud-updater -cue-file /radio/shows/latest.cue -show-name "Weekly Show"
```

**Script for Multiple Shows:**
```bash
#!/bin/bash
# Process all CUE files in a directory
for cue_file in /radio/shows/*.cue; do
    show_date=$(date +"%m-%d-%Y")
    ./mixcloud-updater -cue-file "$cue_file" -show-name "Radio Show $show_date"
done
```

**Integration with Radio Software:**
```bash
# Add this to your radio automation software's post-show hook
/path/to/mixcloud-updater -cue-file "$SHOW_CUE_FILE" -show-name "$SHOW_NAME"
```

### Troubleshooting

- **OAuth is automatic** - browser launches when authentication needed
- **Tokens persist** - once authorized, runs unattended
- Use `-dry-run` to test without uploading
- Check logs for CUE parsing or API errors
- Verify show name matches Mixcloud URL format
- To force re-auth: delete `access_token` from config.toml