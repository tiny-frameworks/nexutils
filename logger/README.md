[![Codeberg Release](https://img.shields.io/codeberg/v/release/tiny-frameworks/nexutils?logo=codeberg&logoColor=white&color=2196F3)](https://codeberg.org/tiny-frameworks/nexutils)
[![Go Version](https://img.shields.io/badge/Go-1.26%2B-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://www.apache.org/licenses/LICENSE-2.0)

## nexutils/logging
<sup>the *logging module*, part of **GSF-nexutils**, member of the **tiny-frameworks** family</sup>

---

The `logging` module provides a lightweight, structured logging interface designed for high-performance Go applications.
Built on top of Go's standard `log/slog`, it offers pre-configured handlers (JSON and Text), context-aware logging,
thread-safe dynamic log-level adjustment at runtime, and direct integration with structured error domain types.

---

## Features

* **Zero External Dependencies:** Built natively on Go's standard `log/slog` library.
* **Dynamic Runtime Level Switching:** Change console and file log levels independently on-the-fly without restarting the application or re-initializing handlers.
* **Dual Output Formats:** Easy switching between human-readable console text and machine-readable JSON for production log aggregators.
* **Domain Error Aware:** Native support for formatting and logging structured `nexutils/errors` with automatic extraction of error codes, component paths, and causes.
* **Context Support:** Pass contextual key-value pairs (`slog.Attr`) through application boundaries.

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
	// Setup logging with initial config
	cfg := &logging.LoggerConfig{
		Filename: "app.log",
		Level:    logging.LevelInfo,
	}
	
	if err := logging.SetupLogging(cfg); err != nil {
		panic(err)
	}

	logging.Logger.Info("Application starting", "version", "1.0.0")

	// Dynamically lower console level to DEBUG at runtime (thread-safe)
	logging.SetConsoleLevel(logging.LevelDebug)
	logging.Logger.Debug("Trace information enabled on-the-fly")

	// Log structured errors seamlessly
	err := errors.New(errors.InvalidFormat, "invalid email payload", "auth.handler")
	logging.Logger.Error("Validation failed", "err", err)
}

```

---

## Core Concepts

### Dynamic Log-Level Switching

The module utilizes Go's atomic `slog.LevelVar` to allow thread-safe log level modifications at runtime.
Console and file outputs can be adjusted independently without losing log messages or locking handlers:

```go
// Adjust levels independently from an HTTP admin endpoint, signal handler, or config watcher
logging.SetConsoleLevel(logging.LevelWarn) // Silences console to WARN and ERROR
logging.SetFileLevel(logging.LevelDebug)   // Keeps file logging verbose for troubleshooting

```

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
| `SetupLogging(cfg)` | Initializes global logger and sets up console and optional file handlers. |
| `SetConsoleLevel(level)` / `SetConsoleLevelFromString(string)` | Dynamically changes the active console log level at runtime. |
| `GetConsoleLevel()` | Returns the currently active console log level. |
| `SetFileLevel(level)` / `SetFileLevelFromString(string)` | Dynamically changes the active file log level at runtime. |
| `GetFileLevel()` | Returns the currently active file log level. |
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

