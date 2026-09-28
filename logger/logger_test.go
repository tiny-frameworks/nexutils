// Copyright 2026 Georg Hagn (tiny-frameworks)
// SPDX-License-Identifier: Apache-2.0

package logger_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"codeberg.org/tiny-frameworks/nexutils/logger"
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

	cfg := &logger.LoggerConfig{
		Filename:   logFile,
		UseLocking: false,
		Level:      slog.LevelInfo,
	}

	err := logger.SetupLogging(cfg)
	if err != nil {
		t.Fatalf("SetupLogging failed: %v", err)
	}

	// Test-Log absetzen
	logger.Logger.Info("Standard logger test", "component", "nexgate")

	// Dateiinhalt prüfen
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Could not read log file in the log: %v", err)
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

	cfg := &logger.LoggerConfig{
		Filename:   logFile,
		Timeout:    1 * time.Second,
		Expiry:     2 * time.Second,
		UseLocking: true,
		Level:      slog.LevelDebug,
	}

	err := logger.SetupLogging(cfg)
	if err != nil {
		t.Fatalf("SetupLogging with LockingFileWriter failed: %v", err)
	}

	// Submit test logs
	logger.Logger.Debug("Locking logger debug test")
	logger.Logger.Error("Locking logger error test", "code", 500)

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

	cfg := &logger.LoggerConfig{
		Filename:   logFile,
		UseLocking: false,
		Level:      slog.LevelWarn, // Debug and Info should be ignored.
	}

	if err := logger.SetupLogging(cfg); err != nil {
		t.Fatalf("SetupLogging failed: %v", err)
	}

	logger.Logger.Info("Should NOT be logged")
	logger.Logger.Warn("Should be logged")

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

	if entry.Message != "Should be logged" {
		t.Errorf("wrong message logged: %s", entry.Message)
	}
}

func TestDynamicConsoleLevelSwitching(t *testing.T) {
	// Puffer zum Abfangen der Konsolenausgabe
	var buf bytes.Buffer

	// Handler mit dem dynamischen consoleLevel aufsetzen
	logger.ConsoleLevel.Set(logger.LevelInfo)
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: logger.ConsoleLevel})
	testLogger := slog.New(handler)

	// 1. Phase: Level steht auf INFO
	testLogger.Debug("debug_message_1")
	testLogger.Info("info_message_1")

	output := buf.String()
	if strings.Contains(output, "debug_message_1") {
		t.Errorf("Expected DEBUG message to be ignored, but was logged: %s", output)
	}
	if !strings.Contains(output, "info_message_1") {
		t.Errorf("Expected INFO message to be logged, but was not found: %s", output)
	}

	// Puffer zurücksetzen
	buf.Reset()

	// 2. Phase: Level dynamisch zur Laufzeit auf DEBUG absenken
	logger.SetConsoleLevel(logger.LevelDebug)

	testLogger.Debug("debug_message_2")
	testLogger.Info("info_message_2")

	output = buf.String()
	if !strings.Contains(output, "debug_message_2") {
		t.Errorf("Expected DEBUG message to be logged after level switch, but was missing: %s", output)
	}
	if !strings.Contains(output, "info_message_2") {
		t.Errorf("Expected INFO message to be logged, but was missing: %s", output)
	}

	// 3. Phase: Level dynamisch auf WARN anheben
	buf.Reset()
	logger.SetConsoleLevel(logger.LevelWarn)

	testLogger.Info("info_message_3")
	testLogger.Warn("warn_message_3")

	output = buf.String()
	if strings.Contains(output, "info_message_3") {
		t.Errorf("Expected INFO message to be ignored when level is WARN: %s", output)
	}
	if !strings.Contains(output, "warn_message_3") {
		t.Errorf("Expected WARN message to be logged: %s", output)
	}
}

func TestIndependentConsoleAndFileLevels(t *testing.T) {
	var consoleBuf bytes.Buffer
	var fileBuf bytes.Buffer

	// Konsole auf WARN, File auf DEBUG
	logger.SetConsoleLevel(logger.LevelWarn)
	logger.SetFileLevel(logger.LevelDebug)

	consoleHandler := slog.NewTextHandler(&consoleBuf, &slog.HandlerOptions{Level: logger.ConsoleLevel})
	fileHandler := slog.NewJSONHandler(&fileBuf, &slog.HandlerOptions{Level: logger.FileLevel})

	multiLogger := slog.New(slog.NewMultiHandler(consoleHandler, fileHandler))

	// Debug-Log absetzen
	multiLogger.Debug("multi_debug_test")

	// Konsole sollte die Nachricht ignorieren
	if strings.Contains(consoleBuf.String(), "multi_debug_test") {
		t.Errorf("Console logged DEBUG message despite WARN level: %s", consoleBuf.String())
	}

	// File-Handler sollte die Nachricht verarbeiten
	if !strings.Contains(fileBuf.String(), "multi_debug_test") {
		t.Errorf("File handler missed DEBUG message despite DEBUG level: %s", fileBuf.String())
	}
}

func TestParseLevelFromString(t *testing.T) {

	if level, _ := logger.ParseLevel("warn"); level != logger.LevelWarn {
		t.Errorf("Expected: 'WARN' Result: %s", level)
	}
	if level, _ := logger.ParseLevel("Warn"); level != logger.LevelWarn {
		t.Errorf("Expected: 'WARN' Result: %s", level)
	}
	if level, _ := logger.ParseLevel("Warn"); level != logger.LevelWarn {
		t.Errorf("Expected: 'WARN' Result: %s", level)
	}
	if level, _ := logger.ParseLevel("WaRn"); level != logger.LevelWarn {
		t.Errorf("Expected: 'WARN' Result: %s", level)
	}
}
