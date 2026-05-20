# fix-tool

`fix-tool` 是一个单二进制 FIX 协议联调 CLI，用于连接 FIX 网关、发送管理报文和常用订单报文、解析 raw FIX 报文、执行 YAML 场景脚本，并默认对敏感字段脱敏。

## 安装

从 release 包安装：

```bash
tar -xzf fix-tool_v0.1.0_darwin_arm64.tar.gz
cd fix-tool_v0.1.0_darwin_arm64
sudo install -m 0755 fix-tool /usr/local/bin/fix-tool
fix-tool version
```

使用安装脚本时默认只安装二进制。安装后可通过内嵌文档查看用法：

```bash
fix-tool docs
fix-tool docs user-guide
fix-tool docs faq
```

校验 checksum：

```bash
grep 'fix-tool_v0.1.0_darwin_arm64.tar.gz' checksums.txt | shasum -a 256 -c -
```

从源码构建：

```bash
go version # 需要 Go 1.25.10 或更高 patch 版本
make test
make build VERSION=v0.1.0
./dist/bin/fix-tool version
```

生成 macOS/Linux release 包：

```bash
make release VERSION=v0.1.0
ls dist/release
```

## 从配置到下单

创建公开配置 `config.toml`。完整字段说明见 [`config/config-example.toml`](config/config-example.toml)：

```bash
fix-tool config example --output config-example.toml
fix-tool config example --output config-example.toml --force
```

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
description = "示例敏感字段"

[[profile.custom_field_defs]]
tag = 9002
name = "Desk"
type = "STRING"
required = false
sensitive = false
description = "交易台标识"
enums = { ALPHA = "Alpha desk", BETA = "Beta desk" }
```

创建私有配置 `private.toml`，不要提交到 Git：

```toml
[profile]
username = "replace-with-uat-user"
password = "replace-with-uat-password"

[[profile.logon_tags]]
tag = 9001
value = "replace-with-session-token"
```

`custom_field_defs` 只定义字段名称、枚举和脱敏规则；需要 Logon 发送的自定义字段请放在 `profile.logon_tags` 中。`profile.custom_tags` 为旧配置名，仍兼容读取，但会提示弃用。

验证配置并下单：

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
  --time-in-force day \
  --tag 9002=ALPHA
```

更多说明见 [用户指南](docs/project/fix-tool/user-guide.md) 和 [FAQ / 排障指南](docs/project/fix-tool/faq.md)。

## 配置维护约定

后续只要新增、删除或调整配置项，必须同步更新：

- `config/default.toml`（会内嵌进二进制）
- `config/config-example.toml`
- `docs/project/fix-tool/user-guide.md`
- `docs/project/fix-tool/conventions.md`
- 对应的配置加载、校验和测试代码
