package mockfix_test

import (
	"context"
	"testing"
	"time"

	"fix-tool/internal/fixsession"
	"fix-tool/internal/mockfix"
	rawsvc "fix-tool/internal/raw"
	toolshell "fix-tool/internal/shell"

	"github.com/rs/zerolog"
)

func TestRawServiceSendsMessageAgainstMockAcceptor(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	server := startMockAcceptor(t, mockfix.Options{})
	manager := newRawManager(t, server)
	defer stopManager(t, manager)
	state := toolshell.NewSessionState()
	service := rawsvc.NewService(manager, rawsvc.Options{Timeout: testCommandTimeout, KeepSession: true, SessionState: state})

	result, err := service.Send(ctx, rawsvc.Request{
		MsgType: "D",
		Tags: []string{
			"11=RAW-001",
			"55=AAPL",
			"54=1",
			"38=100",
			"40=1",
		},
	})
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if result.Request == nil || result.Request.MsgType != "D" {
		t.Fatalf("Send() request = %#v, want MsgType D", result.Request)
	}
	if result.Response == nil || result.Response.MsgType != "8" {
		t.Fatalf("Send() response = %#v, want MsgType 8", result.Response)
	}
	if got := fieldValue(result.Response, 11); got != "RAW-001" {
		t.Fatalf("response ClOrdID = %q, want RAW-001", got)
	}
}

func newRawManager(t *testing.T, server *mockfix.Acceptor) fixsession.Manager {
	t.Helper()
	manager, err := fixsession.NewManager(server.ProfileConfig(), zerolog.Nop())
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	return manager
}
