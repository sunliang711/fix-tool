# 任务 04：admin 命令

## 任务目标

实现登录、登出、心跳和 TestRequest 命令，覆盖 FIX 会话管理的基础操作。

## 技术方案

- 在 `internal/cli` 增加 `logon`、`logout`、`heartbeat`、`test-request` 子命令。
- 在 service 层封装 admin message 操作。
- 复用 session manager 发送消息并等待目标响应或超时。
- 输出请求和响应 trace。

## 验收标准

- `fix-tool logon --profile uat` 可发起登录。
- `fix-tool logout --profile uat` 可登出。
- `fix-tool heartbeat --profile uat` 可发送心跳。
- `fix-tool test-request --profile uat --id ping-001` 可发送 TestRequest。
- 每个命令都打印请求包和响应包详情。

## 实现步骤

1. 定义 admin service。
2. 实现 Logon/Logout/Heartbeat/TestRequest 消息构造。
3. 实现等待响应和超时处理。
4. 接入 trace renderer。
5. 添加命令参数校验。
6. 添加单元测试。

## 前置依赖

- 任务 02。
- 任务 03。

## 风险

- Logon 通常由 QuickFIX/Go 自动管理，命令语义需要定义为“启动 session 并等待 Logon 成功”。

