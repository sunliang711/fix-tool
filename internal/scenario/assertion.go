package scenario

import (
	"fmt"
	"strconv"
	"strings"

	"fix-tool/internal/message"
	"fix-tool/internal/trace"
)

type AssertionResult struct {
	Field          string   `json:"field"`
	Operator       string   `json:"operator"`
	Expected       string   `json:"expected,omitempty"`
	ExpectedValues []string `json:"expected_values,omitempty"`
	Actual         string   `json:"actual,omitempty"`
	Passed         bool     `json:"passed"`
	Message        string   `json:"message,omitempty"`
}

const missingValue = "<missing>"

func EvaluateAssertions(messageValue *trace.MessageTrace, assertions []Assertion) []AssertionResult {
	results := make([]AssertionResult, 0, len(assertions))
	for _, assertion := range assertions {
		results = append(results, EvaluateAssertion(messageValue, assertion))
	}
	return results
}

func EvaluateAssertion(messageValue *trace.MessageTrace, assertion Assertion) AssertionResult {
	actual, present := fieldValue(messageValue, assertion.Field)
	if !present {
		actual = missingValue
	}
	switch {
	case assertion.Equals != nil:
		return compareResult(assertion.Field, "equals", *assertion.Equals, actual, present && actual == *assertion.Equals)
	case assertion.NotEqual != nil:
		return compareResult(assertion.Field, "not_equals", *assertion.NotEqual, actual, present && actual != *assertion.NotEqual)
	case assertion.Exists:
		return compareResult(assertion.Field, "exists", "present", actual, present)
	case assertion.NotExist:
		return compareResult(assertion.Field, "not_exists", "absent", actual, !present)
	case len(assertion.In) > 0:
		passed := false
		if present {
			for _, expected := range assertion.In {
				if actual == expected {
					passed = true
					break
				}
			}
		}
		result := AssertionResult{
			Field:          assertion.Field,
			Operator:       "in",
			ExpectedValues: append([]string(nil), assertion.In...),
			Actual:         actual,
			Passed:         passed,
		}
		if !passed {
			result.Message = fmt.Sprintf("field %s expected one of %s, actual %s", assertion.Field, strings.Join(assertion.In, ","), actual)
		}
		return result
	default:
		return AssertionResult{
			Field:    assertion.Field,
			Operator: "invalid",
			Actual:   actual,
			Passed:   false,
			Message:  fmt.Sprintf("field %s has no assertion operator", assertion.Field),
		}
	}
}

func compareResult(field string, operator string, expected string, actual string, passed bool) AssertionResult {
	result := AssertionResult{
		Field:    field,
		Operator: operator,
		Expected: expected,
		Actual:   actual,
		Passed:   passed,
	}
	if !passed {
		result.Message = fmt.Sprintf("field %s expected %s, actual %s", field, expected, actual)
	}
	return result
}

func fieldValue(messageValue *trace.MessageTrace, field string) (string, bool) {
	if messageValue == nil {
		return "", false
	}
	normalized := normalizeField(field)
	if normalized == "raw" {
		return messageValue.Raw, messageValue.Raw != ""
	}
	tag, ok := fieldTag(normalized)
	if !ok {
		return "", false
	}
	for _, item := range messageValue.Fields {
		if item.Tag == tag {
			return item.Value, true
		}
	}
	return "", false
}

func normalizeField(field string) string {
	field = strings.ToLower(strings.TrimSpace(field))
	field = strings.ReplaceAll(field, "-", "_")
	return field
}

func fieldTag(field string) (int, bool) {
	if value, err := strconv.Atoi(field); err == nil && value > 0 {
		return value, true
	}
	switch strings.ReplaceAll(field, "_", "") {
	case "msgtype":
		return message.TagMsgType, true
	case "msgseqnum":
		return message.TagMsgSeqNum, true
	case "clordid":
		return message.TagClOrdID, true
	case "origclordid":
		return message.TagOrigClOrdID, true
	case "orderid":
		return message.TagOrderID, true
	case "symbol":
		return message.TagSymbol, true
	case "side":
		return message.TagSide, true
	case "orderqty", "qty":
		return message.TagOrderQty, true
	case "price":
		return message.TagPrice, true
	case "ordtype":
		return message.TagOrdType, true
	case "timeinforce":
		return message.TagTimeInForce, true
	case "exectype":
		return 150, true
	case "ordstatus":
		return 39, true
	default:
		return 0, false
	}
}
