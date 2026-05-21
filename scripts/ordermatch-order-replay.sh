#!/usr/bin/env bash

set -u
set -o pipefail

# 可重放的 order 子命令联调脚本，默认连接本机 quickfixgo-examples ordermatch。
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ORDERMATCH_DIR="${ORDERMATCH_DIR:-$HOME/Sync/chief/quickfixgo-examples}"

# 主流美股 Symbol 池，基于 Nasdaq-100 成分股；单次运行内按不放回方式随机抽取。
stock_symbols=(
	AAPL ABNB ADBE ADI ADP ADSK AEP ALNY AMAT AMD AMGN AMZN APP ARM ASML AVGO AXON BKNG BKR CCEP CDNS
	CEG CHTR CMCSA COST CPRT CRWD CSCO CSGP CSX CTAS CTSH DASH DDOG DXCM EA EXC FANG FAST FER FTNT GEHC
	GILD GOOG GOOGL HON IDXX INSM INTC INTU ISRG KDP KHC KLAC LIN LRCX MAR MCHP MDLZ MELI META MNST MPWR
	MRVL MSFT MSTR MU NFLX NVDA NXPI ODFL ORLY PANW PAYX PCAR PDD PEP PLTR PYPL QCOM REGN ROP ROST SBUX
	SHOP SNPS STX TEAM TMUS TRI TSLA TTWO TXN VRSK VRTX WBD WDAY WDC WMT XEL ZS
)

assign_stock_symbol() {
	local target="$1"
	local count="${#stock_symbols[@]}"
	local index
	local symbol

	if [[ "$count" -eq 0 ]]; then
		fail "No stock symbols available"
	fi

	index=$((RANDOM % count))
	symbol="${stock_symbols[$index]}"
	stock_symbols=("${stock_symbols[@]:0:$index}" "${stock_symbols[@]:$((index + 1))}")
	printf -v "$target" '%s' "$symbol"
}

random_run_token() {
	local token

	token="$(od -An -N4 -tx1 /dev/urandom 2>/dev/null | tr -d ' \n')"
	if [[ -z "$token" ]]; then
		token="$(printf '%04x%04x' "$RANDOM" "$RANDOM")"
	fi
	printf '%s' "${token:0:8}"
}

