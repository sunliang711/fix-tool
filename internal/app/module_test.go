package app

import (
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"testing"
)

func TestRunPrintsCommandErrors(t *testing.T) {
	stderr := captureStderr(t, func() int {
		return Run(context.Background(), []string{"--bad-flag", "config", "validate"})
	})

	if !strings.Contains(stderr, "unknown flag: --bad-flag") {
		t.Fatalf("stderr = %q, want unknown flag error", stderr)
	}
}

func captureStderr(t *testing.T, run func() int) string {
	t.Helper()
	original := os.Stderr
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("Pipe() error = %v", err)
	}
	os.Stderr = writer
	defer func() {
		os.Stderr = original
	}()

	exitCode := run()
	if exitCode == 0 {
		t.Fatal("Run() exit code = 0, want non-zero")
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	var output bytes.Buffer
	if _, err := io.Copy(&output, reader); err != nil {
		t.Fatalf("Copy() error = %v", err)
	}
	return output.String()
}
