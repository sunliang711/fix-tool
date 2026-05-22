package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"fix-tool/internal/admin"
	"fix-tool/internal/config"
	"fix-tool/internal/dictionary"
	"fix-tool/internal/fixsession"
	"fix-tool/internal/logging"
	"fix-tool/internal/order"
	rawsvc "fix-tool/internal/raw"
	"fix-tool/internal/render"
	"fix-tool/internal/scenario"
	toolshell "fix-tool/internal/shell"
	"fix-tool/internal/trace"
	"fix-tool/internal/validate"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
)

type scenarioRunner struct {
	flags  *flagState
	logger zerolog.Logger
}

type scenarioRunFlags struct {
	jsonOutput bool
	resultFile string
}

func newScenarioCommand(flags *flagState, logger zerolog.Logger) *cobra.Command {
	runner := scenarioRunner{flags: flags, logger: logger}
	runFlags := &scenarioRunFlags{}
	runCmd := &cobra.Command{
		Use:   "run scenario.yaml",
		Short: "Run FIX scenario steps",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runner.run(cmd, args[0], runFlags)
		},
	}
	runCmd.Flags().BoolVar(&runFlags.jsonOutput, "json", false, "write scenario result as JSON")
	runCmd.Flags().StringVar(&runFlags.resultFile, "result-file", "", "write scenario result JSON to file")
	runCmd.Flags().StringVar(&runFlags.resultFile, "output-file", "", "write scenario result JSON to file")
	return runCmd
}

func (r scenarioRunner) run(cmd *cobra.Command, scenarioFile string, runFlags *scenarioRunFlags) error {
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
	configuredLogger, err := logging.New(cmd.ErrOrStderr(), cfg.Log)
	if err != nil {
		r.logger.Error().Err(err).Msg("failed to configure logger")
		return err
	}
	logStartupConfiguration(configuredLogger, cfg)
	loadedScenario, err := scenario.Load(scenarioFile)
	if err != nil {
		configuredLogger.Error().Err(err).Msg("failed to load scenario")
		return err
	}
	manager, err := fixsession.NewManagerWithOptions(cfg.Profile, configuredLogger, fixsession.ManagerOptions{
		MessageOutput: cmd.OutOrStdout(),
	})
	if err != nil {
		configuredLogger.Error().Err(err).Msg("failed to create fix session manager")
		return err
	}
	state := toolshell.NewSessionState()
	runner := scenario.NewRunner(scenario.Options{
		Admin:   admin.NewService(manager, admin.Options{KeepSession: true, SessionState: state}),
		Order:   order.NewService(manager, order.Options{KeepSession: true, SessionState: state}),
		Raw:     rawsvc.NewService(manager, rawsvc.Options{KeepSession: true, SessionState: state}),
		Manager: manager,
	})
	result, runErr := runner.Run(cmd.Context(), loadedScenario)

	if runFlags.resultFile != "" {
		if err := writeScenarioJSONFile(runFlags.resultFile, result); err != nil {
			return err
		}
	}
	if runFlags.jsonOutput {
		if err := writeScenarioJSON(cmd.OutOrStdout(), result); err != nil {
			return err
		}
	} else {
		if err := renderScenarioResult(cmd.Context(), cmd.OutOrStdout(), cfg, result); err != nil {
			return err
		}
	}
	if runErr != nil {
		configuredLogger.Error().Err(runErr).Msg("scenario command failed")
		return runErr
	}
	return nil
}

func writeScenarioJSONFile(path string, result scenario.Result) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create scenario result file %s: %w", path, err)
	}
	defer func() {
		_ = file.Close()
	}()
	if err := writeScenarioJSON(file, result); err != nil {
		return fmt.Errorf("write scenario result file %s: %w", path, err)
	}
	return nil
}

func writeScenarioJSON(out io.Writer, result scenario.Result) error {
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}

func renderScenarioResult(ctx context.Context, out io.Writer, cfg *config.AppConfig, result scenario.Result) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	renderer := render.NewRenderer(dictionary.NewFromConfig(cfg.Profile.CustomFieldDefs), render.Options{
		Format:        render.Format(cfg.Output.Format),
		RawDelimiter:  cfg.Output.RawDelimiter,
		ShowSensitive: !cfg.Output.RedactSensitive,
	})
	if _, err := fmt.Fprintf(out, "Scenario %s: %s\n", result.Scenario, result.Status); err != nil {
		return err
	}
	for _, step := range result.Steps {
		if _, err := fmt.Fprintf(out, "Step %d %s (%s): %s\n", step.Index, step.Name, step.Action, step.Status); err != nil {
			return err
		}
		if step.Error != "" {
			if _, err := fmt.Fprintf(out, "Error: %s\n", step.Error); err != nil {
				return err
			}
		}
		for _, assertion := range step.Assertions {
			if err := renderAssertion(out, step.Name, assertion); err != nil {
				return err
			}
		}
		for i, message := range step.Traces {
			title := fmt.Sprintf("Step %d Trace %d", step.Index, i+1)
			if err := renderScenarioTrace(out, renderer, render.Format(cfg.Output.Format), title, message); err != nil {
				return err
			}
		}
	}
	return nil
}

func renderAssertion(out io.Writer, stepName string, assertion scenario.AssertionResult) error {
	status := scenario.StatusPassed
	if !assertion.Passed {
		status = scenario.StatusFailed
	}
	expected := assertion.Expected
	if len(assertion.ExpectedValues) > 0 {
		data, err := json.Marshal(assertion.ExpectedValues)
		if err != nil {
			return err
		}
		expected = string(data)
	}
	_, err := fmt.Fprintf(
		out,
		"Assert %s field=%s operator=%s expected=%s actual=%s status=%s\n",
		stepName,
		assertion.Field,
		assertion.Operator,
		expected,
		assertion.Actual,
		status,
	)
	return err
}

func renderScenarioTrace(out io.Writer, renderer *render.Renderer, format render.Format, title string, message trace.MessageTrace) error {
	rendered, err := renderer.Render(message, format)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(out, "%s\n%s\n", title, rendered)
	return err
}
