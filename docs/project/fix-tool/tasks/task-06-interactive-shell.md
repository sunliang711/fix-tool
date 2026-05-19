# 任务 06：交互式 shell

## 任务目标

实现 `fix-tool shell --profile uat`，允许用户保持一个 FIX session 并连续执行登录、下单、撤单、改单、心跳、查看 trace 等操作。

## 技术方案

- 在 `internal/cli` 增加 shell command。
- shell 内复用已有 command service，不重复实现业务逻辑。
- 支持基本命令：`logon`、`logout`、`heartbeat`、`test-request`、`order new`、`order cancel`、`order replace`、`trace list`、`exit`。
- shell 退出时优雅关闭 session。

## 验收标准

- shell 模式可持续使用同一个 profile。
- shell 内命令和一次性 CLI 命令行为一致。
- 退出时 session 正常关闭，无 goroutine 泄漏。
- 命令错误不会导致 shell 异常退出。

## 实现步骤

1. 选择轻量交互输入方案。
2. 定义 shell 命令解析器。
3. 复用 admin/order service。
4. 实现 trace 查询命令。
5. 处理退出和 context cancel。
6. 添加基础测试。

## 前置依赖

- 任务 04。
- 任务 05。

## 风险

- shell 内命令解析容易和 Cobra 重复。第一期只做有限命令集，后续再增强补全和历史记录。

