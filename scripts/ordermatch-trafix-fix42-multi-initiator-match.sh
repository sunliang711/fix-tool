#!/usr/bin/env bash

set -euo pipefail

# FIX.4.2 多 initiator TRAFiX 撮合联调脚本。
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ORDERMATCH_DIR="${ORDERMATCH_DIR:-$HOME/Sync/chief/quickfixgo-examples}"

random_run_token() {
	local token

	token="$(od -An -N4 -tx1 /dev/urandom 2>/dev/null | tr -d ' \n')"
	if [[ -z "$token" ]]; then
		token="$(printf '%04x%04x' "$RANDOM" "$RANDOM")"
	fi
	printf '%s' "${token:0:8}"
}

sanitize_run_label() {
	local value="$1"

	value="$(printf '%s' "$value" | tr '[:upper:]' '[:lower:]' | sed 's/[^a-z0-9._-]/-/g; s/-\{1,\}/-/g; s/^-//; s/-$//')"
	if [[ -z "$value" ]]; then
		value="trafix-fix42-multi"
	fi
	printf '%s' "$value"
}

RUN_ID="${RUN_ID:-$(date -u +%Y%m%d%H%M%S)}"
RUN_TOKEN="${RUN_TOKEN:-$(random_run_token)}"
RUN_LABEL="$(sanitize_run_label "${RUN_LABEL:-trafix-fix42-multi-$RUN_TOKEN}")"
OUTPUT_BASE_DIR="${OUTPUT_BASE_DIR:-$REPO_ROOT/tmp/ordermatch-trafix-fix42-multi-initiator-match}"
OUTPUT_DIR="${OUTPUT_DIR:-$OUTPUT_BASE_DIR/$RUN_LABEL-$RUN_ID}"
START_ORDERMATCH="${OM:-${START_ORDERMATCH:-1}}"
CASE_TIMEOUT_SECONDS="${CT:-${CASE_TIMEOUT_SECONDS:-20}}"

ORDERMATCH_BIN="$OUTPUT_DIR/qf"
SERVER_CONFIG="$OUTPUT_DIR/ordermatch-trafix-fix42-multi.cfg"
SERVER_LOG="$OUTPUT_DIR/ordermatch-server.log"
SERVER_TYPESCRIPT="$OUTPUT_DIR/ordermatch.typescript"
MATCH_HELPER="$OUTPUT_DIR/trafix_fix42_multi_initiator_match.go"
SERVER_PID=""
LOG_DIR_PRINTED=0
FINAL_LOG_DIR_PRINTED=0

log() {
	printf '[%s] %s\n' "$(date -u +%H:%M:%S)" "$*"
}

print_log_dir_once() {
	if [[ "$LOG_DIR_PRINTED" == "1" ]]; then
		return
	fi
	LOG_DIR_PRINTED=1
	log "Log directory: $OUTPUT_DIR"
	printf 'LOG_DIR=%s\n' "$OUTPUT_DIR"
}

print_final_log_dir_once() {
	if [[ "$FINAL_LOG_DIR_PRINTED" == "1" ]]; then
		return
	fi
	FINAL_LOG_DIR_PRINTED=1
	log "Final log directory: $OUTPUT_DIR"
	printf 'LOG_DIR=%s\n' "$OUTPUT_DIR"
}

fail() {
	log "ERROR: $*"
	exit 1
}

cleanup() {
	if [[ -n "$SERVER_PID" ]] && kill -0 "$SERVER_PID" 2>/dev/null; then
		log "Stopping ordermatch server pid=$SERVER_PID"
		kill "$SERVER_PID" 2>/dev/null || true
		wait "$SERVER_PID" 2>/dev/null || true
	fi
	print_final_log_dir_once
}

trap cleanup EXIT

require_command() {
	local command="$1"

	command -v "$command" >/dev/null 2>&1 || fail "Missing required command: $command"
}

require_file() {
	local path="$1"

	[[ -e "$path" ]] || fail "Missing required path: $path"
}

