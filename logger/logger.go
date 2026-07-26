// Copyright 2026 Georg Hagn (tiny-frameworks)
// SPDX-License-Identifier: Apache-2.0

package nexlogger

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"codeberg.org/tiny-frameworks/nexutils/lockwriter"
	"codeberg.org/tiny-frameworks/nexutils/nexerrors"
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

	fileHandler, err := newLogHandler(cfg)
	if err != nil {
		return err
	}
	consoleHandler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})

	// 3. Combine and put in api
	multiHandler := slog.NewMultiHandler(consoleHandler, fileHandler)
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
		return nil, nexerrors.Wrap(
			nexerrors.WriteError,
			fmt.Sprintf("%s is no valid filepath", cfg.Filename),
			"nexutils.logger.SetupLogging",
			err,
		)
	}

	f, err := os.OpenFile(logfilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, nexerrors.Wrap(
			nexerrors.WriteError,
			fmt.Sprintf("could not open %s", logfilePath),
			"nexutils.logger.SetupLogging",
			err,
		)
	}
	return slog.NewJSONHandler(f, &slog.HandlerOptions{Level: cfg.Level}), nil
}

func newLockingFileHandler(cfg *LoggerConfig) (slog.Handler, error) {

	lockingWriter := lockwriter.NewLockingFileWriter(cfg.Filename, cfg.Timeout, cfg.Expiry)
	return slog.NewJSONHandler(lockingWriter, &slog.HandlerOptions{Level: cfg.Level}), nil
}
