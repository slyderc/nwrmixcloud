package errorutil

import (
	"fmt"
	"os"
	"path/filepath"
)

// FileOpError provides structured error information for file operations
type FileOpError struct {
	Operation string
	Path      string
	Err       error
}

func (e *FileOpError) Error() string {
	return fmt.Sprintf("%s failed for %s: %v", e.Operation, e.Path, e.Err)
}

func (e *FileOpError) Unwrap() error {
	return e.Err
}

// ValidateFileExists checks if a file exists and is accessible
// Returns FileOpError with context for better error messages
func ValidateFileExists(filePath, operation string) error {
	if filePath == "" {
		return &FileOpError{
			Operation: operation,
			Path:      filePath,
			Err:       fmt.Errorf("empty file path provided"),
		}
	}

	info, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return &FileOpError{
				Operation: operation,
				Path:      filePath,
				Err:       fmt.Errorf("file not found"),
			}
		}
		return &FileOpError{
			Operation: operation,
			Path:      filePath,
			Err:       fmt.Errorf("cannot access file: %w", err),
		}
	}

	if info.IsDir() {
		return &FileOpError{
			Operation: operation,
			Path:      filePath,
			Err:       fmt.Errorf("path is a directory, expected file"),
		}
	}

	return nil
}

// ValidateDirectory checks if a directory exists and optionally creates it
// Consolidates the common pattern of directory validation with creation
func ValidateDirectory(dirPath, operation string, createIfMissing bool) error {
	if dirPath == "" {
		return &FileOpError{
			Operation: operation,
			Path:      dirPath,
			Err:       fmt.Errorf("empty directory path provided"),
		}
	}

	info, err := os.Stat(dirPath)
	if err != nil {
		if os.IsNotExist(err) {
			if createIfMissing {
				if mkdirErr := os.MkdirAll(dirPath, 0755); mkdirErr != nil {
					return &FileOpError{
						Operation: operation,
						Path:      dirPath,
						Err:       fmt.Errorf("failed to create directory: %w", mkdirErr),
					}
				}
				return nil
			}
			return &FileOpError{
				Operation: operation,
				Path:      dirPath,
				Err:       fmt.Errorf("directory not found"),
			}
		}
		return &FileOpError{
			Operation: operation,
			Path:      dirPath,
			Err:       fmt.Errorf("cannot access directory: %w", err),
		}
	}

	if !info.IsDir() {
		return &FileOpError{
			Operation: operation,
			Path:      dirPath,
			Err:       fmt.Errorf("path exists but is not a directory"),
		}
	}

	return nil
}

// ValidateFileReadable checks if a file exists and is readable
// Consolidates file existence and permission checking
func ValidateFileReadable(filePath, operation string) error {
	if err := ValidateFileExists(filePath, operation); err != nil {
		return err
	}

	file, err := os.Open(filePath)
	if err != nil {
		return &FileOpError{
			Operation: operation,
			Path:      filePath,
			Err:       fmt.Errorf("cannot open file for reading: %w", err),
		}
	}
	file.Close()

	return nil
}

// ValidateGlobPattern validates and resolves glob patterns
// Consolidates the common pattern of glob pattern validation and resolution
func ValidateGlobPattern(pattern, baseDir, operation string) ([]string, error) {
	if pattern == "" {
		return nil, &FileOpError{
			Operation: operation,
			Path:      pattern,
			Err:       fmt.Errorf("empty glob pattern provided"),
		}
	}

	var fullPattern string
	if filepath.IsAbs(pattern) {
		fullPattern = pattern
	} else {
		fullPattern = filepath.Join(baseDir, pattern)
	}

	matches, err := filepath.Glob(fullPattern)
	if err != nil {
		return nil, &FileOpError{
			Operation: operation,
			Path:      pattern,
			Err:       fmt.Errorf("invalid glob pattern: %w", err),
		}
	}

	if len(matches) == 0 {
		return nil, &FileOpError{
			Operation: operation,
			Path:      pattern,
			Err:       fmt.Errorf("no files match pattern"),
		}
	}

	return matches, nil
}

// SafeCreateFile creates a file with proper error handling and context
// Consolidates the common pattern of file creation with directory validation
func SafeCreateFile(filePath, operation string, createDir bool) (*os.File, error) {
	if filePath == "" {
		return nil, &FileOpError{
			Operation: operation,
			Path:      filePath,
			Err:       fmt.Errorf("empty file path provided"),
		}
	}

	dir := filepath.Dir(filePath)
	if createDir {
		if err := ValidateDirectory(dir, operation, true); err != nil {
			return nil, err
		}
	}

	file, err := os.Create(filePath)
	if err != nil {
		return nil, &FileOpError{
			Operation: operation,
			Path:      filePath,
			Err:       fmt.Errorf("failed to create file: %w", err),
		}
	}

	return file, nil
}

// SafeWriteFile writes data to a file with proper error handling
// Consolidates the common pattern of file writing with directory creation
func SafeWriteFile(filePath string, data []byte, operation string, createDir bool) error {
	if createDir {
		dir := filepath.Dir(filePath)
		if err := ValidateDirectory(dir, operation, true); err != nil {
			return err
		}
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return &FileOpError{
			Operation: operation,
			Path:      filePath,
			Err:       fmt.Errorf("failed to write file: %w", err),
		}
	}

	return nil
}

// GetLatestFile finds the most recently modified file matching a pattern
// Consolidates the pattern used in CUE file resolution
func GetLatestFile(pattern, baseDir, operation string) (string, error) {
	matches, err := ValidateGlobPattern(pattern, baseDir, operation)
	if err != nil {
		return "", err
	}

	var latestFile string
	var latestTime int64

	for _, match := range matches {
		info, err := os.Stat(match)
		if err != nil {
			continue // Skip files we can't stat
		}

		if info.ModTime().Unix() > latestTime {
			latestTime = info.ModTime().Unix()
			latestFile = match
		}
	}

	if latestFile == "" {
		return "", &FileOpError{
			Operation: operation,
			Path:      pattern,
			Err:       fmt.Errorf("no accessible files found matching pattern"),
		}
	}

	return latestFile, nil
}