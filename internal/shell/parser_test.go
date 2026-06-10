package shell

import (
	"strings"
	"testing"
	"time"
)

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
		{name: "order-stream-start", line: "order stream start --symbol AAPL --side buy --qty 100", want: CommandStreamStart},
		{name: "order-stream-stop", line: "order stream stop", want: CommandStreamStop},
		{name: "order-stream-status", line: "order stream status", want: CommandStreamStatus},
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

func TestParseOrderStreamStartFlags(t *testing.T) {
	command, err := Parse("order stream start --symbol AAPL --side buy --qty 100 --price 10.25 --ord-type limit --time-in-force day --tag 18=A --interval 250ms --count 3 --cl-ord-id-prefix S --cl-ord-id-mode random --start-seq 7 --side-mode alternate --symbol-seq AAPL,MSFT --qty-seq 100,200 --price-seq 10.25,10.30")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if command.StreamRequest.Order.Symbol != "AAPL" {
		t.Fatalf("Symbol = %q, want AAPL", command.StreamRequest.Order.Symbol)
	}
	if command.StreamRequest.Order.Side != "buy" {
		t.Fatalf("Side = %q, want buy", command.StreamRequest.Order.Side)
	}
	if command.StreamRequest.Order.OrderQty != "100" {
		t.Fatalf("OrderQty = %q, want 100", command.StreamRequest.Order.OrderQty)
	}
	if command.StreamRequest.Order.Price != "10.25" {
		t.Fatalf("Price = %q, want 10.25", command.StreamRequest.Order.Price)
	}
	if command.StreamRequest.Order.OrdType != "limit" {
		t.Fatalf("OrdType = %q, want limit", command.StreamRequest.Order.OrdType)
	}
	if command.StreamRequest.Order.TimeInForce != "day" {
		t.Fatalf("TimeInForce = %q, want day", command.StreamRequest.Order.TimeInForce)
	}
	if len(command.StreamRequest.Order.Tags) != 1 || command.StreamRequest.Order.Tags[0] != "18=A" {
		t.Fatalf("Tags = %#v, want one tag", command.StreamRequest.Order.Tags)
	}
	if command.StreamRequest.Interval != 250*time.Millisecond {
		t.Fatalf("Interval = %s, want 250ms", command.StreamRequest.Interval)
	}
	if command.StreamRequest.Count != 3 {
		t.Fatalf("Count = %d, want 3", command.StreamRequest.Count)
	}
	if command.StreamRequest.ClOrdIDPrefix != "S" {
		t.Fatalf("ClOrdIDPrefix = %q, want S", command.StreamRequest.ClOrdIDPrefix)
	}
	if command.StreamRequest.ClOrdIDMode != streamClOrdIDModeRandom {
		t.Fatalf("ClOrdIDMode = %q, want random", command.StreamRequest.ClOrdIDMode)
	}
	if command.StreamRequest.StartSeq != 7 {
		t.Fatalf("StartSeq = %d, want 7", command.StreamRequest.StartSeq)
	}
	if command.StreamRequest.SideMode != streamSideModeAlternate {
		t.Fatalf("SideMode = %q, want alternate", command.StreamRequest.SideMode)
	}
	if strings.Join(command.StreamRequest.SymbolSeq, ",") != "AAPL,MSFT" {
		t.Fatalf("SymbolSeq = %#v, want AAPL,MSFT", command.StreamRequest.SymbolSeq)
	}
	if strings.Join(command.StreamRequest.QtySeq, ",") != "100,200" {
		t.Fatalf("QtySeq = %#v, want 100,200", command.StreamRequest.QtySeq)
	}
	if strings.Join(command.StreamRequest.PriceSeq, ",") != "10.25,10.30" {
		t.Fatalf("PriceSeq = %#v, want 10.25,10.30", command.StreamRequest.PriceSeq)
	}
}

func TestParseOrderStreamStartDefaults(t *testing.T) {
	command, err := Parse("order stream start --symbol AAPL --side buy --qty 100")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if command.StreamRequest.Interval != time.Second {
		t.Fatalf("Interval = %s, want 1s", command.StreamRequest.Interval)
	}
	if command.StreamRequest.Count != 0 {
		t.Fatalf("Count = %d, want 0", command.StreamRequest.Count)
	}
	if command.StreamRequest.ClOrdIDPrefix != "STRM" {
		t.Fatalf("ClOrdIDPrefix = %q, want STRM", command.StreamRequest.ClOrdIDPrefix)
	}
	if command.StreamRequest.ClOrdIDMode != streamClOrdIDModeSequence {
		t.Fatalf("ClOrdIDMode = %q, want sequence", command.StreamRequest.ClOrdIDMode)
	}
	if command.StreamRequest.StartSeq != 1 {
		t.Fatalf("StartSeq = %d, want 1", command.StreamRequest.StartSeq)
	}
	if command.StreamRequest.SideMode != streamSideModeFixed {
		t.Fatalf("SideMode = %q, want fixed", command.StreamRequest.SideMode)
	}
}

func TestParseOrderStreamRejectsBadControlFlags(t *testing.T) {
	tests := []string{
		"order stream start --symbol AAPL --side buy --qty 100 --interval 0s",
		"order stream start --symbol AAPL --side buy --qty 100 --count -1",
		"order stream start --symbol AAPL --side buy --qty 100 --cl-ord-id-mode bad",
		"order stream start --symbol AAPL --side buy --qty 100 --side-mode bad",
		"order stream start --symbol AAPL --side buy --qty 100 --symbol-seq AAPL,,MSFT",
	}
	for _, line := range tests {
		t.Run(line, func(t *testing.T) {
			_, err := Parse(line)
			if err == nil {
				t.Fatal("Parse() error = nil, want error")
			}
		})
	}
}

func TestParseRejectsUnknownFlag(t *testing.T) {
	_, err := Parse("order new --bad value")
	if err == nil {
		t.Fatal("Parse() error = nil, want unknown flag error")
	}
}
