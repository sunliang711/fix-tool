package cli

import (
	"errors"
	"fmt"
	"io"
	"os"

	fixtool "fix-tool"
	"fix-tool/internal/config"
	"fix-tool/internal/logging"
	"fix-tool/internal/validate"
	"fix-tool/internal/version"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
)

type RootCommand struct {
	*cobra.Command
}

type flagState struct {
	defaultConfig string
	configFile    string
	privateFile   string
	profileName   string
	logLevel      string
	verbose       bool
	outputFormat  string
}

func (f *flagState) effectiveLogLevel() string {
	if f == nil {
		return ""
	}
	if f.logLevel != "" {
		return f.logLevel
	}
	if f.verbose {
		return "debug"
	}
	return ""
}

func NewRootCommand(args Args, io IO, logger zerolog.Logger) *RootCommand {
	out := io.Out
	if out == nil {
		out = os.Stdout
	}
	errOut := io.ErrOut
	if errOut == nil {
		errOut = os.Stderr
	}
	in := io.In
	if in == nil {
		in = os.Stdin
	}
	flags := &flagState{}
	root := &cobra.Command{
		Use:              "fix-tool",
		Short:            "FIX protocol testing CLI",
		Version:          version.Version,
		TraverseChildren: true,
		SilenceUsage:     true,
		SilenceErrors:    true,
		Args:             cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
	root.SetArgs([]string(args))
	root.SetIn(in)
	root.SetOut(out)
	root.SetErr(errOut)
	root.PersistentFlags().StringVar(&flags.defaultConfig, "default-config", "", "default configuration file")
	root.PersistentFlags().StringVar(&flags.configFile, "config", "", "configuration file")
	root.PersistentFlags().StringVar(&flags.privateFile, "private", "", "private configuration file")
	root.PersistentFlags().StringVar(&flags.profileName, "profile", "", "profile name")
	root.PersistentFlags().StringVar(&flags.logLevel, "log-level", "", "log level")
	root.PersistentFlags().BoolVarP(&flags.verbose, "verbose", "v", false, "enable debug logging")
	root.PersistentFlags().StringVar(&flags.outputFormat, "output", "", "output format")
	if err := root.PersistentFlags().MarkHidden("default-config"); err != nil {
		logger.Warn().Err(err).Msg("failed to hide default configuration flag")
	}

	root.AddCommand(newVersionCommand(out))
	root.AddCommand(newDocsCommand())
	root.AddCommand(newConfigCommand(flags, logger))
	root.AddCommand(newCheckCommand(flags, logger))
	root.AddCommand(newOrderCommand(flags, logger))
	root.AddCommand(newRawCommand(flags, logger))
	root.AddCommand(newInspectCommand(flags, logger))
	root.AddCommand(newShellCommand(flags, logger))
	root.AddCommand(newScenarioCommand(flags, logger))
	return &RootCommand{Command: root}
}

func newVersionCommand(out io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version",
		RunE: func(_ *cobra.Command, _ []string) error {
			info := version.Current()
			_, err := fmt.Fprintf(out, "version: %s\ncommit: %s\nbuild_time: %s\n", info.Version, info.Commit, info.BuildTime)
			return err
		},
	}
}

func newConfigCommand(flags *flagState, logger zerolog.Logger) *cobra.Command {
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Manage configuration",
	}
	configCmd.AddCommand(newConfigExampleCommand())
	configCmd.AddCommand(&cobra.Command{
		Use:   "validate",
		Short: "Validate configuration",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load(config.LoadOptions{
				DefaultFile:  flags.defaultConfig,
				ConfigFile:   flags.configFile,
				PrivateFile:  flags.privateFile,
				ProfileName:  flags.profileName,
				LogLevel:     flags.effectiveLogLevel(),
				OutputFormat: flags.outputFormat,
			})
			if err != nil {
				logger.Error().Err(err).Msg("failed to load configuration")
				return err
			}
			if err := validate.AppConfig(cfg); err != nil {
				logger.Error().Err(err).Msg("configuration validation failed")
				return err
			}
			configuredLogger, err := logging.New(cmd.ErrOrStderr(), cfg.Log)
			if err != nil {
				logger.Error().Err(err).Msg("failed to configure logger")
				return err
			}
			logLoadedConfigFiles(configuredLogger, cfg)
			if cfg.Profile.TLS.Enabled && cfg.Profile.TLS.InsecureSkipVerify {
				configuredLogger.Warn().Msg("tls certificate verification is disabled")
			}
			configuredLogger.Info().Str("profile", cfg.Profile.Name).Msg("configuration is valid")
			_, err = fmt.Fprintln(cmd.OutOrStdout(), "configuration is valid")
			return err
		},
	})
	return configCmd
}

func logLoadedConfigFiles(logger zerolog.Logger, cfg *config.AppConfig) {
	if cfg == nil {
		return
	}
	logger.Info().Str("source", cfg.DefaultSource).Msg("configuration defaults loaded")
	logger.Info().Strs("config_files", cfg.LoadedFiles).Msg("configuration files loaded")
	for _, key := range cfg.DeprecatedKeys {
		if key == "profile.custom_tags" {
			logger.Warn().Str("deprecated_key", key).Str("replacement", "profile.custom_field_defs").Msg("configuration key is deprecated")
		}
	}
}

func newConfigExampleCommand() *cobra.Command {
	var output string
	var force bool
	command := &cobra.Command{
		Use:   "example",
		Short: "Generate example configuration",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := writeConfigExample(output, force); err != nil {
				return err
			}
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "config example written to %s\n", output)
			return err
		},
	}
	command.Flags().StringVarP(&output, "output", "o", configExampleFile, "output file")
	command.Flags().BoolVar(&force, "force", false, "overwrite existing output file")
	return command
}

const configExampleFile = "config-example.toml"

func writeConfigExample(output string, force bool) error {
	if output == "" {
		return fmt.Errorf("output file is required")
	}
	data := []byte(fixtool.ConfigExampleTOML())
	if force {
		if err := os.WriteFile(output, data, 0644); err != nil {
			return fmt.Errorf("write config example %s: %w", output, err)
		}
		return nil
	}
	file, err := os.OpenFile(output, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return fmt.Errorf("config example %s already exists, use --force to overwrite", output)
		}
		return fmt.Errorf("create config example %s: %w", output, err)
	}
	defer func() {
		_ = file.Close()
	}()
	if _, err := file.Write(data); err != nil {
		return fmt.Errorf("write config example %s: %w", output, err)
	}
	return nil
}
