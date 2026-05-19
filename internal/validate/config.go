package validate

import (
	"fmt"
	"strings"
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
	for i, customTag := range cfg.Profile.CustomTags {
		if customTag.Tag <= 0 {
			return fmt.Errorf("validate profile custom_tags[%d]: tag must be positive", i)
		}
		if strings.TrimSpace(customTag.Name) == "" {
			return fmt.Errorf("validate profile custom_tags[%d]: name is required", i)
		}
		if strings.TrimSpace(customTag.Type) == "" {
			return fmt.Errorf("validate profile custom_tags[%d]: type is required", i)
		}
	}
	return nil
}
