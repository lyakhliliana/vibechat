package config

import (
	"fmt"
	"strings"
)

// ValidationError accumulates all config errors so the caller sees them all at once.
type ValidationError struct {
	errs []fieldError
}

type fieldError struct {
	field   string
	message string
}

func (e *ValidationError) Add(field, message string) {
	e.errs = append(e.errs, fieldError{field: field, message: message})
}

func (e *ValidationError) Addf(field, format string, args ...any) {
	e.Add(field, fmt.Sprintf(format, args...))
}

func (e *ValidationError) HasErrors() bool {
	return len(e.errs) > 0
}

func (e *ValidationError) Err() error {
	if !e.HasErrors() {
		return nil
	}
	return e
}

func (e *ValidationError) AddFrom(section string, other error) {
	if other == nil {
		return
	}
	if ve, ok := other.(*ValidationError); ok {
		for _, fe := range ve.errs {
			e.Add(section+"."+fe.field, fe.message)
		}
		return
	}
	e.Add(section, other.Error())
}

func (e *ValidationError) Error() string {
	lines := make([]string, 0, len(e.errs))
	for _, fe := range e.errs {
		lines = append(lines, fmt.Sprintf("  %-20s %s", fe.field+":", fe.message))
	}
	return "config validation failed:\n" + strings.Join(lines, "\n")
}
