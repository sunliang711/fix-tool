package logging

import (
	"fmt"
	"io"
	"os"
	"time"

	"fix-tool/internal/config"

	"github.com/rs/zerolog"
)

func New(output io.Writer, cfg config.LogConfig) (zerolog.Logger, error) {
	if output == nil {
		output = os.Stderr
	}
	level, err := zerolog.ParseLevel(cfg.Level)
	if err != nil {
		return zerolog.Logger{}, fmt.Errorf("parse log level: %w", err)
	}
	writer := output
	if cfg.Format == "console" {
		writer = zerolog.ConsoleWriter{
			Out:        output,
			TimeFormat: time.RFC3339,
		}
	}
	return zerolog.New(writer).Level(level).With().Timestamp().Logger(), nil
}

func NewDefault(output io.Writer) zerolog.Logger {
	logger, err := New(output, config.LogConfig{
		Level:  "info",
		Format: "json",
	})
	if err == nil {
		return logger
	}
	if output == nil {
		output = os.Stderr
	}
	return zerolog.New(output).Level(zerolog.InfoLevel).With().Timestamp().Logger()
}
