# Go 项目级代码评审报告

## 审查范围

- `cmd/`
- `internal/`
- `config/`
- `testdata/`
- `Makefile`
- `README.md`

## 审查基线

- Go 后端编码与架构规范
- Go 安全红线规范
- Go API / CLI 输入校验与错误处理规范

## 发现并修复的问题

| 序号 | 位置 | 严重等级 | 检查项 | 问题描述 | 修复结果 |
|:---:|------|:---:|:---:|----------|----------|
| 1 | `internal/message/order.go`、`internal/admin/service.go` | 阻断 | GS-04 / GQ-01 | 订单字段、自定义 tag value、TestRequestID 没有统一拒绝真实 SOH 分隔符，恶意或误输入可能污染 FIX 字段边界。 | 为订单核心字段、可选 OrderID、自定义 tag value、TestRequestID 增加 SOH 校验，并补充单元测试。 |
| 2 | `internal/message/order.go` | 警告 | GQ-01 | 数量/价格使用 `big.Rat` 判断正数，会把 `1/2` 这类分数字面量视为合法，不符合 CLI 十进制输入预期。 | 增加十进制字面量校验，保留正数判断，并补充回归测试。 |
| 3 | `internal/trace/parser.go` | 警告 | GQ-01 | FIX 解析器没有校验 CheckSum 是否为最后字段，带尾部追加字段的报文可能被误判为 CheckSum 有效。 | 增加 CheckSum 尾字段校验，并补充回归测试。 |

## 验证结果

- `go test ./internal/admin ./internal/message ./internal/trace -count=1`
- `go test ./... -count=1`
- `go vet ./...`
- `go test ./... -race -count=1`
- `make build`
- `make release VERSION=v0.1.0`
- `make vuln`
- `pre-commit run --files internal/admin/service.go internal/admin/service_test.go internal/message/order.go internal/message/order_test.go internal/trace/parser.go internal/trace/parser_test.go`

## 残余风险

- `govulncheck` 提示依赖模块中存在 1 个不可达漏洞，当前项目代码不调用受影响符号；后续可用 `-show verbose` 评估是否需要依赖升级。
- 当前项目保留 `insecure_skip_verify` 配置能力，并在启用时记录警告；如果要按生产安全红线完全禁止关闭 TLS 证书校验，需要单独确认配置契约变更。
