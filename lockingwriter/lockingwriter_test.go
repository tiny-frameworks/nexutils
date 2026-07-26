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

// TestSimpleWrite prüft das normale Schreiben in eine Datei
func TestSimpleWrite(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "test.log")

	writer := New(logFile, 1*time.Second, 2*time.Second)

	message := []byte("hello world\n")
	n, err := writer.Write(message)
	if err != nil {
		t.Fatalf("Write fehlgeschlagen: %v", err)
	}

	if n != len(message) {
		t.Errorf("Erwartet %d geschriebene Bytes, got %d", len(message), n)
	}

	// Inhalt verifizieren
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Konnte Logdatei nicht lesen: %v", err)
	}

	if !bytes.Equal(content, message) {
		t.Errorf("Dateiinhalt falsch. Got %q, want %q", content, message)
	}

	// Sicherstellen, dass .LOCK gelöscht wurde
	if _, err := os.Stat(logFile + ".LOCK"); !os.IsNotExist(err) {
		t.Error("Lock-Datei wurde nach Write() nicht aufgeräumt!")
	}
}

// TestConcurrentGoroutines prüft die Thread-Sicherheit bei parallelen Goroutines
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
					t.Errorf("Paralleler Write fehlgeschlagen: %v", err)
				}
			}
		}()
	}

	wg.Wait()

	// Prüfen, ob alle Zeilen korrekt und ohne Datenverlust geschrieben wurden
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Konnte Logdatei nicht lesen: %v", err)
	}

	expectedLines := goroutines * writesPerGoroutine
	lines := bytes.Count(content, []byte("\n"))

	if lines != expectedLines {
		t.Errorf("Anzahl Zeilen unvollständig: got %d, want %d", lines, expectedLines)
	}
}

// TestTimeout prüft, ob bei einer blockierten Lock-Datei ein Timeout ausgelöst wird
func TestTimeout(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "timeout.log")
	lockFile := logFile + ".LOCK"

	// Lock-Datei manuell erstellen (blockieren)
	// So erzeugst du im Test ein WIRKLICH gültiges Lock, das nicht als stale gilt:
	lockContent := fmt.Sprintf("%d\n%d\n", os.Getpid(), time.Now().UnixMilli())
	err := os.WriteFile(lockFile, []byte(lockContent), 0644)
	if err != nil {
		t.Fatalf("Konnte Dummy-Lock nicht anlegen: %v", err)
	}

	// Writer mit sehr kurzem Timeout (50ms) und langer Expiry
	writer := New(logFile, 50*time.Millisecond, 10*time.Second)

	_, err = writer.Write([]byte("test"))
	if err == nil {
		t.Fatal("Erwartet: Fehler wegen Timeout. Got: nil")
	}
}

// TestExpiredLockOverwriting prüft, ob veraltete Lock-Dateien ignoriert/überwritten werden
func TestExpiredLockOverwriting(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "expired.log")
	lockFile := logFile + ".LOCK"

	// Dummy-Lock anlegen
	err := os.WriteFile(lockFile, []byte("old lock"), 0644)
	if err != nil {
		t.Fatalf("Konnte Dummy-Lock nicht anlegen: %v", err)
	}

	// Erstelldatum der Lock-Datei künstlich in die Vergangenheit verlegen (5 Minuten alt)
	oldTime := time.Now().Add(-5 * time.Minute)
	err = os.Chtimes(lockFile, oldTime, oldTime)
	if err != nil {
		t.Fatalf("Konnte Zeitstempel der Lock-Datei nicht ändern: %v", err)
	}

	// Writer mit Expiry von 1 Sekunde initialisieren
	writer := New(logFile, 2*time.Second, 1*time.Second)

	// Sollte trotz existierender Lock-Datei erfolgreich schreiben, da die Datei abgelaufen ist
	_, err = writer.Write([]byte("success after expired lock\n"))
	if err != nil {
		t.Fatalf("Schreiben fehlgeschlagen, obwohl Lock abgelaufen war: %v", err)
	}

	// Verifizieren
	content, err := os.ReadFile(logFile)
	if err != nil || !bytes.Contains(content, []byte("success after expired lock")) {
		t.Errorf("Inhalt wurde nicht korrekt geschrieben: %v", err)
	}
}
