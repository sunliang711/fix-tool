package trace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseRawValidatesBodyLengthAndCheckSum(t *testing.T) {
	raw := readMessage(t, "new_order_single.fix")

	parsed, err := ParseRaw(raw)
	if err != nil {
		t.Fatalf("ParseRaw() error = %v", err)
	}
	if !parsed.BodyLengthValid {
		t.Fatalf("BodyLengthValid = false, result = %+v", parsed.BodyLength)
	}
	if !parsed.CheckSumValid {
		t.Fatalf("CheckSumValid = false, result = %+v", parsed.CheckSum)
	}
	if got := firstValue(parsed.Fields, tagMsgType); got != "D" {
		t.Fatalf("MsgType = %q, want %q", got, "D")
	}
	if got := firstValue(parsed.Fields, tagClOrdID); got != "C001" {
		t.Fatalf("ClOrdID = %q, want %q", got, "C001")
	}
	if strings.Contains(parsed.Raw, "|") {
		t.Fatalf("parsed raw still contains display delimiter: %q", parsed.Raw)
	}
	if got := DisplayRaw(parsed.Raw, "|"); !strings.Contains(got, "35=D|") {
		t.Fatalf("DisplayRaw() = %q, want display delimiter", got)
	}
}

func TestParseRawDetectsInvalidBodyLength(t *testing.T) {
	raw := strings.Replace(readMessage(t, "new_order_single.fix"), "9=137|", "9=999|", 1)

	parsed, err := ParseRaw(raw)
	if err != nil {
		t.Fatalf("ParseRaw() error = %v", err)
	}
	if parsed.BodyLengthValid {
		t.Fatalf("BodyLengthValid = true, want false")
	}
	if parsed.BodyLength.Expected != "999" {
		t.Fatalf("BodyLength expected = %q, want %q", parsed.BodyLength.Expected, "999")
	}
	if parsed.BodyLength.Actual != "137" {
		t.Fatalf("BodyLength actual = %q, want %q", parsed.BodyLength.Actual, "137")
	}
}

func TestParseRawDetectsInvalidCheckSum(t *testing.T) {
	raw := strings.Replace(readMessage(t, "new_order_single.fix"), "10=233|", "10=000|", 1)

	parsed, err := ParseRaw(raw)
	if err != nil {
		t.Fatalf("ParseRaw() error = %v", err)
	}
	if parsed.CheckSumValid {
		t.Fatalf("CheckSumValid = true, want false")
	}
	if parsed.CheckSum.Expected != "000" {
		t.Fatalf("CheckSum expected = %q, want %q", parsed.CheckSum.Expected, "000")
	}
	if parsed.CheckSum.Actual != "233" {
		t.Fatalf("CheckSum actual = %q, want %q", parsed.CheckSum.Actual, "233")
	}
}

func TestParseRawRejectsInvalidField(t *testing.T) {
	_, err := ParseRaw("8=FIX.4.4|bad-field|10=000|")
	if err == nil {
		t.Fatal("ParseRaw() error = nil, want error")
	}
}

func TestParseRawRejectsEmptyMessage(t *testing.T) {
	_, err := ParseRaw("  ")
	if err == nil || !strings.Contains(err.Error(), "empty message") {
		t.Fatalf("ParseRaw() error = %v, want empty message error", err)
	}
}

func TestDisplayRawUsesDefaultDelimiter(t *testing.T) {
	got := DisplayRaw("35=D\x0111=C001\x01", "")
	if got != "35=D|11=C001|" {
		t.Fatalf("DisplayRaw() = %q, want default delimiter", got)
	}
}

func readMessage(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "messages", name))
	if err != nil {
		t.Fatalf("read message %s: %v", name, err)
	}
	return string(data)
}
