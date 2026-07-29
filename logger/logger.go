// Copyright 2026 Georg Hagn (tiny-frameworks)
// SPDX-License-Identifier: Apache-2.0

package logger

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"codeberg.org/tiny-frameworks/nexutils/errors"
	"codeberg.org/tiny-frameworks/nexutils/lockingwriter"
)

type LoggerConfig struct {
	Filename   string
	Timeout    time.Duration
	Expiry     time.Duration
	UseLocking bool
	Level      slog.Level
}

// Global anchor for all nexgate components
var Logger *slog.Logger = slog.Default()

func SetupLogging(cfg *LoggerConfig) error {

	if cfg == nil {
		return nil
	}

	// Default is consolehandler
	consoleHandler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})
	multiHandler := slog.NewMultiHandler(consoleHandler)

	// fileHandler only if cfg.fileName is not nil
	if cfg.Filename != "" {
		fileHandler, err := newLogHandler(cfg)
		multiHandler = slog.NewMultiHandler(consoleHandler, fileHandler)
		if err != nil {
			return err
		}
	}

	// Combine and put in api
	Logger = slog.New(multiHandler)

	// Set globally so that third-party libraries can also use slog.
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

	// 1. File-Handler (JSON for better evaluation)
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
	return slog.NewJSONHandler(f, &slog.HandlerOptions{Level: cfg.Level}), nil
}

func newLockingFileHandler(cfg *LoggerConfig) (slog.Handler, error) {

	lWriter := lockingwriter.New(cfg.Filename, cfg.Timeout, cfg.Expiry)
	return slog.NewJSONHandler(lWriter, &slog.HandlerOptions{Level: cfg.Level}), nil
}
