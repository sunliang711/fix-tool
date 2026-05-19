package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

type LoadOptions struct {
	DefaultFile  string
	ConfigFile   string
	PrivateFile  string
	ProfileName  string
	LogLevel     string
	OutputFormat string
}

func Load(opts LoadOptions) (*AppConfig, error) {
	v := viper.New()
	v.SetConfigType("toml")
	v.SetEnvPrefix(EnvPrefix)
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	setDefaults(v)
	if err := bindKnownEnv(v); err != nil {
		return nil, err
	}
	var loadedFiles []string
	if loadedFile, err := mergeConfigFile(v, defaultFile(opts), false); err != nil {
		return nil, err
	} else if loadedFile != "" {
		loadedFiles = append(loadedFiles, loadedFile)
	}
	if loadedFile, err := mergeConfigFile(v, userConfigFile(opts), opts.ConfigFile != ""); err != nil {
		return nil, err
	} else if loadedFile != "" {
		loadedFiles = append(loadedFiles, loadedFile)
	}
	if loadedFile, err := mergeConfigFile(v, privateConfigFile(opts), opts.PrivateFile != ""); err != nil {
		return nil, err
	} else if loadedFile != "" {
		loadedFiles = append(loadedFiles, loadedFile)
	}
	applyFlagOverrides(v, opts)

	var cfg AppConfig
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal configuration: %w", err)
	}
	cfg.LoadedFiles = loadedFiles
	normalizeDeprecatedProfileKeys(&cfg, v)
	return &cfg, nil
}

func normalizeDeprecatedProfileKeys(cfg *AppConfig, v *viper.Viper) {
	if cfg == nil {
		return
	}
	if v.IsSet("profile.custom_tags") {
		cfg.DeprecatedKeys = append(cfg.DeprecatedKeys, "profile.custom_tags")
		if len(cfg.Profile.CustomFieldDefs) == 0 {
			cfg.Profile.CustomFieldDefs = cfg.Profile.CustomTags
		}
	}
	cfg.Profile.CustomTags = nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("app.name", "fix-tool")
	v.SetDefault("log.level", "info")
	v.SetDefault("log.format", "json")
	v.SetDefault("profile.name", "default")
	v.SetDefault("profile.begin_string", "FIX.4.4")
	v.SetDefault("profile.sender_comp_id", "SENDER")
	v.SetDefault("profile.target_comp_id", "TARGET")
	v.SetDefault("profile.username", "")
	v.SetDefault("profile.password", "")
	v.SetDefault("profile.host", "127.0.0.1")
	v.SetDefault("profile.port", 9876)
	v.SetDefault("profile.heartbeat_interval", "30s")
	v.SetDefault("profile.reset_on_logon", true)
	v.SetDefault("profile.data_dictionary", "")
	v.SetDefault("profile.transport_data_dictionary", "")
	v.SetDefault("profile.app_data_dictionary", "")
	v.SetDefault("profile.tls.enabled", true)
	v.SetDefault("profile.tls.insecure_skip_verify", false)
	v.SetDefault("profile.tls.ca_file", "")
	v.SetDefault("profile.tls.cert_file", "")
	v.SetDefault("profile.tls.key_file", "")
	v.SetDefault("output.format", "table")
	v.SetDefault("output.raw_delimiter", "|")
	v.SetDefault("output.redact_sensitive", true)
}

func bindKnownEnv(v *viper.Viper) error {
	for _, key := range knownKeys() {
		if err := v.BindEnv(key); err != nil {
			return fmt.Errorf("bind environment variable %s: %w", key, err)
		}
	}
	return nil
}

func knownKeys() []string {
	return []string{
		"app.name",
		"log.level",
		"log.format",
		"profile.name",
		"profile.begin_string",
		"profile.sender_comp_id",
		"profile.target_comp_id",
		"profile.username",
		"profile.password",
		"profile.host",
		"profile.port",
		"profile.heartbeat_interval",
		"profile.reset_on_logon",
		"profile.data_dictionary",
		"profile.transport_data_dictionary",
		"profile.app_data_dictionary",
		"profile.tls.enabled",
		"profile.tls.insecure_skip_verify",
		"profile.tls.ca_file",
		"profile.tls.cert_file",
		"profile.tls.key_file",
		"output.format",
		"output.raw_delimiter",
		"output.redact_sensitive",
	}
}

func mergeConfigFile(v *viper.Viper, file string, required bool) (string, error) {
	if file == "" {
		return "", nil
	}
	if _, err := os.Stat(file); err != nil {
		if errors.Is(err, os.ErrNotExist) && !required {
			return "", nil
		}
		return "", fmt.Errorf("read configuration file %s: %w", file, err)
	}
	v.SetConfigFile(file)
	if err := v.MergeInConfig(); err != nil {
		return "", fmt.Errorf("merge configuration file %s: %w", file, err)
	}
	return absolutePath(file), nil
}

func absolutePath(file string) string {
	path, err := filepath.Abs(file)
	if err != nil {
		return filepath.Clean(file)
	}
	return path
}

func applyFlagOverrides(v *viper.Viper, opts LoadOptions) {
	if opts.ProfileName != "" {
		v.Set("profile.name", opts.ProfileName)
	}
	if opts.LogLevel != "" {
		v.Set("log.level", opts.LogLevel)
	}
	if opts.OutputFormat != "" {
		v.Set("output.format", opts.OutputFormat)
	}
}

func defaultFile(opts LoadOptions) string {
	if opts.DefaultFile != "" {
		return opts.DefaultFile
	}
	return DefaultConfigFile
}

func userConfigFile(opts LoadOptions) string {
	if opts.ConfigFile != "" {
		return opts.ConfigFile
	}
	return UserConfigFile
}

func privateConfigFile(opts LoadOptions) string {
	if opts.PrivateFile != "" {
		return opts.PrivateFile
	}
	return PrivateConfigFile
}
