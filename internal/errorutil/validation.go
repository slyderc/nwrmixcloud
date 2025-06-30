package errorutil

import (
	"fmt"
	"regexp"
	"strings"
)

// ValidationError represents a collection of validation failures
type ValidationError struct {
	Context string
	Errors  []FieldError
}

// FieldError represents a single field validation failure
type FieldError struct {
	Field   string
	Value   interface{}
	Message string
}

func (e *ValidationError) Error() string {
	if len(e.Errors) == 0 {
		return fmt.Sprintf("%s validation failed", e.Context)
	}

	var messages []string
	for _, fieldErr := range e.Errors {
		messages = append(messages, fmt.Sprintf("%s: %s", fieldErr.Field, fieldErr.Message))
	}

	return fmt.Sprintf("%s validation failed: %s", e.Context, strings.Join(messages, "; "))
}

// ValidationBuilder helps accumulate validation errors using builder pattern
// Consolidates the common pattern of field validation with error accumulation
type ValidationBuilder struct {
	context string
	errors  []FieldError
}

// NewValidationBuilder creates a new validation builder with context
func NewValidationBuilder(context string) *ValidationBuilder {
	return &ValidationBuilder{
		context: context,
		errors:  make([]FieldError, 0),
	}
}

// RequiredString validates that a string field is not empty
func (vb *ValidationBuilder) RequiredString(field, value string) *ValidationBuilder {
	if IsEmptyString(value) {
		vb.errors = append(vb.errors, FieldError{
			Field:   field,
			Value:   value,
			Message: "is required",
		})
	}
	return vb
}

// RequiredInt validates that an integer field has a non-zero value
func (vb *ValidationBuilder) RequiredInt(field string, value int) *ValidationBuilder {
	if value <= 0 {
		vb.errors = append(vb.errors, FieldError{
			Field:   field,
			Value:   value,
			Message: "must be greater than 0",
		})
	}
	return vb
}

// MinLength validates minimum string length
func (vb *ValidationBuilder) MinLength(field, value string, minLen int) *ValidationBuilder {
	if len(value) < minLen {
		vb.errors = append(vb.errors, FieldError{
			Field:   field,
			Value:   value,
			Message: fmt.Sprintf("must be at least %d characters", minLen),
		})
	}
	return vb
}

// MaxLength validates maximum string length
func (vb *ValidationBuilder) MaxLength(field, value string, maxLen int) *ValidationBuilder {
	if len(value) > maxLen {
		vb.errors = append(vb.errors, FieldError{
			Field:   field,
			Value:   value,
			Message: fmt.Sprintf("must be no more than %d characters", maxLen),
		})
	}
	return vb
}

// Pattern validates string against regular expression
func (vb *ValidationBuilder) Pattern(field, value, pattern string) *ValidationBuilder {
	if value == "" {
		return vb // Skip pattern validation for empty strings
	}

	matched, err := regexp.MatchString(pattern, value)
	if err != nil {
		vb.errors = append(vb.errors, FieldError{
			Field:   field,
			Value:   value,
			Message: fmt.Sprintf("pattern validation failed: %v", err),
		})
		return vb
	}

	if !matched {
		vb.errors = append(vb.errors, FieldError{
			Field:   field,
			Value:   value,
			Message: "does not match required pattern",
		})
	}
	return vb
}

// OneOf validates that value is one of the allowed options
func (vb *ValidationBuilder) OneOf(field, value string, options []string) *ValidationBuilder {
	if value == "" {
		return vb // Skip validation for empty strings
	}

	for _, option := range options {
		if value == option {
			return vb
		}
	}

	vb.errors = append(vb.errors, FieldError{
		Field:   field,
		Value:   value,
		Message: fmt.Sprintf("must be one of: %s", strings.Join(options, ", ")),
	})
	return vb
}

// Custom allows adding custom validation with a predicate function
func (vb *ValidationBuilder) Custom(field string, value interface{}, predicate func(interface{}) bool, message string) *ValidationBuilder {
	if !predicate(value) {
		vb.errors = append(vb.errors, FieldError{
			Field:   field,
			Value:   value,
			Message: message,
		})
	}
	return vb
}

// ValidIf conditionally applies validation based on a condition
func (vb *ValidationBuilder) ValidIf(condition bool, validationFunc func(*ValidationBuilder) *ValidationBuilder) *ValidationBuilder {
	if condition {
		return validationFunc(vb)
	}
	return vb
}

// Build returns the validation error if any errors were collected, nil otherwise
func (vb *ValidationBuilder) Build() error {
	if len(vb.errors) == 0 {
		return nil
	}

	return &ValidationError{
		Context: vb.context,
		Errors:  vb.errors,
	}
}

// HasErrors returns true if the builder has collected any validation errors
func (vb *ValidationBuilder) HasErrors() bool {
	return len(vb.errors) > 0
}

// ErrorCount returns the number of validation errors collected
func (vb *ValidationBuilder) ErrorCount() int {
	return len(vb.errors)
}

// Utility functions for common validation patterns

// IsEmptyString checks if a string is empty after trimming whitespace
// Consolidates the common pattern of strings.TrimSpace(x) == ""
func IsEmptyString(s string) bool {
	return strings.TrimSpace(s) == ""
}

// ValidateRequired checks multiple required string fields at once
// Returns error if any are empty, nil otherwise
func ValidateRequired(context string, fields map[string]string) error {
	vb := NewValidationBuilder(context)
	
	for fieldName, fieldValue := range fields {
		vb.RequiredString(fieldName, fieldValue)
	}
	
	return vb.Build()
}

// ValidateStringFields provides a quick way to validate multiple string constraints
type StringConstraint struct {
	Field    string
	Value    string
	Required bool
	MinLen   int
	MaxLen   int
	Pattern  string
	Options  []string
}

// ValidateStringFields validates multiple string fields with various constraints
func ValidateStringFields(context string, constraints []StringConstraint) error {
	vb := NewValidationBuilder(context)
	
	for _, constraint := range constraints {
		if constraint.Required {
			vb.RequiredString(constraint.Field, constraint.Value)
		}
		
		if constraint.MinLen > 0 {
			vb.MinLength(constraint.Field, constraint.Value, constraint.MinLen)
		}
		
		if constraint.MaxLen > 0 {
			vb.MaxLength(constraint.Field, constraint.Value, constraint.MaxLen)
		}
		
		if constraint.Pattern != "" {
			vb.Pattern(constraint.Field, constraint.Value, constraint.Pattern)
		}
		
		if len(constraint.Options) > 0 {
			vb.OneOf(constraint.Field, constraint.Value, constraint.Options)
		}
	}
	
	return vb.Build()
}

// ValidateConfig provides a helper for configuration validation
// Consolidates the common pattern of configuration field validation
func ValidateConfig(configName string, validations func(*ValidationBuilder) *ValidationBuilder) error {
	vb := NewValidationBuilder(configName + " configuration")
	vb = validations(vb)
	return vb.Build()
}