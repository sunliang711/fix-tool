# 任务交付：测试、mock acceptor、样例配置

## 任务背景

根据 `docs/project/fix-tool/tasks/task-09-testing-and-mock.md`，补齐测试覆盖、mock acceptor、集成测试、样例配置和测试说明，降低后续开发对真实外部 FIX 网关的依赖。

## 编排方案

- 实现 Agent：负责 mock acceptor、集成测试、样例配置、样例场景、测试补充和测试说明。
- 验证 Agent：先整理 task09 验证清单，再基于最终代码做独立复审。
- 主 Agent：负责本地验证、flaky 复现、问题回传、进度文档、交付文档和最终提交。

## 实现方案

- 新增 `internal/mockfix`，基于 QuickFIX/Go acceptor 提供本地 mock 网关。
- mock acceptor 默认使用动态端口，提供 `ProfileConfig()` 供 initiator 测试连接。
- mock acceptor 支持 Logon、Heartbeat、TestRequest、NewOrderSingle、OrderCancelRequest、OrderCancelReplaceRequest、Reject 和 BusinessMessageReject 的基础链路。
- 新增 service/scenario/CLI 集成测试，均连接本地 mock acceptor，不依赖真实外部网关。
- 新增安全样例配置 `testdata/configs/mock-acceptor.toml`，用户名和密码为空，TLS 显式关闭用于本地 mock。
- 新增场景样例 `testdata/scenarios/mock-acceptor-basic.yaml`。
- 新增 custom tag 字典样例 `testdata/dictionaries/custom-tags.toml`。
- 补充 message、scenario、trace、validate 的缺口型表驱动和边界测试。
- 新增 `docs/project/fix-tool/testing.md`，记录测试覆盖矩阵、mock acceptor 行为边界和验证命令。

## 文件与配置变更

- 新增 `internal/mockfix/`。
- 新增 `internal/cli/scenario_integration_test.go`。
- 修改 `internal/message/order_test.go`。
- 修改 `internal/scenario/loader_test.go`，新增 `internal/scenario/model_test.go`。
- 修改 `internal/trace/parser_test.go`。
- 修改 `internal/validate/config_test.go`。
- 新增 `testdata/configs/mock-acceptor.toml`。
- 新增 `testdata/scenarios/mock-acceptor-basic.yaml`。
- 新增 `testdata/dictionaries/custom-tags.toml`。
- 新增 `docs/project/fix-tool/testing.md`。
- 更新 `docs/project/fix-tool/PROGRESS.md`。

## 测试结果

- 实现 Agent：`go test ./... -count=1` 通过，`go run ./cmd/fix-tool --config testdata/configs/mock-acceptor.toml config validate` 通过。
- 主 Agent 初验：`go test ./... -count=1` 通过；`go test ./internal/mockfix -count=1` 初次发现间歇性 Logon timeout。
- 实现 Agent 修复：移除 acceptor 启动后的额外 TCP readiness 探测；mockfix 测试使用唯一 SessionID；测试命令超时调整为有界 3s。
- 主 Agent 复验：`go test ./internal/mockfix -count=10` 通过，`go test ./... -count=1` 通过，`go test ./internal/cli -run TestScenarioRunWithMockAcceptor -count=1` 通过，`git diff --check` 通过。
- 验证 Agent 复审：通过；额外执行 `go test ./internal/mockfix -count=10`、`go test ./internal/cli -run TestScenarioRunWithMockAcceptor -count=10`、`go test -race ./internal/mockfix -count=1`、`go test -race ./internal/cli -run TestScenarioRunWithMockAcceptor -count=1` 均通过。

## 评审问题清单与处理结果

- 阻断问题：mockfix 测试存在间歇性 Logon timeout。
  - 处理结果：移除启动探测连接，避免向 QuickFIX acceptor 制造空连接；测试 SessionID 唯一化，避免同进程重复测试中的 QuickFIX session 状态串扰；复验通过。
- 非阻断建议：字典样例注释不要暗示当前测试直接读取。
  - 处理结果：注释调整为文档和本地 mock 场景用 custom tag overlay 样例。
- 非阻断建议：样例配置固定端口可能让手工运行者误解。
  - 处理结果：测试说明补充手工运行时可按实际 mock acceptor 端口调整 `port`。
- 非阻断建议：动态端口使用先申请再释放存在极小 TOCTOU 窗口。
  - 处理结果：当前高频测试未复现，记录为可接受风险；后续 CI 如出现端口竞争可补失败重试。
- 非阻断建议：`ReplaceResponse` 配置路径可补单独测试。
  - 处理结果：当前已覆盖特殊 symbol 触发的 Reject 和 BusinessMessageReject；配置路径留作后续增强。

## 风险与后续建议

- mock acceptor 只用于基础链路验证，不代表真实券商或交易所网关的风控、撮合、序列号策略和自定义字段规则。
- task07 raw service 尚未实现，task09 未补 raw 能力。
- 后续如在 CI 中并发运行大量 QuickFIX 测试，可为动态端口分配和 session cleanup 增加重试与更细粒度日志。
