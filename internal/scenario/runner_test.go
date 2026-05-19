package scenario

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"fix-tool/internal/admin"
	"fix-tool/internal/fixsession"
	"fix-tool/internal/message"
	"fix-tool/internal/order"
	rawsvc "fix-tool/internal/raw"
	"fix-tool/internal/trace"
)

func TestRunnerExecutesStepsAndAssertions(t *testing.T) {
	adminService := &fakeAdminService{}
	orderService := &fakeOrderService{}
	runner := NewRunner(Options{
		Admin: adminService,
		Order: orderService,
		Now:   fixedClock(),
	})
	equalsLogon := "A"
	equalsExecType := "0"
	equalsCanceled := "4"
	scenarioValue := Scenario{
		Name: "order-lifecycle",
		Steps: []Step{
			{
				Name:   "logon",
				Action: ActionLogon,
				Wait:   WaitConfig{MsgType: "A"},
				Assert: []Assertion{{Field: "msg_type", Equals: &equalsLogon}},
			},
			{
				Name:   "new-order",
				Action: ActionOrderNew,
				Input: StepInput{
					ClOrdID: "C001",
					Symbol:  "AAPL",
					Side:    "buy",
					Qty:     "100",
					Price:   "10.25",
				},
				Wait:   WaitConfig{MsgType: "8"},
				Assert: []Assertion{{Field: "exec_type", Equals: &equalsExecType}},
			},
			{
				Name:   "cancel-order",
				Action: ActionOrderCancel,
				Input: StepInput{
					OrigClOrdID: "C001",
					ClOrdID:     "C002",
					Symbol:      "AAPL",
					Side:        "buy",
				},
				Wait:   WaitConfig{MsgType: "8"},
				Assert: []Assertion{{Field: "exec_type", Equals: &equalsCanceled}},
			},
			{
				Name:   "logout",
				Action: ActionLogout,
			},
		},
	}

	result, err := runner.Run(context.Background(), scenarioValue)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Status != StatusPassed {
		t.Fatalf("result status = %q, want passed", result.Status)
	}
	if len(result.Steps) != 4 {
		t.Fatalf("steps = %d, want 4", len(result.Steps))
	}
	if orderService.newRequest.ClOrdID != "C001" {
		t.Fatalf("new request clOrdID = %q, want C001", orderService.newRequest.ClOrdID)
	}
	if orderService.cancelRequest.OrigClOrdID != "C001" {
		t.Fatalf("cancel request origClOrdID = %q, want C001", orderService.cancelRequest.OrigClOrdID)
	}
	if len(result.Steps[1].Traces) != 2 {
		t.Fatalf("new-order traces = %d, want request and response", len(result.Steps[1].Traces))
	}
	if len(result.Steps[1].Assertions) != 1 || !result.Steps[1].Assertions[0].Passed {
		t.Fatalf("new-order assertions = %#v, want passed assertion", result.Steps[1].Assertions)
	}
}

func TestRunnerStopsOnAssertionFailure(t *testing.T) {
	adminService := &fakeAdminService{}
	orderService := &fakeOrderService{newExecType: "8"}
	runner := NewRunner(Options{
		Admin: adminService,
		Order: orderService,
		Now:   fixedClock(),
	})
	equalsExecType := "0"
	scenarioValue := Scenario{
		Name: "failure",
		Steps: []Step{
			{
				Name:   "new-order",
				Action: ActionOrderNew,
				Input: StepInput{
					ClOrdID: "C001",
					Symbol:  "AAPL",
					Side:    "buy",
					Qty:     "100",
					Price:   "10.25",
				},
				Assert: []Assertion{{Field: "exec_type", Equals: &equalsExecType}},
			},
			{
				Name:   "logout",
				Action: ActionLogout,
			},
		},
	}

	result, err := runner.Run(context.Background(), scenarioValue)
	if !errors.Is(err, ErrScenarioFailed) {
		t.Fatalf("Run() error = %v, want ErrScenarioFailed", err)
	}
	if result.Status != StatusFailed {
		t.Fatalf("result status = %q, want failed", result.Status)
	}
	if len(result.Steps) != 1 {
		t.Fatalf("steps = %d, want failed step only", len(result.Steps))
	}
	if !strings.Contains(result.Steps[0].Error, "field exec_type expected 0 actual 8") {
		t.Fatalf("step error = %q, want assertion detail", result.Steps[0].Error)
	}
	if adminService.logoutCalls != 0 {
		t.Fatalf("logout calls = %d, want 0", adminService.logoutCalls)
	}
}

