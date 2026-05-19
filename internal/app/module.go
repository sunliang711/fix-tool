package app

import (
	"context"
	"fmt"
	"os"
	"time"

	"fix-tool/internal/cli"
	"fix-tool/internal/logging"

	"github.com/rs/zerolog"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
)

const shutdownTimeout = 15 * time.Second

var Module = fx.Options(
	fx.Provide(NewLogger),
	fx.Provide(cli.NewRootCommand),
)

func Run(ctx context.Context, args []string) int {
	exitCode := 0
	fxApp := fx.New(
		Module,
		fx.Supply(cli.Args(args)),
		fx.Supply(cli.IO{
			Out:    os.Stdout,
			ErrOut: os.Stderr,
		}),
		fx.Invoke(func(lc fx.Lifecycle, command *cli.RootCommand) {
			lc.Append(fx.Hook{
				OnStart: func(startCtx context.Context) error {
					if err := command.ExecuteContext(startCtx); err != nil {
						exitCode = 1
						_, _ = fmt.Fprintln(command.ErrOrStderr(), err)
						return err
					}
					return nil
				},
			})
		}),
		fx.WithLogger(func() fxevent.Logger {
			return fxevent.NopLogger
		}),
	)

	if err := fxApp.Start(ctx); err != nil {
		return 1
	}
	stopCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := fxApp.Stop(stopCtx); err != nil {
		return 1
	}
	return exitCode
}

func NewLogger(io cli.IO) zerolog.Logger {
	return logging.NewDefault(io.ErrOut)
}
