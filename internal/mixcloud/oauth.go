package mixcloud

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os/exec"
	"runtime"
	"time"

	"golang.org/x/oauth2"

	"github.com/nowwaveradio/mixcloud-updater/internal/config"
	"github.com/nowwaveradio/mixcloud-updater/internal/logger"
)

// AIDEV-NOTE: OAuth flow handler for browser-based Mixcloud authentication
// Implements local server callback and automatic browser opening

const (
	// OAuth callback configuration
	DefaultCallbackPort = 8080
	CallbackPath        = "/oauth/callback"
	CallbackTimeout     = 5 * time.Minute
)

// OAuthFlow handles the complete OAuth authorization flow for Mixcloud
type OAuthFlow struct {
	config     *oauth2.Config
	callbackPort int
	redirectURI  string
	server      *http.Server
	resultChan  chan *oauth2.Token
	errorChan   chan error
}

// NewOAuthFlow creates a new OAuth flow handler
func NewOAuthFlow(clientID, clientSecret string, callbackPort int) *OAuthFlow {
	if callbackPort == 0 {
		callbackPort = DefaultCallbackPort
	}

	redirectURI := fmt.Sprintf("http://localhost:%d%s", callbackPort, CallbackPath)

	oauth2Config := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://www.mixcloud.com/oauth/authorize",
			TokenURL: "https://www.mixcloud.com/oauth/access_token",
		},
		RedirectURL: redirectURI,
		Scopes:      []string{}, // Mixcloud doesn't require specific scopes
	}

	return &OAuthFlow{
		config:      oauth2Config,
		callbackPort: callbackPort,
		redirectURI:  redirectURI,
		resultChan:  make(chan *oauth2.Token, 1),
		errorChan:   make(chan error, 1),
	}
}

// Authorize initiates the OAuth flow and returns the access token
func (o *OAuthFlow) Authorize(ctx context.Context) (*oauth2.Token, error) {
	log := logger.Get()
	
	log.Info("Starting OAuth authorization flow", 
		slog.String("redirect_uri", o.redirectURI),
		slog.Int("callback_port", o.callbackPort))

	// Start the local callback server
	if err := o.startCallbackServer(); err != nil {
		log.Error("Failed to start OAuth callback server", 
			slog.String("error", err.Error()),
			slog.Int("port", o.callbackPort))
		return nil, fmt.Errorf("failed to start callback server: %w", err)
	}

	// Generate the authorization URL using Mixcloud's simple OAuth flow
	// Mixcloud doesn't support Google-style OAuth parameters like access_type=offline
	authURL := o.config.AuthCodeURL("state")
	
	log.Info("Generated OAuth authorization URL", 
		slog.String("url", authURL))

	// Open the browser to the authorization URL
	fmt.Printf("Opening browser for Mixcloud authorization...\n")
	fmt.Printf("If the browser doesn't open automatically, visit this URL:\n")
	fmt.Printf("%s\n\n", authURL)

	if err := o.openBrowser(authURL); err != nil {
		log.Warn("Failed to open browser automatically", 
			slog.String("error", err.Error()),
			slog.String("os", runtime.GOOS))
		fmt.Printf("Please manually open the URL above in your browser.\n\n")
	} else {
		log.Debug("Browser opened successfully")
	}

	// Wait for the OAuth callback or timeout
	fmt.Printf("Waiting for authorization (timeout: %v)...\n", CallbackTimeout)
	
	log.Info("Waiting for OAuth callback", 
		slog.Duration("timeout", CallbackTimeout))

	timeoutCtx, cancel := context.WithTimeout(ctx, CallbackTimeout)
	defer cancel()

	select {
	case token := <-o.resultChan:
		o.shutdown()
		log.Info("OAuth authorization successful", 
			slog.Bool("has_access_token", token.AccessToken != ""),
			slog.Time("expires", token.Expiry))
		fmt.Printf("✓ Authorization successful!\n")
		return token, nil

	case err := <-o.errorChan:
		o.shutdown()
		log.Error("OAuth flow failed", slog.String("error", err.Error()))
		return nil, fmt.Errorf("OAuth flow failed: %w", err)

	case <-timeoutCtx.Done():
		o.shutdown()
		log.Error("OAuth flow timed out", slog.Duration("timeout", CallbackTimeout))
		return nil, fmt.Errorf("OAuth flow timed out after %v", CallbackTimeout)

	case <-ctx.Done():
		o.shutdown()
		log.Error("OAuth flow cancelled", slog.String("error", ctx.Err().Error()))
		return nil, fmt.Errorf("OAuth flow cancelled: %w", ctx.Err())
	}
}

// startCallbackServer starts the local HTTP server to handle OAuth callbacks
func (o *OAuthFlow) startCallbackServer() error {
	mux := http.NewServeMux()
	mux.HandleFunc(CallbackPath, o.handleCallback)
	mux.HandleFunc("/", o.handleRoot)

	o.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", o.callbackPort),
		Handler: mux,
	}

	go func() {
		if err := o.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			o.errorChan <- fmt.Errorf("callback server error: %w", err)
		}
	}()

	// Give the server a moment to start
	time.Sleep(100 * time.Millisecond)

	return nil
}

