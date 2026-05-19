package logging

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
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
		writer = newConsoleWriter(output)
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

func newConsoleWriter(output io.Writer) zerolog.ConsoleWriter {
	return zerolog.ConsoleWriter{
		Out:           output,
		TimeFormat:    time.RFC3339,
		FieldsExclude: []string{"pretty_message", "raw_message"},
		FormatExtra:   writeConsoleMessageBlocks,
	}
}

func writeConsoleMessageBlocks(event map[string]interface{}, buffer *bytes.Buffer) error {
	writeConsoleMessageBlock(buffer, "pretty_message", event["pretty_message"], true)
	writeConsoleMessageBlock(buffer, "raw_message", event["raw_message"], false)
	return nil
}

func writeConsoleMessageBlock(buffer *bytes.Buffer, name string, value interface{}, splitFields bool) {
	message, ok := value.(string)
	if !ok || message == "" {
		return
	}
	buffer.WriteString("\n  ")
	buffer.WriteString(name)
	buffer.WriteString(":")
	for _, line := range consoleMessageLines(message, splitFields) {
		buffer.WriteString("\n    ")
		buffer.WriteString(line)
	}
}

func consoleMessageLines(message string, splitFields bool) []string {
	if !splitFields {
		return []string{message}
	}
	parts := strings.Split(message, "|")
	lines := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			continue
		}
		lines = append(lines, part)
	}
	if len(lines) == 0 {
		return []string{message}
	}
	return lines
}
