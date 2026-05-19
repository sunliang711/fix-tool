# 任务交付：admin 命令

## 任务背景

根据 `docs/project/fix-tool/tasks/task-04-admin-commands.md`，实现登录、登出、心跳和 TestRequest 命令，覆盖 FIX 会话管理的基础操作。

## 编排方案

- 实现 Agent：负责 admin service、CLI 命令、session 发送扩展和测试。
- 验证 Agent：先整理 task04 验证清单，再基于最终代码做独立审查和命令验证。
- 主 Agent：负责基线测试、验收命令、结果集成、进度文档和最终交付。

## 实现方案

- 在 `internal/admin` 新增 service，封装 `Logon`、`Logout`、`Heartbeat`、`TestRequest`。
- `logon` 命令语义定义为启动 QuickFIX session 并等待 `EventLogon`，不手写 Logon 业务发送。
- `logout`、`heartbeat`、`test-request` 通过 session 发送 admin message，并等待目标响应或超时。
- `test-request` 设置 `112=请求ID`，并等待带相同 `112` 的 Heartbeat 响应。
- CLI 新增 `logon`、`logout`、`heartbeat`、`test-request` 子命令。
- 输出复用 task03 renderer，按配置渲染请求和响应 trace。

## 文件与配置变更

- 新增 `internal/admin/service.go`。
- 新增 `internal/admin/service_test.go`。
- 新增 `internal/cli/admin.go`。
- 修改 `internal/cli/root.go`、`internal/cli/root_test.go`。
- 修改 `internal/fixsession/types.go`，为 session 增加 `Send` 能力。
- 更新 `docs/project/fix-tool/PROGRESS.md`。
- 未新增第三方依赖。

## 测试结果

- 实现 Agent：`go test ./...` 通过。
- 主 Agent：`go test ./...` 通过。
- 主 Agent：`go run ./cmd/fix-tool --help` 通过，可见 admin 命令。
- 主 Agent：`go run ./cmd/fix-tool test-request --help` 通过，可见 `--id`。
- 验证 Agent：`go test ./...` 通过，并复核 help 输出。

## 评审问题清单与处理结果

- 阻断问题：无。
- 非阻断建议：
  - 后续可兜底处理极端回调顺序下 request/response trace 缺失。
  - 后续可拒绝 TestRequest ID 中的 SOH/control chars。
  - 后续可为 `--output json` 定义单个可解析的双 trace JSON schema。

## 风险与后续建议

- 当前验证使用 fake manager/session，未连接真实 FIX 网关。
- Heartbeat 对端响应行为可能因网关实现而异，后续真实联调时需要补充场景验证。
- 后续 task05 可复用 admin service 的发送、等待和 trace 输出模式。
