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

func TestAppConfigRejectsInvalidCustomTags(t *testing.T) {
	tests := []struct {
		name      string
		customTag config.CustomTagConfig
		want      string
	}{
		{
			name: "invalid-tag",
			customTag: config.CustomTagConfig{
				Name: "Desk",
				Type: "STRING",
			},
			want: "tag must be positive",
		},
		{
			name: "missing-name",
			customTag: config.CustomTagConfig{
				Tag:  9001,
				Type: "STRING",
			},
			want: "name is required",
		},
		{
			name: "missing-type",
			customTag: config.CustomTagConfig{
				Tag:  9001,
				Name: "Desk",
			},
			want: "type is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			cfg.Profile.CustomTags = []config.CustomTagConfig{tt.customTag}
			err := validate.AppConfig(cfg)
			if err == nil {
				t.Fatal("AppConfig() error = nil, want custom tag validation error")
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
