// Copyright 2026 Georg Hagn (tiny-frameworks)
// SPDX-License-Identifier: Apache-2.0

package errors

import (
	stdErrors "errors"
	"fmt"
)

type Code string

type Error struct {
	Code    Code
	Message string
	Path    string
	Cause   error
}

// Implementation of the standard error interface
func (e *Error) Error() string {
	pathStr := ""
	if e.Path != "" {
		pathStr = fmt.Sprintf(" (Path: %s)", e.Path)
	}

	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s%s: %v", e.Code, e.Message, pathStr, e.Cause)
	}
	return fmt.Sprintf("[%s] %s%s", e.Code, e.Message, pathStr)
}

func New(code Code, msg string, path string) *Error {
	return &Error{
		Code:    code,
		Message: msg,
		Path:    path,
	}
}

func Wrap(code Code, msg string, path string, cause error) *Error {
	return &Error{
		Code:    code,
		Message: msg,
		Path:    path,
		Cause:   cause,
	}
}

func (e *Error) Unwrap() error {
	return e.Cause
}

// GetExitCode finds the correct code, no matter how deeply the error is wrapped.
func GetExitCode(err error) int {
	var nexErr *Error
	if As(err, &nexErr) {
		if info, exists := exitCodeMap[nexErr.Code]; exists {
			return info.ExitCode
		}
	}
	return 1 // Standard fallback
}

// Error Logging Section
// ErrorLogger is an interface that can be implemented by any logger supporting
// an error method with variable arguments.
type ErrorLogger interface {
	Error(msg string, args ...any)
}

// LogError logs an error in a structured manner to the provided slog.Logger.
func LogError(log ErrorLogger, err error) {
	if err == nil {
		return
	}

	var ne *Error
	if As(err, &ne) { // Structured nexgate error
		// We collect all attributes for this log entry
		args := []any{
			"code", ne.Code,
			"path", ne.Path,
		}

		// If an underlying cause exists, we add it as "cause".
		if ne.Cause != nil {
			args = append(args, "cause", ne.Cause)
		}

		log.Error(ne.Message, args...)
	} else { // Unknown / technical error (e.g., standard Go errors)
		log.Error(err.Error())
	}
}

// -------------------------------------------------------------
// We pass the analysis functions through to the standard.:
// -------------------------------------------------------------

func Is(err, target error) bool {
	return stdErrors.Is(err, target)
}

func As(err error, target any) bool {
	return stdErrors.As(err, target)
}

func Unwrap(err error) error {
	return stdErrors.Unwrap(err)
}
