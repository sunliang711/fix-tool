# fix-tool FAQ 与排障指南

## 1. `fix-tool version` 只显示 `dev`

直接 `go run` 或未传入 `-ldflags` 时会显示默认值：

```text
version: dev
commit: none
build_time: unknown
```

使用 Makefile 构建即可注入版本信息：

```bash
make build VERSION=v0.1.0
./dist/bin/fix-tool version
```

## 2. 配置校验失败

先运行：

```bash
fix-tool --config config.toml --private private.toml config validate
```

常见原因：

- `profile.host`、`sender_comp_id`、`target_comp_id` 缺失。
- `profile.port` 不在 `1..65535`。
- `heartbeat_interval` 不是合法 Go duration，例如应写 `30s`。
- `cert_file` 和 `key_file` 只配置了其中一个。
- `output.format` 不是 `table`、`raw` 或 `json`。

## 3. Logon 超时或连不上网关

排查顺序：

```bash
fix-tool --config config.toml --private private.toml config validate
fix-tool --config config.toml --private private.toml --log-level debug check logon
```

需要查看 raw FIX 报文时，把 `--log-level` 临时改为 `debug`。

重点检查：

- host、port 是否能从当前机器访问。
- BeginString、SenderCompID、TargetCompID 是否与对端白名单一致。
- username、password 是否通过 `private.toml` 或环境变量覆盖成功。
- TLS 是否与对端一致，真实网关通常要求 `enabled = true`。
- 上一次会话序列号是否需要与对端协调；测试环境可通过 `reset_on_logon = true` 重置。

## 4. TLS 证书错误

推荐修复方式：

- 配置对端提供的 CA 文件：`ca_file = "./certs/uat-ca.pem"`。
- 确认证书主机名与连接 host 匹配。
- 双向 TLS 时同时配置 `cert_file` 和 `key_file`。

不推荐做法：

```toml
[profile.tls]
enabled = true
insecure_skip_verify = true
```

该配置会跳过证书校验，仅可用于本地隔离测试。真实网关、共享测试环境和生产环境不要使用。

## 5. 输出里看到 `[REDACTED]`

这是默认脱敏行为。字段被判定为敏感时，table、raw、json 输出都会展示 `[REDACTED]`。

内置敏感字段包括 Account、Username、Password、NewPassword、Signature、SecureData、RawData。自定义字段可通过 `profile.custom_field_defs` 中的 `sensitive = true` 标记为敏感。

临时本地排查可以关闭脱敏：

```toml
[output]
redact_sensitive = false
```

关闭脱敏后不要把终端输出、日志或 JSON 结果粘贴到工单、聊天工具或提交记录中。

## 6. `raw send` 被拒绝覆盖协议字段

`raw send` 允许补充 body tag，但不允许覆盖 BeginString、BodyLength、CheckSum、MsgType、MsgSeqNum、SenderCompID、TargetCompID 等协议字段。

正确示例：

```bash
fix-tool raw send --msg-type D --tag 11=RAW-0001 --tag 55=AAPL --tag 54=1
```

错误示例：

```bash
fix-tool raw send --msg-type D --tag 35=Z
```

## 7. BodyLength 或 CheckSum 校验失败

常见原因：

- raw 报文被复制时丢失了字段分隔符。
- 手工修改了字段值，但没有重新计算 `9=` 或 `10=`。
- 对端返回的报文包含非预期编码或截断。

离线解析时可以用 `|` 代替 SOH：

```bash
fix-tool inspect raw '8=FIX.4.4|9=12|35=0|10=000|'
```

真实发送时工具会使用 SOH，不需要用户手写 SOH。

## 8. 场景脚本断言失败

先导出 JSON 结果：

```bash
fix-tool --config config.toml --private private.toml run order-lifecycle.yaml --result-file result.json
```

检查：

- `wait.msg_type` 是否与对端实际响应一致。
- 断言字段名是否为工具支持的字段，例如 `msg_type`、`cl_ord_id`、`order_id`、`exec_type`、`ord_status`。
- 订单 ID、ClOrdID 是否因为重跑脚本而重复。
- 对端是否把业务拒绝返回为 `BusinessMessageReject` 或 `Reject`。

## 9. release 包缺少 checksum

使用 release 目标生成：

```bash
make release VERSION=v0.1.0
ls dist/release/checksums.txt
```

`make cross-build` 只生成跨平台二进制，不生成 tar 包和 checksum。

## 10. CI 中 govulncheck 失败

CI 的漏洞检查命令是：

```bash
go run golang.org/x/vuln/cmd/govulncheck@latest ./...
```

处理建议：

- 先在本地复现同一命令。
- 确认本地 Go 工具链不低于 `go.mod` 要求的 patch 版本。
- 确认报告是否命中当前代码路径。
- 对高风险依赖升级要单独评估兼容性，并补充回归测试。
- 如果是暂未可修复的上游问题，应在主 agent 决策后记录风险，不要静默跳过检查。
