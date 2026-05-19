package shell

import "testing"

func TestParseCommands(t *testing.T) {
	tests := []struct {
		name string
		line string
		want CommandKind
	}{
		{name: "logon", line: "logon", want: CommandLogon},
		{name: "logout", line: "logout", want: CommandLogout},
		{name: "heartbeat", line: "heartbeat", want: CommandHeartbeat},
		{name: "help", line: "help", want: CommandHelp},
		{name: "question-mark", line: "?", want: CommandHelp},
		{name: "test-request", line: "test-request --id ping-001", want: CommandTestRequest},
		{name: "order-new", line: "order new --symbol AAPL --side buy --qty 100 --price 10.25 --tag 59=0", want: CommandOrderNew},
		{name: "order-cancel", line: "order cancel --orig-cl-ord-id C001 --symbol AAPL --side sell", want: CommandOrderCancel},
		{name: "order-replace", line: "order replace --orig-cl-ord-id C001 --symbol AAPL --side buy --qty 50 --price=10.30", want: CommandOrderReplace},
		{name: "trace-list", line: "trace list", want: CommandTraceList},
		{name: "save-start", line: "save transcript.log", want: CommandSaveStart},
		{name: "save-stop", line: "save stop", want: CommandSaveStop},
		{name: "save-status", line: "save status", want: CommandSaveStatus},
		{name: "exit", line: "exit", want: CommandExit},
		{name: "quit", line: "quit", want: CommandExit},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.line)
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}
			if got.Kind != tt.want {
				t.Fatalf("Kind = %q, want %q", got.Kind, tt.want)
			}
		})
	}
}

func TestParseSavePath(t *testing.T) {
	command, err := Parse("save transcript.log")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if command.SavePath != "transcript.log" {
		t.Fatalf("SavePath = %q, want transcript.log", command.SavePath)
	}
}

func TestParseOrderNewFlags(t *testing.T) {
	command, err := Parse("order new --cl-ord-id C001 --symbol AAPL --side buy --qty 100 --price 10.25 --ord-type limit --time-in-force day --tag 18=A --tag 59=0")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if command.NewRequest.ClOrdID != "C001" {
		t.Fatalf("ClOrdID = %q, want C001", command.NewRequest.ClOrdID)
	}
	if command.NewRequest.Symbol != "AAPL" {
		t.Fatalf("Symbol = %q, want AAPL", command.NewRequest.Symbol)
	}
	if command.NewRequest.Side != "buy" {
		t.Fatalf("Side = %q, want buy", command.NewRequest.Side)
	}
	if command.NewRequest.OrderQty != "100" {
		t.Fatalf("OrderQty = %q, want 100", command.NewRequest.OrderQty)
	}
	if command.NewRequest.Price != "10.25" {
		t.Fatalf("Price = %q, want 10.25", command.NewRequest.Price)
	}
	if command.NewRequest.OrdType != "limit" {
		t.Fatalf("OrdType = %q, want limit", command.NewRequest.OrdType)
	}
	if command.NewRequest.TimeInForce != "day" {
		t.Fatalf("TimeInForce = %q, want day", command.NewRequest.TimeInForce)
	}
	if len(command.NewRequest.Tags) != 2 || command.NewRequest.Tags[0] != "18=A" || command.NewRequest.Tags[1] != "59=0" {
		t.Fatalf("Tags = %#v, want two tags", command.NewRequest.Tags)
	}
}

func TestParseRejectsUnknownFlag(t *testing.T) {
	_, err := Parse("order new --bad value")
	if err == nil {
		t.Fatal("Parse() error = nil, want unknown flag error")
	}
}
