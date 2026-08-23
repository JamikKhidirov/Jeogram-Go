package logger

import (
	"io"
	"os"
	"time"

	"github.com/rs/zerolog"
)

// New создаёт настроенный логгер zerolog. В development он использует
// человекочитаемый консольный вывод, в production — структурированный JSON.
func New(env, version string) zerolog.Logger {
	level := zerolog.InfoLevel
	if env == "development" {
		level = zerolog.DebugLevel
	}

	var writer io.Writer
	if env == "development" {
		writer = zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: time.RFC3339,
		}
	} else {
		writer = os.Stdout
	}

	logger := zerolog.New(writer).
		Level(level).
		With().
		Timestamp().
		Str("service", "jeogram").
		Str("version", version).
		Logger()

	return logger
}
