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
	for i, customFieldDef := range cfg.Profile.CustomFieldDefs {
		if customFieldDef.Tag <= 0 {
			return fmt.Errorf("validate profile custom_field_defs[%d]: tag must be positive", i)
		}
		if strings.TrimSpace(customFieldDef.Name) == "" {
			return fmt.Errorf("validate profile custom_field_defs[%d]: name is required", i)
		}
		if strings.TrimSpace(customFieldDef.Type) == "" {
			return fmt.Errorf("validate profile custom_field_defs[%d]: type is required", i)
		}
	}
	for i, logonTag := range cfg.Profile.LogonTags {
		if logonTag.Tag <= 0 {
			return fmt.Errorf("validate profile logon_tags[%d]: tag must be positive", i)
		}
		if isProtectedLogonTag(logonTag.Tag) {
			return fmt.Errorf("validate profile logon_tags[%d]: tag %d cannot override protocol or built-in Logon fields", i, logonTag.Tag)
		}
		if strings.Contains(logonTag.Value, "\x01") {
			return fmt.Errorf("validate profile logon_tags[%d]: value cannot contain SOH delimiter", i)
		}
	}
	return nil
}

func isProtectedLogonTag(tag int) bool {
	switch tag {
	case 8, 9, 10, 34, 35, 49, 52, 56, 98, 108, 141, 553, 554:
		return true
	default:
		return false
	}
}
