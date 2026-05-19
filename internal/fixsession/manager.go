package fixsession

import (
	"context"
	"fmt"
	"sync"

	"fix-tool/internal/config"

	"github.com/quickfixgo/quickfix"
	"github.com/rs/zerolog"
	"go.uber.org/fx"
)

type initiator interface {
	Start() error
	Stop()
}

type QuickFIXManager struct {
	mu        sync.Mutex
	profile   config.ProfileConfig
	logger    zerolog.Logger
	app       Application
	session   session
	initiator initiator
	started   bool
}

var Module = fx.Options(
	fx.Provide(fx.Annotate(NewManager, fx.As(new(Manager)))),
	fx.Invoke(RegisterLifecycle),
)

func NewManager(profile config.ProfileConfig, logger zerolog.Logger) (*QuickFIXManager, error) {
	settings, sessionID, err := SettingsFromProfile(profile)
	if err != nil {
		return nil, err
	}
	app := newApplication(profile, make(chan Event, defaultEventBuffer))
	initiator, err := quickfix.NewInitiator(app, quickfix.NewMemoryStoreFactory(), settings, newZerologLogFactory(logger, profile))
	if err != nil {
		return nil, fmt.Errorf("create quickfix initiator: %w", err)
	}
	if profile.TLS.Enabled && profile.TLS.InsecureSkipVerify {
		logger.Warn().Str("profile", profile.Name).Msg("tls certificate verification is disabled")
	}
	return newManagerWithInitiator(profile, logger, app, sessionID, initiator), nil
}

func RegisterLifecycle(lc fx.Lifecycle, manager Manager) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			return manager.Start(ctx)
		},
		OnStop: func(ctx context.Context) error {
			return manager.Stop(ctx)
		},
	})
}

func newManagerWithInitiator(
	profile config.ProfileConfig,
	logger zerolog.Logger,
	app Application,
	sessionID quickfix.SessionID,
	initiator initiator,
) *QuickFIXManager {
	return &QuickFIXManager{
		profile: profile,
		logger:  logger,
		app:     app,
		session: session{
			id:          sessionID,
			profileName: profile.Name,
		},
		initiator: initiator,
	}
}

func (m *QuickFIXManager) Start(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("start fix session: %w", err)
	}

	m.mu.Lock()
	if m.started {
		m.mu.Unlock()
		return nil
	}
	if err := m.initiator.Start(); err != nil {
		m.mu.Unlock()
		return fmt.Errorf("start quickfix initiator: %w", err)
	}
	m.started = true
	m.mu.Unlock()

	m.logger.Info().
		Str("profile", m.profile.Name).
		Str("session", m.session.ID().String()).
		Msg("fix session manager started")
	return nil
}

func (m *QuickFIXManager) Stop(ctx context.Context) error {
	m.mu.Lock()
	if !m.started {
		m.mu.Unlock()
		return nil
	}
	m.mu.Unlock()

	done := make(chan struct{})
	go func() {
		m.initiator.Stop()
		close(done)
	}()

	select {
	case <-done:
	case <-ctx.Done():
		return fmt.Errorf("stop fix session: %w", ctx.Err())
	}

	m.mu.Lock()
	m.started = false
	m.mu.Unlock()
	m.logger.Info().
		Str("profile", m.profile.Name).
		Str("session", m.session.ID().String()).
		Msg("fix session manager stopped")
	return nil
}

func (m *QuickFIXManager) Events() <-chan Event {
	return m.app.Events()
}

func (m *QuickFIXManager) Session() Session {
	return m.session
}
