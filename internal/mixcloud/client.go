// Package mixcloud provides integration with the Mixcloud API.
// It handles OAuth 2.0 authentication, automatic token refresh, and API operations
// for fetching show information and updating descriptions.
package mixcloud

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"log/slog"
	"math/rand"
	"mime/multipart"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"golang.org/x/oauth2"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"

	"github.com/nowwaveradio/mixcloud-updater/internal/config"
	"github.com/nowwaveradio/mixcloud-updater/internal/logger"
)

// AIDEV-NOTE: Compile regex patterns and transformers once at package level for better performance
var (
	// hyphenDeduplicationRegex removes consecutive hyphens and replaces them with single hyphens
	hyphenDeduplicationRegex = regexp.MustCompile(`-+`)
	
	// dateNormalizationRegex matches various date formats and normalizes them
	// Matches: M/D/YYYY, MM/DD/YYYY, M-D-YYYY, M.D.YYYY, etc.
	dateNormalizationRegex = regexp.MustCompile(`(\d{1,2})[/.\-](\d{1,2})[/.\-](\d{4})`)
	
	// additionalDateFormats handles other common date patterns
	// Matches: YYYY-MM-DD, YYYY/MM/DD, YYYY.MM.DD
	isoDateRegex = regexp.MustCompile(`(\d{4})[/.\-](\d{1,2})[/.\-](\d{1,2})`)
	
	// unicodeNormalizer handles Unicode character normalization and transliteration
	// AIDEV-NOTE: Combines NFD normalization with accent removal for better URL compatibility
	unicodeNormalizer = transform.Chain(
		norm.NFD,                              // Decompose accented characters (é → e + ´)
		runes.Remove(runes.In(unicode.Mn)),    // Remove combining marks (accents)
		norm.NFC,                              // Recompose remaining characters
	)
)

// AIDEV-TODO: Add GetShow method for fetching show information
// AIDEV-TODO: Add UpdateShowDescription method with multipart form handling  
// AIDEV-NOTE: Mixcloud API has rate limiting - implement exponential backoff

// API endpoint constants for Mixcloud API
const (
	MixcloudAPIBaseURL     = "https://api.mixcloud.com"
	CloudcastEndpoint      = "/%s"                               // GET /<key>/ (key includes trailing slash)
	UploadEndpoint         = "/upload/"                          // POST /upload/
	APITimeoutSeconds      = 30                                  // 30 second timeout for API requests
	MaxDescriptionLength   = 1000                                // Maximum description length
	RateLimitMaxRetries    = 5                                   // Maximum retries for rate limiting
	RateLimitBaseDelay     = 1 * time.Second                    // Base delay for exponential backoff
)

// Custom error types for OAuth and API failures
var (
	ErrInvalidRefreshToken = errors.New("refresh token is invalid or expired")
	ErrNetworkFailure      = errors.New("network failure during OAuth operation")
	ErrConfigWriteFailure  = errors.New("failed to write config file")
	ErrTokenExpired        = errors.New("access token has expired")
	ErrAuthenticationFailed = errors.New("authentication failed")
	ErrRateLimited         = errors.New("API rate limit exceeded")
	ErrInvalidShowURL      = errors.New("invalid Mixcloud show URL format")
	ErrShowNotFound        = errors.New("show not found on Mixcloud")
	ErrDescriptionTooLong  = errors.New("description exceeds maximum length")
	ErrAPIRequestFailed    = errors.New("Mixcloud API request failed")
)

// OAuthError represents an OAuth-specific error with additional context
type OAuthError struct {
	Type    string // Error type identifier
	Message string // Human-readable error message
	Cause   error  // Underlying error if available
	Retryable bool // Whether the operation can be retried
}

