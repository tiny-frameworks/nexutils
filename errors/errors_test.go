// Copyright 2026 Georg Hagn (tiny-frameworks)
// SPDX-License-Identifier: Apache-2.0

package errors_test

import (
	stdErrors "errors"
	"testing"

	"codeberg.org/tiny-frameworks/nexutils/errors"
)

// 1. Test for the formatting of the Error() method
func TestError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *errors.Error
		expected string
	}{
		{
			name:     "Error without Cause",
			err:      errors.New("ERR_NOT_FOUND", "resource missing", "/api/v1/users"),
			expected: "[ERR_NOT_FOUND] resource missing (Path: /api/v1/users)",
		},
		{
			name:     "Error with Cause via Wrap",
			err:      errors.Wrap("ERR_DB", "query failed", "/db", stdErrors.New("connection timeout")),
			expected: "[ERR_DB] query failed (Path: /db): connection timeout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.expected {
				t.Errorf("Error() = %q, want %q", got, tt.expected)
			}
		})
	}
}

// 2. Test for Unwrap & Go standard compatibility (Is / As)
func TestError_UnwrapAndStandardCompat(t *testing.T) {
	rootErr := stdErrors.New("root cause")
	nexErr := errors.Wrap("ERR_INTERNAL", "something went wrong", "/core", rootErr)

	// Test direct Unwrap
	if got := errors.Unwrap(nexErr); got != rootErr {
		t.Errorf("Unwrap() = %v, want %v", got, rootErr)
	}

	// Test Go-Standard errors.Is Compatibility via re-export
	if !errors.Is(nexErr, rootErr) {
		t.Errorf("errors.Is() failed to find root cause in error chain")
	}

	// Teste Go-Standard errors.As Compatibility
	var target *errors.Error
	if !errors.As(nexErr, &target) {
		t.Errorf("errors.As() failed to extract *errors.Error from chain")
	} else if target.Code != "ERR_INTERNAL" {
		t.Errorf("errors.As() extracted wrong Code = %q, want %q", target.Code, "ERR_INTERNAL")
	}
}

// 3. Mock-Logger for LogError Test
type mockLogger struct {
	lastMsg  string
	lastArgs []any
}

func (m *mockLogger) Error(msg string, args ...any) {
	m.lastMsg = msg
	m.lastArgs = args
}

// 4. Test for LogError
func TestLogError(t *testing.T) {
	t.Run("Logs structured nexerror", func(t *testing.T) {
		logger := &mockLogger{}
		cause := stdErrors.New("sql syntax error")
		err := errors.Wrap("ERR_SQL", "database error", "/db/query", cause)

		errors.LogError(logger, err)

		if logger.lastMsg != "database error" {
			t.Errorf("LogError msg = %q, want %q", logger.lastMsg, "database error")
		}

		// Check for expected key-value arguments
		if len(logger.lastArgs) < 4 {
			t.Fatalf("LogError produced too few args: %v", logger.lastArgs)
		}
	})

	t.Run("Logs nil error without calling logger", func(t *testing.T) {
		logger := &mockLogger{}
		errors.LogError(logger, nil)

		if logger.lastMsg != "" {
			t.Errorf("LogError called logger on nil error")
		}
	})

	t.Run("Logs standard Go error gracefully", func(t *testing.T) {
		logger := &mockLogger{}
		stdErr := stdErrors.New("standard error")

		errors.LogError(logger, stdErr)

		if logger.lastMsg != "standard error" {
			t.Errorf("LogError msg = %q, want %q", logger.lastMsg, "standard error")
		}
	})
}
