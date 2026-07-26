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

// isProcessAlive prüft unter Unix/Linux, ob eine PID noch existiert
func isProcessAlive(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// Signal 0 sendet kein echtes Signal, prüft aber Existenz & Rechte
	err = process.Signal(syscall.Signal(0))
	if err == nil {
		return true // Prozess existiert und antwortet
	}
	if err == syscall.EPERM {
		return true // Prozess existiert (gehört aber einem anderen User)
	}
	return false // Process dead (ESRCH)
}

func (w *Writer) writeLockInfo(f *os.File) error {
	// UnixMilli statt Unix (für präzise Test-Timeouts im Millisekunden-Bereich!)
	content := fmt.Sprintf("%d\n%d\n", os.Getpid(), time.Now().UnixMilli())
	_, err := f.WriteString(content)
	return err
}

func (w *Writer) isLockStale() bool {
	data, err := os.ReadFile(w.lockPath)
	if err != nil {
		return false // Wenn wir die Datei nicht lesen können, nicht einfach löschen!
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) < 2 {
		return true // Leer oder kaputt -> Stale
	}

	pid, errPid := strconv.Atoi(lines[0])
	tsMilli, errTs := strconv.ParseInt(lines[1], 10, 64)
	if errPid != nil || errTs != nil {
		return true // Ungültig -> Stale
	}

	// 1. Wenn die PID tot ist -> Stale!
	if !isProcessAlive(pid) {
		return true
	}

	// 2. Nur wenn expiry > 0 IST UND der Timestamp die Expiry überschreitet -> Stale!
	if w.expiry > 0 {
		lockTime := time.UnixMilli(tsMilli)
		if time.Since(lockTime) > w.expiry {
			return true
		}
	}

	// Prozess lebt noch und Expiry ist nicht abgelaufen -> LOCK IST GÜLTIG (nicht stale)!
	return false
}

func (w *Writer) tryLock() error {
	start := time.Now()
	for {
		// Try create lock file exclusively
		lock, err := os.OpenFile(w.lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
		if err == nil {
			// Lock geschafft! PID und Timestamp eintragen
			if errWrite := w.writeLockInfo(lock); errWrite != nil {
				lock.Close()
				_ = os.Remove(w.lockPath)
				return errors.Wrap(errors.LockError, "could not write lock info", "nexutils.lockwriter.tryLock()", errWrite)
			}
			lock.Close()
			return nil
		}

		if os.IsExist(err) {
			// Lock-Datei existiert bereits -> Prüfe auf verwaistes Lock (PID tot / Expiry abgelaufen)
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

// unlock löscht die .LOCK-Datei auf der Festplatte
func (w *Writer) unlock() error {
	err := os.Remove(w.lockPath)
	// Fehler nur werfen, wenn er NICHT "IsNotExist" ist
	if err != nil && !os.IsNotExist(err) {
		return errors.Wrap(
			errors.LockError,
			"could not remove lock file",
			"nexutils.lockingwriter.unlock()",
			err)
	}
	return nil
}

// Write implementiert io.Writer.
func (w *Writer) Write(p []byte) (n int, err error) {
	// 1. Internes Thread-Lock (für Goroutines)
	w.mu.Lock()
	defer w.mu.Unlock()

	// 2. Externes Prozess-Lock (über .LOCK-Datei)
	if err := w.tryLock(); err != nil {
		return 0, err
	}
	// Sicherstellen, dass das Lock auf jeden Fall aufgeräumt wird
	defer func() {
		if unlockErr := w.unlock(); unlockErr != nil && err == nil {
			err = unlockErr // Falls beim Schreiben kein Fehler auftrat, aber beim Unlock
		}
	}()

	// 3. Datei öffnen, schreiben, schließen
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