wait_for_port() {
	local host="$1"
	local port="$2"
	local attempts=50
	local i

	for ((i = 1; i <= attempts; i++)); do
		if (: >"/dev/tcp/$host/$port") >/dev/null 2>&1; then
			return 0
		fi
		sleep 0.2
	done
	return 1
}

write_server_config() {
	cat >"$SERVER_CONFIG" <<CONFIG
[DEFAULT]
SocketAcceptPort=5001
SenderCompID=ISLD
TargetCompID=TW1
ResetOnLogon=Y
FileLogPath=$OUTPUT_DIR/qf-log
ProtocolMode=trafix

[SESSION]
BeginString=FIX.4.2

[SESSION]
BeginString=FIX.4.2
TargetCompID=TW2
CONFIG
}

build_binaries() {
	if [[ "$START_ORDERMATCH" == "1" ]]; then
		log "Building quickfixgo-examples qf"
		(cd "$ORDERMATCH_DIR" && go build -o "$ORDERMATCH_BIN" .) || fail "Failed to build qf"
	fi
}

start_ordermatch_with_pty() {
	if script -q /dev/null true >/dev/null 2>&1; then
		script -q "$SERVER_TYPESCRIPT" env TERM=xterm-256color "$ORDERMATCH_BIN" ordermatch "$SERVER_CONFIG" >"$SERVER_LOG" 2>&1 &
		SERVER_PID="$!"
		return
	fi
	if script -q -c "true" /dev/null >/dev/null 2>&1; then
		script -q -c "TERM=xterm-256color \"$ORDERMATCH_BIN\" ordermatch \"$SERVER_CONFIG\"" "$SERVER_TYPESCRIPT" >"$SERVER_LOG" 2>&1 &
		SERVER_PID="$!"
		return
	fi
	fail "script command cannot allocate a pseudo terminal on this system"
}

start_ordermatch() {
	[[ "$START_ORDERMATCH" == "1" ]] || return 0

	require_command script
	if (: >/dev/tcp/127.0.0.1/5001) >/dev/null 2>&1; then
		fail "127.0.0.1:5001 is already in use; stop the existing ordermatch server or run without OM=1"
	fi

	log "Starting ordermatch server in TRAFiX FIX.4.2 multi-initiator mode"
	start_ordermatch_with_pty
	wait_for_port 127.0.0.1 5001 || fail "ordermatch did not listen on 127.0.0.1:5001"
	log "ordermatch server is ready pid=$SERVER_PID"
}

write_match_helper() {
	cat >"$MATCH_HELPER" <<'GO'
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	appconfig "fix-tool/internal/config"
	"fix-tool/internal/fixsession"
	ftmessage "fix-tool/internal/message"

	"github.com/quickfixgo/quickfix"
	"github.com/rs/zerolog"
)

type observedEvent struct {
	profile string
	event   fixsession.Event
	fields  map[string]string
	raw     string
}

type client struct {
	profile string
	manager *fixsession.QuickFIXManager
	events  []observedEvent
	writer  io.Writer
}