default_run_label() {
	local colors=(
		amber aqua azure beige black blue bronze brown burgundy charcoal chocolate clay cobalt copper coral cream crimson cyan
		emerald fuchsia gold graphite green indigo ivory jade lavender lilac lime magenta maroon mint navy ochre olive orange
		pearl pink plum purple red ruby rust sapphire scarlet silver sky slate smoke steel tan teal turquoise vermilion violet white yellow
	)
	local names=(
		adam aiden alice amelia anna arthur ava bella ben blake carter charlie chloe clara daniel diana ella emma ethan felix
		finn grace hannah harper hazel henry iris isabel jack james julia kate leo liam logan lucy mason maya mia nina noah
		nora oliver oscar owen paige peter quinn rachel riley rose ruby ryan sam sarah sean sophia stella theo tom uma victor
		violet william wyatt xavier yara zara zoe
	)
	local color_index=$((RANDOM % ${#colors[@]}))
	local name_index=$((RANDOM % ${#names[@]}))
	local token

	token="$(random_run_token)"
	printf '%s-%s-%s' "${colors[$color_index]}" "${names[$name_index]}" "$token"
}

sanitize_run_label() {
	local value="$1"

	value="$(printf '%s' "$value" | tr '[:upper:]' '[:lower:]' | sed 's/[^a-z0-9._-]/-/g; s/-\{1,\}/-/g; s/^-//; s/-$//')"
	if [[ -z "$value" ]]; then
		value="run"
	fi
	printf '%s' "$value"
}

RUN_ID="${RUN_ID:-$(date -u +%Y%m%d%H%M%S)}"
RUN_LABEL="$(sanitize_run_label "${RUN_LABEL:-$(default_run_label)}")"
OUTPUT_BASE_DIR="${OUTPUT_BASE_DIR:-$REPO_ROOT/tmp/ordermatch-order-replay}"
OUTPUT_DIR="${OUTPUT_DIR:-$OUTPUT_BASE_DIR/$RUN_LABEL-$RUN_ID}"
START_ORDERMATCH="${OM:-${START_ORDERMATCH:-0}}"
CASE_TIMEOUT_SECONDS="${CT:-${CASE_TIMEOUT_SECONDS:-12}}"

FIX_TOOL_BIN="$OUTPUT_DIR/fix-tool"
ORDERMATCH_BIN="$OUTPUT_DIR/qf"
CLIENT_CONFIG="$OUTPUT_DIR/ordermatch-client.toml"
SERVER_LOG="$OUTPUT_DIR/ordermatch-server.log"
SUMMARY_FILE="$OUTPUT_DIR/summary.tsv"
CONNECTIVITY_LOG="$OUTPUT_DIR/connectivity.log"
SERVER_TYPESCRIPT="$OUTPUT_DIR/ordermatch.typescript"
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

require_file() {
	local path="$1"
	[[ -e "$path" ]] || fail "Missing required path: $path"
}

require_command() {
	local command="$1"
	command -v "$command" >/dev/null 2>&1 || fail "Missing required command: $command"
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

check_ordermatch_connectivity() {
	local status

	log "Checking ordermatch TCP connectivity: 127.0.0.1:5001"
	wait_for_port 127.0.0.1 5001 || fail "ordermatch is not reachable at 127.0.0.1:5001; start the service first or run with OM=1"
	log "Checking ordermatch FIX logon"
	run_with_timeout "$CASE_TIMEOUT_SECONDS" "$FIX_TOOL_BIN" --config "$CLIENT_CONFIG" check logon >"$CONNECTIVITY_LOG" 2>&1
	status="$?"
	if [[ "$status" != "0" ]]; then
		fail "ordermatch FIX logon check failed status=$status log=$CONNECTIVITY_LOG"
	fi
	log "ordermatch connectivity check passed log=$CONNECTIVITY_LOG"
}

write_client_config() {
	cat >"$CLIENT_CONFIG" <<'CONFIG'
[app]
name = "fix-tool"

[log]
level = "warn"
format = "console"

[profile]
name = "ordermatch"
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
		script -q "$SERVER_TYPESCRIPT" env TERM=xterm-256color "$ORDERMATCH_BIN" ordermatch "$ORDERMATCH_DIR/config/ordermatch.cfg" >"$SERVER_LOG" 2>&1 &
		SERVER_PID="$!"
		return
	fi
	if script -q -c "true" /dev/null >/dev/null 2>&1; then
		script -q -c "TERM=xterm-256color \"$ORDERMATCH_BIN\" ordermatch \"$ORDERMATCH_DIR/config/ordermatch.cfg\"" "$SERVER_TYPESCRIPT" >"$SERVER_LOG" 2>&1 &
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

	log "Starting ordermatch server"
	start_ordermatch_with_pty

	wait_for_port 127.0.0.1 5001 || fail "ordermatch did not listen on 127.0.0.1:5001"
	log "ordermatch server is ready pid=$SERVER_PID"
}

query_market() {
	local symbol="$1"
	[[ "$START_ORDERMATCH" == "1" ]] || return 0
	[[ -n "$symbol" ]] || return 0

	# PTY 自启动下不再向 TUI stdin 写入 symbol；回放结果以 FIX 响应日志为准。
	return 0
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
	local exec_type
	local ord_status
	local text
	local leaves_qty
	local cum_qty
	local avg_px
	local last_qty
	local last_px
	local result=""

	while IFS= read -r raw; do
		msg_type="$(field_value "$raw" 35)"
		case "$msg_type" in
			8|9|3|j)
				cl_ord_id="$(field_value "$raw" 11)"
				exec_type="$(field_value "$raw" 150)"
				ord_status="$(field_value "$raw" 39)"
				text="$(field_value "$raw" 58)"
				leaves_qty="$(field_value "$raw" 151)"
				cum_qty="$(field_value "$raw" 14)"
				avg_px="$(field_value "$raw" 6)"
				last_qty="$(field_value "$raw" 32)"
				last_px="$(field_value "$raw" 31)"
				result="${result}35=${msg_type},11=${cl_ord_id},150=${exec_type},39=${ord_status},58=${text},151=${leaves_qty},14=${cum_qty},6=${avg_px},32=${last_qty},31=${last_px}; "
				;;
		esac
	done < <(extract_raw_messages "$log_file")

	if [[ -z "$result" ]]; then
		result="no application/admin response captured"
	fi
	printf '%s' "$result"
}

run_with_timeout() {
	local timeout_seconds="$1"
	shift

	"$@" &
	local pid="$!"
	local timeout_marker="$OUTPUT_DIR/.timeout-$pid"
	local timer_pid
	local status

	(
		sleep "$timeout_seconds"
		if kill -0 "$pid" 2>/dev/null; then
			: >"$timeout_marker"
			kill "$pid" 2>/dev/null || true
		fi
	) &
	timer_pid="$!"

	wait "$pid"
	status="$?"
	kill "$timer_pid" 2>/dev/null || true
	wait "$timer_pid" 2>/dev/null || true

	if [[ -f "$timeout_marker" ]]; then
		rm -f "$timeout_marker"
		return 124
	fi
	return "$status"
}

run_case() {
	local name="$1"
	local expected="$2"
	local symbol="$3"
	shift 3

	local log_file="$OUTPUT_DIR/${name}.log"
	local status

	log "Running case: $name"
	run_with_timeout "$CASE_TIMEOUT_SECONDS" "$FIX_TOOL_BIN" --config "$CLIENT_CONFIG" "$@" >"$log_file" 2>&1
	status="$?"
	query_market "$symbol"

	printf '%s\t%s\t%s\t%s\t%s\t%s\n' "$name" "$status" "$symbol" "$expected" "$log_file" "$(summarize_observed "$log_file")" >>"$SUMMARY_FILE"
	log "Case finished: $name status=$status log=$log_file"
}

prepare() {
	require_file "$ORDERMATCH_DIR/config/ordermatch.cfg"
	mkdir -p "$OUTPUT_DIR"
	print_log_dir_once
	write_client_config
	build_binaries
	printf 'case\tstatus\tsymbol\texpected\tlog_file\tobserved\n' >"$SUMMARY_FILE"
}

main() {
	local sym_cross
	local sym_non_cross
	local sym_market
	local sym_cancel
	local sym_cancel_no_order_id
	local sym_cancel_unknown
	local sym_replace
	local sym_full_fill
	local sym_sweep
	local sym_market_sell
	local sym_market_empty
	local sym_duplicate
	local sym_tif_fok
	local sym_tif_ioc
	local sym_isolation_a
	local sym_isolation_b
	local sym_replace_cross

	prepare
	start_ordermatch
	check_ordermatch_connectivity

	assign_stock_symbol sym_cross
	assign_stock_symbol sym_non_cross
	assign_stock_symbol sym_market
	assign_stock_symbol sym_cancel
	assign_stock_symbol sym_cancel_no_order_id
	assign_stock_symbol sym_cancel_unknown
	assign_stock_symbol sym_replace
	assign_stock_symbol sym_full_fill
	assign_stock_symbol sym_sweep
	assign_stock_symbol sym_market_sell
	assign_stock_symbol sym_market_empty
	assign_stock_symbol sym_duplicate
	assign_stock_symbol sym_tif_fok
	assign_stock_symbol sym_tif_ioc
	assign_stock_symbol sym_isolation_a
	assign_stock_symbol sym_isolation_b
	assign_stock_symbol sym_replace_cross

	# 场景 1：先挂买入限价单，预期进入订单簿。
	run_case "01_resting_buy_limit" "new accepted, leaves=100" "$sym_cross" \
		order new --cl-ord-id "FT-$RUN_ID-001" --symbol "$sym_cross" --side buy --qty 100 --price 10.00 --ord-type limit --time-in-force day

	# 场景 2：再挂卖出限价单吃掉部分买单，预期成交 40，买单剩余 60。
	run_case "02_aggressive_sell_crosses" "sell accepted then match 40 against resting buy" "$sym_cross" \
		order new --cl-ord-id "FT-$RUN_ID-002" --symbol "$sym_cross" --side sell --qty 40 --price 9.50 --ord-type limit --time-in-force day

	# 场景 3：买价 10、卖价 11 不应成交，预期双方都留在订单簿。
	run_case "03_non_crossing_buy" "new accepted, should rest" "$sym_non_cross" \
		order new --cl-ord-id "FT-$RUN_ID-003" --symbol "$sym_non_cross" --side buy --qty 100 --price 10.00 --ord-type limit --time-in-force day
	run_case "04_non_crossing_sell_should_rest" "should not match because sell price 11.00 is above bid 10.00" "$sym_non_cross" \
		order new --cl-ord-id "FT-$RUN_ID-004" --symbol "$sym_non_cross" --side sell --qty 100 --price 11.00 --ord-type limit --time-in-force day

	# 场景 4：先挂卖单，再发不带 Price(44) 的市价买单。
	run_case "05_market_seed_offer" "new accepted, offer should rest" "$sym_market" \
		order new --cl-ord-id "FT-$RUN_ID-005" --symbol "$sym_market" --side sell --qty 50 --price 12.00 --ord-type limit --time-in-force day
	run_case "06_market_buy_without_price" "market buy should be handled or rejected cleanly without requiring Price(44)" "$sym_market" \
		order new --cl-ord-id "FT-$RUN_ID-006" --symbol "$sym_market" --side buy --qty 10 --ord-type market --time-in-force ioc

	# 场景 5：撤销挂单。ordermatch 的撤单回报使用原 ClOrdID 作为 ClOrdID，因此这里带上 OrderID 辅助 fix-tool 关联响应。
	run_case "07_cancel_seed_buy" "new accepted, buy should rest" "$sym_cancel" \
		order new --cl-ord-id "FT-$RUN_ID-007" --symbol "$sym_cancel" --side buy --qty 25 --price 7.00 --ord-type limit --time-in-force day
	run_case "08_cancel_existing_with_order_id" "cancel should return OrdStatus=Canceled" "$sym_cancel" \
		order cancel --orig-cl-ord-id "FT-$RUN_ID-007" --cl-ord-id "FT-$RUN_ID-008" --order-id "FT-$RUN_ID-007" --symbol "$sym_cancel" --side buy

	# 场景 6：撤单不带 OrderID，验证 order 子命令是否能只靠 OrigClOrdID 关联回报。
	run_case "09_cancel_no_order_id_seed_buy" "new accepted, buy should rest" "$sym_cancel_no_order_id" \
		order new --cl-ord-id "FT-$RUN_ID-009" --symbol "$sym_cancel_no_order_id" --side buy --qty 15 --price 6.00 --ord-type limit --time-in-force day
	run_case "10_cancel_existing_without_order_id" "cancel should complete without requiring optional OrderID" "$sym_cancel_no_order_id" \
		order cancel --orig-cl-ord-id "FT-$RUN_ID-009" --cl-ord-id "FT-$RUN_ID-010" --symbol "$sym_cancel_no_order_id" --side buy

	# 场景 7：撤不存在订单，预期返回明确拒绝而不是静默无响应。
	run_case "11_cancel_unknown_order" "unknown cancel should return an explicit reject" "$sym_cancel_unknown" \
		order cancel --orig-cl-ord-id "FT-$RUN_ID-404" --cl-ord-id "FT-$RUN_ID-011" --symbol "$sym_cancel_unknown" --side buy

	# 场景 8：改单。ordermatch 未注册 OrderCancelReplaceRequest(G)，预期暴露不支持改单的问题。
	run_case "12_replace_seed_buy" "new accepted, buy should rest before replace" "$sym_replace" \
		order new --cl-ord-id "FT-$RUN_ID-012" --symbol "$sym_replace" --side buy --qty 30 --price 8.00 --ord-type limit --time-in-force day
	run_case "13_replace_order" "replace should amend order or return explicit reject" "$sym_replace" \
		order replace --orig-cl-ord-id "FT-$RUN_ID-012" --cl-ord-id "FT-$RUN_ID-013" --order-id "FT-$RUN_ID-012" --symbol "$sym_replace" --side buy --qty 35 --price 8.50 --ord-type limit --time-in-force day

	# 场景 9：完全成交，双方数量相等时两边都应 Filled。
	run_case "14_full_fill_seed_buy" "new accepted, buy should rest before exact fill" "$sym_full_fill" \
		order new --cl-ord-id "FT-$RUN_ID-014" --symbol "$sym_full_fill" --side buy --qty 20 --price 5.00 --ord-type limit --time-in-force day
	run_case "15_full_fill_aggressive_sell_exact" "exact cross should fill both sides, leaves=0" "$sym_full_fill" \
		order new --cl-ord-id "FT-$RUN_ID-015" --symbol "$sym_full_fill" --side sell --qty 20 --price 4.50 --ord-type limit --time-in-force day

	# 场景 10：多档扫单，市价买单应先吃低价卖单，再吃高价卖单。
	run_case "16_sweep_seed_offer_low" "new accepted, low offer should rest" "$sym_sweep" \
		order new --cl-ord-id "FT-$RUN_ID-016" --symbol "$sym_sweep" --side sell --qty 10 --price 10.00 --ord-type limit --time-in-force day
	run_case "17_sweep_seed_offer_high" "new accepted, high offer should rest behind low offer" "$sym_sweep" \
		order new --cl-ord-id "FT-$RUN_ID-017" --symbol "$sym_sweep" --side sell --qty 15 --price 11.00 --ord-type limit --time-in-force day
	run_case "18_sweep_market_buy_two_levels" "market buy 20 should fill 10 at 10.00 and 10 at 11.00" "$sym_sweep" \
		order new --cl-ord-id "FT-$RUN_ID-018" --symbol "$sym_sweep" --side buy --qty 20 --ord-type market --time-in-force ioc

	# 场景 11：市价卖单不带 Price(44)，应按买一价格成交。
	run_case "19_market_sell_seed_bid" "new accepted, bid should rest" "$sym_market_sell" \
		order new --cl-ord-id "FT-$RUN_ID-019" --symbol "$sym_market_sell" --side buy --qty 12 --price 20.00 --ord-type limit --time-in-force day
	run_case "20_market_sell_without_price" "market sell should trade against resting bid without requiring Price(44)" "$sym_market_sell" \
		order new --cl-ord-id "FT-$RUN_ID-020" --symbol "$sym_market_sell" --side sell --qty 6 --ord-type market --time-in-force ioc

	# 场景 12：没有对手方流动性时，市价单残量不应留在订单簿。
	run_case "21_market_empty_buy_without_liquidity" "market buy with no liquidity should be canceled after acceptance, not rest" "$sym_market_empty" \
		order new --cl-ord-id "FT-$RUN_ID-021" --symbol "$sym_market_empty" --side buy --qty 7 --ord-type market --time-in-force ioc

	# 场景 13：重复 ClOrdID 应返回拒绝，避免同一交易日同一会话重复订单号。
	run_case "22_duplicate_seed_buy" "new accepted, first ClOrdID should be recorded" "$sym_duplicate" \
		order new --cl-ord-id "FT-$RUN_ID-DUP" --symbol "$sym_duplicate" --side buy --qty 5 --price 3.00 --ord-type limit --time-in-force day
	run_case "23_duplicate_clordid_reject" "duplicate ClOrdID should return OrdStatus=Rejected" "$sym_duplicate" \
		order new --cl-ord-id "FT-$RUN_ID-DUP" --symbol "$sym_duplicate" --side buy --qty 5 --price 3.00 --ord-type limit --time-in-force day

	# 场景 14：FOK/IOC 限价无流动性。若只返回 New 并留簿，说明 ordermatch 未实现该 TIF 语义。
	run_case "24_fok_limit_without_liquidity" "FOK without full liquidity should reject or cancel instead of resting" "$sym_tif_fok" \
		order new --cl-ord-id "FT-$RUN_ID-024" --symbol "$sym_tif_fok" --side buy --qty 9 --price 2.00 --ord-type limit --time-in-force fok
	run_case "25_ioc_limit_without_liquidity" "IOC without liquidity should cancel remainder instead of resting" "$sym_tif_ioc" \
		order new --cl-ord-id "FT-$RUN_ID-025" --symbol "$sym_tif_ioc" --side buy --qty 9 --price 2.00 --ord-type limit --time-in-force ioc

	# 场景 15：跨 Symbol 隔离，价格交叉但 symbol 不同不应成交。
	run_case "26_isolation_seed_buy_symbol_a" "new accepted, symbol A bid should rest" "$sym_isolation_a" \
		order new --cl-ord-id "FT-$RUN_ID-026" --symbol "$sym_isolation_a" --side buy --qty 10 --price 30.00 --ord-type limit --time-in-force day
	run_case "27_isolation_sell_symbol_b" "sell on symbol B should not match bid on symbol A" "$sym_isolation_b" \
		order new --cl-ord-id "FT-$RUN_ID-027" --symbol "$sym_isolation_b" --side sell --qty 10 --price 1.00 --ord-type limit --time-in-force day

	# 场景 16：改单后价格穿透对手方，应先返回 Replaced，再触发成交。
	run_case "28_replace_cross_seed_offer" "new accepted, offer should rest before replace-cross" "$sym_replace_cross" \
		order new --cl-ord-id "FT-$RUN_ID-028" --symbol "$sym_replace_cross" --side sell --qty 10 --price 8.00 --ord-type limit --time-in-force day
	run_case "29_replace_cross_seed_buy" "new accepted, buy should rest below offer" "$sym_replace_cross" \
		order new --cl-ord-id "FT-$RUN_ID-029" --symbol "$sym_replace_cross" --side buy --qty 10 --price 7.00 --ord-type limit --time-in-force day
	run_case "30_replace_cross_order" "replace buy to 8.50 should cross and fill both sides" "$sym_replace_cross" \
		order replace --orig-cl-ord-id "FT-$RUN_ID-029" --cl-ord-id "FT-$RUN_ID-030" --order-id "FT-$RUN_ID-029" --symbol "$sym_replace_cross" --side buy --qty 10 --price 8.50 --ord-type limit --time-in-force day

	log "Replay completed"
	log "Summary: $SUMMARY_FILE"
	if [[ "$START_ORDERMATCH" == "1" ]]; then
		log "Server log: $SERVER_LOG"
	fi
}

main "$@"
