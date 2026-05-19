package scenario

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"fix-tool/internal/admin"
	"fix-tool/internal/fixsession"
	"fix-tool/internal/order"
	"fix-tool/internal/trace"
)

var ErrScenarioFailed = errors.New("scenario failed")

type AdminService interface {
	Logon(context.Context) (admin.Result, error)
	Logout(context.Context) (admin.Result, error)
	Heartbeat(context.Context) (admin.Result, error)
	TestRequest(context.Context, string) (admin.Result, error)
}

type OrderService interface {
	NewOrder(context.Context, order.NewRequest) (order.Result, error)
	CancelOrder(context.Context, order.CancelRequest) (order.Result, error)
	ReplaceOrder(context.Context, order.ReplaceRequest) (order.Result, error)
}

type Options struct {
	Admin       AdminService
	Order       OrderService
	Manager     fixsession.Manager
	Recorder    *trace.Recorder
	StopTimeout time.Duration
	Now         func() time.Time
}

type Runner struct {
	admin       AdminService
	order       OrderService
	manager     fixsession.Manager
	recorder    *trace.Recorder
	stopTimeout time.Duration
	now         func() time.Time
}

type Result struct {
	Scenario   string       `json:"scenario"`
	Status     string       `json:"status"`
	Error      string       `json:"error,omitempty"`
	StartedAt  time.Time    `json:"started_at"`
	FinishedAt time.Time    `json:"finished_at"`
	DurationMS int64        `json:"duration_ms"`
	Steps      []StepResult `json:"steps"`
}

type StepResult struct {
	Index      int                  `json:"index"`
	Name       string               `json:"name"`
	Action     string               `json:"action"`
	Status     string               `json:"status"`
	Error      string               `json:"error,omitempty"`
	StartedAt  time.Time            `json:"started_at"`
	FinishedAt time.Time            `json:"finished_at"`
	DurationMS int64                `json:"duration_ms"`
	Traces     []trace.MessageTrace `json:"traces"`
	Assertions []AssertionResult    `json:"assertions,omitempty"`
}

const (
	StatusPassed = "passed"
	StatusFailed = "failed"

	defaultStopTimeout = 5 * time.Second
)

func NewRunner(options Options) *Runner {
	recorder := options.Recorder
	if recorder == nil {
		recorder = trace.NewRecorder()
	}
	stopTimeout := options.StopTimeout
	if stopTimeout <= 0 {
		stopTimeout = defaultStopTimeout
	}
	now := options.Now
	if now == nil {
		now = func() time.Time {
			return time.Now().UTC()
		}
	}
	return &Runner{
		admin:       options.Admin,
		order:       options.Order,
		manager:     options.Manager,
		recorder:    recorder,
		stopTimeout: stopTimeout,
		now:         now,
	}
}

func (r *Runner) Run(ctx context.Context, scenario Scenario) (result Result, err error) {
	startedAt := r.now()
	result = Result{
		Scenario:  scenario.Name,
		Status:    StatusPassed,
		StartedAt: startedAt,
		Steps:     make([]StepResult, 0, len(scenario.Steps)),
	}
	defer func() {
		if stopErr := r.stopSession(); stopErr != nil {
			result.Status = StatusFailed
			result.Error = stopErr.Error()
			if err == nil {
				err = stopErr
			}
		}
		result.FinishedAt = r.now()
		result.DurationMS = result.FinishedAt.Sub(startedAt).Milliseconds()
		if result.Status == "" {
			result.Status = StatusPassed
		}
	}()

	if err := scenario.Validate(); err != nil {
		result.Status = StatusFailed
		return result, err
	}
	for i, step := range scenario.Steps {
		stepResult := r.runStep(ctx, i, step)
		result.Steps = append(result.Steps, stepResult)
		if stepResult.Status == StatusFailed {
			result.Status = StatusFailed
			return result, ErrScenarioFailed
		}
	}
	return result, nil
}

func (r *Runner) runStep(ctx context.Context, index int, step Step) StepResult {
	startedAt := r.now()
	action, err := NormalizeAction(step.Action)
	result := StepResult{
		Index:     index + 1,
		Name:      stepDisplayName(index, step),
		Action:    action,
		Status:    StatusPassed,
		StartedAt: startedAt,
	}
	if err != nil {
		result.Action = step.Action
		return r.finishFailedStep(result, startedAt, err)
	}

	traces, response, err := r.execute(ctx, action, step.Input)
	result.Traces = traces
	if err != nil {
		return r.finishFailedStep(result, startedAt, err)
	}
	if err := checkWait(step.Wait, response); err != nil {
		return r.finishFailedStep(result, startedAt, err)
	}
	result.Assertions = EvaluateAssertions(response, step.Assert)
	if failed := firstFailedAssertion(result.Assertions); failed != nil {
		return r.finishFailedStep(result, startedAt, AssertionError{Step: result.Name, Result: *failed})
	}
	return r.finishPassedStep(result, startedAt)
}

