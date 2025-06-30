// Package dateutil provides unified date formatting utilities for consistent
// date handling across the application. It converts user-friendly date format
// patterns to Go time layouts and formats dates accordingly.
package dateutil

import (
	"strings"
	"time"
)

// FormatDateToGoLayout converts user-friendly date format patterns to Go time reference patterns.
// Supports patterns like "YYYY", "MM", "DD", etc. consistent across the application.
//
// Example conversions:
//   - "YYYY" -> "2006" (4-digit year)
//   - "MM" -> "01" (2-digit month with leading zero)
//   - "DD" -> "02" (2-digit day with leading zero)
//   - "M/D/YYYY" -> "1/2/2006"
//   - "YYYYMMDD" -> "20060102"
func FormatDateToGoLayout(userFormat string) string {
	// AIDEV-NOTE: Order matters - replace longer patterns first to avoid partial matches
	replacer := strings.NewReplacer(
		"YYYY", "2006", // 4-digit year
		"YY", "06",     // 2-digit year
		"MM", "01",     // 2-digit month with leading zero
		"M", "1",       // 1-2 digit month without leading zero
		"DD", "02",     // 2-digit day with leading zero
		"D", "2",       // 1-2 digit day without leading zero
	)
	
	return replacer.Replace(userFormat)
}

// FormatDateWithPattern formats a time using a user-friendly pattern.
// This is a convenience function that combines pattern conversion and formatting.
func FormatDateWithPattern(t time.Time, userPattern string) string {
	goLayout := FormatDateToGoLayout(userPattern)
	return t.Format(goLayout)
}

// ParseFlexibleDate attempts to parse a date string using various common formats.
// This handles different input formats that users might provide.
func ParseFlexibleDate(dateStr string) (time.Time, error) {
	// AIDEV-NOTE: Common date formats users might input - ordered by likelihood
	formats := []string{
		"1/2/2006",      // M/D/YYYY
		"01/02/2006",    // MM/DD/YYYY
		"2006-01-02",    // YYYY-MM-DD
		"2006/01/02",    // YYYY/MM/DD
		"2/1/2006",      // D/M/YYYY
		"02/01/2006",    // DD/MM/YYYY
		"1-2-2006",      // M-D-YYYY
		"01-02-2006",    // MM-DD-YYYY
		"2006.01.02",    // YYYY.MM.DD
		"20060102",      // YYYYMMDD
	}
	
	for _, format := range formats {
		if parsed, err := time.Parse(format, dateStr); err == nil {
			return parsed, nil
		}
	}
	
	return time.Time{}, &time.ParseError{
		Layout: "multiple common formats",
		Value:  dateStr,
	}
}