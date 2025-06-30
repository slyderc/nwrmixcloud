package shows

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nowwaveradio/mixcloud-updater/internal/config"
)

func TestNewCueResolver(t *testing.T) {
	baseDir := "/test/path"
	resolver := NewCueResolver(baseDir)
	
	if resolver == nil {
		t.Fatal("NewCueResolver() returned nil")
	}
	
	if resolver.GetBaseDir() != baseDir {
		t.Errorf("GetBaseDir() = %v, want %v", resolver.GetBaseDir(), baseDir)
	}
}

func TestSetBaseDir(t *testing.T) {
	resolver := NewCueResolver("/initial/path")
	newPath := "/new/path"
	
	resolver.SetBaseDir(newPath)
	
	if resolver.GetBaseDir() != newPath {
		t.Errorf("After SetBaseDir(), GetBaseDir() = %v, want %v", resolver.GetBaseDir(), newPath)
	}
}

func TestResolveDirectMapping(t *testing.T) {
	// Create temporary directory and files for testing
	tmpDir := t.TempDir()
	resolver := NewCueResolver(tmpDir)
	
	// Create test CUE file
	testFile := filepath.Join(tmpDir, "test.cue")
	err := os.WriteFile(testFile, []byte("TEST CONTENT"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	
	tests := []struct {
		name      string
		showCfg   *config.ShowConfig
		wantError bool
		wantFile  string
	}{
		{
			name: "relative path mapping",
			showCfg: &config.ShowConfig{
				CueFileMapping: "test.cue",
			},
			wantError: false,
			wantFile:  testFile,
		},
		{
			name: "absolute path mapping",
			showCfg: &config.ShowConfig{
				CueFileMapping: testFile,
			},
			wantError: false,
			wantFile:  testFile,
		},
		{
			name: "non-existent file",
			showCfg: &config.ShowConfig{
				CueFileMapping: "nonexistent.cue",
			},
			wantError: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := resolver.ResolveCueFile(tt.showCfg)
			
			if tt.wantError {
				if err == nil {
					t.Error("ResolveCueFile() should return error")
				}
			} else {
				if err != nil {
					t.Errorf("ResolveCueFile() error = %v", err)
				}
				if result != tt.wantFile {
					t.Errorf("ResolveCueFile() = %v, want %v", result, tt.wantFile)
				}
			}
		})
	}
}

func TestResolvePattern(t *testing.T) {
	// Create temporary directory and files for testing
	tmpDir := t.TempDir()
	resolver := NewCueResolver(tmpDir)
	
	// Create test CUE files with different modification times
	files := []string{"MYR001.cue", "MYR002.cue", "MYR003.cue"}
	var createdFiles []string
	
	for i, filename := range files {
		filePath := filepath.Join(tmpDir, filename)
		err := os.WriteFile(filePath, []byte("TEST CONTENT"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file %s: %v", filename, err)
		}
		createdFiles = append(createdFiles, filePath)
		
		// Set different modification times (newer files have later times)
		modTime := time.Now().Add(-time.Duration(len(files)-i) * time.Hour)
		err = os.Chtimes(filePath, modTime, modTime)
		if err != nil {
			t.Fatalf("Failed to set modification time for %s: %v", filename, err)
		}
	}
	
	tests := []struct {
		name      string
		showCfg   *config.ShowConfig
		wantError bool
		wantFile  string // Expected latest file
	}{
		{
			name: "pattern matching finds latest",
			showCfg: &config.ShowConfig{
				CueFilePattern: "MYR*.cue",
			},
			wantError: false,
			wantFile:  createdFiles[2], // MYR003.cue should be newest
		},
		{
			name: "specific pattern",
			showCfg: &config.ShowConfig{
				CueFilePattern: "MYR001.cue",
			},
			wantError: false,
			wantFile:  createdFiles[0],
		},
		{
			name: "no matching files",
			showCfg: &config.ShowConfig{
				CueFilePattern: "NOMATCH*.cue",
			},
			wantError: true,
		},
		{
			name: "invalid pattern",
			showCfg: &config.ShowConfig{
				CueFilePattern: "[",
			},
			wantError: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := resolver.ResolveCueFile(tt.showCfg)
			
			if tt.wantError {
				if err == nil {
					t.Error("ResolveCueFile() should return error")
				}
			} else {
				if err != nil {
					t.Errorf("ResolveCueFile() error = %v", err)
				}
				if result != tt.wantFile {
					t.Errorf("ResolveCueFile() = %v, want %v", result, tt.wantFile)
				}
			}
		})
	}
}

func TestResolveCueFileErrors(t *testing.T) {
	resolver := NewCueResolver("/tmp")
	
	// Test nil config
	_, err := resolver.ResolveCueFile(nil)
	if err == nil {
		t.Error("ResolveCueFile() should return error for nil config")
	}
	
	// Test config with no CUE file source
	showCfg := &config.ShowConfig{
		ShowNamePattern: "Test Show",
	}
	_, err = resolver.ResolveCueFile(showCfg)
	if err == nil {
		t.Error("ResolveCueFile() should return error for config with no CUE file source")
	}
}

func TestFindLatestFile(t *testing.T) {
	// Create temporary directory and files
	tmpDir := t.TempDir()
	resolver := NewCueResolver(tmpDir)
	
	// Create files with different modification times
	files := []string{"old.cue", "newer.cue", "newest.cue"}
	var filePaths []string
	
	for i, filename := range files {
		filePath := filepath.Join(tmpDir, filename)
		err := os.WriteFile(filePath, []byte("TEST"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
		filePaths = append(filePaths, filePath)
		
		// Set modification time (newer files have later times)
		modTime := time.Now().Add(-time.Duration(len(files)-i) * time.Hour)
		err = os.Chtimes(filePath, modTime, modTime)
		if err != nil {
			t.Fatalf("Failed to set modification time: %v", err)
		}
	}
	
	tests := []struct {
		name      string
		files     []string
		wantError bool
		expected  string
	}{
		{
			name:      "multiple files - newest first",
			files:     filePaths,
			wantError: false,
			expected:  filePaths[2], // newest.cue
		},
		{
			name:      "single file",
			files:     []string{filePaths[0]},
			wantError: false,
			expected:  filePaths[0],
		},
		{
			name:      "no files",
			files:     []string{},
			wantError: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := resolver.findLatestFile(tt.files)
			
			if tt.wantError {
				if err == nil {
					t.Error("findLatestFile() should return error")
				}
			} else {
				if err != nil {
					t.Errorf("findLatestFile() error = %v", err)
				}
				if result != tt.expected {
					t.Errorf("findLatestFile() = %v, want %v", result, tt.expected)
				}
			}
		})
	}
}

func TestValidateCueFile(t *testing.T) {
	tmpDir := t.TempDir()
	resolver := NewCueResolver(tmpDir)
	
	// Create valid CUE file
	validFile := filepath.Join(tmpDir, "valid.cue")
	err := os.WriteFile(validFile, []byte("VALID CONTENT"), 0644)
	if err != nil {
		t.Fatalf("Failed to create valid test file: %v", err)
	}
	
	// Create empty CUE file
	emptyFile := filepath.Join(tmpDir, "empty.cue")
	err = os.WriteFile(emptyFile, []byte(""), 0644)
	if err != nil {
		t.Fatalf("Failed to create empty test file: %v", err)
	}
	
	// Create non-CUE file
	nonCueFile := filepath.Join(tmpDir, "notcue.txt")
	err = os.WriteFile(nonCueFile, []byte("NOT CUE"), 0644)
	if err != nil {
		t.Fatalf("Failed to create non-CUE test file: %v", err)
	}
	
	tests := []struct {
		name      string
		filePath  string
		wantError bool
		errorText string
	}{
		{
			name:      "valid CUE file",
			filePath:  validFile,
			wantError: false,
		},
		{
			name:      "non-existent file",
			filePath:  filepath.Join(tmpDir, "nonexistent.cue"),
			wantError: true,
			errorText: "does not exist",
		},
		{
			name:      "empty CUE file",
			filePath:  emptyFile,
			wantError: true,
			errorText: "is empty",
		},
		{
			name:      "non-CUE extension",
			filePath:  nonCueFile,
			wantError: true,
			errorText: "does not have .cue extension",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := resolver.ValidateCueFile(tt.filePath)
			
			if tt.wantError {
				if err == nil {
					t.Error("ValidateCueFile() should return error")
				} else if tt.errorText != "" && !contains(err.Error(), tt.errorText) {
					t.Errorf("Error should contain %q, got: %v", tt.errorText, err)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateCueFile() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestListCueFiles(t *testing.T) {
	tmpDir := t.TempDir()
	resolver := NewCueResolver(tmpDir)
	
	// Create test files
	testFiles := []string{"show1.cue", "show2.cue", "notcue.txt"}
	expectedCueFiles := 2
	
	for _, filename := range testFiles {
		filePath := filepath.Join(tmpDir, filename)
		err := os.WriteFile(filePath, []byte("TEST"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file %s: %v", filename, err)
		}
	}
	
	files, err := resolver.ListCueFiles()
	if err != nil {
		t.Errorf("ListCueFiles() error = %v", err)
	}
	
	if len(files) != expectedCueFiles {
		t.Errorf("ListCueFiles() returned %d files, want %d", len(files), expectedCueFiles)
	}
	
	// Verify all returned files have .cue extension
	for _, file := range files {
		if filepath.Ext(file) != ".cue" {
			t.Errorf("ListCueFiles() returned non-CUE file: %s", file)
		}
	}
}

func TestFindCueFilesByPattern(t *testing.T) {
	tmpDir := t.TempDir()
	resolver := NewCueResolver(tmpDir)
	
	// Create test files
	testFiles := []string{"MYR001.cue", "MYR002.cue", "OTHER001.cue", "notcue.txt"}
	
	for _, filename := range testFiles {
		filePath := filepath.Join(tmpDir, filename)
		err := os.WriteFile(filePath, []byte("TEST"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file %s: %v", filename, err)
		}
	}
	
	tests := []struct {
		name        string
		pattern     string
		expectedCount int
		wantError   bool
	}{
		{
			name:        "MYR pattern",
			pattern:     "MYR*.cue",
			expectedCount: 2,
			wantError:   false,
		},
		{
			name:        "OTHER pattern",
			pattern:     "OTHER*.cue",
			expectedCount: 1,
			wantError:   false,
		},
		{
			name:        "no matches",
			pattern:     "NOMATCH*.cue",
			expectedCount: 0,
			wantError:   false,
		},
		{
			name:        "invalid pattern",
			pattern:     "[",
			expectedCount: 0,
			wantError:   true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files, err := resolver.FindCueFilesByPattern(tt.pattern)
			
			if tt.wantError {
				if err == nil {
					t.Error("FindCueFilesByPattern() should return error")
				}
			} else {
				if err != nil {
					t.Errorf("FindCueFilesByPattern() error = %v", err)
				}
				if len(files) != tt.expectedCount {
					t.Errorf("FindCueFilesByPattern() returned %d files, want %d", len(files), tt.expectedCount)
				}
			}
		})
	}
}

func TestGetFileAge(t *testing.T) {
	tmpDir := t.TempDir()
	resolver := NewCueResolver(tmpDir)
	
	// Create test file
	testFile := filepath.Join(tmpDir, "test.cue")
	err := os.WriteFile(testFile, []byte("TEST"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	
	// Set file modification time to 1 hour ago
	pastTime := time.Now().Add(-1 * time.Hour)
	err = os.Chtimes(testFile, pastTime, pastTime)
	if err != nil {
		t.Fatalf("Failed to set file time: %v", err)
	}
	
	age, err := resolver.GetFileAge(testFile)
	if err != nil {
		t.Errorf("GetFileAge() error = %v", err)
	}
	
	// Age should be approximately 1 hour (with some tolerance)
	expectedAge := 1 * time.Hour
	tolerance := 5 * time.Minute
	
	if age < expectedAge-tolerance || age > expectedAge+tolerance {
		t.Errorf("GetFileAge() = %v, want approximately %v", age, expectedAge)
	}
}

func TestIsFileNewer(t *testing.T) {
	tmpDir := t.TempDir()
	resolver := NewCueResolver(tmpDir)
	
	// Create two test files
	file1 := filepath.Join(tmpDir, "newer.cue")
	file2 := filepath.Join(tmpDir, "older.cue")
	
	err := os.WriteFile(file1, []byte("TEST"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file 1: %v", err)
	}
	
	err = os.WriteFile(file2, []byte("TEST"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file 2: %v", err)
	}
	
	// Set different modification times
	newerTime := time.Now()
	olderTime := newerTime.Add(-1 * time.Hour)
	
	err = os.Chtimes(file1, newerTime, newerTime)
	if err != nil {
		t.Fatalf("Failed to set file1 time: %v", err)
	}
	
	err = os.Chtimes(file2, olderTime, olderTime)
	if err != nil {
		t.Fatalf("Failed to set file2 time: %v", err)
	}
	
	// Test if newer file is indeed newer
	isNewer, err := resolver.IsFileNewer(file1, file2)
	if err != nil {
		t.Errorf("IsFileNewer() error = %v", err)
	}
	
	if !isNewer {
		t.Error("IsFileNewer() should return true for newer file")
	}
	
	// Test reverse
	isNewer, err = resolver.IsFileNewer(file2, file1)
	if err != nil {
		t.Errorf("IsFileNewer() error = %v", err)
	}
	
	if isNewer {
		t.Error("IsFileNewer() should return false for older file")
	}
}