package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	fixtool "fix-tool"

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

	if err := mergeEmbeddedDefault(v); err != nil {
		return nil, err
	}
	if err := bindKnownEnv(v); err != nil {
		return nil, err
	}
	var loadedFiles []string
	if opts.DefaultFile != "" {
		if loadedFile, err := mergeConfigFile(v, opts.DefaultFile, true); err != nil {
			return nil, err
		} else if loadedFile != "" {
			loadedFiles = append(loadedFiles, loadedFile)
		}
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
	cfg.DefaultSource = EmbeddedDefaultConfigSource
	cfg.LoadedFiles = loadedFiles
	normalizeDeprecatedProfileKeys(&cfg, v)
	return &cfg, nil
}

func mergeEmbeddedDefault(v *viper.Viper) error {
	if err := v.ReadConfig(bytes.NewBufferString(fixtool.DefaultConfigTOML())); err != nil {
		return fmt.Errorf("merge embedded default configuration: %w", err)
	}
	return nil
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
