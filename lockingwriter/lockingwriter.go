// Copyright 2026 Georg Hagn (tiny-frameworks)
// SPDX-License-Identifier: Apache-2.0

package lockingwriter

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"codeberg.org/tiny-frameworks/nexutils/errors"
)

type Writer struct {
	filePath string
	lockPath string
	timeout  time.Duration
	expiry   time.Duration
	sleep    time.Duration
	mu       sync.Mutex
}

// const sleepDuration = 5 * time.Millisecond
const sleepDuration = 5 * time.Millisecond

func New(filename string, timeout, expiry time.Duration) *Writer {
	return &Writer{
		filePath: filename,
		lockPath: filename + ".LOCK",
		timeout:  timeout,
		expiry:   expiry,
		sleep:    sleepDuration,
	}
}

// isProcessAlive checks under Unix/Linux whether a PID still exists
func isProcessAlive(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// Signal 0 does not send a real signal, but checks for existence and permissions
	err = process.Signal(syscall.Signal(0))
	if err == nil {
		return true // Process exists and is responding
	}
	if err == syscall.EPERM {
		return true // Process exists (but belongs to another user)
	}
	return false // Process dead (ESRCH)
}

func (w *Writer) writeLockInfo(f *os.File) error {
	// UnixMilli instead of Unix (for precise test timeouts in the millisecond range!)
	content := fmt.Sprintf("%d\n%d\n", os.Getpid(), time.Now().UnixMilli())
	_, err := f.WriteString(content)
	return err
}

func (w *Writer) isLockStale() bool {
	data, err := os.ReadFile(w.lockPath)
	if err != nil {
		return false // If we cannot read the file, do not simply delete it!
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) < 2 {
		return true // Empty or broken -> Stale
	}

	pid, errPid := strconv.Atoi(lines[0])
	tsMilli, errTs := strconv.ParseInt(lines[1], 10, 64)
	if errPid != nil || errTs != nil {
		return true // Invalid -> Stale
	}

	// 1. If the PID is dead -> Stale!
	if !isProcessAlive(pid) {
		return true
	}

	// 2. Only if expiry > 0 AND the timestamp exceeds the expiry -> Stale!
	if w.expiry > 0 {
		lockTime := time.UnixMilli(tsMilli)
		if time.Since(lockTime) > w.expiry {
			return true
		}
	}

	// Process is still running and expiration has not passed -> LOCK IS VALID (not stale)!
	return false
}

func (w *Writer) tryLock() error {
	start := time.Now()
	for {
		// Try create lock file exclusively
		lock, err := os.OpenFile(w.lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
		if err == nil {
			// Lock acquired! Enter PID and timestamp.
			if errWrite := w.writeLockInfo(lock); errWrite != nil {
				lock.Close()
				_ = os.Remove(w.lockPath)
				return errors.Wrap(errors.LockError, "could not write lock info", "nexutils.lockwriter.tryLock()", errWrite)
			}
			lock.Close()
			return nil
		}

		if os.IsExist(err) {
			// Lock file already exists -> Check for orphaned lock (PID dead / expired)
			if w.isLockStale() {
				_ = os.Remove(w.lockPath)
				continue
			}

			if time.Since(start) >= w.timeout {
				return errors.New(errors.LockError, "timeout waiting for lock", "nexutils.lockwriter.tryLock()")
			}
			time.Sleep(w.sleep)
		} else {
			return errors.Wrap(errors.LockError, "could not create lock file", "nexutils.lockwriter.tryLock()", err)
		}
	}
}

// unlock deletes the .LOCK file on the hard drive.
func (w *Writer) unlock() error {
	err := os.Remove(w.lockPath)
	// Only throw an error if it is NOT "IsNotExist".
	if err != nil && !os.IsNotExist(err) {
		return errors.Wrap(
			errors.LockError,
			"could not remove lock file",
			"nexutils.lockingwriter.unlock()",
			err)
	}
	return nil
}

// Write implements io.Writer.
func (w *Writer) Write(p []byte) (n int, err error) {
	// 1. Internal thread lock (for goroutines)
	w.mu.Lock()
	defer w.mu.Unlock()

	// 2. External process lock (via .LOCK file)
	if err := w.tryLock(); err != nil {
		return 0, err
	}
	// Ensure that the lock is cleaned up in any case.
	defer func() {
		if unlockErr := w.unlock(); unlockErr != nil && err == nil {
			err = unlockErr // If no error occurred during writing, but during unlocking
		}
	}()

	// 3. Open, write to, and close the file
	f, err := os.OpenFile(w.filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return 0, errors.Wrap(
			errors.WriteError,
			"could not open file",
			"nexutils.lockwriter.Write()", err)
	}
	defer f.Close()

	return f.Write(p)
}
