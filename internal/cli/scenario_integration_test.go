package cli

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"fix-tool/internal/mockfix"

	"github.com/rs/zerolog"
)

func TestScenarioRunWithMockAcceptor(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	server, err := mockfix.NewAcceptor(mockfix.Options{
		ProfileName:     "mock-cli",
		InitiatorCompID: "CLI-SENDER",
		AcceptorCompID:  "CLI-TARGET",
	})
	if err != nil {
		t.Fatalf("NewAcceptor() error = %v", err)
	}
	if err := server.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	t.Cleanup(func() {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer stopCancel()
		if err := server.Stop(stopCtx); err != nil {
			t.Fatalf("Stop() error = %v", err)
		}
	})

	configFile := writeMockConfig(t, server)
	scenarioFile := filepath.Clean("../../testdata/scenarios/mock-acceptor-basic.yaml")
	var out bytes.Buffer
	var errOut bytes.Buffer
	command := NewRootCommand(Args{"--config", configFile, "run", scenarioFile, "--json"}, IO{
		Out:    &out,
		ErrOut: &errOut,
	}, zerolog.Nop())

	if err := command.ExecuteContext(ctx); err != nil {
		t.Fatalf("ExecuteContext() error = %v, stderr = %s", err, errOut.String())
	}
	if !strings.Contains(out.String(), `"scenario": "mock-acceptor-basic"`) {
		t.Fatalf("scenario output = %s, want scenario name", out.String())
	}
	if !strings.Contains(out.String(), `"status": "passed"`) {
		t.Fatalf("scenario output = %s, want passed status", out.String())
	}
}

func TestCheckLogonWritesFIXMessagesToStdout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	server, err := mockfix.NewAcceptor(mockfix.Options{
		ProfileName:     "mock-cli-logon",
		InitiatorCompID: "CLI-LOGON-SENDER",
		AcceptorCompID:  "CLI-LOGON-TARGET",
	})
	if err != nil {
		t.Fatalf("NewAcceptor() error = %v", err)
	}
	if err := server.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	t.Cleanup(func() {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer stopCancel()
		if err := server.Stop(stopCtx); err != nil {
			t.Fatalf("Stop() error = %v", err)
		}
	})

	configFile := writeMockConfig(t, server)
	var out bytes.Buffer
	var errOut bytes.Buffer
	command := NewRootCommand(Args{"--config", configFile, "check", "logon"}, IO{
		Out:    &out,
		ErrOut: &errOut,
	}, zerolog.Nop())

	if err := command.ExecuteContext(ctx); err != nil {
		t.Fatalf("ExecuteContext() error = %v, stderr = %s", err, errOut.String())
	}
	output := out.String()
	for _, want := range []string{
		"===> Outgoing FIX Msg: ===>",
		"<=== Incoming FIX Msg: <===",
		"Session:     FIX.4.4:CLI-LOGON-SENDER->CLI-LOGON-TARGET",
		"Content:",
		"  Raw:",
		"  Pretty:",
		"35=A|",
		"MsgType:Logon",
		"35 = A",
		"35=5|",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("stdout = %s, want %q", output, want)
		}
	}
	if strings.Contains(output, "TraceID") {
		t.Fatalf("stdout = %s, want formatted FIX messages without rendered trace details", output)
	}
	logOutput := errOut.String()
	for _, unwanted := range []string{"Outgoing FIX Msg", "Incoming FIX Msg", "-> Logon(A)", "<- Logon(A)"} {
		if strings.Contains(logOutput, unwanted) {
			t.Fatalf("stderr = %s, want no direct FIX message output %q", logOutput, unwanted)
		}
	}
	if !strings.Contains(logOutput, "fix session manager stopped") {
		t.Fatalf("stderr = %s, want manager stop log", logOutput)
	}
}

func writeMockConfig(t *testing.T, server *mockfix.Acceptor) string {
	t.Helper()
	profile := server.ProfileConfig()
	path := filepath.Join(t.TempDir(), "mock-config.toml")
	content := fmt.Sprintf(`
[profile]
name = %q
begin_string = %q
sender_comp_id = %q
target_comp_id = %q
username = ""
password = ""
host = %q
port = %d
heartbeat_interval = %q
reset_on_logon = true

[profile.tls]
enabled = false
insecure_skip_verify = false
ca_file = ""
cert_file = ""
key_file = ""
`, profile.Name, profile.BeginString, profile.SenderCompID, profile.TargetCompID, profile.Host, profile.Port, profile.HeartbeatInterval)
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	return path
}