func TestRunnerExecutesRawAction(t *testing.T) {
	rawService := &fakeRawService{}
	runner := NewRunner(Options{
		Raw: rawService,
		Now: fixedClock(),
	})
	equalsExecType := "0"
	scenarioValue := Scenario{
		Name: "raw",
		Steps: []Step{
			{
				Name:   "raw",
				Action: ActionRaw,
				Input: StepInput{
					MsgType: "D",
					Tags:    []string{"11=RAW-1"},
				},
				Wait:   WaitConfig{MsgType: "8"},
				Assert: []Assertion{{Field: "exec_type", Equals: &equalsExecType}},
			},
		},
	}

	result, err := runner.Run(context.Background(), scenarioValue)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if rawService.request.MsgType != "D" {
		t.Fatalf("raw msg type = %q, want D", rawService.request.MsgType)
	}
	if len(rawService.request.Tags) != 1 || rawService.request.Tags[0] != "11=RAW-1" {
		t.Fatalf("raw tags = %#v, want scenario tags", rawService.request.Tags)
	}
	if len(result.Steps[0].Traces) != 2 {
		t.Fatalf("raw traces = %d, want request and response", len(result.Steps[0].Traces))
	}
}

func TestRunnerReportsRawServiceMissing(t *testing.T) {
	runner := NewRunner(Options{Now: fixedClock()})
	result, err := runner.Run(context.Background(), Scenario{
		Name: "raw",
		Steps: []Step{
			{Name: "raw", Action: ActionRaw, Input: StepInput{MsgType: "D"}},
		},
	})
	if !errors.Is(err, ErrScenarioFailed) {
		t.Fatalf("Run() error = %v, want ErrScenarioFailed", err)
	}
	if !strings.Contains(result.Steps[0].Error, "raw service is required") {
		t.Fatalf("step error = %q, want raw service error", result.Steps[0].Error)
	}
}

func TestRunnerMarksResultFailedWhenStopFails(t *testing.T) {
	stopErr := errors.New("stop failed")
	runner := NewRunner(Options{
		Admin:   &fakeAdminService{},
		Manager: fakeStopManager{err: stopErr},
		Now:     fixedClock(),
	})
	result, err := runner.Run(context.Background(), Scenario{
		Name: "cleanup",
		Steps: []Step{
			{Name: "logon", Action: ActionLogon},
		},
	})
	if !errors.Is(err, stopErr) {
		t.Fatalf("Run() error = %v, want stop error", err)
	}
	if result.Status != StatusFailed {
		t.Fatalf("result status = %q, want failed", result.Status)
	}
	if result.Error != stopErr.Error() {
		t.Fatalf("result error = %q, want %q", result.Error, stopErr.Error())
	}
	if len(result.Steps) != 1 || result.Steps[0].Status != StatusPassed {
		t.Fatalf("steps = %#v, want executed step to stay passed", result.Steps)
	}
}

type fakeStopManager struct {
	err error
}

func (m fakeStopManager) Start(context.Context) error {
	return nil
}

func (m fakeStopManager) Stop(context.Context) error {
	return m.err
}

func (m fakeStopManager) Events() <-chan fixsession.Event {
	return nil
}

func (m fakeStopManager) Session() fixsession.Session {
	return nil
}

type fakeAdminService struct {
	logoutCalls int
}

func (s *fakeAdminService) Logon(context.Context) (admin.Result, error) {
	return admin.Result{
		Request:  adminTrace("A", trace.DirectionOutbound),
		Response: adminTrace("A", trace.DirectionInbound),
	}, nil
}

func (s *fakeAdminService) Logout(context.Context) (admin.Result, error) {
	s.logoutCalls++
	return admin.Result{
		Request:  adminTrace("5", trace.DirectionOutbound),
		Response: adminTrace("5", trace.DirectionInbound),
	}, nil
}

func (s *fakeAdminService) Heartbeat(context.Context) (admin.Result, error) {
	return admin.Result{
		Request:  adminTrace("0", trace.DirectionOutbound),
		Response: adminTrace("0", trace.DirectionInbound),
	}, nil
}