func main() {
	var outputDir string
	var symbol string
	var host string
	var port int
	var timeoutSeconds int

	flag.StringVar(&outputDir, "output-dir", "", "output directory")
	flag.StringVar(&symbol, "symbol", "", "test symbol")
	flag.StringVar(&host, "host", "127.0.0.1", "ordermatch host")
	flag.IntVar(&port, "port", 5001, "ordermatch port")
	flag.IntVar(&timeoutSeconds, "timeout", 20, "timeout seconds")
	flag.Parse()

	if outputDir == "" {
		fatalf("missing --output-dir")
	}
	if symbol == "" {
		fatalf("missing --symbol")
	}

	eventPath := filepath.Join(outputDir, "events.tsv")
	summaryPath := filepath.Join(outputDir, "summary.tsv")
	eventFile, err := os.Create(eventPath)
	if err != nil {
		fatalf("create events file: %v", err)
	}
	defer eventFile.Close()
	summaryFile, err := os.Create(summaryPath)
	if err != nil {
		fatalf("create summary file: %v", err)
	}
	defer summaryFile.Close()
	fmt.Fprintln(eventFile, "time\tprofile\tevent_type\tmsg_type\tcl_ord_id\texec_type\tord_status\tleaves_qty\tcum_qty\tavg_px\tlast_qty\tlast_px\traw")
	fmt.Fprintln(summaryFile, "check\tstatus\texpected\tobserved")

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSeconds)*time.Second)
	defer cancel()

	tw1 := newClient("TW1", host, port, eventFile)
	tw2 := newClient("TW2", host, port, eventFile)
	defer stopClient(tw2)
	defer stopClient(tw1)

	if err := tw1.manager.Start(ctx); err != nil {
		writeFailure(summaryFile, "tw1_start", "TW1 initiator starts", err.Error())
		fatalf("start TW1: %v", err)
	}
	if err := tw2.manager.Start(ctx); err != nil {
		writeFailure(summaryFile, "tw2_start", "TW2 initiator starts", err.Error())
		fatalf("start TW2: %v", err)
	}
	if err := tw1.waitLogon(ctx); err != nil {
		writeFailure(summaryFile, "tw1_logon", "TW1 logon succeeds", err.Error())
		fatalf("wait TW1 logon: %v", err)
	}
	if err := tw2.waitLogon(ctx); err != nil {
		writeFailure(summaryFile, "tw2_logon", "TW2 logon succeeds", err.Error())
		fatalf("wait TW2 logon: %v", err)
	}

	buyID := "TFX42-MI-BUY"
	sellID := "TFX42-MI-SELL"
	buy, err := newTrafixOrder(buyID, symbol, ftmessage.SideBuy, "100", "10.00")
	if err != nil {
		writeFailure(summaryFile, "build_buy", "build TW1 TRAFiX buy order", err.Error())
		fatalf("build buy: %v", err)
	}
	sell, err := newTrafixOrder(sellID, symbol, ftmessage.SideSell, "40", "9.50")
	if err != nil {
		writeFailure(summaryFile, "build_sell", "build TW2 TRAFiX sell order", err.Error())
		fatalf("build sell: %v", err)
	}

	if err := tw1.manager.Session().Send(buy); err != nil {
		writeFailure(summaryFile, "tw1_send_buy", "TW1 sends buy order", err.Error())
		fatalf("send TW1 buy: %v", err)
	}
	buyNew, err := tw1.waitExecutionReport(ctx, map[string]string{"11": buyID, "150": "0", "39": "0"})
	if err != nil {
		writeFailure(summaryFile, "tw1_buy_new", "TW1 receives New for buy", err.Error())
		fatalf("wait TW1 buy new: %v", err)
	}

	if err := tw2.manager.Session().Send(sell); err != nil {
		writeFailure(summaryFile, "tw2_send_sell", "TW2 sends sell order", err.Error())
		fatalf("send TW2 sell: %v", err)
	}
	sellNew, err := tw2.waitExecutionReport(ctx, map[string]string{"11": sellID, "150": "0", "39": "0"})
	if err != nil {
		writeFailure(summaryFile, "tw2_sell_new", "TW2 receives New for sell", err.Error())
		fatalf("wait TW2 sell new: %v", err)
	}
	buyFill, err := tw1.waitExecutionReport(ctx, map[string]string{"11": buyID, "150": "1", "39": "1"})
	if err != nil {
		writeFailure(summaryFile, "tw1_buy_partial_fill", "TW1 receives partial fill for resting buy", err.Error())
		fatalf("wait TW1 buy fill: %v", err)
	}
	sellFill, err := tw2.waitExecutionReport(ctx, map[string]string{"11": sellID, "150": "2", "39": "2"})
	if err != nil {
		writeFailure(summaryFile, "tw2_sell_filled", "TW2 receives filled report for aggressive sell", err.Error())
		fatalf("wait TW2 sell fill: %v", err)
	}

	failures := 0
	failures += assertFields(summaryFile, "tw1_buy_new", "FIX.4.2 TW1 buy New leaves=100 LastQty/LastPx=0", buyNew, map[string]string{
		"8": "FIX.4.2", "35": "8", "49": "ISLD", "56": "TW1", "11": buyID, "55": symbol,
		"150": "0", "39": "0", "151": "100.00", "14": "0.00", "6": "0.00", "32": "0.00", "31": "0.00",
	})
	failures += assertFields(summaryFile, "tw2_sell_new", "FIX.4.2 TW2 sell New leaves=40 LastQty/LastPx=0", sellNew, map[string]string{
		"8": "FIX.4.2", "35": "8", "49": "ISLD", "56": "TW2", "11": sellID, "55": symbol,
		"150": "0", "39": "0", "151": "40.00", "14": "0.00", "6": "0.00", "32": "0.00", "31": "0.00",
	})
	failures += assertFields(summaryFile, "tw1_buy_partial_fill", "resting buy gets partial fill 40 @ 10.00 and leaves 60", buyFill, map[string]string{
		"8": "FIX.4.2", "35": "8", "49": "ISLD", "56": "TW1", "11": buyID, "55": symbol,
		"150": "1", "39": "1", "151": "60.00", "14": "40.00", "6": "10.00", "32": "40.00", "31": "10.00",
	})
	failures += assertFields(summaryFile, "tw2_sell_filled", "aggressive sell gets full fill 40 @ 10.00 and leaves 0", sellFill, map[string]string{
		"8": "FIX.4.2", "35": "8", "49": "ISLD", "56": "TW2", "11": sellID, "55": symbol,
		"150": "2", "39": "2", "151": "0.00", "14": "40.00", "6": "10.00", "32": "40.00", "31": "10.00",
	})

	fmt.Printf("SUMMARY_FILE=%s\n", summaryPath)
	fmt.Printf("EVENT_FILE=%s\n", eventPath)
	if failures > 0 {
		fatalf("%d semantic checks failed", failures)
	}
}

