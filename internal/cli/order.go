package cli

import (
	"context"
	"fmt"
	"io"

	"fix-tool/internal/config"
	"fix-tool/internal/dictionary"
	"fix-tool/internal/fixsession"
	"fix-tool/internal/logging"
	"fix-tool/internal/order"
	"fix-tool/internal/render"
	"fix-tool/internal/trace"
	"fix-tool/internal/validate"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
)

type orderRunner struct {
	flags  *flagState
	logger zerolog.Logger
}

func newOrderCommand(flags *flagState, logger zerolog.Logger) *cobra.Command {
	runner := orderRunner{flags: flags, logger: logger}
	orderCmd := &cobra.Command{
		Use:   "order",
		Short: "Send FIX order messages",
	}

	newRequest := order.NewRequest{}
	newCmd := &cobra.Command{
		Use:   "new",
		Short: "Send NewOrderSingle",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runner.run(cmd.Context(), cmd.OutOrStdout(), cmd.ErrOrStderr(), func(ctx context.Context, service *order.Service) (order.Result, error) {
				return service.NewOrder(ctx, newRequest)
			})
		},
	}
	newCmd.Flags().StringVar(&newRequest.ClOrdID, "cl-ord-id", "", "client order id")
	newCmd.Flags().StringVar(&newRequest.Symbol, "symbol", "", "symbol")
	newCmd.Flags().StringVar(&newRequest.Side, "side", "", "side: buy/sell or 1/2")
	newCmd.Flags().StringVar(&newRequest.OrderQty, "qty", "", "order quantity")
	newCmd.Flags().StringVar(&newRequest.Price, "price", "", "price")
	newCmd.Flags().StringVar(&newRequest.OrdType, "ord-type", "", "order type: market/limit or 1/2")
	newCmd.Flags().StringVar(&newRequest.TimeInForce, "time-in-force", "", "time in force: day/gtc/ioc/fok or 0/1/3/4")
	newCmd.Flags().StringArrayVar(&newRequest.Tags, "tag", nil, "custom FIX body tag as key=value")

	cancelRequest := order.CancelRequest{}
	cancelCmd := &cobra.Command{
		Use:   "cancel",
		Short: "Send OrderCancelRequest",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runner.run(cmd.Context(), cmd.OutOrStdout(), cmd.ErrOrStderr(), func(ctx context.Context, service *order.Service) (order.Result, error) {
				return service.CancelOrder(ctx, cancelRequest)
			})
		},
	}
	cancelCmd.Flags().StringVar(&cancelRequest.OrigClOrdID, "orig-cl-ord-id", "", "original client order id")
	cancelCmd.Flags().StringVar(&cancelRequest.ClOrdID, "cl-ord-id", "", "client order id")
	cancelCmd.Flags().StringVar(&cancelRequest.OrderID, "order-id", "", "order id")
	cancelCmd.Flags().StringVar(&cancelRequest.Symbol, "symbol", "", "symbol")
	cancelCmd.Flags().StringVar(&cancelRequest.Side, "side", "", "side: buy/sell or 1/2")
	cancelCmd.Flags().StringArrayVar(&cancelRequest.Tags, "tag", nil, "custom FIX body tag as key=value")

	replaceRequest := order.ReplaceRequest{}
	replaceCmd := &cobra.Command{
		Use:   "replace",
		Short: "Send OrderCancelReplaceRequest",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runner.run(cmd.Context(), cmd.OutOrStdout(), cmd.ErrOrStderr(), func(ctx context.Context, service *order.Service) (order.Result, error) {
				return service.ReplaceOrder(ctx, replaceRequest)
			})
		},
	}
	replaceCmd.Flags().StringVar(&replaceRequest.OrigClOrdID, "orig-cl-ord-id", "", "original client order id")
	replaceCmd.Flags().StringVar(&replaceRequest.ClOrdID, "cl-ord-id", "", "client order id")
	replaceCmd.Flags().StringVar(&replaceRequest.OrderID, "order-id", "", "order id")
	replaceCmd.Flags().StringVar(&replaceRequest.Symbol, "symbol", "", "symbol")
	replaceCmd.Flags().StringVar(&replaceRequest.Side, "side", "", "side: buy/sell or 1/2")
	replaceCmd.Flags().StringVar(&replaceRequest.OrderQty, "qty", "", "order quantity")
	replaceCmd.Flags().StringVar(&replaceRequest.Price, "price", "", "price")
	replaceCmd.Flags().StringVar(&replaceRequest.OrdType, "ord-type", "", "order type: market/limit or 1/2")
	replaceCmd.Flags().StringVar(&replaceRequest.TimeInForce, "time-in-force", "", "time in force: day/gtc/ioc/fok or 0/1/3/4")
	replaceCmd.Flags().StringArrayVar(&replaceRequest.Tags, "tag", nil, "custom FIX body tag as key=value")

	orderCmd.AddCommand(newCmd, cancelCmd, replaceCmd)
	return orderCmd
}

func (r orderRunner) run(
	ctx context.Context,
	out io.Writer,
	errOut io.Writer,
	operation func(context.Context, *order.Service) (order.Result, error),
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
	service := order.NewService(manager, order.Options{})
	result, err := operation(ctx, service)
	if err != nil {
		configuredLogger.Error().Err(err).Msg("order command failed")
		return err
	}
	return renderOrderResult(out, cfg, result)
}

func renderOrderResult(out io.Writer, cfg *config.AppConfig, result order.Result) error {
	renderer := render.NewRenderer(dictionary.NewFromConfig(cfg.Profile.CustomFieldDefs), render.Options{
		Format:        render.Format(cfg.Output.Format),
		RawDelimiter:  cfg.Output.RawDelimiter,
		ShowSensitive: !cfg.Output.RedactSensitive,
	})
	if result.Request != nil {
		if err := renderOrderTrace(out, renderer, render.Format(cfg.Output.Format), "Request", *result.Request); err != nil {
			return err
		}
	}
	if result.Response != nil {
		if err := renderOrderTrace(out, renderer, render.Format(cfg.Output.Format), "Response", *result.Response); err != nil {
			return err
		}
	}
	return nil
}

func renderOrderTrace(out io.Writer, renderer *render.Renderer, format render.Format, title string, message trace.MessageTrace) error {
	rendered, err := renderer.Render(message, format)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "%s\n%s\n", title, rendered); err != nil {
		return err
	}
	return nil
}
