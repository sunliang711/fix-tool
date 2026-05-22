package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"fix-tool/internal/admin"
	"fix-tool/internal/config"
	"fix-tool/internal/fixsession"
	"fix-tool/internal/logging"
	"fix-tool/internal/validate"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
)

type adminRunner struct {
	flags  *flagState
	logger zerolog.Logger
}

const adminCommandStopTimeout = 5 * time.Second

func newCheckCommand(flags *flagState, logger zerolog.Logger) *cobra.Command {
	runner := adminRunner{flags: flags, logger: logger}
	checkCmd := &cobra.Command{
		Use:   "check",
		Short: "Run one-shot FIX session checks",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
	logonCmd := &cobra.Command{
		Use:   "logon",
		Short: "Check FIX Logon handshake and auto logout",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runner.run(cmd.Context(), cmd.OutOrStdout(), cmd.ErrOrStderr(), "Logon", func(ctx context.Context, service *admin.Service) (admin.Result, error) {
				return service.Logon(ctx)
			})
		},
	}
	logoutCmd := &cobra.Command{
		Use:   "logout",
		Short: "Check FIX Logout handshake after Logon",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runner.run(cmd.Context(), cmd.OutOrStdout(), cmd.ErrOrStderr(), "Logout", func(ctx context.Context, service *admin.Service) (admin.Result, error) {
				return service.Logout(ctx)
			})
		},
	}
	var testRequestID string
	testRequestCmd := &cobra.Command{
		Use:   "test-request",
		Short: "Check FIX TestRequest after Logon",
		RunE: func(cmd *cobra.Command, _ []string) error {
			testRequestID = strings.TrimSpace(testRequestID)
			if testRequestID == "" {
				return admin.ErrTestRequestIDRequired
			}
			return runner.run(cmd.Context(), cmd.OutOrStdout(), cmd.ErrOrStderr(), "TestRequest", func(ctx context.Context, service *admin.Service) (admin.Result, error) {
				return service.TestRequest(ctx, testRequestID)
			})
		},
	}
	testRequestCmd.Flags().StringVar(&testRequestID, "id", "", "test request id")
	if err := testRequestCmd.MarkFlagRequired("id"); err != nil {
		logger.Warn().Err(err).Msg("failed to mark test request id flag required")
	}

	checkCmd.AddCommand(logonCmd, logoutCmd, testRequestCmd)
	return checkCmd
}

func (r adminRunner) run(
	ctx context.Context,
	out io.Writer,
	errOut io.Writer,
	title string,
	operation func(context.Context, *admin.Service) (admin.Result, error),
) (err error) {
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
	configuredLogger, err := logging.New(errOut, cfg.Log)
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
	configuredLogger.Info().
		Str("command", "check "+strings.ToLower(title)).
		Str("lifecycle", "start session -> logon -> command -> stop session").
		Msg("running one-shot check command")
	defer func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), adminCommandStopTimeout)
		defer cancel()
		if stopErr := manager.Stop(stopCtx); stopErr != nil && err == nil {
			err = fmt.Errorf("stop fix session: %w", stopErr)
		}
	}()
	service := admin.NewService(manager, admin.Options{KeepSession: true})
	_, err = operation(ctx, service)
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
	return nil
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
