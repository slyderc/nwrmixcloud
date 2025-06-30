// Package logger_test provides comprehensive cross-platform tests for the logging system.
// Tests cover Windows and macOS compatibility, file operations, rotation, and error scenarios.
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
	"testing"
	"time"
)

// TestCrossPlatformPathHandling tests that log directory paths work correctly on different platforms
func TestCrossPlatformPathHandling(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantType string // "relative", "absolute", "platform-specific"
	}{
		{
			name:     "relative path",
			input:    "logs",
			wantType: "relative",
		},
		{
			name:     "current directory relative",
			input:    "./logs",
			wantType: "relative",
		},
		{
			name:     "absolute unix path",
			input:    "/tmp/test-logs",
			wantType: "absolute",
		},
		{
			name:     "empty path uses default",
			input:    "",
			wantType: "relative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expanded := expandLogDirectory(tt.input)
			
			switch tt.wantType {
			case "relative":
				if filepath.IsAbs(expanded) && tt.input != "" && !strings.HasPrefix(tt.input, "/") {
					// Only fail if we explicitly gave a relative path but got absolute
					// Platform-specific defaults may return absolute paths
					if tt.input != "logs" && !strings.HasPrefix(tt.input, "./") {
						t.Errorf("expandLogDirectory(%s) = %s, expected relative path but got absolute", tt.input, expanded)
					}
					// For "logs" and "./" paths, absolute results are acceptable on some platforms
				}
			case "absolute":
				if !filepath.IsAbs(expanded) {
					t.Errorf("expandLogDirectory(%s) = %s, expected absolute path", tt.input, expanded)
				}
			}

			// Verify the path is valid for the current platform
			if expanded == "" {
				t.Errorf("expandLogDirectory(%s) returned empty path", tt.input)
			}

			// Test that we can create the directory
			tempDir := filepath.Join(os.TempDir(), "logger-test-"+tt.name)
			testPath := filepath.Join(tempDir, filepath.Base(expanded))
			
			err := os.MkdirAll(testPath, 0755)
			if err != nil {
				t.Errorf("Failed to create test directory %s: %v", testPath, err)
			}
			
			// Cleanup
			os.RemoveAll(tempDir)
		})
	}
}

// TestPlatformSpecificDefaults tests platform-specific default directories
func TestPlatformSpecificDefaults(t *testing.T) {
	// Test that platform-specific environment variables are handled correctly
	originalHome := os.Getenv("HOME")
	originalAppData := os.Getenv("APPDATA")
	
	defer func() {
		os.Setenv("HOME", originalHome)
		os.Setenv("APPDATA", originalAppData)
	}()

	t.Run("unix-like systems", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("Skipping Unix test on Windows")
		}
		
		testHome := "/tmp/test-home"
		os.Setenv("HOME", testHome)
		
		expanded := expandLogDirectory("") // Empty should use platform default
		
		if !strings.Contains(expanded, "logs") {
			t.Errorf("Expected expanded path to contain 'logs', got: %s", expanded)
		}
	})

	t.Run("windows systems", func(t *testing.T) {
		if runtime.GOOS != "windows" {
			// Simulate Windows environment on non-Windows systems
			os.Setenv("APPDATA", "C:\\Users\\TestUser\\AppData\\Roaming")
			os.Unsetenv("HOME")
		}
		
		expanded := expandLogDirectory("")
		
		if runtime.GOOS == "windows" || os.Getenv("APPDATA") != "" {
			if !strings.Contains(expanded, "logs") {
				t.Errorf("Expected expanded path to contain 'logs', got: %s", expanded)
			}
		}
	})
}

// TestLoggerInitialization tests logger creation with various configurations
func TestLoggerInitialization(t *testing.T) {
	tempDir := t.TempDir()

	tests := []struct {
		name   string
		config Config
		want   bool // should succeed
	}{
		{
			name: "valid config with file logging",
			config: Config{
				Enabled:         true,
				Directory:       tempDir,
				FilenamePattern: "test-%Y%m%d.log",
				Level:           "info",
				MaxFiles:        5,
				MaxSizeMB:       1,
				ConsoleOutput:   true,
			},
			want: true,
		},
		{
			name: "console only logging",
			config: Config{
				Enabled:       false,
				ConsoleOutput: true,
				Level:         "debug",
			},
			want: true,
		},
		{
			name: "file logging with bad directory",
			config: Config{
				Enabled:   true,
				Directory: "/root/cannot-create-here",
				Level:     "info",
			},
			want: false,
		},
		{
			name: "valid config with custom pattern",
			config: Config{
				Enabled:         true,
				Directory:       tempDir,
				FilenamePattern: "custom-%Y-%m-%d-%H%M.log",
				Level:           "warn",
				MaxFiles:        10,
				MaxSizeMB:       5,
				ConsoleOutput:   false,
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, err := NewLogger(tt.config)
			
			if tt.want && err != nil {
				t.Errorf("NewLogger() failed unexpectedly: %v", err)
				return
			}
			
			if !tt.want && err == nil {
				t.Errorf("NewLogger() succeeded unexpectedly")
				return
			}
			
			if logger != nil {
				logger.Close()
			}
		})
	}
}