func newClient(senderCompID string, host string, port int, writer io.Writer) *client {
	profile := appconfig.ProfileConfig{
		Name:              "trafix-fix42-" + strings.ToLower(senderCompID),
		BeginString:       "FIX.4.2",
		SenderCompID:      senderCompID,
		TargetCompID:      "ISLD",
		Host:              host,
		Port:              port,
		HeartbeatInterval: "30s",
		ResetOnLogon:      true,
	}
	manager, err := fixsession.NewManagerWithOptions(profile, zerolog.Nop(), fixsession.ManagerOptions{})
	if err != nil {
		fatalf("create %s initiator: %v", senderCompID, err)
	}
	return &client{profile: senderCompID, manager: manager, writer: writer}
}

func stopClient(c *client) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = c.manager.Stop(ctx)
}

func newTrafixOrder(clOrdID string, symbol string, side string, qty string, price string) (*quickfix.Message, error) {
	return ftmessage.BuildNewOrderSingle(ftmessage.NewOrderSingleRequest{
		ClOrdID:     clOrdID,
		Symbol:      symbol,
		Side:        side,
		OrderQty:    qty,
		Price:       price,
		OrdType:     ftmessage.OrdTypeLimit,
		TimeInForce: ftmessage.TimeInForceDay,
		Tags:        []string{"47=A"},
	})
}

func (c *client) waitLogon(ctx context.Context) error {
	for {
		event, err := c.nextEvent(ctx)
		if err != nil {
			return err
		}
		if event.event.Type == fixsession.EventLogon {
			return nil
		}
	}
}