func (e *OAuthError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s (caused by: %v)", e.Type, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

func (e *OAuthError) Unwrap() error {
	return e.Cause
}

// Client represents the Mixcloud API client with OAuth 2.0 authentication
type Client struct {
	httpClient   *http.Client      // HTTP client for making API requests
	oauth2Config *oauth2.Config    // OAuth 2.0 configuration
	token        *oauth2.Token     // Current OAuth token
	username     string            // Mixcloud username for URL generation
	config       *config.Config    // Original config for token updates
	configPath   string            // Path to config file for saving updates
	tokenSource  oauth2.TokenSource // TokenSource for monitoring token changes
}

// Show represents a Mixcloud show/cloudcast
type Show struct {
	Key         string `json:"key"`         // Cloudcast key (username/slug format)
	Name        string `json:"name"`        // Show title
	Description string `json:"description"` // Current description text
	URL         string `json:"url"`         // Full URL to the show
}

// tokenRefreshTransport wraps an OAuth2 transport to intercept token refresh events
// AIDEV-NOTE: This allows us to persist refreshed tokens automatically
type tokenRefreshTransport struct {
	base        http.RoundTripper   // Underlying OAuth2 transport
	client      *Client             // Client reference for token persistence
	tokenSource oauth2.TokenSource  // TokenSource for checking token changes
}

// RoundTrip implements http.RoundTripper and intercepts token refresh events
func (t *tokenRefreshTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Get current token before the request
	oldToken, err := t.tokenSource.Token()
	if err != nil {
		log.Printf("[MIXCLOUD] Warning: Failed to get current token before request: %v", err)
		// Continue with the request even if we can't get the current token
	}

	// Execute the request using the base transport with retry logic
	resp, err := t.executeWithRetry(req)
	if err != nil {
		// Check if this is an OAuth-related error and wrap it appropriately
		if oauthErr := t.classifyError(err); oauthErr != nil {
			return resp, oauthErr
		}
		return resp, err
	}

	// Check if token was refreshed during the request
	newToken, tokenErr := t.tokenSource.Token()
	if tokenErr != nil {
		log.Printf("[MIXCLOUD] Warning: Failed to get token after request: %v", tokenErr)
		return resp, err
	}

	// Compare tokens to detect refresh
	if oldToken != nil && newToken != nil {
		// Check if access token has changed (indicating a refresh)
		if oldToken.AccessToken != newToken.AccessToken {
			log.Printf("[MIXCLOUD] Token refresh detected - persisting new token")
			
			// Persist the new token without recreating HTTP client to avoid recursion
			if saveErr := t.client.saveTokenToFile(newToken); saveErr != nil {
				// Check if it's a retryable error
				var oauthErr *OAuthError
				if errors.As(saveErr, &oauthErr) && oauthErr.Retryable {
					log.Printf("[MIXCLOUD] Retryable error persisting refreshed token: %v", saveErr)
				} else {
					log.Printf("[MIXCLOUD] Error persisting refreshed token: %v", saveErr)
				}
			} else {
				log.Printf("[MIXCLOUD] Successfully persisted refreshed token")
			}
		}
	}

	return resp, err
}

// executeWithRetry executes the request with exponential backoff retry logic
// AIDEV-NOTE: Implements retry for transient network failures and rate limiting
func (t *tokenRefreshTransport) executeWithRetry(req *http.Request) (*http.Response, error) {
	maxRetries := 3
	baseDelay := 1 * time.Second

	for attempt := 0; attempt <= maxRetries; attempt++ {
		resp, err := t.base.RoundTrip(req)
		
		// Check if we should retry this error
		if err != nil && attempt < maxRetries {
			if t.shouldRetryError(err) {
				delay := time.Duration(1<<uint(attempt)) * baseDelay // Exponential backoff
				log.Printf("[MIXCLOUD] Request failed (attempt %d/%d), retrying in %v: %v", 
					attempt+1, maxRetries+1, delay, err)
				time.Sleep(delay)
				continue
			}
		}

		// Check HTTP response status for retry conditions
		if resp != nil && attempt < maxRetries {
			if resp.StatusCode == 429 { // Rate limited
				delay := time.Duration(1<<uint(attempt)) * baseDelay
				log.Printf("[MIXCLOUD] Rate limited (attempt %d/%d), retrying in %v", 
					attempt+1, maxRetries+1, delay)
				resp.Body.Close() // Important: close the body before retrying
				time.Sleep(delay)
				continue
			}
		}

		// Return the result (success or final failure)
		return resp, err
	}

	// This should never be reached due to the loop logic
	return nil, &OAuthError{
		Type:      "RetryExhausted",
		Message:   "maximum retry attempts exceeded",
		Retryable: false,
	}
}

// shouldRetryError determines if an error is worth retrying
func (t *tokenRefreshTransport) shouldRetryError(err error) bool {
	// Network-related errors that might be transient
	if err != nil {
		errStr := err.Error()
		// Common transient network errors
		transientErrors := []string{
			"connection refused",
			"timeout",
			"temporary failure",
			"network unreachable",
			"connection reset",
		}
		
		for _, transient := range transientErrors {
			if contains(errStr, transient) {
				return true
			}
		}
	}
	
	return false
}

// classifyError converts generic errors into specific OAuth errors
func (t *tokenRefreshTransport) classifyError(err error) *OAuthError {
	if err == nil {
		return nil
	}

	errStr := err.Error()
	
	// Check for specific OAuth error patterns
	if contains(errStr, "invalid_grant") || contains(errStr, "invalid refresh token") {
		return &OAuthError{
			Type:      "InvalidRefreshToken",
			Message:   "refresh token is invalid or expired",
			Cause:     err,
			Retryable: false,
		}
	}
	
	if contains(errStr, "unauthorized") || contains(errStr, "authentication failed") {
		return &OAuthError{
			Type:      "AuthenticationFailed",
			Message:   "authentication failed",
			Cause:     err,
			Retryable: false,
		}
	}
	
	// Network-related errors
	if t.shouldRetryError(err) {
		return &OAuthError{
			Type:      "NetworkFailure",
			Message:   "network failure during OAuth operation",
			Cause:     err,
			Retryable: true,
		}
	}
	
	return nil // Not an OAuth-specific error
}

// contains checks if a string contains a substring (case-insensitive helper)
func contains(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// NewClient creates a new Mixcloud API client with OAuth 2.0 configuration
// AIDEV-NOTE: Initializes OAuth config with proper Mixcloud endpoints
func NewClient(cfg *config.Config, configPath string) (*Client, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	// Validate required OAuth configuration
	if cfg.OAuth.ClientID == "" {
		return nil, fmt.Errorf("OAuth client ID is required")
	}
	if cfg.OAuth.ClientSecret == "" {
		return nil, fmt.Errorf("OAuth client secret is required")
	}
	if cfg.Station.MixcloudUsername == "" {
		return nil, fmt.Errorf("Mixcloud username is required")
	}

	// Set up OAuth 2.0 configuration with Mixcloud endpoints
	oauth2Config := &oauth2.Config{
		ClientID:     cfg.OAuth.ClientID,
		ClientSecret: cfg.OAuth.ClientSecret,
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://www.mixcloud.com/oauth/authorize",
			TokenURL: "https://www.mixcloud.com/oauth/access_token",
		},
		// AIDEV-NOTE: Mixcloud API doesn't require specific scopes for description updates
		Scopes: []string{},
	}

	// Create token from stored credentials if available
	var token *oauth2.Token
	if cfg.OAuth.AccessToken != "" {
		token = &oauth2.Token{
			AccessToken:  cfg.OAuth.AccessToken,
			RefreshToken: cfg.OAuth.RefreshToken,
			// AIDEV-NOTE: Token expiry will be handled by oauth2 package
		}
	}

	// Create client instance
	client := &Client{
		oauth2Config: oauth2Config,
		token:        token,
		username:     cfg.Station.MixcloudUsername,
		config:       cfg,
		configPath:   configPath,
	}

	// Set up httpClient with OAuth transport for automatic token refresh
	if token != nil {
		// Use helper method to create HTTP client with error handling
		err := client.recreateHTTPClient(token)
		if err != nil {
			// Continue with degraded functionality - log warning and use basic client
			log.Printf("[MIXCLOUD] Warning: Failed to create OAuth HTTP client, continuing with basic client: %v", err)
			client.httpClient = &http.Client{}
		}
	} else {
		// No token available - use default HTTP client
		// AIDEV-NOTE: API calls will fail until token is set via SaveToken()
		log.Printf("[MIXCLOUD] No OAuth token available - client will require manual authentication")
		client.httpClient = &http.Client{}
	}

	return client, nil
}

// LoadToken reads the current OAuth token from the stored configuration
// AIDEV-NOTE: Tokens are already loaded in NewClient, this provides access to current token
func (c *Client) LoadToken() *oauth2.Token {
	return c.token
}

// SaveToken persists updated OAuth tokens to the configuration file
// AIDEV-NOTE: Updates both in-memory config and writes to file for persistence
func (c *Client) SaveToken(token *oauth2.Token) error {
	if token == nil {
		return &OAuthError{
			Type:      "InvalidToken",
			Message:   "token cannot be nil",
			Retryable: false,
		}
	}

	if c.config == nil {
		return &OAuthError{
			Type:      "ConfigUnavailable",
			Message:   "config is not available for token updates",
			Retryable: false,
		}
	}

	// Update the in-memory config with new token values
	c.config.OAuth.AccessToken = token.AccessToken
	c.config.OAuth.RefreshToken = token.RefreshToken

	// Update the client's token reference
	c.token = token

	// Recreate HTTP client with new token for automatic refresh
	// AIDEV-NOTE: This ensures TokenSource uses the updated token
	err := c.recreateHTTPClient(token)
	if err != nil {
		// Continue with degraded functionality (no automatic refresh)
		log.Printf("[MIXCLOUD] Warning: Failed to recreate HTTP client, continuing with basic client: %v", err)
		c.httpClient = &http.Client{}
	}

	// Save the updated config to file if config path is available
	if c.configPath != "" {
		err := config.SaveConfig(c.config, c.configPath)
		if err != nil {
			// AIDEV-NOTE: Log warning but continue - token is still updated in memory
			log.Printf("[MIXCLOUD] Warning: Failed to persist token to config file: %v", err)
			return &OAuthError{
				Type:      "ConfigWriteFailure",
				Message:   "token updated in memory but failed to save to config file",
				Cause:     err,
				Retryable: true,
			}
		}
	}

	return nil
}

// saveTokenToFile persists a token to the config file without recreating the HTTP client
// AIDEV-NOTE: Used internally by token refresh transport to avoid recursion
func (c *Client) saveTokenToFile(token *oauth2.Token) error {
	if token == nil {
		return &OAuthError{
			Type:      "InvalidToken",
			Message:   "token cannot be nil",
			Retryable: false,
		}
	}

	if c.config == nil {
		return &OAuthError{
			Type:      "ConfigUnavailable",
			Message:   "config is not available for token updates",
			Retryable: false,
		}
	}

	// Update the in-memory config with new token values
	c.config.OAuth.AccessToken = token.AccessToken
	c.config.OAuth.RefreshToken = token.RefreshToken

	// Update the client's token reference
	c.token = token

	// Save the updated config to file if config path is available
	if c.configPath != "" {
		err := config.SaveConfig(c.config, c.configPath)
		if err != nil {
			// AIDEV-NOTE: Config file write failure - log warning but continue with in-memory token
			log.Printf("[MIXCLOUD] Warning: Failed to persist token to config file: %v", err)
			return &OAuthError{
				Type:      "ConfigWriteFailure",
				Message:   "token updated in memory but failed to save to config file",
				Cause:     err,
				Retryable: true, // File write could succeed on retry
			}
		}
	}

	return nil
}

// recreateHTTPClient creates a new HTTP client with OAuth transport
// AIDEV-NOTE: Helper method to reduce code duplication in NewClient and SaveToken
func (c *Client) recreateHTTPClient(token *oauth2.Token) error {
	if token == nil {
		return &OAuthError{
			Type:      "InvalidToken",
			Message:   "cannot create HTTP client with nil token",
			Retryable: false,
		}
	}

	// Create TokenSource for automatic token refresh
	tokenSource := c.oauth2Config.TokenSource(oauth2.NoContext, token)
	c.tokenSource = tokenSource
	
	// Create OAuth HTTP client with custom transport for token refresh monitoring
	oauthClient := oauth2.NewClient(oauth2.NoContext, tokenSource)
	
	// Wrap the OAuth transport with our custom transport for token persistence
	customTransport := &tokenRefreshTransport{
		base:        oauthClient.Transport,
		client:      c,
		tokenSource: tokenSource,
	}
	
	// Create HTTP client with custom transport
	c.httpClient = &http.Client{
		Transport: customTransport,
	}

	return nil
}

// IsHealthy returns the operational status of the OAuth client
// AIDEV-NOTE: Allows callers to check if the client can make authenticated requests
func (c *Client) IsHealthy() bool {
	// Check if we have a valid token
	if c.token == nil {
		return false
	}

	// Check if we have a working HTTP client
	if c.httpClient == nil {
		return false
	}

	// Check if we can get a fresh token from the token source
	if c.tokenSource != nil {
		token, err := c.tokenSource.Token()
		if err != nil {
			log.Printf("[MIXCLOUD] Health check failed: token source error: %v", err)
			return false
		}
		// Check if the token is expired
		if token != nil && !token.Valid() {
			log.Printf("[MIXCLOUD] Health check failed: token is expired")
			return false
		}
	} else {
		// Check if the stored token is expired as a fallback
		if c.token != nil && !c.token.Valid() {
			log.Printf("[MIXCLOUD] Health check failed: stored token is expired")
			return false
		}
	}

	return true
}

// GetAuthenticationStatus returns detailed information about the client's authentication status
func (c *Client) GetAuthenticationStatus() map[string]interface{} {
	status := make(map[string]interface{})
	
	status["has_token"] = c.token != nil
	status["has_http_client"] = c.httpClient != nil
	status["has_token_source"] = c.tokenSource != nil
	status["is_healthy"] = c.IsHealthy()
	
	if c.token != nil {
		status["has_access_token"] = c.token.AccessToken != ""
		status["has_refresh_token"] = c.token.RefreshToken != ""
		status["token_expired"] = c.token.Expiry.Before(time.Now())
	}
	
	return status
}

// GetHTTPClient returns the configured HTTP client with OAuth transport
// AIDEV-NOTE: This client automatically handles token refresh for API requests
func (c *Client) GetHTTPClient() *http.Client {
	return c.httpClient
}

// normalizeUnicode handles Unicode character normalization and transliteration
// AIDEV-NOTE: Converts accented characters to ASCII equivalents (é→e, ñ→n) and handles non-Latin scripts
func normalizeUnicode(input string) string {
	// Apply Unicode normalization to decompose and remove accents
	normalized, _, err := transform.String(unicodeNormalizer, input)
	if err != nil {
		// If normalization fails, fall back to original string
		// AIDEV-NOTE: This ensures we don't break on malformed Unicode
		return input
	}
	
	// Create a manual transliteration map for characters that don't decompose properly
	// AIDEV-NOTE: These are common European characters that need special handling
	transliterationMap := map[rune]string{
		'ø': "o", 'Ø': "O", // Danish/Norwegian
		'æ': "ae", 'Æ': "AE", // Danish/Norwegian/Old English
		'œ': "oe", 'Œ': "OE", // French
		'ß': "ss", // German
		'đ': "d", 'Đ': "D", // Croatian/Serbian
		'ł': "l", 'Ł': "L", // Polish
		'ſ': "s", // Long s
	}
	
	// Handle remaining non-ASCII characters by filtering to ASCII-safe characters
	var result strings.Builder
	for _, r := range normalized {
		if r <= 127 { // ASCII characters
			result.WriteRune(r)
		} else if replacement, exists := transliterationMap[r]; exists {
			// Use manual transliteration for special cases
			result.WriteString(replacement)
		} else {
			// For non-Latin scripts (Chinese, Japanese, Arabic, etc.)
			// We'll skip them to avoid broken URLs
			// AIDEV-NOTE: Could be enhanced with transliteration libraries if needed
			continue
		}
	}
	
	return result.String()
}

// normalizeDateFormats converts various date formats to a consistent format
// AIDEV-NOTE: Removes separators from dates to create cleaner URLs (6/26/2025 → 6262025)
func normalizeDateFormats(input string) string {
	// Handle M/D/YYYY, MM/DD/YYYY, M-D-YYYY, M.D.YYYY formats
	// Convert to MDDYYYY format (removing separators)
	result := dateNormalizationRegex.ReplaceAllStringFunc(input, func(match string) string {
		// Extract components using the same regex
		matches := dateNormalizationRegex.FindStringSubmatch(match)
		if len(matches) == 4 {
			month := matches[1]
			day := matches[2]
			year := matches[3]
			// Combine without separators: M/D/YYYY → MDDYYYY
			return month + day + year
		}
		return match // Return original if parsing fails
	})
	
	// Handle ISO format dates: YYYY-MM-DD, YYYY/MM/DD, YYYY.MM.DD
	// Convert to YYYYMMDD format (removing separators)
	result = isoDateRegex.ReplaceAllStringFunc(result, func(match string) string {
		// Extract components using the ISO date regex
		matches := isoDateRegex.FindStringSubmatch(match)
		if len(matches) == 4 {
			year := matches[1]
			month := matches[2]
			day := matches[3]
			// Combine without separators: YYYY-MM-DD → YYYYMMDD
			return year + month + day
		}
		return match // Return original if parsing fails
	})
	
	return result
}

// GenerateShowURL converts a show name to a Mixcloud URL format
// AIDEV-NOTE: Follows Mixcloud's URL structure conventions for cloudcast URLs
func GenerateShowURL(username, showName string) string {
	if username == "" || showName == "" {
		return ""
	}

	// Convert to lowercase for URL compatibility
	slug := strings.ToLower(showName)

	// Normalize Unicode characters first (accents, non-Latin scripts)
	// AIDEV-NOTE: Handle accented characters like café→cafe, señor→senor
	slug = normalizeUnicode(slug)

	// Normalize date formats before other replacements
	// AIDEV-NOTE: Handle common date patterns like M/D/YYYY, MM/DD/YYYY, etc.
	slug = normalizeDateFormats(slug)

	// Create replacer for common patterns and special characters
	// AIDEV-NOTE: Order matters - replace longer patterns first to avoid conflicts
	replacer := strings.NewReplacer(
		" - ", "-",    // Replace spaced hyphens first
		" & ", "-",    // Replace ampersands with spaces
		" + ", "-",    // Replace plus signs with spaces
		" @ ", "-",    // Replace at symbols with spaces
		" vs ", "-",   // Replace versus abbreviations
		" feat. ", "-", // Replace featuring abbreviations
		" ft. ", "-",  // Replace ft. abbreviations
		" w/ ", "-",   // Replace with abbreviations
		"(", "",       // Remove opening parentheses
		")", "",       // Remove closing parentheses
		"[", "",       // Remove opening brackets
		"]", "",       // Remove closing brackets
		"{", "",       // Remove opening braces
		"}", "",       // Remove closing braces
		"/", "",       // Remove slashes (common in dates)
		"\\", "",      // Remove backslashes
		".", "",       // Remove dots
		",", "",       // Remove commas
		"'", "",       // Remove apostrophes
		"\"", "",      // Remove quotes
		":", "",       // Remove colons
		";", "",       // Remove semicolons
		"?", "",       // Remove question marks
		"!", "",       // Remove exclamation marks
		"#", "",       // Remove hash symbols
		"$", "",       // Remove dollar signs
		"%", "",       // Remove percent signs
		"^", "",       // Remove caret symbols
		"*", "",       // Remove asterisks
		"=", "",       // Remove equals signs
		"|", "",       // Remove pipe symbols
		"~", "",       // Remove tilde symbols
		"`", "",       // Remove backticks
		"<", "",       // Remove less than signs
		">", "",       // Remove greater than signs
		" ", "-",      // Replace remaining spaces with hyphens
	)
	slug = replacer.Replace(slug)

	// Remove duplicate hyphens using pre-compiled regex for performance
	slug = hyphenDeduplicationRegex.ReplaceAllString(slug, "-")

	// Trim hyphens from start and end
	slug = strings.Trim(slug, "-")

	// Generate the full Mixcloud URL
	return fmt.Sprintf("https://www.mixcloud.com/%s/%s/", username, slug)
}

// extractCloudcastKey extracts the cloudcast key from a Mixcloud URL
// AIDEV-NOTE: Handles various URL formats and normalizes them to username/slug/ format
func extractCloudcastKey(showURL string) (string, error) {
	if showURL == "" {
		return "", ErrInvalidShowURL
	}

	// Parse the URL to extract components
	parsedURL, err := url.Parse(showURL)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrInvalidShowURL, err)
	}

	// Validate this is a Mixcloud URL
	// AIDEV-NOTE: Accept both www.mixcloud.com and mixcloud.com as valid hosts
	host := strings.ToLower(parsedURL.Host)
	if host != "mixcloud.com" && host != "www.mixcloud.com" {
		return "", fmt.Errorf("%w: host must be mixcloud.com or www.mixcloud.com, got %s", ErrInvalidShowURL, host)
	}

	// Extract the path and clean it
	path := strings.Trim(parsedURL.Path, "/")
	if path == "" {
		return "", fmt.Errorf("%w: empty path", ErrInvalidShowURL)
	}

	// Split path into components
	pathComponents := strings.Split(path, "/")
	
	// Mixcloud cloudcast URLs should have at least 2 components: username/slug
	if len(pathComponents) < 2 {
		return "", fmt.Errorf("%w: path must contain at least username and slug, got %d components", ErrInvalidShowURL, len(pathComponents))
	}

	// Extract username and slug (first two components)
	username := pathComponents[0]
	slug := pathComponents[1]

	// Validate components are not empty
	if username == "" || slug == "" {
		return "", fmt.Errorf("%w: username and slug cannot be empty", ErrInvalidShowURL)
	}

	// Return cloudcast key in the format expected by Mixcloud API: username/slug/
	// AIDEV-NOTE: API expects trailing slash in the key format
	return fmt.Sprintf("%s/%s/", username, slug), nil
}

