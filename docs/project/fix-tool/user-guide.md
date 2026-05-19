# fix-tool 用户指南

## 1. 安装与版本检查

release 包按平台区分，当前 Makefile 默认生成：

- `darwin/amd64`
- `darwin/arm64`
- `linux/amd64`
- `linux/arm64`

安装二进制：

```bash
tar -xzf fix-tool_v0.1.0_linux_amd64.tar.gz
cd fix-tool_v0.1.0_linux_amd64
install -m 0755 fix-tool "$HOME/.local/bin/fix-tool"
fix-tool version
```

`fix-tool version` 会输出三项构建信息：

```text
version: v0.1.0
commit: 1a2b3c4
build_time: 2026-05-19T06:30:00Z
```

发布包外层的 `checksums.txt` 用于校验 tar 包：

```bash
grep 'fix-tool_v0.1.0_linux_amd64.tar.gz' checksums.txt | shasum -a 256 -c -
```

## 2. 配置加载顺序

配置合并顺序为：

```text
内嵌 config/default.toml < config.toml < private.toml < FIX_TOOL_* 环境变量 < CLI flags
```

常用全局参数：

```bash
fix-tool --config config.toml --private private.toml --profile uat --output json config validate
```

注意：当前配置模型一次只激活一个 `profile`，`--profile` 用于覆盖当前 profile 名称，不会在一个文件中选择多个 profile。

## 3. 完整配置示例

公开配置 `config.toml` 可提交到内部配置仓库，但不要包含真实账号、密码、Token、私钥：

```toml
[app]
name = "fix-tool"

[log]
level = "info"
format = "console"

[profile]
name = "uat"
begin_string = "FIX.4.4"
sender_comp_id = "CLIENT01"
target_comp_id = "BROKER01"
username = ""
password = ""
host = "fix-uat.example.com"
port = 9876
heartbeat_interval = "30s"
reset_on_logon = true
data_dictionary = ""
transport_data_dictionary = ""
app_data_dictionary = ""

[profile.tls]
enabled = true
insecure_skip_verify = false
ca_file = "./certs/uat-ca.pem"
cert_file = ""
key_file = ""

[output]
format = "table"
raw_delimiter = "|"
redact_sensitive = true

[[profile.custom_field_defs]]
tag = 9001
name = "SessionToken"
type = "STRING"
required = false
sensitive = true
description = "会话 Token，输出时默认脱敏"

[[profile.custom_field_defs]]
tag = 9002
name = "Desk"
type = "STRING"
required = false
sensitive = false
description = "交易台标识"
enums = { ALPHA = "Alpha desk", BETA = "Beta desk" }
```

私有配置 `private.toml` 只放本机凭据：

```toml
[profile]
username = "replace-with-uat-user"
password = "replace-with-uat-password"

[[profile.logon_tags]]
tag = 9001
value = "replace-with-session-token"
```

`custom_field_defs` 只定义字段名称、枚举和脱敏规则；需要 Logon 发送的自定义字段请放在 `profile.logon_tags` 中。`profile.custom_tags` 为旧配置名，仍兼容读取，但会提示弃用。

也可以用环境变量覆盖私有字段：

```bash
export FIX_TOOL_PROFILE_USERNAME="replace-with-uat-user"
export FIX_TOOL_PROFILE_PASSWORD="replace-with-uat-password"
```

## 4. 从配置到下单

先验证配置：

```bash
chmod 600 private.toml
fix-tool --config config.toml --private private.toml config validate
```

检查登录并发送一笔一次性新单：

```bash
fix-tool --config config.toml --private private.toml check logon

fix-tool --config config.toml --private private.toml order new \
  --cl-ord-id DEMO-0001 \
  --symbol AAPL \
  --side buy \
  --qty 100 \
  --price 10.25 \
  --ord-type limit \
  --time-in-force day \
  --tag 9002=ALPHA
```

撤单与改单：

```bash
fix-tool --config config.toml --private private.toml order cancel \
  --orig-cl-ord-id DEMO-0001 \
  --cl-ord-id DEMO-0002 \
  --symbol AAPL \
  --side buy

fix-tool --config config.toml --private private.toml order replace \
  --orig-cl-ord-id DEMO-0001 \
  --cl-ord-id DEMO-0003 \
  --symbol AAPL \
  --side buy \
  --qty 150 \
  --price 10.30 \
  --ord-type limit \
  --time-in-force day
```

