package scenario

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadYAMLScenario(t *testing.T) {
	path := filepath.Join(t.TempDir(), "scenario.yaml")
	data := []byte(`
name: smoke
steps:
  - name: logon
    action: logon
    wait:
      msg_type: A
    assert:
      - field: msg_type
        equals: A
  - name: new-order
    action: order.new
    input:
      cl_ord_id: C001
      symbol: AAPL
      side: buy
      qty: "100"
      price: "10.25"
      ord_type: limit
      tags:
        - 10001=desk-a
    wait:
      msg_type: "8"
    assert:
      - field: exec_type
        in: ["0", "4"]
`)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	scenarioValue, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if scenarioValue.Name != "smoke" {
		t.Fatalf("scenario name = %q, want smoke", scenarioValue.Name)
	}
	if len(scenarioValue.Steps) != 2 {
		t.Fatalf("steps = %d, want 2", len(scenarioValue.Steps))
	}
	if scenarioValue.Steps[1].Input.Tags[0] != "10001=desk-a" {
		t.Fatalf("tag = %q, want custom tag", scenarioValue.Steps[1].Input.Tags[0])
	}
	if scenarioValue.Steps[1].Assert[0].In[1] != "4" {
		t.Fatalf("assert in = %#v, want enum values", scenarioValue.Steps[1].Assert[0].In)
	}
}

func TestSampleScenarioLoads(t *testing.T) {
	scenarioValue, err := Load(filepath.Clean("../../testdata/scenarios/order-lifecycle.yaml"))
	if err != nil {
		t.Fatalf("Load() sample error = %v", err)
	}
	if scenarioValue.Name != "order-lifecycle" {
		t.Fatalf("sample name = %q, want order-lifecycle", scenarioValue.Name)
	}
	if len(scenarioValue.Steps) != 4 {
		t.Fatalf("sample steps = %d, want 4", len(scenarioValue.Steps))
	}
}

func TestLoadRejectsUnsupportedFileType(t *testing.T) {
	_, err := Load("scenario.toml")
	if err == nil || !strings.Contains(err.Error(), "only YAML is supported") {
		t.Fatalf("Load() error = %v, want unsupported YAML error", err)
	}
}
