package validate

import (
	"fmt"
	"time"

	"fix-tool/internal/config"

	"github.com/go-playground/validator/v10"
)

func AppConfig(cfg *config.AppConfig) error {
	if cfg == nil {
		return fmt.Errorf("configuration is nil")
	}
	v := validator.New()
	if err := v.Struct(cfg); err != nil {
		return fmt.Errorf("validate configuration: %w", err)
	}
	if _, err := time.ParseDuration(cfg.Profile.HeartbeatInterval); err != nil {
		return fmt.Errorf("validate profile heartbeat_interval: %w", err)
	}
	if cfg.Profile.TLS.CertFile != "" && cfg.Profile.TLS.KeyFile == "" {
		return fmt.Errorf("validate profile tls: key_file is required when cert_file is set")
	}
	if cfg.Profile.TLS.KeyFile != "" && cfg.Profile.TLS.CertFile == "" {
		return fmt.Errorf("validate profile tls: cert_file is required when key_file is set")
	}
	return nil
}
