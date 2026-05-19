# 任务交付：交互式 shell

## 任务背景

根据 `docs/project/fix-tool/tasks/task-06-interactive-shell.md`，实现 `fix-tool shell --profile uat`，允许用户保持一个 FIX session 并连续执行基础 admin、order 和 trace 查询操作。

## 编排方案

- 实现 Agent：负责 shell runner、命令解析、CLI 接入、service 生命周期扩展和测试。
- 验证 Agent：先整理 task06 验证清单，再基于最终代码做独立审查和复审。
- 主 Agent：负责基线测试、验收命令、阻断问题回传、进度文档、交付文档和最终提交。

## 实现方案

- 在 `internal/shell` 新增 `Runner`、有限命令解析器和共享 `SessionState`。
- 在 `internal/cli` 新增 `shell` 命令，读取 `stdin`，输出 prompt 和命令结果。
- shell 内支持 `logon`、`logout`、`heartbeat`、`test-request`、`order new`、`order cancel`、`order replace`、`trace list`、`exit`。
- shell 内 admin/order service 共享同一个 `fixsession.Manager` 和登录状态。
- admin/order service 新增 `KeepSession` 和 `SessionState` 选项，一次性 CLI 默认行为保持每条命令后 stop。
- shell 退出、EOF、scanner error、context cancel 都会执行 session stop。
- `trace list` 使用 task03 renderer 展示 shell 期间记录的请求/响应 trace。

## 文件与配置变更

- 新增 `internal/shell/`。
- 新增 `internal/cli/shell.go`。
- 修改 `internal/cli/root.go`、`internal/cli/root_test.go`、`internal/cli/types.go`。
- 修改 `internal/admin/service.go`、`internal/admin/service_test.go`。
- 修改 `internal/order/service.go`、`internal/order/service_test.go`。
- 更新 `docs/project/fix-tool/PROGRESS.md`。
- 未新增第三方依赖。

## 测试结果

- 实现 Agent：`go test ./...` 通过。
- 主 Agent 初验：`go test ./...` 通过，`go run ./cmd/fix-tool shell --help` 通过。
- 验证 Agent 初审：发现 1 个阻断问题。
- 实现 Agent 修复：`go test ./...` 通过，`git diff --check` 通过。
- 验证 Agent 复审：通过，无阻断问题；追加 `go test -race ./internal/shell` 通过。
- 主 Agent 最终验证：`go test ./...`、`git diff --check`、`go run ./cmd/fix-tool shell --help` 通过。

## 评审问题清单与处理结果

- 阻断问题：shell 空闲等待输入时无法响应 `context cancel`，导致 session stop 不能保证执行。
  - 处理结果：输入读取改为受控 goroutine + channel，主循环同时监听输入和 `ctx.Done()`；context cancel 时中断可关闭输入源并等待读循环退出。
- 非阻断建议：
  - parser 当前不支持 quoted value，符合第一期限界。
  - 如果使用不可关闭且永久阻塞的自定义 `io.Reader`，Go 本身无法外部强制中断读取；默认 CLI 使用的 `os.Stdin` 是可关闭输入源。
  - shell help 后续可补充 shell 内可用命令说明。

## 风险与后续建议

- 当前验证使用 fake manager/session，未连接真实 FIX 网关。
- `SessionState` 由 service 成功路径更新，后续如需要处理对端异步 logout/disconnect，可增加事件监听状态同步。
- 后续 task07 可复用 shell 的有限 parser 和 trace 输出能力。