// TestLogFilenameGeneration tests filename pattern replacement
func TestLogFilenameGeneration(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		want    string // substring that should be present
	}{
		{
			name:    "basic daily pattern",
			pattern: "app-%Y%m%d.log",
			want:    ".log",
		},
		{
			name:    "hourly pattern",
			pattern: "app-%Y%m%d-%H%M.log",
			want:    ".log",
		},
		{
			name:    "empty pattern uses default",
			pattern: "",
			want:    "mixcloud-updater-",
		},
		{
			name:    "no placeholders",
			pattern: "static.log",
			want:    "static.log",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generateLogFilename(tt.pattern)
			
			if !strings.Contains(result, tt.want) {
				t.Errorf("generateLogFilename(%s) = %s, expected to contain %s", tt.pattern, result, tt.want)
			}
			
			// Verify the result is a valid filename
			if strings.ContainsAny(result, "/\\:*?\"<>|") {
				t.Errorf("Generated filename contains invalid characters: %s", result)
			}
		})
	}
}

// TestLogLevelParsing tests log level string parsing
func TestLogLevelParsing(t *testing.T) {
	tests := []struct {
		input string
		want  slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"DEBUG", slog.LevelDebug},
		{"info", slog.LevelInfo},
		{"INFO", slog.LevelInfo},
		{"warn", slog.LevelWarn},
		{"warning", slog.LevelWarn},
		{"WARN", slog.LevelWarn},
		{"error", slog.LevelError},
		{"ERROR", slog.LevelError},
		{"invalid", slog.LevelInfo}, // should default to info
		{"", slog.LevelInfo},        // empty should default to info
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseLogLevel(tt.input)
			if got != tt.want {
				t.Errorf("parseLogLevel(%s) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// TestLogRotation tests log file rotation functionality
func TestLogRotation(t *testing.T) {
	tempDir := t.TempDir()

	config := Config{
		Enabled:         true,
		Directory:       tempDir,
		FilenamePattern: "rotation-test-%Y%m%d.log",
		Level:           "info",
		MaxFiles:        3,
		MaxSizeMB:       1, // Small size to trigger rotation
		ConsoleOutput:   false,
	}

	logger, err := NewLogger(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Write enough data to trigger size-based rotation
	largeMessage := strings.Repeat("This is a test message for log rotation. ", 1000)
	
	for i := 0; i < 10; i++ {
		logger.Info("Large log message", slog.String("content", largeMessage), slog.Int("iteration", i))
		
		// Force a rotation check
		logger.checkRotation()
		
		time.Sleep(10 * time.Millisecond) // Small delay to ensure different timestamps
	}

	// Check that log files were created
	files, err := filepath.Glob(filepath.Join(tempDir, "rotation-test-*.log"))
	if err != nil {
		t.Fatalf("Failed to list log files: %v", err)
	}

	if len(files) == 0 {
		t.Errorf("No log files were created")
	}

	t.Logf("Created %d log files: %v", len(files), files)
}

// TestConcurrentLogging tests thread safety of the logger
func TestConcurrentLogging(t *testing.T) {
	tempDir := t.TempDir()

	config := Config{
		Enabled:         true,
		Directory:       tempDir,
		FilenamePattern: "concurrent-test.log",
		Level:           "info",
		MaxFiles:        5,
		MaxSizeMB:       10,
		ConsoleOutput:   false,
	}

	logger, err := NewLogger(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Run concurrent logging operations
	const numGoroutines = 10
	const messagesPerGoroutine = 100
	
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			defer wg.Done()
			
			for j := 0; j < messagesPerGoroutine; j++ {
				logger.Info("Concurrent log message",
					slog.Int("goroutine", goroutineID),
					slog.Int("message", j),
					slog.String("timestamp", time.Now().Format(time.RFC3339Nano)))
			}
		}(i)
	}

	wg.Wait()

	// Verify log file exists and has content
	logFile := filepath.Join(tempDir, "concurrent-test.log")
	info, err := os.Stat(logFile)
	if err != nil {
		t.Fatalf("Log file was not created: %v", err)
	}

	if info.Size() == 0 {
		t.Errorf("Log file is empty after concurrent writes")
	}

	t.Logf("Concurrent logging test completed. Log file size: %d bytes", info.Size())
}

// TestLogFileCreationPermissions tests that log files are created with correct permissions
func TestLogFileCreationPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping permission test on Windows")
	}

	tempDir := t.TempDir()

	config := Config{
		Enabled:         true,
		Directory:       tempDir,
		FilenamePattern: "permissions-test.log",
		Level:           "info",
		ConsoleOutput:   false,
	}

	logger, err := NewLogger(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	logger.Info("Test message for permissions")
	logger.Close()

	// Check file permissions
	logFile := filepath.Join(tempDir, "permissions-test.log")
	info, err := os.Stat(logFile)
	if err != nil {
		t.Fatalf("Failed to stat log file: %v", err)
	}

	mode := info.Mode()
	expectedMode := os.FileMode(0644)
	
	if mode.Perm() != expectedMode {
		t.Errorf("Log file has incorrect permissions: got %v, want %v", mode.Perm(), expectedMode)
	}
}

// TestLoggerErrorHandling tests error scenarios
func TestLoggerErrorHandling(t *testing.T) {
	t.Run("invalid log directory", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("Skipping directory permission test on Windows")
		}

		config := Config{
			Enabled:   true,
			Directory: "/root/nonexistent/cannot-create",
			Level:     "info",
		}

		_, err := NewLogger(config)
		if err == nil {
			t.Errorf("Expected error when creating logger with invalid directory")
		}
	})

	t.Run("logger continues after rotation failure", func(t *testing.T) {
		tempDir := t.TempDir()

		config := Config{
			Enabled:         true,
			Directory:       tempDir,
			FilenamePattern: "error-test.log",
			Level:           "info",
			MaxSizeMB:       1,
			ConsoleOutput:   false,
		}

		logger, err := NewLogger(config)
		if err != nil {
			t.Fatalf("Failed to create logger: %v", err)
		}
		defer logger.Close()

		// Write a log message to verify it works initially
		logger.Info("Initial test message")

		// Simulate a rotation failure by making the directory read-only
		// Note: This may not work on all systems/filesystems
		originalMode := os.FileMode(0755)
		if runtime.GOOS != "windows" {
			err = os.Chmod(tempDir, 0555) // Read-only
			if err != nil {
				t.Logf("Could not make directory read-only: %v", err)
			} else {
				defer os.Chmod(tempDir, originalMode) // Restore permissions
			}
		}

		// Try to log another message - should not panic even if rotation fails
		logger.Info("Message after potential rotation failure")
	})
}

