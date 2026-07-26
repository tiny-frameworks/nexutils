## nexutils/errors
<sup>the *errors module*, part of **GSF-nexutils**, member of the **tiny-frameworks** family</sup>

---

The `errors` module provides a structured, lightweight error handling framework for Go applications. It combines domain-specific error codes, contextual tracking paths, full compatibility with standard Go error unwrapping (`errors.Is` / `errors.As`), and seamless integration with structured loggers.

---

## Features

* **Structured Error Codes:** Categorize errors with type-safe, domain-specific error codes.
* **Context & Paths:** Attach call-site or component path information for easy debugging.
* **Go Standard Compatibility:** Implements `Unwrap()` to work seamlessly with `errors.Is()` and `errors.As()`.
* **Exit-Code Mapping:** Central registry to map application error codes directly to OS exit codes (ideal for CLI tools and microservices).
* **Structured Logging Integration:** Built-in helper (`LogError`) to pass structured key-value context directly to loggers implementing an `Error(msg string, args ...any)` interface (e.g., `slog`).

---

## Installation

```bash
go get codeberg.org/tiny-frameworks/nexutils/errors

```

---

## Quick Start

### 1. Creating and Wrapping Errors

```go
package main

import (
	stdErrors "errors"
	"fmt"

	"codeberg.org/tiny-frameworks/nexutils/errors"
)

func main() {
	// Create a new structured error
	err := errors.New(
		errors.MissingField,
		"mandatory field 'email' is missing",
		"user.service.Validate()",
	)
	fmt.Println(err)
	// Output: [MISSING_FIELD] mandatory field 'email' is missing (Path: user.service.Validate())

	// Wrap an existing underlying error
	cause := stdErrors.New("connection timeout")
	wrappedErr := errors.Wrap(
		errors.ReadError,
		"failed to read configuration from remote",
		"config.loader.Fetch()",
		cause,
	)
	fmt.Println(wrappedErr)
	// Output: [READ_ERROR] failed to read configuration from remote (Path: config.loader.Fetch()): connection timeout
}

```

---

## Core Concepts

### Go Standard Compatibility (`Is` & `As`)

The `errors` module re-exports `errors.Is`, `errors.As`, and `errors.Unwrap` from the Go standard library, ensuring full chain inspection while maintaining a single import footprint:

```go
// Check if a specific root cause exists anywhere in the error chain
if errors.Is(err, stdErrors.ErrNotExist) {
	// Handle missing resource...
}

// Extract a structured *errors.Error from an unknown error chain
var nexErr *errors.Error
if errors.As(err, &nexErr) {
	fmt.Printf("Error Code: %s, Path: %s\n", nexErr.Code, nexErr.Path)
}

```

---

### Structured Logging with `LogError`

The `LogError` utility automatically extracts structured fields (`code`, `path`, `cause`) from an error and forwards them as key-value pairs to any compatible logger (such as `log/slog` or custom loggers):

```go
type Logger interface {
	Error(msg string, args ...any)
}

// Usage in your application code
errors.LogError(logger, err)
// Automatically logs: msg="failed to read..." code="READ_ERROR" path="config.loader" cause="..."

```

---

### Exit Code Registry

Errors can be mapped to predefined OS exit codes for consistent process exit behavior across services and CLI utilities:

```go
// Retrieve the numerical exit code associated with a specific Error Code
exitCode := errors.GetExitCode(err)
os.Exit(exitCode)

```

#### Exit Code Range Convention

| Range | Domain / Category | Description |
| --- | --- | --- |
| **10** | Input / Config | Missing parameters, invalid payload format, bad config. |
| **30** | Validation & Business | Rule violations, inconsistent data state, mismatch errors. |
| **40** | Infrastructure & I/O | Engine errors, timeout, lock contention, file read/write issues. |
| **90–99** | System / Internal | Unhandled runtime errors, internal panics, or unimplemented features. |

---

## API Reference

| Function / Method | Description |
| --- | --- |
| `New(code, msg, path)` | Creates a new structured `*Error` with the given code and component path. |
| `Wrap(code, msg, path, cause)` | Wraps an underlying `error` as the root cause of a new `*Error`. |
| `LogError(logger, err)` | Safely extracts error attributes and logs them via a structured logger interface. |
| `GetExitCode(err)` | Resolves the mapped numerical exit code (defaults to `99` / `InternalError`). |
| `Is(err, target)` | Re-exported `errors.Is` from Go standard library. |
| `As(err, target)` | Re-exported `errors.As` from Go standard library. |
| `Unwrap(err)` | Extracts the immediate cause underlying a wrapped error. |

---

## Testing

Run unit tests and verify error chain behavior:

```bash
go test -v ./...

```

---

## Organizational & Standards

* **Copyright:** © 2026 Georg Hagn.
* **Namespace:** `codeberg.org/tiny-frameworks/nexutils/errors`
* **License:** Apache License, Version 2.0.

*GSF-nexutils/errors is an independent open-source project and is not affiliated with any corporation of a similar name.*

---

## Contact

If you have questions or feedback, feel free to reach out:

📧 *georghagn [at] tiny-frameworks.io*

```

---

