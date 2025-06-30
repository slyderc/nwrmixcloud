// Package shows provides CUE file resolution functionality for finding
// and selecting CUE files based on patterns, direct mappings, and date-based selection.
package shows

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/nowwaveradio/mixcloud-updater/internal/config"
)

// CueResolver handles CUE file detection and pattern matching
type CueResolver struct {
	baseDir string // Base directory for CUE file searches
}

// NewCueResolver creates a new CUE file resolver with the specified base directory
func NewCueResolver(baseDir string) *CueResolver {
	return &CueResolver{
		baseDir: baseDir,
	}
}

// ResolveCueFile resolves a CUE file based on the show configuration
// Returns the absolute path to the CUE file to use
func (cr *CueResolver) ResolveCueFile(showCfg *config.ShowConfig) (string, error) {
	if showCfg == nil {
		return "", fmt.Errorf("show configuration cannot be nil")
	}

	// Direct file mapping takes precedence
	if showCfg.CueFileMapping != "" {
		return cr.resolveDirectMapping(showCfg.CueFileMapping)
	}

	// Pattern-based matching
	if showCfg.CueFilePattern != "" {
		return cr.resolvePattern(showCfg.CueFilePattern)
	}

	return "", fmt.Errorf("no CUE file source configured (cue_file_pattern or cue_file_mapping required)")
}

// resolveDirectMapping handles direct file path mapping
func (cr *CueResolver) resolveDirectMapping(mapping string) (string, error) {
	// Handle both absolute and relative paths
	var fullPath string
	if filepath.IsAbs(mapping) {
		fullPath = mapping
	} else {
		fullPath = filepath.Join(cr.baseDir, mapping)
	}

	// Check if the file exists
	if _, err := os.Stat(fullPath); err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("CUE file not found: %s", fullPath)
		}
		return "", fmt.Errorf("cannot access CUE file %s: %w", fullPath, err)
	}

	return fullPath, nil
}

// resolvePattern handles glob pattern matching and finds the latest file
func (cr *CueResolver) resolvePattern(pattern string) (string, error) {
	// Construct the full pattern path
	var fullPattern string
	if filepath.IsAbs(pattern) {
		fullPattern = pattern
	} else {
		fullPattern = filepath.Join(cr.baseDir, pattern)
	}

	// Find matching files
	matches, err := filepath.Glob(fullPattern)
	if err != nil {
		return "", fmt.Errorf("invalid glob pattern %s: %w", fullPattern, err)
	}

	if len(matches) == 0 {
		return "", fmt.Errorf("no files match pattern: %s", fullPattern)
	}

	// Return the most recent file
	latestFile, err := cr.findLatestFile(matches)
	if err != nil {
		return "", fmt.Errorf("finding latest file: %w", err)
	}

	return latestFile, nil
}

// findLatestFile returns the most recently modified file from a list of file paths
func (cr *CueResolver) findLatestFile(files []string) (string, error) {
	if len(files) == 0 {
		return "", fmt.Errorf("no files provided")
	}

	// Single file case
	if len(files) == 1 {
		return files[0], nil
	}

	// For multiple files, sort by modification time (newest first)
	type fileInfo struct {
		path    string
		modTime time.Time
	}

	var fileInfos []fileInfo
	for _, file := range files {
		info, err := os.Stat(file)
		if err != nil {
			// Skip files that can't be accessed
			continue
		}
		fileInfos = append(fileInfos, fileInfo{
			path:    file,
			modTime: info.ModTime(),
		})
	}

	if len(fileInfos) == 0 {
		return "", fmt.Errorf("no accessible files found")
	}

	// Sort by modification time (newest first)
	sort.Slice(fileInfos, func(i, j int) bool {
		return fileInfos[i].modTime.After(fileInfos[j].modTime)
	})

	return fileInfos[0].path, nil
}

// ValidateCueFile performs basic validation on a CUE file
func (cr *CueResolver) ValidateCueFile(filePath string) error {
	// Check if file exists
	info, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("CUE file does not exist: %s", filePath)
		}
		return fmt.Errorf("cannot access CUE file %s: %w", filePath, err)
	}

	// Check if it's a regular file
	if !info.Mode().IsRegular() {
		return fmt.Errorf("path is not a regular file: %s", filePath)
	}

	// Check file extension (optional validation)
	ext := strings.ToLower(filepath.Ext(filePath))
	if ext != ".cue" {
		return fmt.Errorf("file does not have .cue extension: %s", filePath)
	}

	// Check if file is readable and not empty
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("cannot open CUE file %s: %w", filePath, err)
	}
	defer file.Close()

	// Check file size
	if info.Size() == 0 {
		return fmt.Errorf("CUE file is empty: %s", filePath)
	}

	return nil
}

// ListCueFiles returns all CUE files in the base directory
func (cr *CueResolver) ListCueFiles() ([]string, error) {
	pattern := filepath.Join(cr.baseDir, "*.cue")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("listing CUE files: %w", err)
	}

	// Filter to only regular files
	var validFiles []string
	for _, match := range matches {
		if info, err := os.Stat(match); err == nil && info.Mode().IsRegular() {
			validFiles = append(validFiles, match)
		}
	}

	return validFiles, nil
}

// FindCueFilesByPattern returns all CUE files matching a specific pattern
func (cr *CueResolver) FindCueFilesByPattern(pattern string) ([]string, error) {
	// Construct the full pattern path
	var fullPattern string
	if filepath.IsAbs(pattern) {
		fullPattern = pattern
	} else {
		fullPattern = filepath.Join(cr.baseDir, pattern)
	}

	// Find matching files
	matches, err := filepath.Glob(fullPattern)
	if err != nil {
		return nil, fmt.Errorf("invalid glob pattern %s: %w", fullPattern, err)
	}

	// Filter to only regular files with .cue extension
	var validFiles []string
	for _, match := range matches {
		if info, err := os.Stat(match); err == nil && info.Mode().IsRegular() {
			ext := strings.ToLower(filepath.Ext(match))
			if ext == ".cue" {
				validFiles = append(validFiles, match)
			}
		}
	}

	return validFiles, nil
}

// GetFileAge returns the age of a file as a time.Duration
func (cr *CueResolver) GetFileAge(filePath string) (time.Duration, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return 0, fmt.Errorf("getting file info for %s: %w", filePath, err)
	}

	return time.Since(info.ModTime()), nil
}

// IsFileNewer checks if file1 is newer than file2
func (cr *CueResolver) IsFileNewer(file1, file2 string) (bool, error) {
	info1, err := os.Stat(file1)
	if err != nil {
		return false, fmt.Errorf("getting file info for %s: %w", file1, err)
	}

	info2, err := os.Stat(file2)
	if err != nil {
		return false, fmt.Errorf("getting file info for %s: %w", file2, err)
	}

	return info1.ModTime().After(info2.ModTime()), nil
}

// GetBaseDir returns the base directory used for CUE file resolution
func (cr *CueResolver) GetBaseDir() string {
	return cr.baseDir
}

// SetBaseDir updates the base directory used for CUE file resolution
func (cr *CueResolver) SetBaseDir(baseDir string) {
	cr.baseDir = baseDir
}