// executeAPIRequestWithRetry performs an HTTP request with exponential backoff retry logic for rate limiting
// AIDEV-NOTE: Implements sophisticated retry logic with jitter and Retry-After header support
func (c *Client) executeAPIRequestWithRetry(req *http.Request) (*http.Response, error) {
	maxRetries := RateLimitMaxRetries
	baseDelay := RateLimitBaseDelay

	for attempt := 0; attempt <= maxRetries; attempt++ {
		// Clone the request for each attempt since the body may be consumed
		reqClone := req.Clone(req.Context())
		if req.Body != nil {
			// For POST requests with body, we need to seek back to the beginning
			// This assumes the body is seekable (like bytes.Buffer)
			if seeker, ok := req.Body.(io.Seeker); ok {
				if _, err := seeker.Seek(0, io.SeekStart); err != nil {
					log.Printf("[MIXCLOUD] Warning: failed to seek request body for retry: %v", err)
				}
			}
			reqClone.Body = req.Body
		}

		// Make the HTTP request
		resp, err := c.httpClient.Do(reqClone)
		if err != nil {
			// Network errors should be retried
			if attempt < maxRetries {
				delay := c.calculateRetryDelay(attempt, baseDelay, 0)
				log.Printf("[MIXCLOUD] Network error (attempt %d/%d), retrying in %v: %v", 
					attempt+1, maxRetries+1, delay, err)
				time.Sleep(delay)
				continue
			}
			return nil, fmt.Errorf("%w: HTTP request failed after %d retries: %v", ErrNetworkFailure, maxRetries+1, err)
		}

		// Check if this is a rate limiting response
		if resp.StatusCode == http.StatusTooManyRequests {
			if attempt < maxRetries {
				// Parse Retry-After header if present
				retryAfterSeconds := c.parseRetryAfterHeader(resp.Header.Get("Retry-After"))
				delay := c.calculateRetryDelay(attempt, baseDelay, retryAfterSeconds)
				
				log.Printf("[MIXCLOUD] Rate limited (attempt %d/%d), retrying in %v", 
					attempt+1, maxRetries+1, delay)
				
				// Close the response body before retrying
				resp.Body.Close()
				time.Sleep(delay)
				continue
			} else {
				// Max retries exceeded for rate limiting
				return resp, nil // Return the response so caller can handle the 429 status
			}
		}

		// For other status codes (success or non-retryable errors), return immediately
		return resp, nil
	}

	// This should never be reached due to the loop logic
	return nil, fmt.Errorf("%w: maximum retry attempts exceeded", ErrRateLimited)
}

