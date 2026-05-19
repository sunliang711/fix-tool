package fixsession

import (
	"context"
	"errors"
	"testing"

	"github.com/quickfixgo/quickfix"
	"github.com/rs/zerolog"
	"go.uber.org/fx"
)

func TestManagerStartStopUsesInitiator(t *testing.T) {
	profile := validProfile()
	fake := &fakeInitiator{}
	app := NewApplication(profile)
	manager := newManagerWithInitiator(profile, zerolog.Nop(), app, quickfix.SessionID{
		BeginString:  profile.BeginString,
		SenderCompID: profile.SenderCompID,
		TargetCompID: profile.TargetCompID,
	}, fake)

	if err := manager.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if err := manager.Start(context.Background()); err != nil {
		t.Fatalf("second Start() error = %v", err)
	}
	if fake.starts != 1 {
		t.Fatalf("starts = %d, want 1", fake.starts)
	}

	if err := manager.Stop(context.Background()); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	if err := manager.Stop(context.Background()); err != nil {
		t.Fatalf("second Stop() error = %v", err)
	}
	if fake.stops != 1 {
		t.Fatalf("stops = %d, want 1", fake.stops)
	}
}

func TestManagerStartReturnsInitiatorError(t *testing.T) {
	profile := validProfile()
	want := errors.New("boom")
	manager := newManagerWithInitiator(profile, zerolog.Nop(), NewApplication(profile), quickfix.SessionID{}, &fakeInitiator{
		startErr: want,
	})

	err := manager.Start(context.Background())
	if err == nil {
		t.Fatal("Start() error = nil, want error")
	}
	if !errors.Is(err, want) {
		t.Fatalf("Start() error = %v, want %v", err, want)
	}
}

func TestRegisterLifecycleUsesManager(t *testing.T) {
	lifecycle := &fakeLifecycle{}
	manager := &fakeManager{}

	RegisterLifecycle(lifecycle, manager)
	if len(lifecycle.hooks) != 1 {
		t.Fatalf("hooks = %d, want 1", len(lifecycle.hooks))
	}
	if err := lifecycle.hooks[0].OnStart(context.Background()); err != nil {
		t.Fatalf("OnStart() error = %v", err)
	}
	if err := lifecycle.hooks[0].OnStop(context.Background()); err != nil {
		t.Fatalf("OnStop() error = %v", err)
	}
	if manager.starts != 1 {
		t.Fatalf("starts = %d, want 1", manager.starts)
	}
	if manager.stops != 1 {
		t.Fatalf("stops = %d, want 1", manager.stops)
	}
}

type fakeInitiator struct {
	startErr error
	starts   int
	stops    int
}

func (i *fakeInitiator) Start() error {
	i.starts++
	return i.startErr
}

func (i *fakeInitiator) Stop() {
	i.stops++
}

type fakeLifecycle struct {
	hooks []fx.Hook
}

func (l *fakeLifecycle) Append(hook fx.Hook) {
	l.hooks = append(l.hooks, hook)
}

type fakeManager struct {
	starts int
	stops  int
}

func (m *fakeManager) Start(context.Context) error {
	m.starts++
	return nil
}

func (m *fakeManager) Stop(context.Context) error {
	m.stops++
	return nil
}

func (m *fakeManager) Events() <-chan Event {
	return nil
}

func (m *fakeManager) Session() Session {
	return nil
}
