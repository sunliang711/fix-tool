package scenario

import (
	"strings"
	"testing"
)

func TestNormalizeActionAliases(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{name: "trim-logon", value: " logon ", want: ActionLogon},
		{name: "test-request-dot", value: "test.request", want: ActionTestRequest},
		{name: "order-new-dash", value: "order-new", want: ActionOrderNew},
		{name: "order-cancel-alias", value: "cancel-order", want: ActionOrderCancel},
		{name: "order-replace-underscore", value: "order_replace", want: ActionOrderReplace},
		{name: "raw", value: "raw", want: ActionRaw},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeAction(tt.value)
			if err != nil {
				t.Fatalf("NormalizeAction(%q) error = %v", tt.value, err)
			}
			if got != tt.want {
				t.Fatalf("NormalizeAction(%q) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
}

func TestStepValidateRequiresTestRequestID(t *testing.T) {
	err := Step{Action: ActionTestRequest}.Validate(0)
	if err == nil || !strings.Contains(err.Error(), "test_request_id is required") {
		t.Fatalf("Validate() error = %v, want test_request_id error", err)
	}
}

func TestStepValidateRequiresRawMsgType(t *testing.T) {
	err := Step{Action: ActionRaw}.Validate(0)
	if err == nil || !strings.Contains(err.Error(), "msg_type is required") {
		t.Fatalf("Validate() error = %v, want msg_type error", err)
	}
}
