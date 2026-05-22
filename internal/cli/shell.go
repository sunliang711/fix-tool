package cli

import (
	"fmt"
	"os"

	"fix-tool/internal/admin"
	"fix-tool/internal/config"
	"fix-tool/internal/dictionary"
	"fix-tool/internal/fixsession"
	"fix-tool/internal/logging"
	"fix-tool/internal/order"
	"fix-tool/internal/render"
	toolshell "fix-tool/internal/shell"
	"fix-tool/internal/validate"

	"github.com/mattn/go-isatty"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
)

type shellRunner struct {
	flags  *flagState
	logger zerolog.Logger
}

func newShellCommand(flags *flagState, logger zerolog.Logger) *cobra.Command {
	runner := shellRunner{flags: flags, logger: logger}
	return &cobra.Command{
		Use:   "shell",
		Short: "Start interactive FIX shell",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runner.run(cmd)
		},
	}
}

func (r shellRunner) run(cmd *cobra.Command) error {
	cfg, err := config.Load(config.LoadOptions{
		DefaultFile:  r.flags.defaultConfig,
		ConfigFile:   r.flags.configFile,
		PrivateFile:  r.flags.privateFile,
		ProfileName:  r.flags.profileName,
		LogLevel:     r.flags.effectiveLogLevel(),
		OutputFormat: r.flags.outputFormat,
	})
	if err != nil {
		r.logger.Error().Err(err).Msg("failed to load configuration")
		return err
	}
	if err := validate.AppConfig(cfg); err != nil {
		r.logger.Error().Err(err).Msg("configuration validation failed")
		return err
	}
	if !isTerminal(cmd.InOrStdin()) || !isTerminal(cmd.OutOrStdout()) {
		return fmt.Errorf("shell requires an interactive terminal")
	}
	prompt := "fix-tool> "
	lineReader := toolshell.NewTUILineReader()
	tuiOutput := toolshell.NewTUIOutputWriter()
	transcript := toolshell.NewTranscriptRecorder(prompt)
	out := transcript.Wrap(tuiOutput)
	configuredLogger, err := logging.NewWithOptions(out, cfg.Log, logging.Options{NoColor: true})
	if err != nil {
		r.logger.Error().Err(err).Msg("failed to configure logger")
		return err
	}
	logStartupConfiguration(configuredLogger, cfg)
	manager, err := fixsession.NewManagerWithOptions(cfg.Profile, configuredLogger, fixsession.ManagerOptions{
		MessageOutput: out,
	})
	if err != nil {
		configuredLogger.Error().Err(err).Msg("failed to create fix session manager")
		return err
	}
	state := toolshell.NewSessionState()
	renderer := render.NewRenderer(dictionary.NewFromConfig(cfg.Profile.CustomFieldDefs), render.Options{
		Format:        render.Format(cfg.Output.Format),
		RawDelimiter:  cfg.Output.RawDelimiter,
		ShowSensitive: !cfg.Output.RedactSensitive,
	})
	runner := toolshell.NewRunner(toolshell.Options{
		In:         cmd.InOrStdin(),
		Out:        out,
		ErrOut:     out,
		LineReader: lineReader,
		Admin:      admin.NewService(manager, admin.Options{KeepSession: true, SessionState: state}),
		Order:      order.NewService(manager, order.Options{KeepSession: true, SessionState: state}),
		Manager:    manager,
		Renderer:   renderer,
		Transcript: transcript,
		Format:     render.Format(cfg.Output.Format),
		Prompt:     prompt,
	})
	if err := toolshell.RunTUI(cmd.Context(), toolshell.TUIOptions{
		In:         cmd.InOrStdin(),
		Out:        cmd.OutOrStdout(),
		Prompt:     prompt,
		Runner:     runner,
		LineReader: lineReader,
		Output:     tuiOutput,
	}); err != nil {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "shell command failed: %v\n", err)
		return err
	}
	return nil
}

func isTerminal(value any) bool {
	file, ok := value.(*os.File)
	return ok && isatty.IsTerminal(file.Fd())
}
