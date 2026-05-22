package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	fixtool "fix-tool"
	"fix-tool/internal/config"

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
	for _, want := range []string{"docs", "check", "order", "raw", "inspect", "shell", "run"} {
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
	for _, want := range []string{"Run one-shot FIX session checks", "logon", "logout", "test-request"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("help output = %q, want %q", out.String(), want)
		}
	}
	if strings.Contains(out.String(), "heartbeat") {
		t.Fatalf("help output = %q, want no check heartbeat command", out.String())
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

func TestDocsIndexShowsTopics(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	command := NewRootCommand(Args{"docs"}, IO{
		Out:    &out,
		ErrOut: &errOut,
	}, zerolog.Nop())

	if err := command.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("ExecuteContext() error = %v", err)
	}
	for _, want := range []string{"Bundled documentation topics", "user-guide", "faq"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("docs output = %q, want %q", out.String(), want)
		}
	}
}

func TestDocsUserGuideShowsBundledDocument(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	command := NewRootCommand(Args{"docs", "user-guide"}, IO{
		Out:    &out,
		ErrOut: &errOut,
	}, zerolog.Nop())

	if err := command.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("ExecuteContext() error = %v", err)
	}
	if out.String() != fixtool.UserGuideMarkdown() {
		t.Fatalf("docs output mismatch")
	}
}

func TestDocsFAQShowsBundledDocument(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	command := NewRootCommand(Args{"docs", "faq"}, IO{
		Out:    &out,
		ErrOut: &errOut,
	}, zerolog.Nop())

	if err := command.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("ExecuteContext() error = %v", err)
	}
	if out.String() != fixtool.FAQMarkdown() {
		t.Fatalf("docs output mismatch")
	}
}

func TestDocsRejectsUnknownTopic(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	command := NewRootCommand(Args{"docs", "missing"}, IO{
		Out:    &out,
		ErrOut: &errOut,
	}, zerolog.Nop())

	err := command.ExecuteContext(context.Background())
	if err == nil {
		t.Fatal("ExecuteContext() error = nil, want unknown topic error")
	}
	for _, want := range []string{"unknown docs topic", "user-guide", "faq"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("ExecuteContext() error = %v, want %q", err, want)
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

func TestVersionFlagUsesLongFlagOnly(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	command := NewRootCommand(Args{"--help"}, IO{
		Out:    &out,
		ErrOut: &errOut,
	}, zerolog.Nop())

	if err := command.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("ExecuteContext() error = %v", err)
	}
	help := out.String()
	if !strings.Contains(help, "-v, --verbose") {
		t.Fatalf("help output = %q, want verbose shorthand", help)
	}
	if !strings.Contains(help, "--version") {
		t.Fatalf("help output = %q, want version flag", help)
	}
	if strings.Contains(help, "-v, --version") {
		t.Fatalf("help output = %q, want no version shorthand", help)
	}
}

func TestRootVersionFlag(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	command := NewRootCommand(Args{"--version"}, IO{
		Out:    &out,
		ErrOut: &errOut,
	}, zerolog.Nop())

	if err := command.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("ExecuteContext() error = %v", err)
	}
	if !strings.Contains(out.String(), "fix-tool version dev") {
		t.Fatalf("version output = %q, want root version", out.String())
	}
}

func TestVerboseFlagSetsDebugLogLevel(t *testing.T) {
	tests := []struct {
		name  string
		flags flagState
		want  string
	}{
		{name: "default", flags: flagState{}, want: ""},
		{name: "verbose", flags: flagState{verbose: true}, want: "debug"},
		{name: "explicit-log-level-wins", flags: flagState{verbose: true, logLevel: "warn"}, want: "warn"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.flags.effectiveLogLevel(); got != tt.want {
				t.Fatalf("effectiveLogLevel() = %q, want %q", got, tt.want)
			}
		})
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

func TestCheckHeartbeatIsNotRegistered(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	command := NewRootCommand(Args{"check", "heartbeat"}, IO{
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

func TestShellCommandRequiresInteractiveTerminal(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	command := NewRootCommand(Args{"shell"}, IO{
		In:     strings.NewReader("exit\n"),
		Out:    &out,
		ErrOut: &errOut,
	}, zerolog.Nop())

	err := command.ExecuteContext(context.Background())
	if err == nil {
		t.Fatal("ExecuteContext() error = nil, want interactive terminal error")
	}
	if !strings.Contains(err.Error(), "shell requires an interactive terminal") {
		t.Fatalf("ExecuteContext() error = %v, want interactive terminal error", err)
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

func TestLogStartupConfigurationIncludesSafeFields(t *testing.T) {
	var out bytes.Buffer
	logger := zerolog.New(&out)
	cfg := &config.AppConfig{
		Log: config.LogConfig{
			Level:  "info",
			Format: "json",
		},
		Profile: config.ProfileConfig{
			Name:              "uat",
			BeginString:       "FIX.4.4",
			SenderCompID:      "CLIENT01",
			TargetCompID:      "BROKER01",
			Username:          "alice",
			Password:          "secret",
			Host:              "fix.example.test",
			Port:              9876,
			HeartbeatInterval: "30s",
			ResetOnLogon:      true,
			TLS: config.TLSConfig{
				Enabled:            true,
				InsecureSkipVerify: true,
			},
			CustomFieldDefs: []config.CustomFieldDefConfig{
				{Tag: 9001, Name: "SessionToken", Type: "STRING", Sensitive: true},
			},
			LogonTags: []config.LogonTagConfig{
				{Tag: 9001, Value: "token-secret"},
			},
		},
		Output: config.OutputConfig{
			Format:          "table",
			RawDelimiter:    "|",
			RedactSensitive: true,
		},
		DefaultSource: "embedded:config/default.toml",
		LoadedFiles:   []string{"config.toml", "private.toml"},
	}

	logStartupConfiguration(logger, cfg)
	logOutput := out.String()
	for _, want := range []string{
		`"profile_name":"uat"`,
		`"session_id":"FIX.4.4:CLIENT01->BROKER01"`,
		`"begin_string":"FIX.4.4"`,
		`"sender_comp_id":"CLIENT01"`,
		`"target_comp_id":"BROKER01"`,
		`"host":"fix.example.test"`,
		`"port":9876`,
		`"heartbeat_interval":"30s"`,
		`"reset_on_logon":true`,
		`"tls_enabled":true`,
		`"tls_insecure_skip_verify":true`,
		`"username_configured":true`,
		`"password_configured":true`,
		`"custom_field_defs_count":1`,
		`"logon_tags_count":1`,
		`"message":"tls certificate verification is disabled"`,
	} {
		if !strings.Contains(logOutput, want) {
			t.Fatalf("log output = %s, want %s", logOutput, want)
		}
	}
	for _, unwanted := range []string{"alice", "secret", "token-secret"} {
		if strings.Contains(logOutput, unwanted) {
			t.Fatalf("log output = %s, want no sensitive value %q", logOutput, unwanted)
		}
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
