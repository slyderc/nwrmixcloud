// Package logger provides cross-platform file-based logging for the Mixcloud updater.
// It offers structured logging with different levels, automatic rotation, and both
// file and console output support for Windows and macOS environments.
package logger

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// Config represents logging configuration
type Config struct {
	Enabled         bool   `toml:"enabled"`
	Directory       string `toml:"directory"`
	FilenamePattern string `toml:"filename_pattern"`
	Level           string `toml:"level"`
	MaxFiles        int    `toml:"max_files"`
	MaxSizeMB       int    `toml:"max_size_mb"`
	ConsoleOutput   bool   `toml:"console_output"`
}

// Logger wraps slog.Logger with file management capabilities
type Logger struct {
	*slog.Logger
	config      Config
	file        *os.File
	fileName    string
	fileSize    int64
	mu          sync.Mutex
	multiWriter io.Writer
}

var (
	// Global logger instance
	globalLogger *Logger
	once         sync.Once
)

// Initialize creates and configures the global logger instance
func Initialize(config Config) error {
	var initErr error
	once.Do(func() {
		globalLogger, initErr = NewLogger(config)
	})
	return initErr
}

// Get returns the global logger instance
func Get() *Logger {
	if globalLogger == nil {
		// Fallback to console-only logger if not initialized
		consoleLogger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		}))
		globalLogger = &Logger{Logger: consoleLogger}
	}
	return globalLogger
}

// NewLogger creates a new logger with the given configuration
func NewLogger(config Config) (*Logger, error) {
	logger := &Logger{
		config: config,
	}

	// Parse log level
	level := parseLogLevel(config.Level)

	// Set up writers based on configuration
	writers := []io.Writer{}

	if config.ConsoleOutput {
		writers = append(writers, os.Stdout)
	}

	if config.Enabled {
		// Create log directory
		logDir := expandLogDirectory(config.Directory)
		if err := os.MkdirAll(logDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create log directory: %w", err)
		}

		// Open log file
		logFile, err := logger.openLogFile()
		if err != nil {
			return nil, fmt.Errorf("failed to open log file: %w", err)
		}
		logger.file = logFile
		writers = append(writers, logFile)
	}

	// Create multi-writer
	if len(writers) == 0 {
		// Fallback to stdout if no writers configured
		writers = append(writers, os.Stdout)
	}
	logger.multiWriter = io.MultiWriter(writers...)

	// Create slog handler with custom formatting
	handler := slog.NewTextHandler(logger.multiWriter, &slog.HandlerOptions{
		Level: level,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// Custom time format
			if a.Key == slog.TimeKey {
				return slog.String(slog.TimeKey, a.Value.Time().Format("2006-01-02T15:04:05.000-07:00"))
			}
			// Shorten source paths for readability
			if a.Key == slog.SourceKey {
				source := a.Value.Any().(*slog.Source)
				return slog.String(slog.SourceKey, fmt.Sprintf("%s:%d", filepath.Base(source.File), source.Line))
			}
			return a
		},
	})

	logger.Logger = slog.New(handler)
	
	// Log initialization
	logger.Info("Logger initialized",
		slog.String("log_file", logger.fileName),
		slog.String("level", config.Level),
		slog.Bool("console", config.ConsoleOutput))

	return logger, nil
}

// openLogFile creates or opens the current log file
func (l *Logger) openLogFile() (*os.File, error) {
	logDir := expandLogDirectory(l.config.Directory)
	
	// Generate filename from pattern
	fileName := generateLogFilename(l.config.FilenamePattern)
	filePath := filepath.Join(logDir, fileName)
	
	// Open file in append mode
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}
	
	// Get file info for size tracking
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, err
	}
	
	l.fileName = filePath
	l.fileSize = info.Size()
	
	return file, nil
}

// expandLogDirectory expands the log directory path with platform-specific defaults
func expandLogDirectory(dir string) string {
	if dir == "" {
		dir = "logs"
	}
	
	// Handle absolute paths
	if filepath.IsAbs(dir) {
		return dir
	}
	
	// Handle relative paths
	if dir == "logs" || strings.HasPrefix(dir, "./") {
		// Use working directory
		return dir
	}
	
	// Platform-specific default directories
	switch runtime.GOOS {
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData != "" {
			return filepath.Join(appData, "NowWaveRadio", "MixcloudUpdater", "logs")
		}
	case "darwin", "linux":
		home := os.Getenv("HOME")
		if home != "" {
			return filepath.Join(home, ".nowwaveradio", "mixcloud-updater", "logs")
		}
	}
	
	// Fallback to working directory
	return "logs"
}

