# fix-tool 测试说明

## 覆盖矩阵

| 模块 | 测试类型 | 覆盖重点 | 代表文件 |
|---|---|---|---|
| `internal/config` | 单元测试 | 默认值、配置文件、私有配置、环境变量和 flag 覆盖顺序 | `internal/config/loader_test.go` |
| `internal/validate` | 单元测试 | 配置必填项、端口、心跳间隔、样例配置安全性 | `internal/validate/config_test.go` |
| `internal/dictionary` | 单元测试 | 标准字段、自定义 tag、枚举、敏感字段识别 | `internal/dictionary/dictionary_test.go` |
| `internal/message` | 单元测试 | New/Cancel/Replace 构造、枚举归一化、保护字段、自定义 tag、market 单不带价格 | `internal/message/order_test.go` |
| `internal/raw` | 单元测试 | raw message 构造、保护字段拒绝、SOH 与展示分隔符处理 | `internal/raw/builder_test.go` |
| `internal/trace` | 单元测试 | raw 归一化、BodyLength、CheckSum、非法字段、空报文 | `internal/trace/parser_test.go` |
| `internal/render` | 单元测试 | table/raw/json 输出、敏感字段脱敏和显式展示 | `internal/render/render_test.go` |
| `internal/admin` | 单元测试 | Logon、Heartbeat、TestRequest、Logout、超时、KeepSession 状态复用 | `internal/admin/service_test.go` |
| `internal/order` | 单元测试 | 订单发送、ExecutionReport/Reject 关联匹配、超时、参数错误前置返回 | `internal/order/service_test.go` |
| `internal/scenario` | 单元测试 | YAML 加载、action alias、断言、失败中断、raw action 接入 | `internal/scenario/*_test.go` |
| `internal/shell` | 单元测试 | 命令解析、交互 runner、trace list | `internal/shell/*_test.go` |
| `internal/mockfix` | 集成测试 | QuickFIX initiator 与 mock acceptor 的 Logon、Heartbeat、TestRequest、New、Replace、Cancel、Reject 链路 | `internal/mockfix/acceptor_test.go` |
| `internal/cli` | 集成测试 | `fix-tool run` 通过 mock acceptor 执行样例场景 | `internal/cli/scenario_integration_test.go` |

## Mock Acceptor

`internal/mockfix` 提供测试可启动/停止的 QuickFIX/Go acceptor。端口默认动态分配，测试通过 `ProfileConfig()` 生成 initiator profile，避免依赖真实外部 FIX 网关。

已覆盖的模拟行为：

- Logon：由 QuickFIX/Go acceptor 完成会话登录。
- Heartbeat：收到客户端 Heartbeat 后回送 Heartbeat。
- TestRequest：收到 TestRequest 后回送携带相同 `112=TestReqID` 的 Heartbeat。
- NewOrderSingle：回送 `35=8`、`150=0`、`39=0` 的 ExecutionReport。
- OrderCancelRequest：回送 `35=8`、`150=4`、`39=4` 的 ExecutionReport。
- OrderCancelReplaceRequest：默认回送 `35=8`、`150=5`、`39=0` 的 ExecutionReport；也可通过 `ReplaceResponse` 配置为 Reject 或 BusinessMessageReject。
- Reject 触发：订单 `symbol=MOCK-REJECT` 时回送 `35=3`。
- BusinessMessageReject 触发：订单 `symbol=MOCK-BUSINESS-REJECT` 时回送 `35=j`。

边界说明：

- mock acceptor 只用于基础链路验证，不代表真实券商或交易所网关的风控、撮合、序列号策略和自定义字段规则。
- task07 raw service 已接入，场景 runner 的 `raw` action 覆盖基础发送链路；mock acceptor 只覆盖有限 MsgType 响应。
- mock acceptor 使用内存 store，测试结束必须调用 `Stop(ctx)` 释放 QuickFIX session 和监听端口。

## 样例文件

- 样例配置：`testdata/configs/mock-acceptor.toml`
- 字典样例：`testdata/dictionaries/custom-tags.toml`
- raw 报文样例：`testdata/messages/*.fix`
- 样例场景：`testdata/scenarios/mock-acceptor-basic.yaml`
- 既有订单生命周期场景：`testdata/scenarios/order-lifecycle.yaml`

样例配置使用空用户名和空密码，仅包含本地 mock 连接参数，不包含真实账号、密码、Token 或证书。手工运行样例时，可按本地 mock acceptor 实际监听端口调整 `port`。字典样例只演示 custom field definition 的字段名、类型、枚举和敏感标记，不代表某个真实交易对手方的数据字典。

## 验证命令

```bash
go test ./...
go test ./internal/mockfix -run TestMockAcceptorSupportsAdminAndOrderLifecycle
go test ./internal/cli -run TestScenarioRunWithMockAcceptor
go run ./cmd/fix-tool --config testdata/configs/mock-acceptor.toml config validate
```
