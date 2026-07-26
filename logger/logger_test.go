// Copyright 2026 Georg Hagn (tiny-frameworks)
// SPDX-License-Identifier: Apache-2.0

package nexlogger

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type logEntry struct {
	Level     string `json:"level"`
	Message   string `json:"msg"`
	Component string `json:"component"`
}

// TestSetupLogging_StandardFile tests the setup without LockingFileWriter
func TestSetupLogging_StandardFile(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "standard.log")

	cfg := &LoggerConfig{
		Filename:   logFile,
		UseLocking: false,
		Level:      slog.LevelInfo,
	}

	err := SetupLogging(cfg)
	if err != nil {
		t.Fatalf("SetupLogging failed: %v", err)
	}

	// Test-Log absetzen
	Logger.Info("Standard logger test", "component", "nexgate")

	// Dateiinhalt prüfen
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Could not read log file" in the log: %v", err)
	}

	var entry logEntry
	if err := json.Unmarshal(content, &entry); err != nil {
		t.Fatalf("Log-Entry is no valid JSON: %v", err)
	}

	if entry.Message != "Standard logger test" || entry.Component != "nexgate" {
		t.Errorf("Unexpected content in the log: %+v", entry)
	}
}

// TestSetupLogging_LockingFile tests the setup with LockingFileWriter
func TestSetupLogging_LockingFile(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "locking.log")

	cfg := &LoggerConfig{
		Filename:   logFile,
		Timeout:    1 * time.Second,
		Expiry:     2 * time.Second,
		UseLocking: true,
		Level:      slog.LevelDebug,
	}

	err := SetupLogging(cfg)
	if err != nil {
		t.Fatalf("SetupLogging with LockingFileWriter failed: %v", err)
	}

	// Submit test logs
	Logger.Debug("Locking logger debug test")
	Logger.Error("Locking logger error test", "code", 500)

	// Check if the file exists and is not empty.
	info, err := os.Stat(logFile)
	if err != nil {
		t.Fatalf("Log file was not created.: %v", err)
	}

	if info.Size() == 0 {
		t.Error("Logfile is empty!")
	}

	// Ensure that no orphaned .LOCK file remains.
	if _, err := os.Stat(logFile + ".LOCK"); !os.IsNotExist(err) {
		t.Error("Lock file was not cleaned up after the write operation.!")
	}
}

// TestLogLevelFiltering checks whether the configured log level is adhered to
func TestLogLevelFiltering(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "level.log")

	cfg := &LoggerConfig{
		Filename:   logFile,
		UseLocking: false,
		Level:      slog.LevelWarn, // Debug and Info should be ignored.
	}

	if err := SetupLogging(cfg); err != nil {
		t.Fatalf("SetupLogging failed: %v", err)
	}

	Logger.Info("Should NOT be logged")
	Logger.Warn("Should be logged")

	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Could not read log file: %v", err)
	}

	if len(content) == 0 {
		t.Fatal("Expected: At least one Log-Entry (Warn)")
	}

	var entry logEntry
	if err := json.Unmarshal(content, &entry); err != nil {
		t.Fatalf("JSON not valid: %v", err)
	}

	if entry.Message != "Sollte geloggt werden" {
		t.Errorf("wrong message logged: %s", entry.Message)
	}
}
