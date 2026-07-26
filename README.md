## GSF-nexutils

<sup>part of the **tiny-frameworks** ecosystem</sup>

---

`nexutils` is a high-performance, modular collection of essential Go utilities designed for robust microservices, CLI engines, and distributed peer-to-peer applications. Built with a strict **zero external dependency** philosophy, it provides standardized foundations for error handling, thread/process synchronization, in-memory caching, and structured logging.

---

## Module Overview

| Module | Subpackage Path | Description |
| --- | --- | --- |
| **`cache`** | `nexutils/cache` | Thread-safe LRU cache with TTL, background cleanup, and JSON persistence. |
| **`errors`** | `nexutils/errors` | Domain-driven error handling with codes, call paths, and OS exit code mapping. |
| **`lockwriter`** | `nexutils/lockwriter` | Process-safe `io.Writer` using `.LOCK` files with PID & timestamp stale detection. |
| **`logging`** | `nexutils/logging` | `slog`-based structured logger with native `nexutils/errors` support. |

---

## Design Principles

1. **Zero External Dependencies:** Relies purely on the Go standard library (`sync`, `time`, `os`, `log/slog`, `errors`).
2. **Modular Architecture:** Each subpackage can be imported independently without pulling unnecessary code into your binary.
3. **Fail-Fast & Self-Healing:** Built-in safeguards like stale-lock recovery, bounded LRU memory caps, and type-safe error chains.
4. **Developer Experience:** First-class support for Go `Example` tests and clear API signatures.

---

## Installation

Import only the modules you need in your Go code:

```bash
go get codeberg.org/tiny-frameworks/nexutils

```

```go
import (
	"codeberg.org/tiny-frameworks/nexutils/cache"
	"codeberg.org/tiny-frameworks/nexutils/errors"
	"codeberg.org/tiny-frameworks/nexutils/lockwriter"
	"codeberg.org/tiny-frameworks/nexutils/logging"
)

```

---

## Project Structure

```text
nexutils/
├── cache/            # LRU Cache with TTL & JSON storage
├── errors/           # Domain error codes & exit-code mapping
├── lockwriter/       # Thread- & process-safe file writer
├── logging/          # Structured JSON/Text logger
├── go.mod
├── LICENSE
└── README.md

```

---

## Running Tests

Run unit tests across all utility modules simultaneously:

```bash
go test -v ./...

```

---

## Organizational & Standards

* **Copyright:** © 2026 Georg Hagn.
* **Repository:** `codeberg.org/tiny-frameworks/nexutils`
* **License:** Apache License, Version 2.0.

*GSF-nexutils is an independent open-source project and is not affiliated with any corporation of a similar name.*

---

## Contact & Support

For inquiries, architectural discussions, or security issues, please contact:

📧 **georghagn [at] tiny-frameworks.io**

---