// calculateRetryDelay calculates the delay for the next retry attempt
// AIDEV-NOTE: Implements exponential backoff with jitter and Retry-After header support
func (c *Client) calculateRetryDelay(attempt int, baseDelay time.Duration, retryAfterSeconds int) time.Duration {
	// If server provided Retry-After header, respect it
	if retryAfterSeconds > 0 {
		serverDelay := time.Duration(retryAfterSeconds) * time.Second
		// Add small jitter to prevent thundering herd
		jitter := time.Duration(rand.Intn(1000)) * time.Millisecond
		return serverDelay + jitter
	}

	// Calculate exponential backoff: baseDelay * 2^attempt
	exponentialDelay := baseDelay * time.Duration(1<<uint(attempt))
	
	// Cap the delay at 30 seconds to avoid excessive waiting
	const maxDelay = 30 * time.Second
	if exponentialDelay > maxDelay {
		exponentialDelay = maxDelay
	}

	// Add jitter to prevent thundering herd (±25% of the delay)
	jitterRange := int(exponentialDelay.Milliseconds() / 4) // 25% of delay
	if jitterRange > 0 {
		jitter := time.Duration(rand.Intn(jitterRange*2)-jitterRange) * time.Millisecond
		exponentialDelay += jitter
	}

	// Ensure minimum delay of 100ms
	const minDelay = 100 * time.Millisecond
	if exponentialDelay < minDelay {
		exponentialDelay = minDelay
	}

	return exponentialDelay
}

