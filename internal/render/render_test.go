package render

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"fix-tool/internal/dictionary"
	"fix-tool/internal/trace"
)

func TestRendererRedactsSensitiveValuesByDefault(t *testing.T) {
	message := newTrace(t)
	renderer := newTestRenderer(false)

	raw, err := renderer.Render(message, FormatRaw)
	if err != nil {
		t.Fatalf("Render(raw) error = %v", err)
	}
	for _, secret := range []string{"account", "secret", "token-value"} {
		if strings.Contains(raw, secret) {
			t.Fatalf("raw output contains sensitive value %q: %s", secret, raw)
		}
	}
	if count := strings.Count(raw, redactedValue); count != 3 {
		t.Fatalf("redacted count = %d, want 3 in %s", count, raw)
	}
}

func TestRendererTableIncludesFieldNamesAndEnums(t *testing.T) {
	message := newTrace(t)
	renderer := newTestRenderer(false)

	output, err := renderer.Render(message, FormatTable)
	if err != nil {
		t.Fatalf("Render(table) error = %v", err)
	}
	for _, want := range []string{"Raw", "MsgType", "NewOrderSingle", "Side", "Buy", "SessionToken", redactedValue} {
		if !strings.Contains(output, want) {
			t.Fatalf("table output missing %q:\n%s", want, output)
		}
	}
	if strings.Contains(output, "token-value") {
		t.Fatalf("table output contains sensitive value:\n%s", output)
	}
}

func TestRendererJSONIncludesRedactedFields(t *testing.T) {
	message := newTrace(t)
	renderer := newTestRenderer(false)

	output, err := renderer.Render(message, FormatJSON)
	if err != nil {
		t.Fatalf("Render(json) error = %v", err)
	}
	if strings.Contains(output, "secret") || strings.Contains(output, "token-value") {
		t.Fatalf("json output contains sensitive value:\n%s", output)
	}
	var payload struct {
		Raw    string `json:"raw"`
		Fields []struct {
			Tag   int    `json:"tag"`
			Name  string `json:"name"`
			Value string `json:"value"`
			Enum  string `json:"enum"`
		} `json:"fields"`
	}
	if err := json.Unmarshal([]byte(output), &payload); err != nil {
		t.Fatalf("json unmarshal: %v", err)
	}
	if !strings.Contains(payload.Raw, redactedValue) {
		t.Fatalf("json raw = %q, want redacted value", payload.Raw)
	}
	assertFieldView(t, payload.Fields, 35, "MsgType", "D", "NewOrderSingle")
	assertFieldView(t, payload.Fields, 9001, "SessionToken", redactedValue, "")
}

func TestRendererCanShowSensitiveValuesWhenRequested(t *testing.T) {
	message := newTrace(t)
	renderer := newTestRenderer(true)

	raw, err := renderer.Render(message, FormatRaw)
	if err != nil {
		t.Fatalf("Render(raw) error = %v", err)
	}
	if !strings.Contains(raw, "553=account|") {
		t.Fatalf("raw output = %q, want username visible", raw)
	}
	if !strings.Contains(raw, "554=secret|") {
		t.Fatalf("raw output = %q, want password visible", raw)
	}
}

func TestRendererRejectsUnsupportedFormat(t *testing.T) {
	_, err := newTestRenderer(false).Render(newTrace(t), Format("xml"))
	if err == nil {
		t.Fatal("Render(xml) error = nil, want error")
	}
}

func newTrace(t *testing.T) trace.MessageTrace {
	t.Helper()
	message, err := trace.NewMessageTrace(trace.BuildOptions{
		TraceID:   "trace-test",
		Profile:   "uat",
		Direction: trace.DirectionOutbound,
		Raw:       readRenderMessage(t, "new_order_single.fix"),
	})
	if err != nil {
		t.Fatalf("NewMessageTrace() error = %v", err)
	}
	return message
}

func newTestRenderer(showSensitive bool) *Renderer {
	return NewRenderer(dictionary.New([]dictionary.CustomTag{
		{Tag: 9001, Name: "SessionToken", Type: "STRING", Sensitive: true},
	}), Options{RawDelimiter: "|", ShowSensitive: showSensitive})
}

func readRenderMessage(t *testing.T, name string) string {
	t.Helper()
	message := trace.DisplayRaw(readFixture(t, name), "|")
	return message
}

func readFixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "messages", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return string(data)
}

func assertFieldView(t *testing.T, fields []struct {
	Tag   int    `json:"tag"`
	Name  string `json:"name"`
	Value string `json:"value"`
	Enum  string `json:"enum"`
}, tag int, name string, value string, enum string) {
	t.Helper()
	for _, field := range fields {
		if field.Tag != tag {
			continue
		}
		if field.Name != name || field.Value != value || field.Enum != enum {
			t.Fatalf("field %d = %+v, want name=%q value=%q enum=%q", tag, field, name, value, enum)
		}
		return
	}
	t.Fatalf("field %d not found in %+v", tag, fields)
}
