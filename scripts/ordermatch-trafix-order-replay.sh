#!/usr/bin/env bash

set -euo pipefail

# TRAFiX 专用联调脚本：使用 raw send 明确发送 TRAFiX 必填 tag。
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
		value="trafix"
	fi
	printf '%s' "$value"
}

RUN_ID="${RUN_ID:-$(date -u +%Y%m%d%H%M%S)}"
RUN_TOKEN="${RUN_TOKEN:-$(random_run_token)}"
RUN_LABEL="$(sanitize_run_label "${RUN_LABEL:-trafix-$RUN_TOKEN}")"
OUTPUT_BASE_DIR="${OUTPUT_BASE_DIR:-$REPO_ROOT/tmp/ordermatch-trafix-order-replay}"
OUTPUT_DIR="${OUTPUT_DIR:-$OUTPUT_BASE_DIR/$RUN_LABEL-$RUN_ID}"
START_ORDERMATCH="${OM:-${START_ORDERMATCH:-0}}"
CASE_TIMEOUT_SECONDS="${CT:-${CASE_TIMEOUT_SECONDS:-12}}"

FIX_TOOL_BIN="$OUTPUT_DIR/fix-tool"
ORDERMATCH_BIN="$OUTPUT_DIR/qf"
CLIENT_CONFIG="$OUTPUT_DIR/ordermatch-trafix-client.toml"
SERVER_CONFIG="$OUTPUT_DIR/ordermatch-trafix.cfg"
SERVER_LOG="$OUTPUT_DIR/ordermatch-trafix-server.log"
SERVER_TYPESCRIPT="$OUTPUT_DIR/ordermatch-trafix.typescript"
SUMMARY_FILE="$OUTPUT_DIR/summary.tsv"
CONNECTIVITY_LOG="$OUTPUT_DIR/connectivity.log"
SERVER_PID=""
LOG_DIR_PRINTED=0
FINAL_LOG_DIR_PRINTED=0
FAILED_CASES=0

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

write_client_config() {
	cat >"$CLIENT_CONFIG" <<'CONFIG'
[app]
name = "fix-tool"

[log]
level = "warn"
format = "console"

[profile]
name = "ordermatch-trafix"
begin_string = "FIX.4.4"
sender_comp_id = "TW"
target_comp_id = "ISLD"
username = ""
password = ""
host = "127.0.0.1"
port = 5001
heartbeat_interval = "30s"
reset_on_logon = true
data_dictionary = ""
transport_data_dictionary = ""
app_data_dictionary = ""

[profile.tls]
enabled = false
insecure_skip_verify = false
ca_file = ""
cert_file = ""
key_file = ""

[output]
format = "table"
raw_delimiter = "|"
redact_sensitive = true
CONFIG
}

write_server_config() {
	cat >"$SERVER_CONFIG" <<'CONFIG'
[DEFAULT]
SocketAcceptPort=5001
SenderCompID=ISLD
TargetCompID=TW
ResetOnLogon=Y
FileLogPath=tmp
ProtocolMode=trafix

[SESSION]
BeginString=FIX.4.0

[SESSION]
BeginString=FIX.4.1

[SESSION]
BeginString=FIX.4.2

[SESSION]
BeginString=FIX.4.3

[SESSION]
BeginString=FIX.4.4

[SESSION]
BeginString=FIXT.1.1
DefaultApplVerID=7
CONFIG
}

build_binaries() {
	log "Building fix-tool"
	(cd "$REPO_ROOT" && go build -o "$FIX_TOOL_BIN" ./cmd/fix-tool) || fail "Failed to build fix-tool"

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

	log "Starting ordermatch server in TRAFiX mode"
	start_ordermatch_with_pty
	wait_for_port 127.0.0.1 5001 || fail "ordermatch did not listen on 127.0.0.1:5001"
	log "ordermatch server is ready pid=$SERVER_PID"
}

run_with_timeout() {
	local timeout_seconds="$1"
	shift

	"$@" &
	local pid="$!"
	local timeout_marker="$OUTPUT_DIR/.timeout-$pid"
	local timer_pid
	local status=0

	(
		sleep "$timeout_seconds"
		if kill -0 "$pid" 2>/dev/null; then
			: >"$timeout_marker"
			kill "$pid" 2>/dev/null || true
		fi
	) &
	timer_pid="$!"

	wait "$pid" || status="$?"
	kill "$timer_pid" 2>/dev/null || true
	wait "$timer_pid" 2>/dev/null || true

	if [[ -f "$timeout_marker" ]]; then
		rm -f "$timeout_marker"
		return 124
	fi
	return "$status"
}

