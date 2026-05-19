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

type Options struct {
	NoColor bool
}

func New(output io.Writer, cfg config.LogConfig) (zerolog.Logger, error) {
	return NewWithOptions(output, cfg, Options{})
}

func NewWithOptions(output io.Writer, cfg config.LogConfig, options Options) (zerolog.Logger, error) {
	if output == nil {
		output = os.Stderr
	}
	level, err := zerolog.ParseLevel(cfg.Level)
	if err != nil {
		return zerolog.Logger{}, fmt.Errorf("parse log level: %w", err)
	}
	writer := output
	if cfg.Format == "console" {
		writer = newConsoleWriter(output, options.NoColor)
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

func newConsoleWriter(output io.Writer, noColor bool) zerolog.ConsoleWriter {
	return zerolog.ConsoleWriter{
		Out:           output,
		NoColor:       noColor,
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
	fields := make([]consolePrettyField, 0, len(parts))
	maxNameWidth := 0
	maxTagWidth := 0
	for _, part := range parts {
		if part == "" {
			continue
		}
		field := parseConsolePrettyField(part)
		fields = append(fields, field)
		if field.ok {
			if len(field.name) > maxNameWidth {
				maxNameWidth = len(field.name)
			}
			if len(field.tag) > maxTagWidth {
				maxTagWidth = len(field.tag)
			}
		}
	}
	if len(fields) == 0 {
		return []string{message}
	}
	lines := make([]string, 0, len(fields))
	for _, field := range fields {
		lines = append(lines, consolePrettyFieldLine(field, maxNameWidth, maxTagWidth))
	}
	return lines
}

type consolePrettyField struct {
	tag   string
	name  string
	value string
	raw   string
	ok    bool
}

func parseConsolePrettyField(field string) consolePrettyField {
	tag, rest, ok := strings.Cut(field, "(")
	if !ok {
		return consolePrettyField{raw: field}
	}
	name, value, ok := strings.Cut(rest, ")=")
	if !ok {
		return consolePrettyField{raw: field}
	}
	return consolePrettyField{
		tag:   tag,
		name:  name,
		value: value,
		raw:   field,
		ok:    true,
	}
}

func consolePrettyFieldLine(field consolePrettyField, nameWidth int, tagWidth int) string {
	if !field.ok {
		return field.raw
	}
	return fmt.Sprintf("%-*s %*s = %s", nameWidth, field.name, tagWidth, field.tag, field.value)
}
