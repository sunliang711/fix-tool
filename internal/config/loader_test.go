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
	wantLoadedFiles := []string{defaultFile, configFile, privateFile}
	if len(cfg.LoadedFiles) != len(wantLoadedFiles) {
		t.Fatalf("loaded files = %#v, want %#v", cfg.LoadedFiles, wantLoadedFiles)
	}
	for i, want := range wantLoadedFiles {
		if cfg.LoadedFiles[i] != want {
			t.Fatalf("loaded files[%d] = %q, want %q", i, cfg.LoadedFiles[i], want)
		}
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

func TestLoadCustomFieldDefEnums(t *testing.T) {
	path := filepath.Join(t.TempDir(), "custom-field-defs.toml")
	writeFile(t, path, `
[[profile.custom_field_defs]]
tag = 9002
name = "Desk"
type = "STRING"
enums = { ALPHA = "Alpha desk" }

[[profile.logon_tags]]
tag = 9002
value = "ALPHA"

[[profile.logon_tags]]
tag = 9003
value = "BETA"
`)

	cfg, err := Load(LoadOptions{ConfigFile: path})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(cfg.Profile.CustomFieldDefs) != 1 {
		t.Fatalf("custom field defs = %d, want 1", len(cfg.Profile.CustomFieldDefs))
	}
	if cfg.Profile.CustomFieldDefs[0].Enums["alpha"] != "Alpha desk" {
		t.Fatalf("custom field def enums = %#v, want alpha", cfg.Profile.CustomFieldDefs[0].Enums)
	}
	if len(cfg.Profile.LogonTags) != 2 || cfg.Profile.LogonTags[0].Value != "ALPHA" || cfg.Profile.LogonTags[1].Value != "BETA" {
		t.Fatalf("logon tags = %#v, want ALPHA and BETA", cfg.Profile.LogonTags)
	}
}

func TestLoadDeprecatedCustomTagsFallback(t *testing.T) {
	path := filepath.Join(t.TempDir(), "custom-tags.toml")
	writeFile(t, path, `
[[profile.custom_tags]]
tag = 9002
name = "Desk"
type = "STRING"
`)

	cfg, err := Load(LoadOptions{ConfigFile: path})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(cfg.Profile.CustomFieldDefs) != 1 {
		t.Fatalf("custom field defs = %d, want fallback from custom_tags", len(cfg.Profile.CustomFieldDefs))
	}
	if len(cfg.Profile.CustomTags) != 0 {
		t.Fatalf("custom tags = %#v, want cleared deprecated field", cfg.Profile.CustomTags)
	}
	if len(cfg.DeprecatedKeys) != 1 || cfg.DeprecatedKeys[0] != "profile.custom_tags" {
		t.Fatalf("deprecated keys = %#v, want profile.custom_tags", cfg.DeprecatedKeys)
	}
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("write file %s: %v", path, err)
	}
}
