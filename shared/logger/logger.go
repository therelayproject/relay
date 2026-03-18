// Package logger provides structured, leveled logging for all Relay services.
// It wraps zerolog with opinionated defaults: JSON in production, pretty-print in dev.
package logger

import (
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Config controls logger behaviour.
type Config struct {
	Level   string // "debug", "info", "warn", "error"
	Pretty  bool   // human-readable output (dev mode)
	Service string // injected into every log line
}

// Init initialises the global zerolog logger. Call once in main().
func Init(cfg Config) zerolog.Logger {
	level, err := zerolog.ParseLevel(cfg.Level)
	if err != nil {
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)
	zerolog.TimeFieldFormat = time.RFC3339Nano

	var logger zerolog.Logger
	if cfg.Pretty {
		logger = zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339}).
			With().
			Timestamp().
			Str("service", cfg.Service).
			Logger()
	} else {
		logger = zerolog.New(os.Stderr).
			With().
			Timestamp().
			Str("service", cfg.Service).
			Logger()
	}

	log.Logger = logger
	return logger
}

// With returns a child logger with the given key-value pairs.
func With(l zerolog.Logger, fields map[string]string) zerolog.Logger {
	ctx := l.With()
	for k, v := range fields {
		ctx = ctx.Str(k, v)
	}
	return ctx.Logger()
}
