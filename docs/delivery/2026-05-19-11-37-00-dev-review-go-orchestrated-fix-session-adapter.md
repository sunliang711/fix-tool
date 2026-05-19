# 任务交付：QuickFIX/Go session adapter 与 profile 加载

## 任务背景

根据 `docs/project/fix-tool/tasks/task-02-fix-session-adapter.md`，在任务 01 的 CLI 骨架基础上封装 QuickFIX/Go initiator，支持根据强类型 profile 创建、启动、停止 FIX session，并捕获基础会话事件。

## 编排方案

- 实现 Agent：负责 `internal/fixsession` 的 adapter、settings 映射、事件出口、Fx 生命周期和单元测试。
- 验证 Agent：独立只读复核 task02 验收标准，检查实现正确性与测试覆盖。
- 主 Agent：负责调度、集成检查、进度文档和最终交付。

## 实现方案

- 新增 `Manager`、`Session`、`Application` 接口及 `QuickFIXManager` 实现。
- 将 `config.ProfileConfig` 映射为 QuickFIX/Go settings，覆盖 BeginString、SenderCompID、TargetCompID、SocketConnectHost、SocketConnectPort、HeartBtInt、DataDictionary、Transport/App Dictionary 和 TLS 配置。
- 使用 QuickFIX/Go `Application` 回调捕获 `Logon`、`Logout`、`ToAdmin`、`FromAdmin`、`ToApp`、`FromApp` 事件。
- 对用户名、密码和标记为 sensitive 的 custom tag 做事件消息脱敏。
- 通过 `fixsession.Module` 与 `RegisterLifecycle` 支持 Fx 生命周期管理。
- TLS 默认证书校验开启，配置关闭校验时输出英文风险日志。

## 文件与配置变更

- 新增 `internal/fixsession/application.go`。
- 新增 `internal/fixsession/manager.go`。
- 新增 `internal/fixsession/settings.go`。
- 新增 `internal/fixsession/types.go`。
- 新增 `internal/fixsession/*_test.go`。
- 更新 `go.mod`、`go.sum`，新增 QuickFIX/Go 依赖。
- 更新 `docs/project/fix-tool/PROGRESS.md`。

## 测试结果

- 实现 Agent 执行 `go test ./...`：通过。
- 验证 Agent 独立执行 `go test ./...`：通过。
- 验证 Agent 独立执行 `go test -count=1 ./...`：通过。
- 主 Agent 执行 `go test ./...`：通过。

## 评审问题清单与处理结果

- 阻断问题：无。
- 非阻断建议：`internal/fixsession.Module` 当前未纳入 `internal/app.Module`，因为现有 CLI 还没有全局供应 `config.ProfileConfig`；后续具体连接命令接入 session 时再组合该模块。
- 非阻断建议：后续可补 logger capture 或 `NewManager` 构造测试，加强 TLS 风险日志路径覆盖。

## 风险与后续建议

- 当前 adapter 不连接真实外部 FIX 服务，启动/停止行为通过 fake initiator 单元测试覆盖；真实联调应在后续 mock acceptor 或集成测试任务中补齐。
- 任务 03 可基于本次事件出口继续实现报文捕获、解析、字段名映射和脱敏渲染。
