package cli

import (
	"context"
	"fmt"
	"io"
	"time"

	"fix-tool/internal/config"
	"fix-tool/internal/dictionary"
	"fix-tool/internal/fixsession"
	"fix-tool/internal/logging"
	rawsvc "fix-tool/internal/raw"
	"fix-tool/internal/render"
	"fix-tool/internal/trace"
	"fix-tool/internal/validate"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
)

type rawRunner struct {
	flags  *flagState
	logger zerolog.Logger
}

type inspectRunner struct {
	flags  *flagState
	logger zerolog.Logger
}

func newRawCommand(flags *flagState, logger zerolog.Logger) *cobra.Command {
	runner := rawRunner{flags: flags, logger: logger}
	request := rawsvc.Request{}
	rawCmd := &cobra.Command{
		Use:   "raw",
		Short: "Send raw FIX messages",
	}
	sendCmd := &cobra.Command{
		Use:   "send",
		Short: "Send a raw FIX message built from msg type and tags",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runner.runSend(cmd.Context(), cmd.OutOrStdout(), cmd.ErrOrStderr(), request)
		},
	}
	sendCmd.Flags().StringVar(&request.MsgType, "msg-type", "", "FIX MsgType")
	sendCmd.Flags().StringArrayVar(&request.Tags, "tag", nil, "FIX body tag as key=value")
	rawCmd.AddCommand(sendCmd)
	return rawCmd
}

func newInspectCommand(flags *flagState, logger zerolog.Logger) *cobra.Command {
	runner := inspectRunner{flags: flags, logger: logger}
	inspectCmd := &cobra.Command{
		Use:   "inspect",
		Short: "Inspect FIX messages offline",
	}
	rawCmd := &cobra.Command{
		Use:   "raw message",
		Short: "Inspect a raw FIX message",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runner.runRaw(cmd.OutOrStdout(), args[0])
		},
	}
	inspectCmd.AddCommand(rawCmd)
	return inspectCmd
}

func (r rawRunner) runSend(ctx context.Context, out io.Writer, errOut io.Writer, request rawsvc.Request) error {
	msgType, tags, err := rawsvc.ValidateRequest(request)
	if err != nil {
		return err
	}
	if err := renderRawRisk(errOut, msgType, len(tags)); err != nil {
		return err
	}

	cfg, err := config.Load(config.LoadOptions{
		DefaultFile:  r.flags.defaultConfig,
		ConfigFile:   r.flags.configFile,
		PrivateFile:  r.flags.privateFile,
		ProfileName:  r.flags.profileName,
		LogLevel:     r.flags.logLevel,
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
	configuredLogger, err := logging.New(errOut, cfg.Log)
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
	service := rawsvc.NewService(manager, rawsvc.Options{})
	result, err := service.Send(ctx, request)
	if err != nil {
		configuredLogger.Error().Err(err).Msg("raw command failed")
		return err
	}
	return renderRawResult(out, cfg, result)
}

func (r inspectRunner) runRaw(out io.Writer, rawMessage string) error {
	cfg, err := config.Load(config.LoadOptions{
		DefaultFile:  r.flags.defaultConfig,
		ConfigFile:   r.flags.configFile,
		PrivateFile:  r.flags.privateFile,
		ProfileName:  r.flags.profileName,
		LogLevel:     r.flags.logLevel,
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
	messageTrace, err := trace.NewMessageTrace(trace.BuildOptions{
		TraceID:   fmt.Sprintf("inspect-raw-%d", time.Now().UTC().UnixNano()),
		Profile:   cfg.Profile.Name,
		Direction: trace.DirectionInbound,
		Raw:       rawMessage,
	})
	if err != nil {
		return fmt.Errorf("raw FIX 报文解析失败: %w", err)
	}
	renderer := newRenderer(cfg)
	rendered, err := renderer.Render(messageTrace, render.Format(cfg.Output.Format))
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(out, rendered)
	return err
}

func renderRawRisk(out io.Writer, msgType string, tagCount int) error {
	if _, err := fmt.Fprintln(out, "风险提示：raw send 会绕过业务参数校验，请确认 tag 与对端协议兼容。"); err != nil {
		return err
	}
	_, err := fmt.Fprintf(out, "校验结果：MsgType=%s BodyTags=%d ProtectedTags=refused\n", msgType, tagCount)
	return err
}

func renderRawResult(out io.Writer, cfg *config.AppConfig, result rawsvc.Result) error {
	renderer := newRenderer(cfg)
	if result.Request != nil {
		if err := renderRawTrace(out, renderer, render.Format(cfg.Output.Format), "Request", *result.Request); err != nil {
			return err
		}
	}
	if result.Response != nil {
		if err := renderRawTrace(out, renderer, render.Format(cfg.Output.Format), "Response", *result.Response); err != nil {
			return err
		}
	}
	return nil
}

func renderRawTrace(out io.Writer, renderer *render.Renderer, format render.Format, title string, message trace.MessageTrace) error {
	rendered, err := renderer.Render(message, format)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(out, "%s\n%s\n", title, rendered)
	return err
}

func newRenderer(cfg *config.AppConfig) *render.Renderer {
	return render.NewRenderer(dictionary.NewFromConfig(cfg.Profile.CustomFieldDefs), render.Options{
		Format:        render.Format(cfg.Output.Format),
		RawDelimiter:  cfg.Output.RawDelimiter,
		ShowSensitive: !cfg.Output.RedactSensitive,
	})
}