func (s *fakeAdminService) TestRequest(_ context.Context, id string) (admin.Result, error) {
	return admin.Result{
		Request:  adminTrace("1", trace.DirectionOutbound),
		Response: adminTraceWithField("0", trace.DirectionInbound, 112, id),
	}, nil
}

type fakeOrderService struct {
	newRequest    order.NewRequest
	cancelRequest order.CancelRequest
	newExecType   string
}

type fakeRawService struct {
	request rawsvc.Request
}

func (s *fakeRawService) Send(_ context.Context, request rawsvc.Request) (rawsvc.Result, error) {
	s.request = request
	return rawsvc.Result{
		Request:  orderTrace(request.MsgType, "RAW-1", "", trace.DirectionOutbound),
		Response: executionReport("RAW-1", "0"),
	}, nil
}

func (s *fakeOrderService) NewOrder(_ context.Context, request order.NewRequest) (order.Result, error) {
	s.newRequest = request
	execType := s.newExecType
	if execType == "" {
		execType = "0"
	}
	return order.Result{
		Request:  orderTrace(message.MsgTypeNewOrderSingle, request.ClOrdID, "", trace.DirectionOutbound),
		Response: executionReport(request.ClOrdID, execType),
	}, nil
}

func (s *fakeOrderService) CancelOrder(_ context.Context, request order.CancelRequest) (order.Result, error) {
	s.cancelRequest = request
	return order.Result{
		Request:  orderTrace(message.MsgTypeOrderCancelRequest, request.ClOrdID, request.OrigClOrdID, trace.DirectionOutbound),
		Response: executionReport(request.ClOrdID, "4"),
	}, nil
}

func (s *fakeOrderService) ReplaceOrder(_ context.Context, request order.ReplaceRequest) (order.Result, error) {
	return order.Result{
		Request:  orderTrace(message.MsgTypeOrderCancelReplaceRequest, request.ClOrdID, request.OrigClOrdID, trace.DirectionOutbound),
		Response: executionReport(request.ClOrdID, "5"),
	}, nil
}

func adminTrace(msgType string, direction trace.Direction) *trace.MessageTrace {
	return adminTraceWithField(msgType, direction, 0, "")
}

func adminTraceWithField(msgType string, direction trace.Direction, tag int, value string) *trace.MessageTrace {
	fields := []trace.Field{{Tag: message.TagMsgType, Value: msgType}}
	if tag > 0 {
		fields = append(fields, trace.Field{Tag: tag, Value: value})
	}
	return &trace.MessageTrace{
		TraceID:   "admin-" + msgType,
		Direction: direction,
		MsgType:   msgType,
		Raw:       "35=" + msgType + "|",
		Fields:    fields,
	}
}

func orderTrace(msgType string, clOrdID string, origClOrdID string, direction trace.Direction) *trace.MessageTrace {
	fields := []trace.Field{
		{Tag: message.TagMsgType, Value: msgType},
		{Tag: message.TagClOrdID, Value: clOrdID},
	}
	if origClOrdID != "" {
		fields = append(fields, trace.Field{Tag: message.TagOrigClOrdID, Value: origClOrdID})
	}
	return &trace.MessageTrace{
		TraceID:   "order-" + msgType,
		Direction: direction,
		MsgType:   msgType,
		ClOrdID:   clOrdID,
		Raw:       "35=" + msgType + "|11=" + clOrdID + "|",
		Fields:    fields,
	}
}

func executionReport(clOrdID string, execType string) *trace.MessageTrace {
	return &trace.MessageTrace{
		TraceID:   "exec-" + clOrdID,
		Direction: trace.DirectionInbound,
		MsgType:   message.MsgTypeExecutionReport,
		ClOrdID:   clOrdID,
		ExecType:  execType,
		Fields: []trace.Field{
			{Tag: message.TagMsgType, Value: message.MsgTypeExecutionReport},
			{Tag: message.TagClOrdID, Value: clOrdID},
			{Tag: 150, Value: execType},
			{Tag: 39, Value: execType},
		},
		Raw: "35=8|11=" + clOrdID + "|150=" + execType + "|39=" + execType + "|",
	}
}

func fixedClock() func() time.Time {
	now := time.Date(2026, 5, 19, 10, 0, 0, 0, time.UTC)
	return func() time.Time {
		return now
	}
}