// handleCallback processes the OAuth callback from Mixcloud
func (o *OAuthFlow) handleCallback(w http.ResponseWriter, r *http.Request) {
	// Parse the callback URL parameters
	query := r.URL.Query()
	
	// Check for errors
	if errCode := query.Get("error"); errCode != "" {
		errDesc := query.Get("error_description")
		err := fmt.Errorf("OAuth error: %s - %s", errCode, errDesc)
		o.errorChan <- err
		o.writeErrorResponse(w, err)
		return
	}

	// Get the authorization code
	code := query.Get("code")
	if code == "" {
		err := fmt.Errorf("no authorization code received")
		o.errorChan <- err
		o.writeErrorResponse(w, err)
		return
	}

	// Exchange the code for an access token
	token, err := o.config.Exchange(context.Background(), code)
	if err != nil {
		err = fmt.Errorf("failed to exchange code for token: %w", err)
		o.errorChan <- err
		o.writeErrorResponse(w, err)
		return
	}

	// Validate the token
	if token.AccessToken == "" {
		err := fmt.Errorf("received empty access token")
		o.errorChan <- err
		o.writeErrorResponse(w, err)
		return
	}

	// Send success response to browser
	o.writeSuccessResponse(w)

	// Send the token through the result channel
	o.resultChan <- token
}

// handleRoot provides a simple landing page for the OAuth server
func (o *OAuthFlow) handleRoot(w http.ResponseWriter, r *http.Request) {
	html := `
<!DOCTYPE html>
<html>
<head>
    <title>Mixcloud OAuth</title>
    <style>
        body { font-family: Arial, sans-serif; text-align: center; margin-top: 50px; }
        .container { max-width: 500px; margin: 0 auto; }
    </style>
</head>
<body>
    <div class="container">
        <h1>Mixcloud Authorization</h1>
        <p>Waiting for Mixcloud authorization...</p>
        <p>This page will update automatically when the authorization is complete.</p>
    </div>
</body>
</html>`
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(html))
}

// writeSuccessResponse sends a success page to the browser
func (o *OAuthFlow) writeSuccessResponse(w http.ResponseWriter) {
	html := `
<!DOCTYPE html>
<html>
<head>
    <title>Authorization Successful</title>
    <style>
        body { font-family: Arial, sans-serif; text-align: center; margin-top: 50px; }
        .container { max-width: 500px; margin: 0 auto; }
        .success { color: #28a745; }
    </style>
</head>
<body>
    <div class="container">
        <h1 class="success">✓ Authorization Successful!</h1>
        <p>Your Mixcloud account has been successfully authorized.</p>
        <p>You can now close this browser window and return to the terminal.</p>
    </div>
</body>
</html>`
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(html))
}

// writeErrorResponse sends an error page to the browser
func (o *OAuthFlow) writeErrorResponse(w http.ResponseWriter, err error) {
	html := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <title>Authorization Failed</title>
    <style>
        body { font-family: Arial, sans-serif; text-align: center; margin-top: 50px; }
        .container { max-width: 500px; margin: 0 auto; }
        .error { color: #dc3545; }
    </style>
</head>
<body>
    <div class="container">
        <h1 class="error">✗ Authorization Failed</h1>
        <p>There was an error during the authorization process:</p>
        <p><strong>%s</strong></p>
        <p>Please close this browser window and try again.</p>
    </div>
</body>
</html>`, err.Error())
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusBadRequest)
	w.Write([]byte(html))
}

// openBrowser attempts to open the given URL in the default browser
func (o *OAuthFlow) openBrowser(url string) error {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "windows":
		cmd = "cmd"
		args = []string{"/c", "start"}
	case "darwin":
		cmd = "open"
	default: // "linux", "freebsd", "openbsd", "netbsd"
		cmd = "xdg-open"
	}
	args = append(args, url)
	return exec.Command(cmd, args...).Start()
}

// shutdown gracefully shuts down the callback server
func (o *OAuthFlow) shutdown() {
	if o.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := o.server.Shutdown(ctx); err != nil {
			log := logger.Get()
			log.Error("Error shutting down OAuth callback server", slog.String("error", err.Error()))
		}
	}
}

// AuthorizeAndSave performs the complete OAuth flow and saves tokens to config
func AuthorizeAndSave(cfg *config.Config, configPath string) error {
	// Validate OAuth configuration
	if cfg.OAuth.ClientID == "" || cfg.OAuth.ClientSecret == "" {
		return fmt.Errorf("OAuth client_id and client_secret must be configured")
	}

	// Create OAuth flow
	flow := NewOAuthFlow(cfg.OAuth.ClientID, cfg.OAuth.ClientSecret, DefaultCallbackPort)

	// Perform authorization
	ctx := context.Background()
	token, err := flow.Authorize(ctx)
	if err != nil {
		return fmt.Errorf("authorization failed: %w", err)
	}

	// Update config with new tokens
	cfg.OAuth.AccessToken = token.AccessToken
	cfg.OAuth.RefreshToken = token.RefreshToken

	// Save updated config to file
	if err := config.SaveConfig(cfg, configPath); err != nil {
		return fmt.Errorf("failed to save tokens to config file: %w", err)
	}

	fmt.Printf("✓ OAuth tokens saved to config file: %s\n", configPath)
	return nil
}