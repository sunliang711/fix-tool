package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/rs/zerolog"
)

func TestRootHelp(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	command := NewRootCommand(Args{"--help"}, IO{
		Out:    &out,
		ErrOut: &errOut,
	}, zerolog.Nop())

	if err := command.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("ExecuteContext() error = %v", err)
	}
	if !strings.Contains(out.String(), "FIX protocol testing CLI") {
		t.Fatalf("help output = %q, want root help", out.String())
	}
	for _, want := range []string{"logon", "logout", "heartbeat", "test-request"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("help output = %q, want command %q", out.String(), want)
		}
	}
}

func TestTestRequestHelpShowsIDFlag(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	command := NewRootCommand(Args{"test-request", "--help"}, IO{
		Out:    &out,
		ErrOut: &errOut,
	}, zerolog.Nop())

	if err := command.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("ExecuteContext() error = %v", err)
	}
	if !strings.Contains(out.String(), "--id") {
		t.Fatalf("help output = %q, want --id flag", out.String())
	}
}

func TestTestRequestRequiresID(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	command := NewRootCommand(Args{"test-request"}, IO{
		Out:    &out,
		ErrOut: &errOut,
	}, zerolog.Nop())

	err := command.ExecuteContext(context.Background())
	if err == nil {
		t.Fatal("ExecuteContext() error = nil, want required id error")
	}
	if !strings.Contains(err.Error(), "required flag(s) \"id\"") {
		t.Fatalf("ExecuteContext() error = %v, want required id error", err)
	}
}

func TestConfigValidate(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	command := NewRootCommand(Args{"config", "validate"}, IO{
		Out:    &out,
		ErrOut: &errOut,
	}, zerolog.Nop())

	if err := command.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("ExecuteContext() error = %v", err)
	}
	if !strings.Contains(out.String(), "configuration is valid") {
		t.Fatalf("validate output = %q, want success message", out.String())
	}
}
