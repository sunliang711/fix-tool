package scenario

import (
	"testing"

	"fix-tool/internal/message"
	"fix-tool/internal/trace"
)

func TestEvaluateAssertionOperators(t *testing.T) {
	equals := "8"
	notEquals := "D"
	tests := []struct {
		name      string
		assertion Assertion
		wantPass  bool
	}{
		{
			name:      "equals",
			assertion: Assertion{Field: "msg_type", Equals: &equals},
			wantPass:  true,
		},
		{
			name:      "not-equals",
			assertion: Assertion{Field: "msg_type", NotEqual: &notEquals},
			wantPass:  true,
		},
		{
			name:      "exists",
			assertion: Assertion{Field: "cl_ord_id", Exists: true},
			wantPass:  true,
		},
		{
			name:      "not-exists",
			assertion: Assertion{Field: "58", NotExist: true},
			wantPass:  true,
		},
		{
			name:      "in",
			assertion: Assertion{Field: "exec_type", In: []string{"0", "4"}},
			wantPass:  true,
		},
		{
			name:      "missing-equals",
			assertion: Assertion{Field: "58", Equals: &equals},
			wantPass:  false,
		},
	}

	messageTrace := trace.MessageTrace{
		MsgType:  "8",
		ClOrdID:  "C001",
		ExecType: "0",
		Fields: []trace.Field{
			{Tag: message.TagMsgType, Value: "8"},
			{Tag: message.TagClOrdID, Value: "C001"},
			{Tag: 150, Value: "0"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := EvaluateAssertion(&messageTrace, tt.assertion)
			if result.Passed != tt.wantPass {
				t.Fatalf("EvaluateAssertion() passed = %t, want %t, result = %#v", result.Passed, tt.wantPass, result)
			}
			if !tt.wantPass && result.Actual != missingValue {
				t.Fatalf("EvaluateAssertion() actual = %q, want %q", result.Actual, missingValue)
			}
		})
	}
}

func TestAssertionValidateRequiresOneOperator(t *testing.T) {
	equals := "8"
	tests := []struct {
		name      string
		assertion Assertion
		wantErr   bool
	}{
		{
			name:      "valid",
			assertion: Assertion{Field: "msg_type", Equals: &equals},
		},
		{
			name:      "missing-field",
			assertion: Assertion{Equals: &equals},
			wantErr:   true,
		},
		{
			name:      "missing-operator",
			assertion: Assertion{Field: "msg_type"},
			wantErr:   true,
		},
		{
			name:      "multiple-operators",
			assertion: Assertion{Field: "msg_type", Equals: &equals, Exists: true},
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.assertion.Validate()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %t", err, tt.wantErr)
			}
		})
	}
}