输出中会包含 Request、Response、MsgType、BodyLength/CheckSum 校验结果、raw 报文和字段解释。

## 5. 常用命令

一次性检查类：

```bash
fix-tool --config config.toml --private private.toml check logon
fix-tool --config config.toml --private private.toml check heartbeat
fix-tool --config config.toml --private private.toml check test-request --id ping-001
fix-tool --config config.toml --private private.toml check logout
```

raw 报文：

```bash
fix-tool --config config.toml --private private.toml raw send \
  --msg-type D \
  --tag 11=RAW-0001 \
  --tag 55=AAPL \
  --tag 54=1 \
  --tag 38=100 \
  --tag 40=2 \
  --tag 44=10.25
```

离线解析 raw 报文，输入可用 `|` 代替 SOH：

```bash
fix-tool --config config.toml inspect raw '8=FIX.4.4|9=12|35=0|10=000|'
```

交互式 shell：

```bash
fix-tool --config config.toml --private private.toml shell
```

## 6. 场景脚本

场景脚本使用 YAML，支持 `logon`、`logout`、`heartbeat`、`test-request`、`order.new`、`order.cancel`、`order.replace`、`raw`。

示例：

```yaml
name: order-lifecycle
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
      cl_ord_id: DEMO-0001
      symbol: AAPL
      side: buy
      qty: "100"
      price: "10.25"
      ord_type: limit
      time_in_force: day
      tags:
        - 9002=ALPHA
    wait:
      msg_type: "8"
    assert:
      - field: cl_ord_id
        equals: DEMO-0001
      - field: exec_type
        in: ["0", "4", "8"]

  - name: logout
    action: logout
    wait:
      msg_type: "5"
```

运行并导出结果：

```bash
fix-tool --config config.toml --private private.toml run order-lifecycle.yaml --result-file result.json
fix-tool --config config.toml --private private.toml run order-lifecycle.yaml --json
```

仓库内置样例：

- `testdata/scenarios/order-lifecycle.yaml`
- `testdata/scenarios/mock-acceptor-basic.yaml`
- `testdata/dictionaries/custom-tags.toml`

## 7. 脱敏策略

默认配置：

```toml
[output]
redact_sensitive = true
```

默认敏感字段包括 Account、Username、Password、NewPassword、Signature、SecureData、RawData，以及名称包含 `password`、`token`、`secret`、`signature`、`rawdata`、`account` 的自定义字段定义。也可以显式设置：

```toml
[[profile.custom_field_defs]]
tag = 9001
name = "SessionToken"
type = "STRING"
sensitive = true
```

生产和共享日志中不要关闭脱敏。只有在本地隔离环境排查协议字段时，才考虑临时设置：

```toml
[output]
redact_sensitive = false
```

## 8. TLS 配置风险

默认推荐：

```toml
[profile.tls]
enabled = true
insecure_skip_verify = false
ca_file = "./certs/uat-ca.pem"
cert_file = ""
key_file = ""
```

风险说明：

- `enabled = false` 只适合本机 mock acceptor 或明确隔离的内网测试。
- `insecure_skip_verify = true` 会跳过证书校验，可能被中间人攻击利用；工具会输出英文风险日志 `tls certificate verification is disabled`。
- 双向 TLS 必须同时配置 `cert_file` 和 `key_file`，只配置其中一个会校验失败。
- 不要把私钥提交到 Git；私钥文件权限建议设置为 `0600`。

本地 mock acceptor 样例可以关闭 TLS：

```toml
[profile.tls]
enabled = false
insecure_skip_verify = false
```

真实网关联调时，请优先让对端提供 CA 证书或受信任证书链。

## 9. 发布构建

本地构建：

```bash
go version # 需要 Go 1.25.10 或更高 patch 版本
make build VERSION=v0.1.0
./dist/bin/fix-tool version
```

交叉编译：

```bash
make cross-build VERSION=v0.1.0
```

生成 release 包和 checksum：

```bash
make release VERSION=v0.1.0
cat dist/release/checksums.txt
```

CI 会执行：

- `go test ./... -count=1`
- `go run golang.org/x/vuln/cmd/govulncheck@latest ./...`
- `make cross-build`
