## nexutils/logging
<sup>the *logging module*, part of **GSF-nexutils**, member of the **tiny-frameworks** family</sup>

---

The `logging` module provides a lightweight, structured logging interface designed for high-performance Go applications. Built on top of Go's standard `log/slog`, it offers pre-configured handlers (JSON and Text), context-aware logging, and direct integration with structured error domain types.

---

## Features

* **Zero External Dependencies:** Built natively on Go's standard `log/slog` library.
* **Dual Output Formats:** Easy switching between human-readable console text and machine-readable JSON for production log aggregators.
* **Domain Error Aware:** Native support for formatting and logging structured `nexutils/errors` with automatic extraction of error codes, component paths, and causes.
* **Context Support:** Pass contextual key-value pairs (`slog.Attr`) through application boundaries.
* **Configurable Log Levels:** Global log-level management (Debug, Info, Warn, Error).

---

## Installation

```bash
go get codeberg.org/tiny-frameworks/nexutils/logging

```

---

## Quick Start

```go
package main

import (
	"codeberg.org/tiny-frameworks/nexutils/errors"
	"codeberg.org/tiny-frameworks/nexutils/logging"
)

func main() {
	// Initialize a JSON logger writing to stdout at INFO level
	log := logging.NewJSONLogger(logging.LevelInfo)

	log.Info("Application starting", "version", "1.0.0", "env", "production")

	// Log structured errors seamlessly
	err := errors.New(errors.InvalidFormat, "invalid email payload", "auth.handler")
	log.Error("Validation failed", "err", err)
}

```

---

## Core Concepts

### Structured Error Formatting

When an error created via `nexutils/errors` is passed to the logger, the module automatically unravels its structured metadata:

```json
{
  "time": "2026-07-26T14:00:00Z",
  "level": "ERROR",
  "msg": "Failed to load configuration",
  "code": "READ_ERROR",
  "path": "config.loader.Fetch",
  "cause": "file not found"
}

```

---

## API Reference

| Function / Method | Description |
| --- | --- |
| `NewJSONLogger(level)` | Creates a structured JSON logger for production environments. |
| `NewTextLogger(level)` | Creates a human-readable text/console logger for development. |
| `With(args...)` | Returns a child logger with pre-attached contextual key-value attributes. |
| `Debug(msg, args...)` | Logs a debug-level message with structured key-value pairs. |
| `Info(msg, args...)` | Logs an info-level message with structured key-value pairs. |
| `Warn(msg, args...)` | Logs a warning-level message with structured key-value pairs. |
| `Error(msg, args...)` | Logs an error-level message with structured key-value pairs. |

---

## Testing

Run unit tests and benchmark logging throughput:

```bash
go test -v ./...

```

---

## Organizational & Standards

* **Copyright:** © 2026 Georg Hagn.
* **Namespace:** `codeberg.org/tiny-frameworks/nexutils/logging`
* **License:** Apache License, Version 2.0.

*GSF-nexutils/logging is an independent open-source project and is not affiliated with any corporation of a similar name.*

---

## Contact

If you have questions or feedback, feel free to reach out:

📧 *georghagn [at] tiny-frameworks.io*

---

