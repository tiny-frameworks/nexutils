
<sup>🌍 **Language:** 🇩🇪 [German →](README.de.md)</sup>

---
|[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](./LICENSE)| |
|----|----|
|![GSF-Suite-Logo](nexutils.png)| ***GSF-nexutils***<br>Modular, reusable Go utilities for applications, services, processing engines, and distributed systems |
<sup>***GSF*** stands for ***Go Small Frameworks*** — minimalistic tools for robust applications.</sup>

---

## Overview

GSF-nexutils is a collection of small, focused, reusable Go components for applications, services, processing engines, and distributed systems.
The project provides common building blocks for error handling, synchronization, in-memory caching, structured logging, and related infrastructure.


---

## Module Overview

| Module | Subpackage Path | Description |
| --- | --- | --- |
| **`cache`** | `nexutils/cache` | Thread-safe LRU cache with TTL, background cleanup, and JSON persistence. |
| **`errors`** | `nexutils/errors` | Domain-driven error handling with codes, call paths, and OS exit code mapping. |
| **`lockingwriter`** | `nexutils/lockingwriter` | Process-safe `io.Writer` using `.LOCK` files with PID & timestamp stale detection. |
| **`logging`** | `nexutils/logging` | `slog`-based structured logger with native `nexutils/errors` support. |
| **`p2p`** | `nexutils/p2p` | `nexutils/p2p` is a lightweight, high-performance JSON-RPC 2.0 over WebSocket package for Go.|

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
	"codeberg.org/tiny-frameworks/nexutils/p2p/rpc"
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
├── p2p/              # lightweight, high-performance JSON-RPC 2.0 over WebSocket Peer package
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

