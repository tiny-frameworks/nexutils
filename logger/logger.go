// Copyright 2026 Georg Hagn (tiny-frameworks)
// SPDX-License-Identifier: Apache-2.0

package logger

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"codeberg.org/tiny-frameworks/nexutils/errors"
	"codeberg.org/tiny-frameworks/nexutils/lockingwriter"
)

type Level = slog.Level

const (
	LevelDebug Level = slog.LevelDebug
	LevelInfo  Level = slog.LevelInfo
	LevelWarn  Level = slog.LevelWarn
	LevelError Level = slog.LevelError
)

type LoggerConfig struct {
	Filename   string        `json:"file_name"`
	Timeout    time.Duration `json:"timeout"`
	Expiry     time.Duration `json:"expiry"`
	UseLocking bool          `json:"use_locking"`
	Level      Level         `json:"level"`
}

var (
	Logger       *slog.Logger = slog.Default()
	ConsoleLevel              = new(slog.LevelVar)
	FileLevel                 = new(slog.LevelVar)
)

// Dynamische Level-Steuerung zur Laufzeit
func SetConsoleLevel(l Level) { ConsoleLevel.Set(l) }
func GetConsoleLevel() Level  { return ConsoleLevel.Level() }

func SetFileLevel(l Level) { FileLevel.Set(l) }
func GetFileLevel() Level  { return FileLevel.Level() }

func SetupLogging(cfg *LoggerConfig) error {
	if cfg == nil {
		return nil
	}

	// Beide Levels zentral aus Config oder Default setzen
	ConsoleLevel.Set(cfg.Level)
	FileLevel.Set(cfg.Level)

	handlers := make([]slog.Handler, 0, 2)

	consoleHandler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: ConsoleLevel})
	handlers = append(handlers, consoleHandler)

	if cfg.Filename != "" {
		fileHandler, err := newLogHandler(cfg)
		if err != nil {
			return err
		}
		handlers = append(handlers, fileHandler)
	}

	Logger = slog.New(slog.NewMultiHandler(handlers...))
	slog.SetDefault(Logger)

	return nil
}

func newLogHandler(cfg *LoggerConfig) (slog.Handler, error) {
	if cfg.UseLocking {
		return newLockingFileHandler(cfg)
	}
	return newStandardFileHandler(cfg)
}

func newStandardFileHandler(cfg *LoggerConfig) (slog.Handler, error) {
	logfilePath, err := filepath.Abs(cfg.Filename)
	if err != nil {
		return nil, errors.Wrap(
			errors.WriteError,
			fmt.Sprintf("%s is no valid filepath", cfg.Filename),
			"nexutils.logger.SetupLogging",
			err,
		)
	}

	f, err := os.OpenFile(logfilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, errors.Wrap(
			errors.WriteError,
			fmt.Sprintf("could not open %s", logfilePath),
			"nexutils.logger.SetupLogging",
			err,
		)
	}

	return slog.NewJSONHandler(f, &slog.HandlerOptions{Level: FileLevel}), nil
}

func newLockingFileHandler(cfg *LoggerConfig) (slog.Handler, error) {
	lWriter := lockingwriter.New(cfg.Filename, cfg.Timeout, cfg.Expiry)
	return slog.NewJSONHandler(lWriter, &slog.HandlerOptions{Level: FileLevel}), nil
}

// ParseLevel konvertiert einen String (z.B. "DEBUG", "info", "WARN") in ein slog.Level.
func ParseLevel(s string) (Level, error) {
	var l Level
	err := l.UnmarshalText([]byte(strings.TrimSpace(s)))
	return l, err
}

// SetConsoleFromString setzt das Konsolen-Loglevel per String.
func SetConsoleLevelFromString(s string) error {
	l, err := ParseLevel(s)
	if err != nil {
		return err
	}
	SetConsoleLevel(l)
	return nil
}

// SetFileLevelFromString setzt das Konsolen-Loglevel per String.
func SetFileLevelFromString(s string) error {
	l, err := ParseLevel(s)
	if err != nil {
		return err
	}
	SetFileLevel(l)
	return nil
}