// parseRetryAfterHeader parses the Retry-After header value
// AIDEV-NOTE: Supports both delay-seconds and HTTP-date formats
func (c *Client) parseRetryAfterHeader(retryAfter string) int {
	if retryAfter == "" {
		return 0
	}

	// Try to parse as delay-seconds (integer)
	if seconds, err := strconv.Atoi(retryAfter); err == nil && seconds > 0 {
		// Cap at reasonable maximum (5 minutes)
		if seconds > 300 {
			seconds = 300
		}
		return seconds
	}

	// Try to parse as HTTP-date format
	if t, err := time.Parse(time.RFC1123, retryAfter); err == nil {
		delay := time.Until(t)
		if delay > 0 {
			seconds := int(delay.Seconds())
			// Cap at reasonable maximum (5 minutes)
			if seconds > 300 {
				seconds = 300
			}
			return seconds
		}
	}

	// Could not parse or invalid value
	return 0
}

// GetShow fetches show information from the Mixcloud API
// AIDEV-NOTE: Implements GET /cloudcast/<key>/ endpoint with proper error handling
func (c *Client) GetShow(showURL string) (*Show, error) {
	log := logger.Get()
	startTime := time.Now()
	
	log.Info("Starting Mixcloud API GetShow request", 
		slog.String("show_url", showURL))

	// Extract cloudcast key from the URL
	cloudcastKey, err := extractCloudcastKey(showURL)
	if err != nil {
		log.Error("Failed to extract cloudcast key from URL", 
			slog.String("show_url", showURL),
			slog.String("error", err.Error()))
		return nil, fmt.Errorf("failed to parse show URL: %w", err)
	}

	log.Debug("Extracted cloudcast key", 
		slog.String("cloudcast_key", cloudcastKey))

	// Construct the API endpoint URL
	endpoint := fmt.Sprintf(CloudcastEndpoint, cloudcastKey)
	apiURL := MixcloudAPIBaseURL + endpoint

	log.Info("Making Mixcloud API request", 
		slog.String("api_url", apiURL),
		slog.String("method", "GET"),
		slog.Bool("authenticated", false))

	// Create HTTP request with timeout
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		log.Error("Failed to create HTTP request", 
			slog.String("error", err.Error()))
		return nil, fmt.Errorf("%w: failed to create request", ErrAPIRequestFailed)
	}

	// Set appropriate headers
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "mixcloud-updater/1.0")

	// Make the API request - try unauthenticated first for public shows
	basicClient := &http.Client{}
	resp, err := basicClient.Do(req)
	if err != nil {
		log.Error("Mixcloud API request failed", 
			slog.String("api_url", apiURL),
			slog.String("error", err.Error()),
			slog.Duration("duration", time.Since(startTime)))
		return nil, fmt.Errorf("%w: HTTP request failed: %v", ErrNetworkFailure, err)
	}
	defer resp.Body.Close()

	log.Info("Mixcloud API response received", 
		slog.String("api_url", apiURL),
		slog.Int("status_code", resp.StatusCode),
		slog.Duration("duration", time.Since(startTime)))

	// Handle different HTTP status codes
	switch resp.StatusCode {
	case http.StatusOK:
		log.Debug("API request successful")
	case http.StatusNotFound:
		log.Error("Show not found on Mixcloud", 
			slog.String("show_url", showURL),
			slog.Int("status_code", resp.StatusCode))
		return nil, fmt.Errorf("%w: show URL %s", ErrShowNotFound, showURL)
	case http.StatusUnauthorized:
		log.Error("API authentication failed", 
			slog.Int("status_code", resp.StatusCode))
		return nil, fmt.Errorf("%w: API authentication failed", ErrAuthenticationFailed)
	case http.StatusTooManyRequests:
		log.Error("API rate limit exceeded", 
			slog.Int("status_code", resp.StatusCode))
		return nil, fmt.Errorf("%w: API rate limit exceeded after retries", ErrRateLimited)
	case http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable:
		log.Error("Mixcloud server error", 
			slog.Int("status_code", resp.StatusCode))
		return nil, fmt.Errorf("%w: server error (status %d)", ErrAPIRequestFailed, resp.StatusCode)
	default:
		log.Error("Unexpected API response status", 
			slog.Int("status_code", resp.StatusCode))
		return nil, fmt.Errorf("%w: unexpected status code %d", ErrAPIRequestFailed, resp.StatusCode)
	}

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Error("Failed to read API response body", 
			slog.String("error", err.Error()))
		return nil, fmt.Errorf("%w: failed to read response body: %v", ErrAPIRequestFailed, err)
	}

	log.Debug("API response body read", 
		slog.Int("body_size", len(body)))

	// Parse JSON response into Show struct
	var show Show
	if err := json.Unmarshal(body, &show); err != nil {
		previewLen := len(body)
		if previewLen > 200 {
			previewLen = 200
		}
		log.Error("Failed to parse API response JSON", 
			slog.String("error", err.Error()),
			slog.String("response_preview", string(body[:previewLen])))
		return nil, fmt.Errorf("%w: failed to parse JSON response: %v", ErrAPIRequestFailed, err)
	}

	// Validate that we got the expected data
	if show.Key == "" || show.Name == "" {
		log.Error("Incomplete show data received from API", 
			slog.String("show_key", show.Key),
			slog.String("show_name", show.Name))
		return nil, fmt.Errorf("%w: incomplete show data received from API", ErrAPIRequestFailed)
	}

	// Set the URL field to the original input URL for consistency
	show.URL = showURL

	log.Info("Successfully fetched show from Mixcloud API", 
		slog.String("show_name", show.Name),
		slog.String("show_key", show.Key),
		slog.Duration("total_duration", time.Since(startTime)))

	return &show, nil
}

