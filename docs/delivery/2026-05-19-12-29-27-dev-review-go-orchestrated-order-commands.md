# 任务交付：order 命令

## 任务背景

根据 `docs/project/fix-tool/tasks/task-05-order-commands.md`，实现新单、撤单、改单命令，并能够匹配 ExecutionReport、Reject 和 BusinessMessageReject。

## 编排方案

- 实现 Agent：负责订单消息构造、order service、CLI 命令和单元测试。
- 验证 Agent：先整理 task05 验证清单，再基于最终代码做独立审查和复审。
- 主 Agent：负责基线测试、验收命令、阻断问题回传、结果集成、进度文档和最终交付。

## 实现方案

- 在 `internal/message` 新增 `NewOrderSingle(D)`、`OrderCancelRequest(F)`、`OrderCancelReplaceRequest(G)` 构造。
- 支持 `side`、`ord type`、`time in force` 枚举转换。
- 支持 `--tag key=value` 添加自定义 body tag，并拒绝覆盖协议头尾、会话头和订单核心字段。
- 在 `internal/order` 新增 service，封装启动 session、发送订单消息、等待响应或超时。
- 响应 matcher 支持识别 `ExecutionReport(8)`、`Reject(3)`、`BusinessMessageReject(j)`，并通过 `ClOrdID`、`OrigClOrdID`、`OrderID` 关联响应。
- 在 CLI 新增 `order new`、`order cancel`、`order replace`。
- 输出复用 task03 renderer，打印请求和响应 trace。

## 文件与配置变更

- 新增 `internal/message/order.go`。
- 新增 `internal/message/order_test.go`。
- 新增 `internal/order/service.go`。
- 新增 `internal/order/service_test.go`。
- 新增 `internal/cli/order.go`。
- 修改 `internal/cli/root.go`、`internal/cli/root_test.go`。
- 更新 `docs/project/fix-tool/PROGRESS.md`。
- 未新增第三方依赖。

## 测试结果

- 实现 Agent：`go test ./...` 通过。
- 验证 Agent 初审：发现 2 个阻断问题。
- 实现 Agent 修复：`go test ./...` 通过，`git diff --check` 通过。
- 验证 Agent 复审：通过，无阻断问题。
- 主 Agent：`go test ./...` 通过。
- 主 Agent：`go run ./cmd/fix-tool order --help`、`order new --help`、`order cancel --help`、`order replace --help` 通过。

## 评审问题清单与处理结果

- 阻断问题 1：`--tag` 可覆盖订单核心字段，破坏请求语义和响应 matcher。
  - 处理结果：已拒绝覆盖 `8/9/10/34/35/49/52/56/11/21/37/38/40/41/44/54/55/59/60`。
- 阻断问题 2：`OrderCancelReplaceRequest(G)` 可缺少 `Symbol(55)` 和 `Side(54)`。
  - 处理结果：已改为必填校验，缺失时返回中文错误。
- 非阻断建议：后续可结合 `RefMsgType(372)` 等字段进一步收紧无关联 Reject / BusinessMessageReject 匹配。

## 风险与后续建议

- 当前验证使用 fake manager/session，未连接真实 FIX 网关。
- 不同网关可能要求更多订单字段，第一期通过 `--tag` 添加非核心自定义字段扩展。
- 后续 task06 可复用 order service 的发送、等待和 trace 输出模式。