func (r *Runner) execute(ctx context.Context, action string, input StepInput) ([]trace.MessageTrace, *trace.MessageTrace, error) {
	switch action {
	case ActionLogon:
		if r.admin == nil {
			return nil, nil, fmt.Errorf("admin service is required")
		}
		result, err := r.admin.Logon(ctx)
		return r.recordAdminResult(result), result.Response, err
	case ActionLogout:
		if r.admin == nil {
			return nil, nil, fmt.Errorf("admin service is required")
		}
		result, err := r.admin.Logout(ctx)
		return r.recordAdminResult(result), result.Response, err
	case ActionHeartbeat:
		if r.admin == nil {
			return nil, nil, fmt.Errorf("admin service is required")
		}
		result, err := r.admin.Heartbeat(ctx)
		return r.recordAdminResult(result), result.Response, err
	case ActionTestRequest:
		if r.admin == nil {
			return nil, nil, fmt.Errorf("admin service is required")
		}
		result, err := r.admin.TestRequest(ctx, input.TestRequestID)
		return r.recordAdminResult(result), result.Response, err
	case ActionOrderNew:
		if r.order == nil {
			return nil, nil, fmt.Errorf("order service is required")
		}
		result, err := r.order.NewOrder(ctx, order.NewRequest{
			ClOrdID:     input.ClOrdID,
			Symbol:      input.Symbol,
			Side:        input.Side,
			OrderQty:    input.Qty,
			Price:       input.Price,
			OrdType:     input.OrdType,
			TimeInForce: input.TimeInForce,
			Tags:        input.Tags,
		})
		return r.recordOrderResult(result), result.Response, err
	case ActionOrderCancel:
		if r.order == nil {
			return nil, nil, fmt.Errorf("order service is required")
		}
		result, err := r.order.CancelOrder(ctx, order.CancelRequest{
			OrigClOrdID: input.OrigClOrdID,
			ClOrdID:     input.ClOrdID,
			OrderID:     input.OrderID,
			Symbol:      input.Symbol,
			Side:        input.Side,
			Tags:        input.Tags,
		})
		return r.recordOrderResult(result), result.Response, err
	case ActionOrderReplace:
		if r.order == nil {
			return nil, nil, fmt.Errorf("order service is required")
		}
		result, err := r.order.ReplaceOrder(ctx, order.ReplaceRequest{
			OrigClOrdID: input.OrigClOrdID,
			ClOrdID:     input.ClOrdID,
			OrderID:     input.OrderID,
			Symbol:      input.Symbol,
			Side:        input.Side,
			OrderQty:    input.Qty,
			Price:       input.Price,
			OrdType:     input.OrdType,
			TimeInForce: input.TimeInForce,
			Tags:        input.Tags,
		})
		return r.recordOrderResult(result), result.Response, err
	case ActionRaw:
		// task07 尚未提供 raw service，这里保留清晰失败入口，后续可直接替换为真实执行。
		return nil, nil, fmt.Errorf("raw action is not available before task07 is implemented")
	default:
		return nil, nil, fmt.Errorf("unsupported action %q", action)
	}
}

func (r *Runner) recordAdminResult(result admin.Result) []trace.MessageTrace {
	return r.recordMessages(result.Request, result.Response)
}

func (r *Runner) recordOrderResult(result order.Result) []trace.MessageTrace {
	return r.recordMessages(result.Request, result.Response)
}

func (r *Runner) recordMessages(messages ...*trace.MessageTrace) []trace.MessageTrace {
	traces := make([]trace.MessageTrace, 0, len(messages))
	for _, messageValue := range messages {
		if messageValue == nil {
			continue
		}
		traces = append(traces, r.recorder.Record(*messageValue))
	}
	return traces
}

func (r *Runner) finishPassedStep(result StepResult, startedAt time.Time) StepResult {
	result.Status = StatusPassed
	result.FinishedAt = r.now()
	result.DurationMS = result.FinishedAt.Sub(startedAt).Milliseconds()
	return result
}

func (r *Runner) finishFailedStep(result StepResult, startedAt time.Time, err error) StepResult {
	result.Status = StatusFailed
	if err != nil {
		result.Error = err.Error()
	}
	result.FinishedAt = r.now()
	result.DurationMS = result.FinishedAt.Sub(startedAt).Milliseconds()
	return result
}

func (r *Runner) stopSession() error {
	if r.manager == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), r.stopTimeout)
	defer cancel()
	return r.manager.Stop(ctx)
}

func checkWait(wait WaitConfig, response *trace.MessageTrace) error {
	if wait.MsgType == "" {
		return nil
	}
	if response == nil {
		return fmt.Errorf("wait msg_type expected %s, actual %s", wait.MsgType, missingValue)
	}
	if response.MsgType != wait.MsgType {
		return fmt.Errorf("wait msg_type expected %s, actual %s", wait.MsgType, response.MsgType)
	}
	return nil
}

func firstFailedAssertion(results []AssertionResult) *AssertionResult {
	for i := range results {
		if !results[i].Passed {
			return &results[i]
		}
	}
	return nil
}

type AssertionError struct {
	Step   string
	Result AssertionResult
}

func (e AssertionError) Error() string {
	return fmt.Sprintf(
		"step %s assertion failed: field %s expected %s actual %s",
		e.Step,
		e.Result.Field,
		expectedText(e.Result),
		e.Result.Actual,
	)
}

func expectedText(result AssertionResult) string {
	if len(result.ExpectedValues) > 0 {
		return strings.Join(result.ExpectedValues, ",")
	}
	return result.Expected
}
