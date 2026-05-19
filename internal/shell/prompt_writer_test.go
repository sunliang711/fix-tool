package shell

import (
	"bytes"
	"strings"
	"testing"
)

func TestPromptLogWriterReprintsPromptWhenActive(t *testing.T) {
	var logs bytes.Buffer
	var out bytes.Buffer
	writer := NewPromptLogWriter(&logs, &out, "fix-tool> ")

	writer.SetPromptActive(true)
	if _, err := writer.Write([]byte("log line\n")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	if !strings.Contains(logs.String(), "\r\nlog line\n") {
		t.Fatalf("logs = %q, want log on fresh line", logs.String())
	}
	if out.String() != "fix-tool> " {
		t.Fatalf("out = %q, want prompt reprinted", out.String())
	}
}

func TestPromptLogWriterDoesNotReprintPromptWhenInactive(t *testing.T) {
	var logs bytes.Buffer
	var out bytes.Buffer
	writer := NewPromptLogWriter(&logs, &out, "fix-tool> ")

	if _, err := writer.Write([]byte("log line\n")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	if logs.String() != "log line\n" {
		t.Fatalf("logs = %q, want plain log line", logs.String())
	}
	if out.String() != "" {
		t.Fatalf("out = %q, want no prompt", out.String())
	}
}
