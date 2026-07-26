## nexutils/lockwriter
<sup>the *lockwriter module*, part of **GSF-nexutils**, member of the **tiny-frameworks** family</sup>

---

The `lockwriter` module provides a thread-safe and process-safe implementation of Go's `io.Writer`. It combines **in-memory mutexes** for goroutine synchronization with **file-based `.LOCK` markers** (containing process ID and timestamp metadata) to safely coordinate file writes across multiple concurrent OS processes.

---

## Features

* **Dual-Layer Locking:** Combines `sync.Mutex` (thread-safety within the process) with OS-level `.LOCK` files (process-safety across the system).
* **Self-Healing & Stale Lock Detection:** Lock files store the owner's **PID** and a **millisecond timestamp**. Verwaiste Locks von abgestürzten Prozessen oder abgelaufene Locks (Expiry) werden automatisch erkannt und aufgeräumt.
* **Standard `io.Writer` Interface:** Implements `Write([]byte) (int, error)`, making it a drop-in replacement for any standard Go writer.
* **Configurable Timeouts & Expiry:** Define maximum wait durations for acquiring a lock and maximum lifetimes before a lock is considered stale.
* **Structured Error Integration:** Fully integrated with `nexutils/errors` for precise error wrapping (`LockError`, `WriteError`).

---

## Installation

```bash
go get codeberg.org/tiny-frameworks/nexutils/lockwriter

```

---

## Quick Start

```go
package main

import (
	"fmt"
	"time"

	"codeberg.org/tiny-frameworks/nexutils/lockwriter"
)

func main() {
	// Create a writer: timeout after 2 seconds, consider locks stale after 10 seconds
	writer := lockwriter.New("app.log", 2*time.Second, 10*time.Second)

	// Safe concurrent write (acquires mutex + creates app.log.LOCK)
	data := []byte("2026-07-26 [INFO] Event processed successfully\n")
	n, err := writer.Write(data)
	if err != nil {
		fmt.Printf("Failed to write to file: %v\n", err)
		return
	}

	fmt.Printf("Successfully wrote %d bytes\n", n)
}

```

---

## How It Works

```
  +-------------------------------------------------------+
  |              lockwriter.Writer.Write()                |
  +-------------------------------------------------------+
                             |
                   1. In-Memory Lock (sync.Mutex)
                             |
                   2. File Lock (os.O_EXCL)
                             v
           +----------------------------------+
           | Does app.log.LOCK exist on disk? |
           +----------------------------------+
              /                            \
      [ NO ] /                              \ [ YES ]
            v                                v
  Create .LOCK file              Inspect .LOCK Metadata
  Write: PID + Timestamp         +---------------------------------+
  Grant access!                  | 1. Is owner PID still alive?   |
                                 | 2. Has expiry time passed?      |
                                 +---------------------------------+
                                    /                            \
                            [ STALE ] /                            \ [ VALID ]
                                     v                              v
                           Remove stale lock & retry       Wait (time.Sleep)
                                                           until timeout

```

---

## API Reference

### `New(filename string, timeout, expiry time.Duration) *Writer`

Creates a new `Writer` instance targeting `filename`. The corresponding lock file will automatically be set to `filename + ".LOCK"`.

* **`filename`:** Path to the target output file.
* **`timeout`:** Maximum time `Write()` will wait to acquire the file lock before returning a `LockError`.
* **`expiry`:** Lifetime after which a lock file is considered stale/orphaned, even if the file exists (set to `0` to disable time-based expiry).

### `Write(p []byte) (n int, err error)`

Implements `io.Writer`. Acquires both the internal mutex and external process lock, appends bytes to the file (`os.O_APPEND`), closes the file, and removes the `.LOCK` file.

### `Close() error`

Implements `io.Closer` (returns `nil`, as files are opened and closed per write operation).

---

## Testing

Run unit tests covering timeout handling, stale lock cleanup, and concurrent write access:

```bash
go test -v ./...

```

---

## Organizational & Standards

* **Copyright:** © 2026 Georg Hagn.
* **Namespace:** `codeberg.org/tiny-frameworks/nexutils/lockwriter`
* **License:** Apache License, Version 2.0.

*GSF-nexutils/lockwriter is an independent open-source project and is not affiliated with any corporation of a similar name.*

---

## Contact

If you have questions or feedback, feel free to reach out:

📧 *georghagn [at] tiny-frameworks.io*

---

