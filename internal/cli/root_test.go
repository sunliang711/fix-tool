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
	for _, want := range []string{"logon", "logout", "heartbeat", "test-request", "order", "shell"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("help output = %q, want command %q", out.String(), want)
		}
	}
}

func TestShellHelp(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	command := NewRootCommand(Args{"shell", "--help"}, IO{
		Out:    &out,
		ErrOut: &errOut,
	}, zerolog.Nop())

	if err := command.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("ExecuteContext() error = %v", err)
	}
	if !strings.Contains(out.String(), "Start interactive FIX shell") {
		t.Fatalf("help output = %q, want shell help", out.String())
	}
}

func TestShellCommandReadsInjectedInput(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	command := NewRootCommand(Args{"shell"}, IO{
		In:     strings.NewReader("exit\n"),
		Out:    &out,
		ErrOut: &errOut,
	}, zerolog.Nop())

	if err := command.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("ExecuteContext() error = %v", err)
	}
	if !strings.Contains(out.String(), "fix-tool> ") {
		t.Fatalf("shell output = %q, want prompt", out.String())
	}
}

func TestOrderHelpShowsSubcommands(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	command := NewRootCommand(Args{"order", "--help"}, IO{
		Out:    &out,
		ErrOut: &errOut,
	}, zerolog.Nop())

	if err := command.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("ExecuteContext() error = %v", err)
	}
	for _, want := range []string{"new", "cancel", "replace"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("help output = %q, want order subcommand %q", out.String(), want)
		}
	}
}

func TestOrderSubcommandHelpShowsFlags(t *testing.T) {
	tests := []struct {
		name string
		args Args
		want []string
	}{
		{
			name: "new",
			args: Args{"order", "new", "--help"},
			want: []string{"--symbol", "--side", "--qty", "--price", "--cl-ord-id", "--ord-type", "--time-in-force", "--tag"},
		},
		{
			name: "cancel",
			args: Args{"order", "cancel", "--help"},
			want: []string{"--orig-cl-ord-id", "--symbol", "--side", "--cl-ord-id", "--order-id", "--tag"},
		},
		{
			name: "replace",
			args: Args{"order", "replace", "--help"},
			want: []string{"--orig-cl-ord-id", "--qty", "--price", "--cl-ord-id", "--order-id", "--symbol", "--side", "--ord-type", "--time-in-force", "--tag"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			var errOut bytes.Buffer
			command := NewRootCommand(tt.args, IO{
				Out:    &out,
				ErrOut: &errOut,
			}, zerolog.Nop())

			if err := command.ExecuteContext(context.Background()); err != nil {
				t.Fatalf("ExecuteContext() error = %v", err)
			}
			for _, want := range tt.want {
				if !strings.Contains(out.String(), want) {
					t.Fatalf("help output = %q, want flag %q", out.String(), want)
				}
			}
		})
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
