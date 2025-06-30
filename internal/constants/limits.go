package constants

import "time"

// Mixcloud API limits and constraints
const (
	// MixcloudDescriptionLimit is the maximum character limit for show descriptions
	MixcloudDescriptionLimit = 1000
	
	// MixcloudRateLimit defines the maximum requests per time window
	MixcloudRateLimit = 60
	MixcloudRateLimitWindow = time.Hour
)

// HTTP and network configuration
const (
	// DefaultTimeoutSeconds for HTTP requests
	DefaultTimeoutSeconds = 30
	
	// DefaultRetryAttempts for failed requests
	DefaultRetryAttempts = 3
	
	// DefaultRetryDelaySeconds for exponential backoff
	DefaultRetryDelaySeconds = 1
	
	// MaxRetryDelaySeconds caps the exponential backoff
	MaxRetryDelaySeconds = 30
)

// Processing and batch configuration
const (
	// DefaultBatchSize for processing multiple shows
	DefaultBatchSize = 5
	
	// MaxBatchSize to prevent resource exhaustion
	MaxBatchSize = 20
	
	// DefaultProcessingTimeoutMinutes for individual show processing
	DefaultProcessingTimeoutMinutes = 10
)

// File and logging configuration
const (
	// DefaultMaxLogFiles to keep in rotation
	DefaultMaxLogFiles = 7
	
	// DefaultMaxLogSizeMB per log file
	DefaultMaxLogSizeMB = 10
	
	// DefaultLogRotationHours for automatic rotation
	DefaultLogRotationHours = 24
)