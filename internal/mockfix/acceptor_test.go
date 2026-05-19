package mockfix_test

import (
	"context"
	"fmt"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"fix-tool/internal/admin"
	"fix-tool/internal/fixsession"
	"fix-tool/internal/message"
	"fix-tool/internal/mockfix"
	"fix-tool/internal/order"
	"fix-tool/internal/scenario"
	toolshell "fix-tool/internal/shell"
	"fix-tool/internal/trace"

	"github.com/rs/zerolog"
)

const testCommandTimeout = 3 * time.Second

var mockSessionCounter uint64

func TestMockAcceptorSupportsAdminAndOrderLifecycle(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	server := startMockAcceptor(t, mockfix.Options{})
	manager := newManager(t, server)
	defer stopManager(t, manager)

	state := toolshell.NewSessionState()
	adminService := admin.NewService(manager, admin.Options{Timeout: testCommandTimeout, KeepSession: true, SessionState: state})
	orderService := order.NewService(manager, order.Options{Timeout: testCommandTimeout, KeepSession: true, SessionState: state})

	logon, err := adminService.Logon(ctx)
	if err != nil {
		t.Fatalf("Logon() error = %v", err)
	}
	if logon.Response == nil || logon.Response.MsgType != "A" {
		t.Fatalf("Logon() response = %#v, want MsgType A", logon.Response)
	}

	heartbeat, err := adminService.Heartbeat(ctx)
	if err != nil {
		t.Fatalf("Heartbeat() error = %v", err)
	}
	if heartbeat.Response == nil || heartbeat.Response.MsgType != "0" {
		t.Fatalf("Heartbeat() response = %#v, want MsgType 0", heartbeat.Response)
	}

	testRequest, err := adminService.TestRequest(ctx, "ping-001")
	if err != nil {
		t.Fatalf("TestRequest() error = %v", err)
	}
	if got := fieldValue(testRequest.Response, 112); got != "ping-001" {
		t.Fatalf("TestRequest() response TestReqID = %q, want ping-001", got)
	}

	newOrder, err := orderService.NewOrder(ctx, order.NewRequest{
		ClOrdID:  "C001",
		Symbol:   "AAPL",
		Side:     "buy",
		OrderQty: "100",
		Price:    "10.25",
		OrdType:  "limit",
	})
	if err != nil {
		t.Fatalf("NewOrder() error = %v", err)
	}
	if got := fieldValue(newOrder.Response, 150); got != "0" {
		t.Fatalf("NewOrder() exec type = %q, want 0", got)
	}

	replace, err := orderService.ReplaceOrder(ctx, order.ReplaceRequest{
		OrigClOrdID: "C001",
		ClOrdID:     "C003",
		Symbol:      "AAPL",
		Side:        "buy",
		OrderQty:    "150",
		Price:       "10.35",
		OrdType:     "limit",
	})
	if err != nil {
		t.Fatalf("ReplaceOrder() error = %v", err)
	}
	if got := fieldValue(replace.Response, 150); got != "5" {
		t.Fatalf("ReplaceOrder() exec type = %q, want 5", got)
	}

	cancelOrder, err := orderService.CancelOrder(ctx, order.CancelRequest{
		OrigClOrdID: "C003",
		ClOrdID:     "C004",
		Symbol:      "AAPL",
		Side:        "buy",
	})
	if err != nil {
		t.Fatalf("CancelOrder() error = %v", err)
	}
	if got := fieldValue(cancelOrder.Response, 150); got != "4" {
		t.Fatalf("CancelOrder() exec type = %q, want 4", got)
	}

	logout, err := adminService.Logout(ctx)
	if err != nil {
		t.Fatalf("Logout() error = %v", err)
	}
	if logout.Response == nil || logout.Response.MsgType != "5" {
		t.Fatalf("Logout() response = %#v, want MsgType 5", logout.Response)
	}
}

