package cli

import (
	"fmt"
	"io"
	"os"

	"fix-tool/internal/config"
	"fix-tool/internal/logging"
	"fix-tool/internal/validate"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
)

const Version = "dev"

type RootCommand struct {
	*cobra.Command
}

type flagState struct {
	defaultConfig string
	configFile    string
	privateFile   string
	profileName   string
	logLevel      string
	outputFormat  string
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
	flags := &flagState{}
	root := &cobra.Command{
		Use:           "fix-tool",
		Short:         "FIX protocol testing CLI",
		Version:       Version,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
	root.SetArgs([]string(args))
	root.SetOut(out)
	root.SetErr(errOut)
	root.PersistentFlags().StringVar(&flags.defaultConfig, "default-config", "", "default configuration file")
	root.PersistentFlags().StringVar(&flags.configFile, "config", "", "configuration file")
	root.PersistentFlags().StringVar(&flags.privateFile, "private", "", "private configuration file")
	root.PersistentFlags().StringVar(&flags.profileName, "profile", "", "profile name")
	root.PersistentFlags().StringVar(&flags.logLevel, "log-level", "", "log level")
	root.PersistentFlags().StringVar(&flags.outputFormat, "output", "", "output format")
	if err := root.PersistentFlags().MarkHidden("default-config"); err != nil {
		logger.Warn().Err(err).Msg("failed to hide default configuration flag")
	}

	root.AddCommand(newVersionCommand(out))
	root.AddCommand(newConfigCommand(flags, logger))
	root.AddCommand(newAdminCommands(flags, logger)...)
	return &RootCommand{Command: root}
}

func newVersionCommand(out io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version",
		RunE: func(_ *cobra.Command, _ []string) error {
			_, err := fmt.Fprintf(out, "fix-tool %s\n", Version)
			return err
		},
	}
}

func newConfigCommand(flags *flagState, logger zerolog.Logger) *cobra.Command {
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Manage configuration",
	}
	configCmd.AddCommand(&cobra.Command{
		Use:   "validate",
		Short: "Validate configuration",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load(config.LoadOptions{
				DefaultFile:  flags.defaultConfig,
				ConfigFile:   flags.configFile,
				PrivateFile:  flags.privateFile,
				ProfileName:  flags.profileName,
				LogLevel:     flags.logLevel,
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