check_ordermatch_connectivity() {
	local status

	log "Checking ordermatch TCP connectivity: 127.0.0.1:5001"
	wait_for_port 127.0.0.1 5001 || fail "ordermatch is not reachable at 127.0.0.1:5001; start the service first or run with OM=1"
	log "Checking ordermatch FIX logon"
	if run_with_timeout "$CASE_TIMEOUT_SECONDS" "$FIX_TOOL_BIN" --config "$CLIENT_CONFIG" check logon >"$CONNECTIVITY_LOG" 2>&1; then
		status=0
	else
		status="$?"
	fi
	if [[ "$status" != "0" ]]; then
		fail "ordermatch FIX logon check failed status=$status log=$CONNECTIVITY_LOG"
	fi
	log "ordermatch connectivity check passed log=$CONNECTIVITY_LOG"
}

extract_raw_messages() {
	local log_file="$1"
	sed -n '/Raw:/{n;s/^[[:space:]]*//;p;}' "$log_file"
}

field_value() {
	local raw="$1"
	local tag="$2"

	awk -v want="$tag" '{
		n = split($0, fields, "|")
		for (i = 1; i <= n; i++) {
			split(fields[i], pair, "=")
			if (pair[1] == want) {
				print pair[2]
				exit
			}
		}
	}' <<<"$raw"
}

summarize_observed() {
	local log_file="$1"
	local raw
	local msg_type
	local cl_ord_id
	local orig_cl_ord_id
	local order_id
	local symbol
	local symbol_sfx
	local exec_type
	local ord_status
	local text
	local leaves_qty
	local cum_qty
	local avg_px
	local last_qty
	local last_px
	local ref_tag
	local ref_msg_type
	local reject_reason
	local result=""

	while IFS= read -r raw; do
		msg_type="$(field_value "$raw" 35)"
		case "$msg_type" in
			8|9|3|j|P)
				cl_ord_id="$(field_value "$raw" 11)"
				orig_cl_ord_id="$(field_value "$raw" 41)"
				order_id="$(field_value "$raw" 37)"
				symbol="$(field_value "$raw" 55)"
				symbol_sfx="$(field_value "$raw" 65)"
				exec_type="$(field_value "$raw" 150)"
				ord_status="$(field_value "$raw" 39)"
				text="$(field_value "$raw" 58)"
				leaves_qty="$(field_value "$raw" 151)"
				cum_qty="$(field_value "$raw" 14)"
				avg_px="$(field_value "$raw" 6)"
				last_qty="$(field_value "$raw" 32)"
				last_px="$(field_value "$raw" 31)"
				ref_tag="$(field_value "$raw" 371)"
				ref_msg_type="$(field_value "$raw" 372)"
				reject_reason="$(field_value "$raw" 373)"
				result="${result}35=${msg_type},11=${cl_ord_id},41=${orig_cl_ord_id},37=${order_id},55=${symbol},65=${symbol_sfx},150=${exec_type},39=${ord_status},58=${text},151=${leaves_qty},14=${cum_qty},6=${avg_px},32=${last_qty},31=${last_px},371=${ref_tag},372=${ref_msg_type},373=${reject_reason}; "
				;;
		esac
	done < <(extract_raw_messages "$log_file")

	if [[ -z "$result" ]]; then
		result="no application/admin response captured"
	fi
	printf '%s' "$result"
}

transact_time() {
	date -u +%Y%m%d-%H:%M:%S.000
}

run_case() {
	local name="$1"
	local expected="$2"
	local expect_regex="$3"
	local symbol="$4"
	shift 4

	local log_file="$OUTPUT_DIR/${name}.log"
	local status
	local observed

	log "Running case: $name"
	if run_with_timeout "$CASE_TIMEOUT_SECONDS" "$FIX_TOOL_BIN" --config "$CLIENT_CONFIG" "$@" >"$log_file" 2>&1; then
		status=0
	else
		status="$?"
	fi
	observed="$(summarize_observed "$log_file")"

	printf '%s\t%s\t%s\t%s\t%s\t%s\n' "$name" "$status" "$symbol" "$expected" "$log_file" "$observed" >>"$SUMMARY_FILE"
	if [[ -n "$expect_regex" && ! "$observed" =~ $expect_regex ]]; then
		FAILED_CASES=$((FAILED_CASES + 1))
		log "Assertion failed: $name expected_regex=$expect_regex observed=$observed"
	fi
	log "Case finished: $name status=$status log=$log_file"
}

