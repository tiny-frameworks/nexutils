// Copyright 2026 Georg Hagn
// SPDX-License-Identifier: Apache-2.0

package lockingwriter

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// TestSimpleWrite tests standard writing to a file.
func TestSimpleWrite(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "test.log")

	writer := New(logFile, 1*time.Second, 2*time.Second)

	message := []byte("hello world\n")
	n, err := writer.Write(message)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	if n != len(message) {
		t.Errorf("Expected %d bytes written, got %d", len(message), n)
	}

	// Verify content
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Could not read log file: %v", err)
	}

	if !bytes.Equal(content, message) {
		t.Errorf("Incorrect file content. Got %q, want %q", content, message)
	}

	// Ensure that .LOCK has been deleted.
	if _, err := os.Stat(logFile + ".LOCK"); !os.IsNotExist(err) {
		t.Error("Lock file was not cleaned up after Write()!")
	}
}

// TestConcurrentGoroutines checks for thread safety with parallel goroutines
func TestConcurrentGoroutines(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "concurrent.log")

	writer := New(logFile, 5*time.Second, 10*time.Second)

	goroutines := 20
	writesPerGoroutine := 10
	var wg sync.WaitGroup

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < writesPerGoroutine; j++ {
				_, err := writer.Write([]byte("log entry\n"))
				if err != nil {
					t.Errorf("Paralleler Write failed: %v", err)
				}
			}
		}()
	}

	wg.Wait()

	// Check whether all lines were written correctly and without data loss.
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Could not read log file: %v", err)
	}

	expectedLines := goroutines * writesPerGoroutine
	lines := bytes.Count(content, []byte("\n"))

	if lines != expectedLines {
		t.Errorf("Number of lines incomplete: got %d, want %d", lines, expectedLines)
	}
}

// TestTimeout checks whether a timeout is triggered when a lock file is blocked
func TestTimeout(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "timeout.log")
	lockFile := logFile + ".LOCK"

	// Manually create a lock file (block)
	// Here is how to generate a TRULY valid lock during testing—one that isn't considered stale:
	lockContent := fmt.Sprintf("%d\n%d\n", os.Getpid(), time.Now().UnixMilli())
	err := os.WriteFile(lockFile, []byte(lockContent), 0644)
	if err != nil {
		t.Fatalf("Could not create dummy lock: %v", err)
	}

	// Writer with a very short timeout (50ms) and a long expiry.
	writer := New(logFile, 50*time.Millisecond, 10*time.Second)

	_, err = writer.Write([]byte("test"))
	if err == nil {
		t.Fatal("Expected: Timeout error. Got: nil")
	}
}

// TestExpiredLockOverwriting checks whether outdated lock files are ignored/overwritten
func TestExpiredLockOverwriting(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "expired.log")
	lockFile := logFile + ".LOCK"

	// create Dummy-Lock
	err := os.WriteFile(lockFile, []byte("old lock"), 0644)
	if err != nil {
		t.Fatalf("Could not create dummy lock: %v", err)
	}

	// Artificially set the lock file's creation date to the past (5 minutes old)
	oldTime := time.Now().Add(-5 * time.Minute)
	err = os.Chtimes(lockFile, oldTime, oldTime)
	if err != nil {
		t.Fatalf("Could not change the lock file's timestamp.: %v", err)
	}

	// Initialize writer with an expiry of 1 second.
	writer := New(logFile, 2*time.Second, 1*time.Second)

	// Should write successfully despite the existing lock file, as the file has expired.
	_, err = writer.Write([]byte("success after expired lock\n"))
	if err != nil {
		t.Fatalf("Write failed even though the lock had expired.: %v", err)
	}

	// Verifizieren
	content, err := os.ReadFile(logFile)
	if err != nil || !bytes.Contains(content, []byte("success after expired lock")) {
		t.Errorf("The content was not written correctly.: %v", err)
	}
}