// generateLogFilename creates a filename from the pattern
func generateLogFilename(pattern string) string {
	if pattern == "" {
		pattern = "mixcloud-updater-%Y%m%d.log"
	}
	
	// Replace date/time placeholders
	now := time.Now()
	replacements := map[string]string{
		"%Y": fmt.Sprintf("%04d", now.Year()),
		"%m": fmt.Sprintf("%02d", now.Month()),
		"%d": fmt.Sprintf("%02d", now.Day()),
		"%H": fmt.Sprintf("%02d", now.Hour()),
		"%M": fmt.Sprintf("%02d", now.Minute()),
	}
	
	fileName := pattern
	for placeholder, value := range replacements {
		fileName = strings.ReplaceAll(fileName, placeholder, value)
	}
	
	return fileName
}

// parseLogLevel converts string level to slog.Level
func parseLogLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// checkRotation checks if log rotation is needed
func (l *Logger) checkRotation() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	
	if l.file == nil || !l.config.Enabled {
		return nil
	}
	
	// Check file size
	maxSize := int64(l.config.MaxSizeMB) * 1024 * 1024
	if maxSize > 0 && l.fileSize >= maxSize {
		return l.rotate()
	}
	
	// Check if date has changed (for daily rotation)
	currentFileName := generateLogFilename(l.config.FilenamePattern)
	if filepath.Base(l.fileName) != currentFileName {
		return l.rotate()
	}
	
	return nil
}

// rotate performs log file rotation
func (l *Logger) rotate() error {
	// Close current file
	if l.file != nil {
		l.file.Close()
	}
	
	// Open new file
	file, err := l.openLogFile()
	if err != nil {
		return err
	}
	
	l.file = file
	
	// Update multi-writer
	writers := []io.Writer{}
	if l.config.ConsoleOutput {
		writers = append(writers, os.Stdout)
	}
	writers = append(writers, l.file)
	l.multiWriter = io.MultiWriter(writers...)
	
	// Recreate handler with new writer - use same options as original
	handler := slog.NewTextHandler(l.multiWriter, &slog.HandlerOptions{
		Level: parseLogLevel(l.config.Level),
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// Custom time format
			if a.Key == slog.TimeKey {
				return slog.String(slog.TimeKey, a.Value.Time().Format("2006-01-02T15:04:05.000-07:00"))
			}
			// Shorten source paths for readability
			if a.Key == slog.SourceKey {
				source := a.Value.Any().(*slog.Source)
				return slog.String(slog.SourceKey, fmt.Sprintf("%s:%d", filepath.Base(source.File), source.Line))
			}
			return a
		},
	})
	l.Logger = slog.New(handler)
	
	// Clean old files if needed
	if l.config.MaxFiles > 0 {
		go l.cleanOldFiles()
	}
	
	return nil
}

// cleanOldFiles removes log files older than MaxFiles
func (l *Logger) cleanOldFiles() {
	logDir := filepath.Dir(l.fileName)
	pattern := strings.ReplaceAll(l.config.FilenamePattern, "%Y", "*")
	pattern = strings.ReplaceAll(pattern, "%m", "*")
	pattern = strings.ReplaceAll(pattern, "%d", "*")
	pattern = strings.ReplaceAll(pattern, "%H", "*")
	pattern = strings.ReplaceAll(pattern, "%M", "*")
	
	matches, err := filepath.Glob(filepath.Join(logDir, pattern))
	if err != nil {
		return
	}
	
	// Sort by modification time
	type fileInfo struct {
		path    string
		modTime time.Time
	}
	
	files := make([]fileInfo, 0, len(matches))
	for _, match := range matches {
		info, err := os.Stat(match)
		if err != nil {
			continue
		}
		files = append(files, fileInfo{path: match, modTime: info.ModTime()})
	}
	
	// Sort newest first
	for i := 0; i < len(files)-1; i++ {
		for j := i + 1; j < len(files); j++ {
			if files[i].modTime.Before(files[j].modTime) {
				files[i], files[j] = files[j], files[i]
			}
		}
	}
	
	// Remove old files
	for i := l.config.MaxFiles; i < len(files); i++ {
		os.Remove(files[i].path)
	}
}

// Write implements io.Writer interface with rotation check
func (l *Logger) Write(p []byte) (n int, err error) {
	// Check rotation before writing
	if err := l.checkRotation(); err != nil {
		// Log rotation error but continue writing
		fmt.Fprintf(os.Stderr, "Log rotation error: %v\n", err)
	}
	
	n, err = l.multiWriter.Write(p)
	l.fileSize += int64(n)
	return
}

// Close closes the log file
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	
	if l.file != nil {
		return l.file.Close()
	}
	return nil
}

// LogExecutionSummary logs a formatted execution summary for audit purposes
func (l *Logger) LogExecutionSummary(startTime time.Time, configFile string, mode string, results []string, exitCode int) {
	duration := time.Since(startTime)
	
	l.Info("=== EXECUTION SUMMARY ===")
	l.Info("Execution details",
		slog.Time("start_time", startTime),
		slog.String("config_file", configFile),
		slog.String("mode", mode),
		slog.Duration("total_duration", duration),
		slog.Int("exit_code", exitCode))
	
	for _, result := range results {
		l.Info(result)
	}
}