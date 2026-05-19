package validate_test

import (
	"path/filepath"
	"strings"
	"testing"

	"fix-tool/internal/config"
	"fix-tool/internal/validate"
)

func TestAppConfigValid(t *testing.T) {
	cfg := validConfig()
	if err := validate.AppConfig(cfg); err != nil {
		t.Fatalf("AppConfig() error = %v", err)
	}
}

func TestAppConfigAllowsTraceLogLevel(t *testing.T) {
	cfg := validConfig()
	cfg.Log.Level = "trace"
	if err := validate.AppConfig(cfg); err != nil {
		t.Fatalf("AppConfig() error = %v", err)
	}
}

func TestAppConfigRejectsInvalidPort(t *testing.T) {
	cfg := validConfig()
	cfg.Profile.Port = 0
	err := validate.AppConfig(cfg)
	if err == nil {
		t.Fatal("AppConfig() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "validate configuration") {
		t.Fatalf("AppConfig() error = %v, want validation error", err)
	}
}

func TestAppConfigRejectsInvalidHeartbeatInterval(t *testing.T) {
	cfg := validConfig()
	cfg.Profile.HeartbeatInterval = "bad"
	err := validate.AppConfig(cfg)
	if err == nil {
		t.Fatal("AppConfig() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "heartbeat_interval") {
		t.Fatalf("AppConfig() error = %v, want heartbeat error", err)
	}
}

func TestAppConfigRejectsInvalidCustomFieldDefs(t *testing.T) {
	tests := []struct {
		name           string
		customFieldDef config.CustomFieldDefConfig
		want           string
	}{
		{
			name: "invalid-tag",
			customFieldDef: config.CustomFieldDefConfig{
				Name: "Desk",
				Type: "STRING",
			},
			want: "tag must be positive",
		},
		{
			name: "missing-name",
			customFieldDef: config.CustomFieldDefConfig{
				Tag:  9001,
				Type: "STRING",
			},
			want: "name is required",
		},
		{
			name: "missing-type",
			customFieldDef: config.CustomFieldDefConfig{
				Tag:  9001,
				Name: "Desk",
			},
			want: "type is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			cfg.Profile.CustomFieldDefs = []config.CustomFieldDefConfig{tt.customFieldDef}
			err := validate.AppConfig(cfg)
			if err == nil {
				t.Fatal("AppConfig() error = nil, want custom field def validation error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("AppConfig() error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestAppConfigRejectsInvalidLogonTags(t *testing.T) {
	tests := []struct {
		name     string
		logonTag config.LogonTagConfig
		want     string
	}{
		{
			name: "invalid-tag",
			logonTag: config.LogonTagConfig{
				Value: "ALPHA",
			},
			want: "tag must be positive",
		},
		{
			name: "protected-tag",
			logonTag: config.LogonTagConfig{
				Tag:   553,
				Value: "account",
			},
			want: "cannot override",
		},
		{
			name: "soh-value",
			logonTag: config.LogonTagConfig{
				Tag:   9001,
				Value: "hello\x0135=A",
			},
			want: "SOH delimiter",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			cfg.Profile.LogonTags = []config.LogonTagConfig{tt.logonTag}
			err := validate.AppConfig(cfg)
			if err == nil {
				t.Fatal("AppConfig() error = nil, want logon tag validation error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("AppConfig() error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestSampleMockConfigValidates(t *testing.T) {
	cfg, err := config.Load(config.LoadOptions{
		ConfigFile: filepath.Clean("../../testdata/configs/mock-acceptor.toml"),
	})
	if err != nil {
		t.Fatalf("Load() sample error = %v", err)
	}
	if err := validate.AppConfig(cfg); err != nil {
		t.Fatalf("AppConfig() sample error = %v", err)
	}
	if cfg.Profile.Username != "" || cfg.Profile.Password != "" {
		t.Fatalf("sample credentials should be empty")
	}
}

func TestConfigExampleValidates(t *testing.T) {
	cfg, err := config.Load(config.LoadOptions{
		ConfigFile: filepath.Clean("../../config-example.toml"),
	})
	if err != nil {
		t.Fatalf("Load() config example error = %v", err)
	}
	if err := validate.AppConfig(cfg); err != nil {
		t.Fatalf("AppConfig() config example error = %v", err)
	}
}

func validConfig() *config.AppConfig {
	return &config.AppConfig{
		App: config.AppSettings{
			Name: "fix-tool",
		},
		Log: config.LogConfig{
			Level:  "info",
			Format: "console",
		},
		Profile: config.ProfileConfig{
			Name:              "default",
			BeginString:       "FIX.4.4",
			SenderCompID:      "SENDER",
			TargetCompID:      "TARGET",
			Host:              "127.0.0.1",
			Port:              9876,
			HeartbeatInterval: "30s",
			TLS: config.TLSConfig{
				Enabled: true,
			},
		},
		Output: config.OutputConfig{
			Format:          "table",
			RawDelimiter:    "|",
			RedactSensitive: true,
		},
	}
}