prepare() {
	require_command go
	require_command awk
	require_command sed
	require_file "$ORDERMATCH_DIR/config/ordermatch.cfg"
	mkdir -p "$OUTPUT_DIR"
	print_log_dir_once
	write_client_config
	write_server_config
	build_binaries
	printf 'case\tstatus\tsymbol\texpected\tlog_file\tobserved\n' >"$SUMMARY_FILE"
}

main() {
	local suffix="${RUN_TOKEN:0:6}"
	local sym_accept="TFXA$suffix"
	local sym_rule80a="TFXB$suffix"
	local sym_cancel="TFXC$suffix"
	local sym_replace="TFXD$suffix"
	local sym_short="TFXE$suffix"
	local sym_option="TFXF$suffix"
	local sym_alloc="TFXG$suffix"
	local sym_sfx="TFXH$suffix"

	prepare
	start_ordermatch
	check_ordermatch_connectivity

	run_case "01_new_limit_accepts_trafix_required_tags" "ExecutionReport New" "35=8.*39=0" "$sym_accept" \
		raw send --msg-type D \
			--tag "11=TFX-$RUN_ID-001" --tag "21=1" --tag "38=100" --tag "40=2" --tag "44=10.00" \
			--tag "47=A" --tag "54=1" --tag "55=$sym_accept" --tag "59=0" --tag "60=$(transact_time)"

	run_case "02_new_missing_rule80a_rejects" "Session Reject RefTagID=47" "35=3.*371=47" "$sym_rule80a" \
		raw send --msg-type D \
			--tag "11=TFX-$RUN_ID-002" --tag "21=1" --tag "38=100" --tag "40=2" --tag "44=10.00" \
			--tag "54=1" --tag "55=$sym_rule80a" --tag "59=0" --tag "60=$(transact_time)"

	run_case "03_cancel_seed_buy" "ExecutionReport New before cancel" "35=8.*39=0" "$sym_cancel" \
		raw send --msg-type D \
			--tag "11=TFX-$RUN_ID-003" --tag "21=1" --tag "38=25" --tag "40=2" --tag "44=7.00" \
			--tag "47=A" --tag "54=1" --tag "55=$sym_cancel" --tag "59=0" --tag "60=$(transact_time)"
	run_case "04_cancel_with_order_qty_accepts" "ExecutionReport Canceled" "35=8.*39=4" "$sym_cancel" \
		raw send --msg-type F \
			--tag "41=TFX-$RUN_ID-003" --tag "11=TFX-$RUN_ID-004" --tag "37=TFX-$RUN_ID-003" \
			--tag "38=25" --tag "54=1" --tag "55=$sym_cancel" --tag "60=$(transact_time)"

	run_case "05_cancel_missing_order_qty_rejects" "Session Reject RefTagID=38" "35=3.*371=38" "$sym_cancel" \
		raw send --msg-type F \
			--tag "41=TFX-$RUN_ID-404" --tag "11=TFX-$RUN_ID-005" \
			--tag "54=1" --tag "55=$sym_cancel" --tag "60=$(transact_time)"

	run_case "06_replace_seed_buy" "ExecutionReport New before replace" "35=8.*39=0" "$sym_replace" \
		raw send --msg-type D \
			--tag "11=TFX-$RUN_ID-006" --tag "21=1" --tag "38=30" --tag "40=2" --tag "44=8.00" \
			--tag "47=A" --tag "54=1" --tag "55=$sym_replace" --tag "59=0" --tag "60=$(transact_time)"
	run_case "07_replace_with_openclose_accepts" "ExecutionReport Replaced" "35=8.*39=5" "$sym_replace" \
		raw send --msg-type G \
			--tag "41=TFX-$RUN_ID-006" --tag "11=TFX-$RUN_ID-007" --tag "37=TFX-$RUN_ID-006" \
			--tag "21=1" --tag "38=30" --tag "40=2" --tag "44=8.50" --tag "54=1" \
			--tag "55=$sym_replace" --tag "59=0" --tag "60=$(transact_time)" --tag "77=O"

	run_case "08_replace_missing_openclose_rejects" "Session Reject RefTagID=77" "35=3.*371=77" "$sym_replace" \
		raw send --msg-type G \
			--tag "41=TFX-$RUN_ID-777" --tag "11=TFX-$RUN_ID-008" --tag "37=TFX-$RUN_ID-777" \
			--tag "21=1" --tag "38=30" --tag "40=2" --tag "44=8.50" --tag "54=1" \
			--tag "55=$sym_replace" --tag "59=0" --tag "60=$(transact_time)"

	run_case "09_sell_short_with_locate_accepts" "ExecutionReport New with locate" "35=8.*39=0" "$sym_short" \
		raw send --msg-type D \
			--tag "11=TFX-$RUN_ID-009" --tag "21=1" --tag "38=10" --tag "40=2" --tag "44=12.00" \
			--tag "47=P" --tag "54=5" --tag "55=$sym_short" --tag "59=0" --tag "60=$(transact_time)" \
			--tag "114=N" --tag "5700=ABCD"

	run_case "10_sell_short_missing_locate_rejects" "Rejects missing LocateReqd(114)" "(35=3.*371=114|35=j.*114)" "$sym_short" \
		raw send --msg-type D \
			--tag "11=TFX-$RUN_ID-010" --tag "21=1" --tag "38=10" --tag "40=2" --tag "44=12.00" \
			--tag "47=P" --tag "54=5" --tag "55=$sym_short" --tag "59=0" --tag "60=$(transact_time)"

	run_case "11_option_order_accepts" "ExecutionReport New for option order" "35=8.*39=0" "$sym_option" \
		raw send --msg-type D \
			--tag "11=TFX-$RUN_ID-011" --tag "21=1" --tag "38=1" --tag "40=2" --tag "44=1.25" \
			--tag "47=A" --tag "54=1" --tag "55=$sym_option" --tag "59=0" --tag "60=$(transact_time)" \
			--tag "167=OPT" --tag "200=202606" --tag "201=1" --tag "202=150.0000" --tag "204=8" --tag "205=19" --tag "77=O"

	run_case "12_symbolsfx_seed_buy_a" "ExecutionReport New with SymbolSfx A" "35=8.*65=A.*39=0" "$sym_sfx.A" \
		raw send --msg-type D \
			--tag "11=TFX-$RUN_ID-012" --tag "21=1" --tag "38=100" --tag "40=2" --tag "44=10.00" \
			--tag "47=A" --tag "54=1" --tag "55=$sym_sfx" --tag "65=A" --tag "59=0" --tag "60=$(transact_time)"
	run_case "13_symbolsfx_crossing_sell_b_should_rest" "ExecutionReport New with SymbolSfx B" "35=8.*65=B.*39=0" "$sym_sfx.B" \
		raw send --msg-type D \
			--tag "11=TFX-$RUN_ID-013" --tag "21=1" --tag "38=100" --tag "40=2" --tag "44=9.00" \
			--tag "47=A" --tag "54=2" --tag "55=$sym_sfx" --tag "65=B" --tag "59=0" --tag "60=$(transact_time)"
	run_case "14_symbolsfx_cancel_b_accepts" "SymbolSfx B order remains cancelable" "35=8.*65=B.*39=4" "$sym_sfx.B" \
		raw send --msg-type F \
			--tag "41=TFX-$RUN_ID-013" --tag "11=TFX-$RUN_ID-014" --tag "37=TFX-$RUN_ID-013" \
			--tag "38=100" --tag "54=2" --tag "55=$sym_sfx" --tag "65=B" --tag "60=$(transact_time)"

	run_case "15_allocation_invalid_noorders_rejects" "Session Reject RefTagID=73" "35=3.*371=73" "$sym_alloc" \
		raw send --msg-type J \
			--tag "53=100" --tag "6=12.0000" --tag "54=1" --tag "55=$sym_alloc" --tag "70=ALLOC-$RUN_ID" \
			--tag "71=0" --tag "73=2" --tag "11=MANUAL" --tag "37=ORDER-$RUN_ID" --tag "75=$(date -u +%Y%m%d)" \
			--tag "78=1" --tag "79=ACCT-1" --tag "80=100"

	log "Replay completed"
	log "Summary: $SUMMARY_FILE"
	if [[ "$START_ORDERMATCH" == "1" ]]; then
		log "Server log: $SERVER_LOG"
	fi
	if [[ "$FAILED_CASES" -gt 0 ]]; then
		fail "$FAILED_CASES case assertions failed; see $SUMMARY_FILE"
	fi
}

main "$@"