func (c *client) waitExecutionReport(ctx context.Context, want map[string]string) (observedEvent, error) {
	for i := 0; i < len(c.events); i++ {
		event := c.events[i]
		if matchFields(event.fields, want) {
			c.events = append(c.events[:i], c.events[i+1:]...)
			return event, nil
		}
	}

	for {
		event, err := c.nextEvent(ctx)
		if err != nil {
			return observedEvent{}, err
		}
		if event.fields["35"] != "8" {
			continue
		}
		if matchFields(event.fields, want) {
			return event, nil
		}
		c.events = append(c.events, event)
	}
}

func (c *client) nextEvent(ctx context.Context) (observedEvent, error) {
	select {
	case event, ok := <-c.manager.Events():
		if !ok {
			return observedEvent{}, fmt.Errorf("%s event stream closed", c.profile)
		}
		observed := observedEvent{
			profile: c.profile,
			event:   event,
			fields:  parseFIX(event.Raw()),
			raw:     printableFIX(event.Raw()),
		}
		writeEvent(c.writer, observed)
		return observed, nil
	case <-ctx.Done():
		return observedEvent{}, ctx.Err()
	}
}

func parseFIX(raw string) map[string]string {
	fields := map[string]string{}
	for _, item := range strings.FieldsFunc(raw, func(r rune) bool { return r == '\x01' || r == '|' }) {
		if item == "" {
			continue
		}
		key, value, ok := strings.Cut(item, "=")
		if !ok {
			continue
		}
		fields[key] = value
	}
	return fields
}

func printableFIX(raw string) string {
	return strings.ReplaceAll(strings.TrimSpace(raw), "\x01", "|")
}

func writeEvent(writer io.Writer, event observedEvent) {
	fields := event.fields
	fmt.Fprintf(writer, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
		event.event.Time.Format(time.RFC3339Nano),
		event.profile,
		event.event.Type,
		fields["35"],
		fields["11"],
		fields["150"],
		fields["39"],
		fields["151"],
		fields["14"],
		fields["6"],
		fields["32"],
		fields["31"],
		event.raw,
	)
}

func matchFields(fields map[string]string, want map[string]string) bool {
	for key, value := range want {
		if fields[key] != value {
			return false
		}
	}
	return true
}

func assertFields(writer io.Writer, name string, expected string, event observedEvent, want map[string]string) int {
	missing := make([]string, 0)
	for key, value := range want {
		if event.fields[key] != value {
			missing = append(missing, fmt.Sprintf("%s=%s(got %s)", key, value, event.fields[key]))
		}
	}
	if len(missing) == 0 {
		fmt.Fprintf(writer, "%s\t0\t%s\t%s\n", name, expected, summarize(event.fields))
		return 0
	}
	fmt.Fprintf(writer, "%s\t1\t%s\tmissing %s observed %s\n", name, expected, strings.Join(missing, ","), summarize(event.fields))
	return 1
}

func summarize(fields map[string]string) string {
	keys := []string{"8", "35", "49", "56", "11", "55", "150", "39", "151", "14", "6", "32", "31", "60"}
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+fields[key])
	}
	return strings.Join(parts, ",")
}

func writeFailure(writer io.Writer, name string, expected string, observed string) {
	fmt.Fprintf(writer, "%s\t1\t%s\t%s\n", name, expected, observed)
}

func fatalf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "ERROR: "+format+"\n", args...)
	os.Exit(1)
}
GO
}

prepare() {
	require_command go
	require_file "$ORDERMATCH_DIR/config/ordermatch.cfg"
	mkdir -p "$OUTPUT_DIR/qf-log"
	print_log_dir_once
	write_server_config
	write_match_helper
	build_binaries
}

main() {
	local symbol="TFX42MI${RUN_TOKEN:0:6}"

	prepare
	start_ordermatch

	log "Running FIX.4.2 multi-initiator TRAFiX match check symbol=$symbol"
	(cd "$REPO_ROOT" && go run "$MATCH_HELPER" --output-dir "$OUTPUT_DIR" --symbol "$symbol" --timeout "$CASE_TIMEOUT_SECONDS")
	log "FIX.4.2 multi-initiator TRAFiX match check passed"
}

main "$@"
