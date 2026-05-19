package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"fix-tool/internal/admin"
	"fix-tool/internal/config"
	"fix-tool/internal/dictionary"
	"fix-tool/internal/fixsession"
	"fix-tool/internal/logging"
	"fix-tool/internal/render"
	"fix-tool/internal/trace"
	"fix-tool/internal/validate"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
)

type adminRunner struct {
	flags  *flagState
	logger zerolog.Logger
}

func newAdminCommands(flags *flagState, logger zerolog.Logger) []*cobra.Command {
	runner := adminRunner{flags: flags, logger: logger}
	logonCmd := &cobra.Command{
		Use:   "logon",
		Short: "Start FIX session and wait for Logon",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runner.run(cmd.Context(), cmd.OutOrStdout(), cmd.ErrOrStderr(), func(ctx context.Context, service *admin.Service) (admin.Result, error) {
				return service.Logon(ctx)
			})
		},
	}
	logoutCmd := &cobra.Command{
		Use:   "logout",
		Short: "Send FIX Logout",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runner.run(cmd.Context(), cmd.OutOrStdout(), cmd.ErrOrStderr(), func(ctx context.Context, service *admin.Service) (admin.Result, error) {
				return service.Logout(ctx)
			})
		},
	}
	heartbeatCmd := &cobra.Command{
		Use:   "heartbeat",
		Short: "Send FIX Heartbeat",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runner.run(cmd.Context(), cmd.OutOrStdout(), cmd.ErrOrStderr(), func(ctx context.Context, service *admin.Service) (admin.Result, error) {
				return service.Heartbeat(ctx)
			})
		},
	}

	var testRequestID string
	testRequestCmd := &cobra.Command{
		Use:   "test-request",
		Short: "Send FIX TestRequest",
		RunE: func(cmd *cobra.Command, _ []string) error {
			testRequestID = strings.TrimSpace(testRequestID)
			if testRequestID == "" {
				return admin.ErrTestRequestIDRequired
			}
			return runner.run(cmd.Context(), cmd.OutOrStdout(), cmd.ErrOrStderr(), func(ctx context.Context, service *admin.Service) (admin.Result, error) {
				return service.TestRequest(ctx, testRequestID)
			})
		},
	}
	testRequestCmd.Flags().StringVar(&testRequestID, "id", "", "test request id")
	if err := testRequestCmd.MarkFlagRequired("id"); err != nil {
		logger.Warn().Err(err).Msg("failed to mark test request id flag required")
	}

	return []*cobra.Command{logonCmd, logoutCmd, heartbeatCmd, testRequestCmd}
}

func (r adminRunner) run(
	ctx context.Context,
	out io.Writer,
	errOut io.Writer,
	operation func(context.Context, *admin.Service) (admin.Result, error),
) error {
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
	service := admin.NewService(manager, admin.Options{})
	result, err := operation(ctx, service)
	if err != nil {
		logEvent := configuredLogger.Error().Err(err)
		if diagnostics, ok := admin.LogonDiagnosticsFromError(err); ok {
			logEvent = appendLogonDiagnosticFields(logEvent, cfg, diagnostics)
		} else if errors.Is(err, admin.ErrTimeout) {
			logEvent = appendAdminTargetFields(logEvent, cfg)
		}
		logEvent.Msg("admin command failed")
		return err
	}
	return renderAdminResult(out, cfg, result)
}

func appendAdminTargetFields(event *zerolog.Event, cfg *config.AppConfig) *zerolog.Event {
	return event.
		Str("target_host", cfg.Profile.Host).
		Int("target_port", cfg.Profile.Port)
}

func appendLogonDiagnosticFields(event *zerolog.Event, cfg *config.AppConfig, diagnostics admin.LogonDiagnostics) *zerolog.Event {
	event = appendAdminTargetFields(event, cfg).
		Bool("tls_enabled", cfg.Profile.TLS.Enabled).
		Str("timeout", diagnostics.Timeout.String()).
		Bool("logon_sent", diagnostics.LogonSent).
		Bool("logon_response_seen", diagnostics.LogonResponseSeen)
	if diagnostics.Session != "" {
		event = event.Str("session", diagnostics.Session)
	}
	if diagnostics.LastEvent != "" {
		event = event.Str("last_event", diagnostics.LastEvent)
	}
	if diagnostics.LastAdminMsgType != "" {
		event = event.Str("last_admin_msg_type", diagnostics.LastAdminMsgType)
	}
	if diagnostics.LastAdminText != "" {
		event = event.Str("last_admin_text", diagnostics.LastAdminText)
	}
	if diagnostics.LastAdminRefSeqNum != "" {
		event = event.Str("last_admin_ref_seq_num", diagnostics.LastAdminRefSeqNum)
	}
	if diagnostics.LastAdminSessionRejectCode != "" {
		event = event.Str("last_admin_session_reject_code", diagnostics.LastAdminSessionRejectCode)
	}
	if diagnostics.LastAdminBusinessRejectID != "" {
		event = event.Str("last_admin_business_reject_id", diagnostics.LastAdminBusinessRejectID)
	}
	if diagnostics.LastAdminBusinessRejectCode != "" {
		event = event.Str("last_admin_business_reject_code", diagnostics.LastAdminBusinessRejectCode)
	}
	return event
}

func renderAdminResult(out io.Writer, cfg *config.AppConfig, result admin.Result) error {
	renderer := render.NewRenderer(dictionary.NewFromConfig(cfg.Profile.CustomFieldDefs), render.Options{
		Format:        render.Format(cfg.Output.Format),
		RawDelimiter:  cfg.Output.RawDelimiter,
		ShowSensitive: !cfg.Output.RedactSensitive,
	})
	if result.Request != nil {
		if err := renderAdminTrace(out, renderer, render.Format(cfg.Output.Format), "Request", *result.Request); err != nil {
			return err
		}
	}
	if result.Response != nil {
		if err := renderAdminTrace(out, renderer, render.Format(cfg.Output.Format), "Response", *result.Response); err != nil {
			return err
		}
	}
	return nil
}

func renderAdminTrace(out io.Writer, renderer *render.Renderer, format render.Format, title string, message trace.MessageTrace) error {
	rendered, err := renderer.Render(message, format)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "%s\n%s\n", title, rendered); err != nil {
		return err
	}
	return nil
}
