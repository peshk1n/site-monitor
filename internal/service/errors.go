package service

import "errors"

var (
	ErrNotFound     = errors.New("object unavailable")
	ErrForbidden    = errors.New("action not allowed")
	ErrConflict     = errors.New("object has changed or action conflicts with its state")
	ErrUnauthorized = errors.New("authentication required")
)

type ValidationError struct{ Message string }

func (e *ValidationError) Error() string { return e.Message }
func Invalid(message string) error       { return &ValidationError{Message: message} }
