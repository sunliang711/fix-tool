package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMergesSourcesInOrder(t *testing.T) {
	dir := t.TempDir()
	defaultFile := filepath.Join(dir, "default.toml")
	configFile := filepath.Join(dir, "config.toml")
	privateFile := filepath.Join(dir, "private.toml")
	writeFile(t, defaultFile, `
[app]
name = "fix-tool"

[log]
level = "info"
format = "console"

[profile]
name = "default"
begin_string = "FIX.4.4"
sender_comp_id = "DEFAULT"
target_comp_id = "TARGET"
host = "default.example"
port = 9876
heartbeat_interval = "30s"
reset_on_logon = true

[profile.tls]
enabled = true
insecure_skip_verify = false

[output]
format = "table"
raw_delimiter = "|"
redact_sensitive = true
`)
	writeFile(t, configFile, `
[profile]
sender_comp_id = "CONFIG"
host = "config.example"
port = 5001
`)
	writeFile(t, privateFile, `
[profile]
target_comp_id = "PRIVATE"
`)
	t.Setenv("FIX_TOOL_PROFILE_HOST", "env.example")

	cfg, err := Load(LoadOptions{
		DefaultFile:  defaultFile,
		ConfigFile:   configFile,
		PrivateFile:  privateFile,
		ProfileName:  "flag-profile",
		LogLevel:     "debug",
		OutputFormat: "json",
	})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Profile.Name != "flag-profile" {
		t.Fatalf("profile name = %q, want %q", cfg.Profile.Name, "flag-profile")
	}
	if cfg.Profile.SenderCompID != "CONFIG" {
		t.Fatalf("sender comp id = %q, want %q", cfg.Profile.SenderCompID, "CONFIG")
	}
	if cfg.Profile.TargetCompID != "PRIVATE" {
		t.Fatalf("target comp id = %q, want %q", cfg.Profile.TargetCompID, "PRIVATE")
	}
	if cfg.Profile.Host != "env.example" {
		t.Fatalf("host = %q, want %q", cfg.Profile.Host, "env.example")
	}
	if cfg.Profile.Port != 5001 {
		t.Fatalf("port = %d, want %d", cfg.Profile.Port, 5001)
	}
	if cfg.Log.Level != "debug" {
		t.Fatalf("log level = %q, want %q", cfg.Log.Level, "debug")
	}
	if cfg.Output.Format != "json" {
		t.Fatalf("output format = %q, want %q", cfg.Output.Format, "json")
	}
}

func TestLoadFailsWhenExplicitConfigFileMissing(t *testing.T) {
	_, err := Load(LoadOptions{
		ConfigFile: filepath.Join(t.TempDir(), "missing.toml"),
	})
	if err == nil {
		t.Fatal("Load() error = nil, want error")
	}
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("write file %s: %v", path, err)
	}
}
