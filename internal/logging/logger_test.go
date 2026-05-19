package logging

import (
	"bytes"
	"regexp"
	"strings"
	"testing"

	"fix-tool/internal/config"
)

func TestConsoleWriterRendersFIXMessagesAsBlocks(t *testing.T) {
	var out bytes.Buffer
	logger, err := New(&out, config.LogConfig{
		Level:  "debug",
		Format: "console",
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	logger.Debug().
		Str("direction", "out").
		Str("pretty_message", "8(BeginString)=FIX.4.4|9(BodyLength)=60|35(MsgType:Heartbeat)=0|").
		Str("source", "quickfix").
		Str("view", "pretty").
		Msg("-> Heartbeat(0)")
	logger.Debug().
		Str("direction", "out").
		Str("raw_message", "8=FIX.4.4|9=60|35=0|").
		Str("source", "quickfix").
		Str("view", "raw").
		Msg("-> Heartbeat(0)")

	got := stripANSICodes(out.String())
	for _, unwanted := range []string{"pretty_message=", "raw_message="} {
		if strings.Contains(got, unwanted) {
			t.Fatalf("log output = %q, want no inline %q", got, unwanted)
		}
	}
	for _, want := range []string{
		"view=pretty",
		"\n  pretty_message:\n    8(BeginString)=FIX.4.4\n    9(BodyLength)=60\n    35(MsgType:Heartbeat)=0",
		"view=raw",
		"\n  raw_message:\n    8=FIX.4.4|9=60|35=0|",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("log output = %q, want %q", got, want)
		}
	}
}

func stripANSICodes(value string) string {
	pattern := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	return pattern.ReplaceAllString(value, "")
}

func TestJSONWriterKeepsFIXMessageFields(t *testing.T) {
	var out bytes.Buffer
	logger, err := New(&out, config.LogConfig{
		Level:  "debug",
		Format: "json",
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	logger.Debug().
		Str("pretty_message", "8(BeginString)=FIX.4.4|").
		Str("raw_message", "8=FIX.4.4|").
		Msg("-> Logon(A)")

	got := out.String()
	for _, want := range []string{
		`"pretty_message":"8(BeginString)=FIX.4.4|"`,
		`"raw_message":"8=FIX.4.4|"`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("log output = %q, want %q", got, want)
		}
	}
}
