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
