# fix-tool 用户指南

`fix-tool` 是一个单二进制 FIX 协议联调 CLI，用于检查 FIX 登录、发送常用订单报文、解析 raw FIX 报文、执行 YAML 场景脚本，并默认对敏感字段脱敏。

## 1. 三分钟快速开始

安装后先确认版本和内嵌文档入口：

```bash
fix-tool version
fix-tool docs
fix-tool --help
```

生成一份公开配置模板：

```bash
fix-tool config example --output config.toml
```

编辑 `config.toml`，至少确认以下字段与对端 FIX 网关一致：

```toml
[profile]
name = "uat"
begin_string = "FIX.4.4"
sender_comp_id = "CLIENT01"
target_comp_id = "BROKER01"
host = "fix-uat.example.com"
port = 9876
heartbeat_interval = "30s"
reset_on_logon = true

[profile.tls]
enabled = true
insecure_skip_verify = false
ca_file = "./certs/uat-ca.pem"
cert_file = ""
key_file = ""
```

创建私有配置 `private.toml`，只放本机凭据和不应提交的 Logon 扩展字段：

```toml
[profile]
username = "replace-with-uat-user"
password = "replace-with-uat-password"

[[profile.logon_tags]]
tag = 9001
value = "replace-with-session-token"
```

校验配置、检查登录并发送一笔新单：

```bash
chmod 600 private.toml
fix-tool --config config.toml --private private.toml config validate
fix-tool --config config.toml --private private.toml check logon
fix-tool --config config.toml --private private.toml order new \
  --cl-ord-id DEMO-0001 \
  --symbol AAPL \
  --side buy \
  --qty 100 \
  --price 10.25 \
  --ord-type limit \
  --time-in-force day
```

## 2. 安装方式与文档位置

使用 `install.sh` 安装时，默认只安装 `fix-tool` 二进制，不会把 release 包里的 Markdown 文档复制到本地目录。安装后可直接查看内嵌文档：

```bash
fix-tool docs
fix-tool docs user-guide
fix-tool docs faq
```

从 release 包手动安装时，包内会包含二进制、README、用户指南、FAQ、样例场景和自定义 tag 样例：

```bash
tar -xzf fix-tool_v0.1.0_linux_amd64.tar.gz
cd fix-tool_v0.1.0_linux_amd64
install -m 0755 fix-tool "$HOME/.local/bin/fix-tool"
fix-tool version
```

发布包外层的 `checksums.txt` 用于校验 tar 包：

```bash
grep 'fix-tool_v0.1.0_linux_amd64.tar.gz' checksums.txt | shasum -a 256 -c -
```

## 3. 命令速查

| 目标 | 命令 |
| --- | --- |
| 查看内嵌文档 | `fix-tool docs` |
| 查看 FAQ | `fix-tool docs faq` |
| 生成完整配置样例 | `fix-tool config example --output config.toml` |
| 覆盖生成配置样例 | `fix-tool config example --output config.toml --force` |
| 校验配置 | `fix-tool --config config.toml --private private.toml config validate` |
| 检查 Logon | `fix-tool --config config.toml --private private.toml check logon` |
| 发送 TestRequest | `fix-tool --config config.toml --private private.toml check test-request --id ping-001` |
| 发送新单 | `fix-tool --config config.toml --private private.toml order new ...` |
| 撤单 | `fix-tool --config config.toml --private private.toml order cancel ...` |
| 改单 | `fix-tool --config config.toml --private private.toml order replace ...` |
| 发送 raw FIX 报文 | `fix-tool --config config.toml --private private.toml raw send ...` |
| 离线解析 raw FIX 报文 | `fix-tool --config config.toml inspect raw '8=FIX.4.4|9=12|35=0|10=000|'` |
| 启动交互式 shell | `fix-tool --config config.toml --private private.toml shell` |
| 执行场景脚本 | `fix-tool --config config.toml --private private.toml run scenario.yaml` |

## 4. 配置模型

配置合并顺序为：

```text
内嵌 config/default.toml < config.toml < private.toml < FIX_TOOL_* 环境变量 < CLI flags
```

推荐拆成两个文件：

- `config.toml`：可提交到内部配置仓库，放网关地址、SenderCompID、TargetCompID、TLS CA、输出格式和自定义字段定义。
- `private.toml`：只放本机账号、密码、Token、私钥路径等敏感或个人化配置。

常用全局参数：

```bash
fix-tool --config config.toml --private private.toml --profile uat --output json config validate
```

注意：当前配置模型一次只激活一个 `profile`，`--profile` 用于覆盖当前 profile 名称，不会在一个文件中选择多个 profile。

也可以用环境变量覆盖私有字段：

```bash
export FIX_TOOL_PROFILE_USERNAME="replace-with-uat-user"
export FIX_TOOL_PROFILE_PASSWORD="replace-with-uat-password"
```

## 5. 自定义字段与 Logon 扩展字段

`custom_field_defs` 只定义字段名称、枚举和脱敏规则，不会自动把字段发送到 Logon：

```toml
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

需要 Logon 发送的自定义字段请放在 `profile.logon_tags` 中：

```toml
[[profile.logon_tags]]
tag = 9001
value = "replace-with-session-token"
```

`profile.custom_tags` 是旧配置名，仍兼容读取，但会提示弃用。

## 6. 订单命令

发送新单：

```bash
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

撤单：

```bash
fix-tool --config config.toml --private private.toml order cancel \
  --orig-cl-ord-id DEMO-0001 \
  --cl-ord-id DEMO-0002 \
  --symbol AAPL \
  --side buy
```

改单：

```bash
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

## 7. raw 报文与离线解析

发送 raw FIX 报文时，`--msg-type` 指定 MsgType，`--tag` 补充 body tag。工具会生成 BodyLength、CheckSum，并使用真实 SOH 发送：

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

`raw send` 不允许覆盖 BeginString、BodyLength、CheckSum、MsgType、MsgSeqNum、SenderCompID、TargetCompID 等协议字段。

离线解析 raw 报文时，输入可用 `|` 代替 SOH：

```bash
fix-tool --config config.toml inspect raw '8=FIX.4.4|9=12|35=0|10=000|'
```

## 8. 交互式 shell

交互式 shell 会保持一个 FIX session，适合连续执行登录检查、下单、撤单、改单、TestRequest 和 trace 查询：

```bash
fix-tool --config config.toml --private private.toml shell
```

进入 shell 后可先输入 `help` 查看可用命令。退出时输入：

```text
exit
```

## 9. 场景脚本

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

源码仓库和 release 包中包含样例文件：

- `testdata/scenarios/order-lifecycle.yaml`
- `testdata/scenarios/mock-acceptor-basic.yaml`
- `testdata/dictionaries/custom-tags.toml`

安装脚本用户默认只有二进制，不会自动安装这些样例文件。

## 10. 输出格式与脱敏

默认输出格式为 `table`，也可以改成 `raw` 或 `json`：

```toml
[output]
format = "table"
raw_delimiter = "|"
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

## 11. TLS 配置风险

真实网关联调通常建议开启 TLS，并校验对端证书：

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

## 12. 开发者附录

从源码构建：

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
