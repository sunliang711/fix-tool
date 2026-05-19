package raw

import (
	"strings"
	"testing"

	"fix-tool/internal/trace"

	"github.com/quickfixgo/quickfix"
)

func TestBuildMessageGeneratesFIXRawWithQuickFIX(t *testing.T) {
	message, err := BuildMessage(Request{
		MsgType: "D",
		Tags: []string{
			"55=AAPL",
			"54=1",
			"9001=desk-a",
		},
	})
	if err != nil {
		t.Fatalf("BuildMessage() error = %v", err)
	}
	message.Header.SetString(quickfix.Tag(tagBeginString), "FIX.4.4")

	rawMessage := message.String()
	if !strings.Contains(rawMessage, soh) {
		t.Fatalf("raw message = %q, want SOH delimiter", rawMessage)
	}
	for _, want := range []string{"8=FIX.4.4" + soh, "9=", "35=D" + soh, "55=AAPL" + soh, "54=1" + soh, "9001=desk-a" + soh, "10="} {
		if !strings.Contains(rawMessage, want) {
			t.Fatalf("raw message = %q, want %q", rawMessage, want)
		}
	}

	parsed, err := trace.ParseRaw(rawMessage)
	if err != nil {
		t.Fatalf("ParseRaw() error = %v", err)
	}
	if !parsed.BodyLengthValid {
		t.Fatalf("BodyLength = %+v, want valid", parsed.BodyLength)
	}
	if !parsed.CheckSumValid {
		t.Fatalf("CheckSum = %+v, want valid", parsed.CheckSum)
	}
}

func TestBuildMessageRejectsProtectedTags(t *testing.T) {
	for _, protectedTag := range []string{"8", "9", "10", "34", "35", "49", "52", "56"} {
		t.Run(protectedTag, func(t *testing.T) {
			_, err := BuildMessage(Request{
				MsgType: "D",
				Tags:    []string{protectedTag + "=X"},
			})
			if err == nil {
				t.Fatal("BuildMessage() error = nil, want protected tag error")
			}
			if !strings.Contains(err.Error(), "不允许覆盖协议字段 "+protectedTag) {
				t.Fatalf("BuildMessage() error = %v, want protected tag %s", err, protectedTag)
			}
		})
	}
}

func TestBuildMessageKeepsDisplayDelimiterAsValue(t *testing.T) {
	message, err := BuildMessage(Request{
		MsgType: "Z",
		Tags:    []string{"58=hello|world"},
	})
	if err != nil {
		t.Fatalf("BuildMessage() error = %v", err)
	}
	message.Header.SetString(quickfix.Tag(tagBeginString), "FIX.4.4")

	rawMessage := message.String()
	parsed, err := trace.ParseRaw(rawMessage)
	if err != nil {
		t.Fatalf("ParseRaw() error = %v", err)
	}
	if got := fieldValue(parsed.Fields, 58); got != "hello|world" {
		t.Fatalf("tag 58 = %q, want literal pipe value", got)
	}
}

func TestParseTagsRejectsRealSOHInValue(t *testing.T) {
	_, err := ParseTags([]string{"58=hello" + soh + "world"})
	if err == nil {
		t.Fatal("ParseTags() error = nil, want SOH value error")
	}
}

func fieldValue(fields []trace.Field, tag int) string {
	for _, field := range fields {
		if field.Tag == tag {
			return field.Value
		}
	}
	return ""
}
