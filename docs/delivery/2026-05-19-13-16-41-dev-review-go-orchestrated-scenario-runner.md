# 任务交付：scenario runner 与断言

## 任务背景

根据 `docs/project/fix-tool/tasks/task-08-scenario-runner.md`，实现 `fix-tool run scenario.yaml`，支持通过场景文件顺序执行 FIX 联调步骤，并对每步响应做基础断言。

## 编排方案

- 实现 Agent：负责 scenario 模型、加载校验、执行器、断言引擎、CLI 接入、样例和测试。
- 验证 Agent：先整理 task08 验证清单，再基于最终代码做独立复审。
- 主 Agent：负责本地验证、评审问题回传、进度文档、交付文档和最终提交。

## 实现方案

- 新增 `internal/scenario`，支持 YAML 场景文件、步骤模型、`wait.msg_type` 校验和基础断言。
- 新增顺序执行器，复用 admin/order service 执行 `logon`、`logout`、`heartbeat`、`test-request`、`order.new`、`order.cancel`、`order.replace`。
- 执行器使用 `KeepSession` 和共享 `SessionState`，避免多步骤场景反复断开 session。
- 断言支持 `equals`、`not_equals`、`exists`、`not_exists`、`in`，失败结果包含步骤、字段、期望值和实际值。
- CLI 新增 `fix-tool run scenario.yaml`，支持 `--json` 输出整体结果，支持 `--result-file` / `--output-file` 写 JSON 文件。
- 新增样例 `testdata/scenarios/order-lifecycle.yaml`。
- task07 尚未实现，`raw` action 当前保留为明确错误入口，等待后续 raw service 接入。

## 文件与配置变更

- 新增 `internal/scenario/`。
- 新增 `internal/cli/scenario.go`。
- 修改 `internal/cli/root.go`、`internal/cli/root_test.go`。
- 新增 `testdata/scenarios/order-lifecycle.yaml`。
- 更新 `go.mod`，将已有 YAML 依赖标记为 direct。
- 更新 `docs/project/fix-tool/PROGRESS.md`。

## 测试结果

- 实现 Agent：`go test ./...` 通过，`go run ./cmd/fix-tool run --help` 通过。
- 主 Agent 初验：`go test ./...`、`go run ./cmd/fix-tool run --help`、`go test ./internal/scenario -run TestRunner -count=1`、`go test ./internal/cli -run TestRunHelpShowsScenarioFlags -count=1`、`git diff --check` 通过。
- 验证 Agent 初审：未发现阻断问题，提出 cleanup stop 失败时结果状态需表达失败的建议。
- 实现 Agent 修复：`Result` 增加整体 `error` 字段；stop 失败时整体状态置为 `failed` 并写入错误；补充单元测试。
- 主 Agent 最终验证：`go test ./internal/scenario -count=1`、`go test ./...`、`git diff --check` 通过。

## 评审问题清单与处理结果

- 阻断问题：无。
- 非阻断建议：session stop 失败时，命令失败但 JSON 结果可能仍显示 `passed`。
  - 处理结果：`Result` 新增 `error` 字段，stop 失败时整体状态置为 `failed`，并补充 `TestRunnerMarksResultFailedWhenStopFails`。
- 非阻断建议：未知字段名在 `not_exists` 断言中可能因拼写错误而通过。
  - 处理结果：记录为后续增强；当前仍支持数字 tag 和常用字段别名。
- 非阻断建议：当前只支持 YAML，不支持 TOML。
  - 处理结果：第一期按任务要求中的 “YAML 或 TOML” 选择 YAML，未扩展第二套解析路径。

## 风险与后续建议

- 当前测试使用 fake service，不连接真实 FIX 网关。
- task07 尚未实现，raw action 暂不可用；task07 完成后应接入真实 raw service。
- 如果后续场景需要独立等待异步消息，应在 service 层暴露可控事件消费接口，避免和现有 order/admin service 的响应等待逻辑抢事件。
