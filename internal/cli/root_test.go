package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
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
	for _, want := range []string{"logon", "logout", "heartbeat", "test-request", "order", "raw", "inspect", "shell", "run"} {
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

func TestRunHelpShowsScenarioFlags(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	command := NewRootCommand(Args{"run", "--help"}, IO{
		Out:    &out,
		ErrOut: &errOut,
	}, zerolog.Nop())

	if err := command.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("ExecuteContext() error = %v", err)
	}
	for _, want := range []string{"Run FIX scenario steps", "--json", "--result-file", "--output-file"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("help output = %q, want %q", out.String(), want)
		}
	}
}

func TestRawSendHelpShowsFlags(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	command := NewRootCommand(Args{"raw", "send", "--help"}, IO{
		Out:    &out,
		ErrOut: &errOut,
	}, zerolog.Nop())

	if err := command.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("ExecuteContext() error = %v", err)
	}
	for _, want := range []string{"Send a raw FIX message", "--msg-type", "--tag"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("help output = %q, want %q", out.String(), want)
		}
	}
}

func TestRawSendRejectsProtectedTagBeforeConfigLoad(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	command := NewRootCommand(Args{"raw", "send", "--msg-type", "D", "--tag", "35=Z"}, IO{
		Out:    &out,
		ErrOut: &errOut,
	}, zerolog.Nop())

	err := command.ExecuteContext(context.Background())
	if err == nil {
		t.Fatal("ExecuteContext() error = nil, want protected tag error")
	}
	if !strings.Contains(err.Error(), "不允许覆盖协议字段 35") {
		t.Fatalf("ExecuteContext() error = %v, want protected tag error", err)
	}
}

func TestInspectRawHelpShowsCommand(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	command := NewRootCommand(Args{"inspect", "raw", "--help"}, IO{
		Out:    &out,
		ErrOut: &errOut,
	}, zerolog.Nop())

	if err := command.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("ExecuteContext() error = %v", err)
	}
	if !strings.Contains(out.String(), "Inspect a raw FIX message") {
		t.Fatalf("help output = %q, want inspect raw help", out.String())
	}
}

func TestInspectRawRendersCustomTags(t *testing.T) {
	configFile := writeInspectConfig(t)
	var out bytes.Buffer
	var errOut bytes.Buffer
	command := NewRootCommand(Args{"--config", configFile, "inspect", "raw", "8=FIX.4.4|9=18|35=Z|9002=ALPHA|10=000|"}, IO{
		Out:    &out,
		ErrOut: &errOut,
	}, zerolog.Nop())

	if err := command.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("ExecuteContext() error = %v", err)
	}
	for _, want := range []string{"Desk", "ALPHA", "Alpha desk"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("inspect output = %q, want %q", out.String(), want)
		}
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

func writeInspectConfig(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "inspect-config.toml")
	content := `
[[profile.custom_tags]]
tag = 9002
name = "Desk"
type = "STRING"
required = false
sensitive = false
description = "Mock trading desk identifier"
enums = { ALPHA = "Alpha desk" }
`
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	return path
}
