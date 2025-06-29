# Mixcloud Updater

Automatically updates Mixcloud show descriptions with formatted tracklists from radio automation CUE files. Perfect for radio stations using Myriad or similar automation software.

## Quick Start

### 1. Download and Setup

1. **Download the latest release** for your platform:
   - Windows: `mixcloud-updater.exe`
   - macOS: `mixcloud-updater-macos`

2. **Make executable** (macOS/Linux only):
   ```bash
   chmod +x mixcloud-updater-macos
   ```

3. **Create configuration file**:
   ```bash
   cp config.toml.example config.toml
   ```

### 2. Configure Mixcloud API Access

1. **Get Mixcloud OAuth credentials**:
   - Go to: https://www.mixcloud.com/developers/create/
   - Create a new application
   - Note your `Client ID` and `Client Secret`

2. **Edit config.toml**:
   ```toml
   [station]
   name = "WXYZ Radio"  # Your station name
   mixcloud_username = "wxyzradio"  # Your Mixcloud username

   [oauth]
   client_id = "your_client_id_here"
   client_secret = "your_client_secret_here"
   # access_token and refresh_token will be auto-filled
   ```

3. **First-time OAuth setup**:
   ```bash
   # Test with dry run first
   ./mixcloud-updater -cue-file sample.cue -show-name "Test Show" -dry-run
   ```
   Follow the OAuth prompts to authorize the application.

### 3. Basic Usage

**Test with dry run** (recommended first):
```bash
./mixcloud-updater -cue-file "MYR12345.cue" -show-name "Morning Show" -dry-run
```

**Update live show**:
```bash
./mixcloud-updater -cue-file "MYR12345.cue" -show-name "Morning Show"
```

**Get help**:
```bash
./mixcloud-updater --help
```

### 4. Integration with Myriad

#### Option A: Manual Integration
1. Export your show as a CUE file from Myriad
2. Upload audio to Mixcloud manually
3. Run the updater to add the tracklist

#### Option B: Automated Integration
Create a batch script that Myriad can call after show upload:

**Windows (update_mixcloud.bat)**:
```batch
@echo off
cd "C:\path\to\mixcloud-updater"
mixcloud-updater.exe -cue-file "%1" -show-name "%2"
if %errorlevel% neq 0 (
    echo ERROR: Failed to update Mixcloud description
    pause
)
```

**macOS/Linux (update_mixcloud.sh)**:
```bash
#!/bin/bash
cd "/path/to/mixcloud-updater"
./mixcloud-updater-macos -cue-file "$1" -show-name "$2"
if [ $? -ne 0 ]; then
    echo "ERROR: Failed to update Mixcloud description"
    exit 1
fi
```

## Configuration Reference

### Station Settings
```toml
[station]
name = "Your Station Name"           # For logging/identification
mixcloud_username = "your_username"  # From your Mixcloud profile URL
```

### OAuth Credentials
```toml
[oauth]
client_id = "..."     # From Mixcloud developers console
client_secret = "..." # From Mixcloud developers console
# Tokens auto-managed - don't edit manually
access_token = ""
refresh_token = ""
```

### Content Filtering

The application automatically filters out non-music content. Customize these lists:

```toml
[filtering]
# Exact artist/title matches (case-insensitive)
excluded_artists = ["Station ID", "Commercial"]
excluded_titles = ["News Update", "Weather Report"]

# Regular expression patterns (case-insensitive)
excluded_artist_patterns = ["(?i)sweeper", "(?i)promo"]
excluded_title_patterns = ["(?i)advertisement", "(?i)sponsored"]
```

## Command Line Options

| Flag | Description | Required | Default |
|------|-------------|----------|---------|
| `-cue-file` | Path to CUE file to parse | ✅ | - |
| `-show-name` | Show name for Mixcloud URL matching | ✅ | - |
| `-config` | Configuration file path | ❌ | `config.toml` |
| `-dry-run` | Preview changes without updating | ❌ | `false` |
| `-version` | Show version information | ❌ | - |
| `-help` | Show help message | ❌ | - |

## Troubleshooting

### Common Issues

**"CUE file not found"**
- Check the file path is correct
- Ensure file has `.cue` extension
- Verify file permissions

**"Show not found on Mixcloud"**
- Verify the show exists and is public
- Check your `mixcloud_username` in config
- Ensure show name matches exactly (try with quotes)

**"OAuth authentication failed"**
- Verify `client_id` and `client_secret` are correct
- Check internet connection
- Try deleting `access_token` and `refresh_token` from config to re-authenticate

**"No tracks remaining after filtering"**
- Check your filtering rules aren't too aggressive
- Use `-dry-run` to see what's being filtered
- Review `excluded_artists` and `excluded_titles` lists

### Getting Help

**View detailed output**:
```bash
./mixcloud-updater -cue-file file.cue -show-name "Show" -dry-run
```

**Check configuration**:
```bash
# The app will validate and show config details on startup
./mixcloud-updater -cue-file file.cue -show-name "Show"
```

**Environment Variables** (override config file):
```bash
export NWRMIXCLOUD_OAUTH_ACCESS_TOKEN="your_token"
export NWRMIXCLOUD_OAUTH_REFRESH_TOKEN="your_refresh_token"
```

## Example Output

```
Mixcloud Updater v1.0.0
=================================

Configuration:
  CUE File: MYR12345.cue
  Show Name: Morning Show
  Config: config.toml
  Dry Run: true

Loading configuration...
Station: WXYZ Radio
Mixcloud Username: wxyzradio
OAuth Tokens: configured
Configuration loaded successfully.

Step 1: Parsing CUE file...
✓ Parsed 25 tracks from CUE file (0.12s)

Step 2: Initializing content filter...
✓ Content filter initialized (0.01s)

Step 3: Filtering tracks...
✓ Filtered tracks: 18 included, 7 excluded (0.02s)

Step 4: Formatting tracklist...
✓ Tracklist formatted (892 characters, 0.01s)

Step 5: Initializing Mixcloud client...
✓ Mixcloud client initialized (0.15s)

Step 6: Locating Mixcloud show...
Generated show URL: https://www.mixcloud.com/wxyzradio/morning-show/
✓ Dry run mode - skipping show verification (0.01s)

Step 7: Updating show description...
DRY RUN MODE - Would update show with:
─────────────────────────────────────────
06:15 - "Good Morning" by Artist Name
08:23 - "Wake Up Song" by Another Artist
...
─────────────────────────────────────────
✓ Dry run completed - no changes made (0.00s)

═══════════════════════════════════════════
             WORKFLOW SUMMARY              
═══════════════════════════════════════════
Status: ✓ SUCCESS
Mode: DRY RUN (no changes made)

Track Processing:
  • Parsed from CUE: 25 tracks
  • Included after filtering: 18 tracks
  • Excluded by filters: 7 tracks
  • Inclusion rate: 72.0%

Output:
  • Formatted tracklist: 892 characters
  • Target show URL: https://www.mixcloud.com/wxyzradio/morning-show/

Performance Timing:
  • Total execution time: 0.32s

💡 This was a dry run. To apply changes, run again without --dry-run
═══════════════════════════════════════════
✓ Done!
```

## Building from Source

Requirements:
- Go 1.19 or later

```bash
# Clone repository
git clone https://github.com/nowwaveradio/mixcloud-updater.git
cd mixcloud-updater

# Install dependencies
go mod download

# Build for current platform
go build ./cmd/mixcloud-updater

# Build for all platforms
make build-all
```

## License

[Add your license information here]

## Support

For issues and feature requests, please open an issue on GitHub.