// Package errors defines typed CLI errors that map to distinct exit codes.
//
// Exit codes:
//
//	0  success
//	1  general / configuration error
//	2  usage / bad input error
//	3  API / connectivity error
package errors

import (
	"errors"
	"fmt"
)

// UsageError signals invalid user input or missing arguments. Exit code 2.
type UsageError struct{ msg string }

func (e *UsageError) Error() string { return e.msg }

// NewUsageError returns a UsageError.
func NewUsageError(format string, args ...any) error {
	return &UsageError{msg: fmt.Sprintf(format, args...)}
}

// APIError signals a failed call to an external API or service. Exit code 3.
type APIError struct{ msg string }

func (e *APIError) Error() string { return e.msg }

// NewAPIError returns an APIError.
func NewAPIError(format string, args ...any) error {
	return &APIError{msg: fmt.Sprintf(format, args...)}
}

// ConfigError signals missing or invalid configuration. Exit code 1.
type ConfigError struct{ msg string }

func (e *ConfigError) Error() string { return e.msg }

// NewConfigError returns a ConfigError.
func NewConfigError(format string, args ...any) error {
	return &ConfigError{msg: fmt.Sprintf(format, args...)}
}

// ExitCode maps an error to the appropriate process exit code.
func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	var u *UsageError
	if errors.As(err, &u) {
		return 2
	}
	var a *APIError
	if errors.As(err, &a) {
		return 3
	}
	return 1
}