func TestMockAcceptorCanReturnRejects(t *testing.T) {
	tests := []struct {
		name    string
		symbol  string
		wantMsg string
	}{
		{name: "session-reject", symbol: mockfix.SymbolSessionReject, wantMsg: message.MsgTypeReject},
		{name: "business-reject", symbol: mockfix.SymbolBusinessReject, wantMsg: message.MsgTypeBusinessMessageReject},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			server := startMockAcceptor(t, mockfix.Options{})
			manager := newManager(t, server)
			defer stopManager(t, manager)
			state := toolshell.NewSessionState()
			adminService := admin.NewService(manager, admin.Options{Timeout: testCommandTimeout, KeepSession: true, SessionState: state})
			orderService := order.NewService(manager, order.Options{Timeout: testCommandTimeout, KeepSession: true, SessionState: state})

			if _, err := adminService.Logon(ctx); err != nil {
				t.Fatalf("Logon() error = %v", err)
			}
			result, err := orderService.NewOrder(ctx, order.NewRequest{
				ClOrdID:  "C-REJECT",
				Symbol:   tt.symbol,
				Side:     "buy",
				OrderQty: "100",
				Price:    "10.25",
			})
			if err != nil {
				t.Fatalf("NewOrder() error = %v", err)
			}
			if result.Response == nil || result.Response.MsgType != tt.wantMsg {
				t.Fatalf("NewOrder() response = %#v, want MsgType %s", result.Response, tt.wantMsg)
			}
		})
	}
}

func TestScenarioRunsAgainstMockAcceptor(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	server := startMockAcceptor(t, mockfix.Options{})
	manager := newManager(t, server)
	state := toolshell.NewSessionState()
	runner := scenario.NewRunner(scenario.Options{
		Admin:   admin.NewService(manager, admin.Options{Timeout: testCommandTimeout, KeepSession: true, SessionState: state}),
		Order:   order.NewService(manager, order.Options{Timeout: testCommandTimeout, KeepSession: true, SessionState: state}),
		Manager: manager,
	})

	loadedScenario, err := scenario.Load(filepath.Clean("../../testdata/scenarios/mock-acceptor-basic.yaml"))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	result, err := runner.Run(ctx, loadedScenario)
	if err != nil {
		t.Fatalf("Run() error = %v, result = %#v", err, result)
	}
	if result.Status != scenario.StatusPassed {
		t.Fatalf("scenario status = %q, want passed", result.Status)
	}
	if len(result.Steps) != 6 {
		t.Fatalf("scenario steps = %d, want 6", len(result.Steps))
	}
}

func startMockAcceptor(t *testing.T, options mockfix.Options) *mockfix.Acceptor {
	t.Helper()
	options = uniqueMockOptions(options)
	server, err := mockfix.NewAcceptor(options)
	if err != nil {
		t.Fatalf("NewAcceptor() error = %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := server.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := server.Stop(ctx); err != nil {
			t.Fatalf("Stop() error = %v", err)
		}
	})
	return server
}

func uniqueMockOptions(options mockfix.Options) mockfix.Options {
	id := atomic.AddUint64(&mockSessionCounter, 1)
	if options.ProfileName == "" {
		options.ProfileName = fmt.Sprintf("mock-%d", id)
	}
	if options.InitiatorCompID == "" {
		options.InitiatorCompID = fmt.Sprintf("SENDER%d", id)
	}
	if options.AcceptorCompID == "" {
		options.AcceptorCompID = fmt.Sprintf("TARGET%d", id)
	}
	return options
}

func newManager(t *testing.T, server *mockfix.Acceptor) fixsession.Manager {
	t.Helper()
	manager, err := fixsession.NewManager(server.ProfileConfig(), zerolog.Nop())
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	return manager
}

func stopManager(t *testing.T, manager fixsession.Manager) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := manager.Stop(ctx); err != nil {
		t.Fatalf("manager Stop() error = %v", err)
	}
}

func fieldValue(messageTrace *trace.MessageTrace, tag int) string {
	if messageTrace == nil {
		return ""
	}
	for _, field := range messageTrace.Fields {
		if field.Tag == tag {
			return field.Value
		}
	}
	return ""
}
