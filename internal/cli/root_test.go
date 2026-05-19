package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	fixtool "fix-tool"

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
	for _, want := range []string{"check", "order", "raw", "inspect", "shell", "run"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("help output = %q, want command %q", out.String(), want)
		}
	}
}

func TestCheckHelp(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	command := NewRootCommand(Args{"check", "--help"}, IO{
		Out:    &out,
		ErrOut: &errOut,
	}, zerolog.Nop())

	if err := command.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("ExecuteContext() error = %v", err)
	}
	for _, want := range []string{"Run one-shot FIX session checks", "logon", "logout", "heartbeat", "test-request"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("help output = %q, want %q", out.String(), want)
		}
	}
}

func TestTopLevelAdminCommandsAreNotRegistered(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	command := NewRootCommand(Args{"logon"}, IO{
		Out:    &out,
		ErrOut: &errOut,
	}, zerolog.Nop())

	err := command.ExecuteContext(context.Background())
	if err == nil {
		t.Fatal("ExecuteContext() error = nil, want unknown command error")
	}
	if !strings.Contains(err.Error(), "unknown command") {
		t.Fatalf("ExecuteContext() error = %v, want unknown command error", err)
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

func TestVersionCommand(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	command := NewRootCommand(Args{"version"}, IO{
		Out:    &out,
		ErrOut: &errOut,
	}, zerolog.Nop())

	if err := command.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("ExecuteContext() error = %v", err)
	}
	for _, want := range []string{"version: dev", "commit: none", "build_time: unknown"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("version output = %q, want %q", out.String(), want)
		}
	}
}

func TestRootRejectsUnknownFlags(t *testing.T) {
	tests := []struct {
		name string
		args Args
	}{
		{name: "before-command", args: Args{"--bad-flag", "config", "validate"}},
		{name: "after-command", args: Args{"config", "validate", "--bad-flag"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			var errOut bytes.Buffer
			command := NewRootCommand(tt.args, IO{
				Out:    &out,
				ErrOut: &errOut,
			}, zerolog.Nop())

			err := command.ExecuteContext(context.Background())
			if err == nil {
				t.Fatal("ExecuteContext() error = nil, want unknown flag error")
			}
			if !strings.Contains(err.Error(), "unknown flag: --bad-flag") {
				t.Fatalf("ExecuteContext() error = %v, want unknown flag error", err)
			}
		})
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

func TestInspectRawRendersCustomFieldDefs(t *testing.T) {
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
	command := NewRootCommand(Args{"check", "test-request", "--help"}, IO{
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
	command := NewRootCommand(Args{"check", "test-request"}, IO{
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

func TestConfigExampleWritesDefaultFile(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	var out bytes.Buffer
	var errOut bytes.Buffer
	command := NewRootCommand(Args{"config", "example"}, IO{
		Out:    &out,
		ErrOut: &errOut,
	}, zerolog.Nop())

	if err := command.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("ExecuteContext() error = %v", err)
	}
	if !strings.Contains(out.String(), "config example written to config-example.toml") {
		t.Fatalf("output = %q, want written message", out.String())
	}
	data, err := os.ReadFile(filepath.Join(dir, "config-example.toml"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(data) != fixtool.ConfigExampleTOML() {
		t.Fatalf("config example content mismatch")
	}
}

func TestConfigExampleWritesOutputFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "example.toml")
	var out bytes.Buffer
	var errOut bytes.Buffer
	command := NewRootCommand(Args{"config", "example", "--output", path}, IO{
		Out:    &out,
		ErrOut: &errOut,
	}, zerolog.Nop())

	if err := command.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("ExecuteContext() error = %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(data) != fixtool.ConfigExampleTOML() {
		t.Fatalf("config example content mismatch")
	}
}

func TestConfigExampleRefusesOverwriteWithoutForce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config-example.toml")
	if err := os.WriteFile(path, []byte("existing"), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	var out bytes.Buffer
	var errOut bytes.Buffer
	command := NewRootCommand(Args{"config", "example", "--output", path}, IO{
		Out:    &out,
		ErrOut: &errOut,
	}, zerolog.Nop())

	err := command.ExecuteContext(context.Background())
	if err == nil {
		t.Fatal("ExecuteContext() error = nil, want overwrite error")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("ExecuteContext() error = %v, want already exists error", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(data) != "existing" {
		t.Fatalf("file content = %q, want existing", string(data))
	}
}

func TestConfigExampleForceOverwrites(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config-example.toml")
	if err := os.WriteFile(path, []byte("existing"), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	var out bytes.Buffer
	var errOut bytes.Buffer
	command := NewRootCommand(Args{"config", "example", "--output", path, "--force"}, IO{
		Out:    &out,
		ErrOut: &errOut,
	}, zerolog.Nop())

	if err := command.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("ExecuteContext() error = %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(data) != fixtool.ConfigExampleTOML() {
		t.Fatalf("config example content mismatch")
	}
}

func writeInspectConfig(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "inspect-config.toml")
	content := `
[[profile.custom_field_defs]]
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
