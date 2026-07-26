// Copyright 2025 Georg Hagn
// SPDX-License-Identifier: Apache-2.0

/*
Package lock provides a portable, atomic file-based locking mechanism
to synchronize access to shared resources between different OS processes.

The primary use case is to coordinate a file rotator (running in a scheduler process)
and a writer (running in a logging process), ensuring that no rotation occurs
while a write operation is in progress.

Atomicity:

The locking mechanism relies on the os.O_EXCL flag during file creation.
This is an atomic operation at the operating system level: the OS guarantees
that if two processes attempt to create the same lock file simultaneously,
only one will succeed, while the other receives an 'already exists' error.

Stale Lock Management:

A common issue with file-based locks is the "stale lock" — a situation where
a process acquires a lock and then crashes before it can release it,
leaving the lock file on disk and blocking other processes indefinitely.

To mitigate this, this package implements a Time-To-Live (TTL) approach
via the 'Expiry' field:
  - When a process encounters an existing lock file, it checks the file's
    modification time (mtime).
  - If the duration since the last modification exceeds the defined Expiry
    period, the lock is considered "stale".
  - The package will then automatically remove the stale lock file and
    attempt to acquire a new one.

PID Tracking:

For debugging and administrative purposes, the PID (Process ID) of the
owner is written into the lock file. This allows system administrators
to identify which process is currently holding a lock or to verify if
a stale lock belongs to a process that is no longer running.

Usage Example:

	l := &lock.FileLock{
	    Path:    "app.log.LOCK",
	    Timeout: 2 * time.Second,
	    Expiry:  5 * time.Minute,
	}

	if err := l.Lock(); err == nil {
	    defer l.Unlock()
	    // Perform protected file operation
	}

Implementation Note:

While highly portable across Windows, Linux, and iOS, file-based locking
is slower than in-memory synchronization (sync.Mutex). It should be used
specifically for inter-process coordination, while internal thread safety
should still be managed via Mutexes within the application.
*/
package lockingwriter