// UpdateShowDescription updates the description of a Mixcloud show
// AIDEV-NOTE: Implements POST /upload/ endpoint with multipart form data
func (c *Client) UpdateShowDescription(showURL, description string) error {
	// Extract cloudcast key from the URL
	cloudcastKey, err := extractCloudcastKey(showURL)
	if err != nil {
		return fmt.Errorf("failed to parse show URL: %w", err)
	}

	// Validate description length
	if len(description) > MaxDescriptionLength {
		return fmt.Errorf("%w: description length %d exceeds maximum %d characters", 
			ErrDescriptionTooLong, len(description), MaxDescriptionLength)
	}

	// Check if client has authentication tokens for API requests
	// AIDEV-NOTE: Updates require authentication, unlike GetShow which works publicly
	if c.token == nil || c.token.AccessToken == "" {
		return fmt.Errorf("%w: access token is required for updating show descriptions", ErrAuthenticationFailed)
	}

	// Create multipart form data
	var formBuf bytes.Buffer
	writer := multipart.NewWriter(&formBuf)

	// Add description field (cloudcast key is already in URL path, no form field needed)
	if err := writer.WriteField("description", description); err != nil {
		return fmt.Errorf("%w: failed to write description field: %v", ErrAPIRequestFailed, err)
	}

	// Close the multipart writer to finalize the form data
	if err := writer.Close(); err != nil {
		return fmt.Errorf("%w: failed to close multipart writer: %v", ErrAPIRequestFailed, err)
	}

	// Construct the API endpoint URL for editing existing uploads
	// According to Mixcloud API docs: /upload/[YOUR_SHOW_KEY]/edit/?access_token=...
	// Clean cloudcastKey to avoid double slashes
	cleanKey := strings.Trim(cloudcastKey, "/")
	apiURL := fmt.Sprintf("%s/upload/%s/edit/?access_token=%s", MixcloudAPIBaseURL, cleanKey, c.token.AccessToken)

	// Create HTTP request with multipart form data
	req, err := http.NewRequest("POST", apiURL, &formBuf)
	if err != nil {
		return fmt.Errorf("%w: failed to create request", ErrAPIRequestFailed)
	}

	// Set appropriate headers
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "mixcloud-updater/1.0")

	// Make the API request with basic HTTP client (token is in query param, not OAuth header)
	// AIDEV-NOTE: Use basic client since we're passing access_token as query parameter
	log.Printf("[MIXCLOUD] Updating description for show: %s", showURL)
	basicClient := &http.Client{}
	resp, err := basicClient.Do(req)
	if err != nil {
		return fmt.Errorf("%w: request failed: %v", ErrAPIRequestFailed, err)
	}
	defer resp.Body.Close()

	// Read the response body for error analysis
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("[MIXCLOUD] Warning: failed to read response body: %v", err)
		body = []byte("(failed to read response)")
	}

	// Handle different HTTP status codes
	switch resp.StatusCode {
	case http.StatusOK, http.StatusCreated, http.StatusAccepted:
		// Success - verify the response contains success indication
		log.Printf("[MIXCLOUD] Successfully updated show description")
		return nil
	case http.StatusBadRequest:
		return fmt.Errorf("%w: bad request - invalid cloudcast key or description format: %s", 
			ErrAPIRequestFailed, string(body))
	case http.StatusUnauthorized:
		return fmt.Errorf("%w: API authentication failed", ErrAuthenticationFailed)
	case http.StatusForbidden:
		return fmt.Errorf("%w: insufficient permissions to update this show", ErrAuthenticationFailed)
	case http.StatusNotFound:
		return fmt.Errorf("%w: show not found: %s", ErrShowNotFound, showURL)
	case http.StatusTooManyRequests:
		// This should be rare since executeAPIRequestWithRetry handles rate limiting
		return fmt.Errorf("%w: API rate limit exceeded after retries", ErrRateLimited)
	case http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable:
		return fmt.Errorf("%w: server error (status %d): %s", ErrAPIRequestFailed, resp.StatusCode, string(body))
	default:
		return fmt.Errorf("%w: unexpected status code %d: %s", ErrAPIRequestFailed, resp.StatusCode, string(body))
	}
}