package cli

import (
	"io"
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
	prompt := "fix-tool> "
	transcript := toolshell.NewTranscriptRecorder(prompt)
	out := transcript.Wrap(cmd.OutOrStdout())
	errOut := transcript.Wrap(cmd.ErrOrStderr())
	promptLogWriter := toolshell.NewPromptLogWriter(errOut, out, prompt)
	lineReader := newShellLineReader(cmd.InOrStdin(), cmd.OutOrStdout(), promptLogWriter)
	configuredLogger, err := logging.New(promptLogWriter, cfg.Log)
	if err != nil {
		r.logger.Error().Err(err).Msg("failed to configure logger")
		return err
	}
	logLoadedConfigFiles(configuredLogger, cfg)
	manager, err := fixsession.NewManager(cfg.Profile, configuredLogger)
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
		ErrOut:     errOut,
		LineReader: lineReader,
		Admin:      admin.NewService(manager, admin.Options{KeepSession: true, SessionState: state}),
		Order:      order.NewService(manager, order.Options{KeepSession: true, SessionState: state}),
		Manager:    manager,
		Renderer:   renderer,
		Transcript: transcript,
		Format:     render.Format(cfg.Output.Format),
		Prompt:     prompt,
		PromptCtl:  promptLogWriter,
	})
	if err := runner.Run(cmd.Context()); err != nil {
		configuredLogger.Error().Err(err).Msg("shell command failed")
		return err
	}
	return nil
}

func newShellLineReader(in io.Reader, out io.Writer, promptCtl toolshell.PromptController) toolshell.LineReader {
	if !isTerminal(in) || !isTerminal(out) {
		return nil
	}
	return toolshell.NewHistoryLineReader(promptCtl)
}

func isTerminal(value any) bool {
	file, ok := value.(*os.File)
	return ok && isatty.IsTerminal(file.Fd())
}