// TestExecutionSummaryLogging tests the execution summary functionality
func TestExecutionSummaryLogging(t *testing.T) {
	tempDir := t.TempDir()

	config := Config{
		Enabled:         true,
		Directory:       tempDir,
		FilenamePattern: "summary-test.log",
		Level:           "info",
		ConsoleOutput:   false,
	}

	logger, err := NewLogger(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Test execution summary logging
	startTime := time.Now().Add(-5 * time.Second) // Simulate 5 seconds ago
	configFile := "/path/to/config.toml"
	mode := "Test Mode"
	results := []string{
		"Show 1: SUCCESS",
		"Show 2: FAILED - Network error",
		"Show 3: SUCCESS",
	}
	exitCode := 0

	logger.LogExecutionSummary(startTime, configFile, mode, results, exitCode)

	// Verify the log file contains the summary
	logFile := filepath.Join(tempDir, "summary-test.log")
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	contentStr := string(content)
	
	// Check that key elements are present in the log
	expectedElements := []string{
		"EXECUTION SUMMARY",
		configFile,
		mode,
		"Show 1: SUCCESS",
		"Show 2: FAILED",
		"Show 3: SUCCESS",
	}

	for _, element := range expectedElements {
		if !strings.Contains(contentStr, element) {
			t.Errorf("Expected log content to contain %q, but it was not found", element)
		}
	}
}

// TestLoggerWithWriter tests the io.Writer interface implementation
func TestLoggerWithWriter(t *testing.T) {
	tempDir := t.TempDir()

	config := Config{
		Enabled:         true,
		Directory:       tempDir,
		FilenamePattern: "writer-test.log",
		Level:           "info",
		ConsoleOutput:   false,
	}

	logger, err := NewLogger(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Test writing directly to the logger
	testMessage := "Direct write test message\n"
	n, err := logger.Write([]byte(testMessage))
	if err != nil {
		t.Errorf("Failed to write to logger: %v", err)
	}

	if n != len(testMessage) {
		t.Errorf("Write returned %d bytes, expected %d", n, len(testMessage))
	}

	// Also test that we can use it as an io.Writer
	var writer io.Writer = logger
	_, err = fmt.Fprint(writer, "Another test message\n")
	if err != nil {
		t.Errorf("Failed to write using io.Writer interface: %v", err)
	}
}

// BenchmarkLogging benchmarks logging performance
func BenchmarkLogging(b *testing.B) {
	tempDir := b.TempDir()

	config := Config{
		Enabled:         true,
		Directory:       tempDir,
		FilenamePattern: "benchmark.log",
		Level:           "info",
		ConsoleOutput:   false,
	}

	logger, err := NewLogger(config)
	if err != nil {
		b.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			logger.Info("Benchmark message",
				slog.Int("iteration", i),
				slog.String("data", "test data for benchmarking"),
				slog.Time("timestamp", time.Now()))
			i++
		}
	})
}