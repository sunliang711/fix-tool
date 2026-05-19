package scenario

import (
	"fmt"
	"strings"
)

type Scenario struct {
	Name  string `json:"name" yaml:"name"`
	Steps []Step `json:"steps" yaml:"steps"`
}

type Step struct {
	Name   string      `json:"name,omitempty" yaml:"name"`
	Action string      `json:"action" yaml:"action"`
	Input  StepInput   `json:"input,omitempty" yaml:"input"`
	Wait   WaitConfig  `json:"wait,omitempty" yaml:"wait"`
	Assert []Assertion `json:"assert,omitempty" yaml:"assert"`
}

type StepInput struct {
	TestRequestID string   `json:"test_request_id,omitempty" yaml:"test_request_id"`
	ClOrdID       string   `json:"cl_ord_id,omitempty" yaml:"cl_ord_id"`
	OrigClOrdID   string   `json:"orig_cl_ord_id,omitempty" yaml:"orig_cl_ord_id"`
	OrderID       string   `json:"order_id,omitempty" yaml:"order_id"`
	Symbol        string   `json:"symbol,omitempty" yaml:"symbol"`
	Side          string   `json:"side,omitempty" yaml:"side"`
	Qty           string   `json:"qty,omitempty" yaml:"qty"`
	Price         string   `json:"price,omitempty" yaml:"price"`
	OrdType       string   `json:"ord_type,omitempty" yaml:"ord_type"`
	TimeInForce   string   `json:"time_in_force,omitempty" yaml:"time_in_force"`
	MsgType       string   `json:"msg_type,omitempty" yaml:"msg_type"`
	Tags          []string `json:"tags,omitempty" yaml:"tags"`
	Raw           string   `json:"raw,omitempty" yaml:"raw"`
}

type WaitConfig struct {
	MsgType string `json:"msg_type,omitempty" yaml:"msg_type"`
}

type Assertion struct {
	Field    string   `json:"field" yaml:"field"`
	Equals   *string  `json:"equals,omitempty" yaml:"equals"`
	NotEqual *string  `json:"not_equals,omitempty" yaml:"not_equals"`
	Exists   bool     `json:"exists,omitempty" yaml:"exists"`
	NotExist bool     `json:"not_exists,omitempty" yaml:"not_exists"`
	In       []string `json:"in,omitempty" yaml:"in"`
}

const (
	ActionLogon        = "logon"
	ActionLogout       = "logout"
	ActionHeartbeat    = "heartbeat"
	ActionTestRequest  = "test-request"
	ActionOrderNew     = "order.new"
	ActionOrderCancel  = "order.cancel"
	ActionOrderReplace = "order.replace"
	ActionRaw          = "raw"
)

func (s Scenario) Validate() error {
	if len(s.Steps) == 0 {
		return fmt.Errorf("scenario must contain at least one step")
	}
	for i, step := range s.Steps {
		if err := step.Validate(i); err != nil {
			return err
		}
	}
	return nil
}

func (s Step) Validate(index int) error {
	action, err := NormalizeAction(s.Action)
	if err != nil {
		return fmt.Errorf("step %d: %w", index+1, err)
	}
	if action == ActionTestRequest && strings.TrimSpace(s.Input.TestRequestID) == "" {
		return fmt.Errorf("step %d: test_request_id is required", index+1)
	}
	if action == ActionRaw && strings.TrimSpace(s.Input.MsgType) == "" {
		return fmt.Errorf("step %d: msg_type is required", index+1)
	}
	for i, assertion := range s.Assert {
		if err := assertion.Validate(); err != nil {
			return fmt.Errorf("step %d assertion %d: %w", index+1, i+1, err)
		}
	}
	return nil
}

func (a Assertion) Validate() error {
	if strings.TrimSpace(a.Field) == "" {
		return fmt.Errorf("field is required")
	}
	operations := 0
	if a.Equals != nil {
		operations++
	}
	if a.NotEqual != nil {
		operations++
	}
	if a.Exists {
		operations++
	}
	if a.NotExist {
		operations++
	}
	if len(a.In) > 0 {
		operations++
	}
	if operations != 1 {
		return fmt.Errorf("exactly one assertion operator is required")
	}
	return nil
}

func NormalizeAction(action string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(action))
	normalized = strings.ReplaceAll(normalized, "_", "-")
	switch normalized {
	case ActionLogon:
		return ActionLogon, nil
	case ActionLogout:
		return ActionLogout, nil
	case ActionHeartbeat:
		return ActionHeartbeat, nil
	case "testrequest", "test.request", "test-request":
		return ActionTestRequest, nil
	case "order.new", "order-new", "new-order":
		return ActionOrderNew, nil
	case "order.cancel", "order-cancel", "cancel-order":
		return ActionOrderCancel, nil
	case "order.replace", "order-replace", "replace-order":
		return ActionOrderReplace, nil
	case ActionRaw:
		return ActionRaw, nil
	default:
		return "", fmt.Errorf("unsupported action %q", action)
	}
}

func stepDisplayName(index int, step Step) string {
	if strings.TrimSpace(step.Name) != "" {
		return strings.TrimSpace(step.Name)
	}
	return fmt.Sprintf("step-%d", index+1)
}
