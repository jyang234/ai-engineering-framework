// Package validator provides HTTP middleware for JSON request body validation.
package validator

import (
	"context"
	"net/http"
)

// ValidationRule defines a validation constraint for a JSON field.
type ValidationRule struct {
	Field     string
	Required  bool
	Type      string // "string", "number", "bool", "array", "object"
	MinLength int
	MaxLength int
}

// Validator validates incoming JSON request bodies.
type Validator struct {
	rules       []ValidationRule
	maxBodySize int64
}

// contextKey is the key type for storing parsed body in context.
type contextKey struct{}

// NewValidator creates a validator with the given rules and max body size.
func NewValidator(rules []ValidationRule, maxBodySize int64) *Validator {
	// TODO: implement
	return &Validator{rules: rules, maxBodySize: maxBodySize}
}

// Middleware returns HTTP middleware that validates JSON request bodies.
func (v *Validator) Middleware(next http.Handler) http.Handler {
	// TODO: implement
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not implemented", http.StatusNotImplemented)
	})
}

// ParsedBody retrieves the validated JSON body from the request context.
func ParsedBody(ctx context.Context) map[string]interface{} {
	// TODO: implement
	return nil
}
