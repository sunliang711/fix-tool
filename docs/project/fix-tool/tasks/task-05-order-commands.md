# 任务 05：order 命令

## 任务目标

实现新单、撤单、改单命令，并能够匹配 ExecutionReport、Reject 和 BusinessMessageReject。

## 技术方案

- 在 `internal/message` 实现 NewOrderSingle、OrderCancelRequest、OrderCancelReplaceRequest 构造。
- 在 `internal/cli` 增加 `order new`、`order cancel`、`order replace`。
- 在 `internal/trace` 或 service 层实现 response matcher。
- 使用 ClOrdID、OrigClOrdID、OrderID、MsgType 做响应关联。
- 支持通过 `--tag key=value` 添加自定义字段。

## 验收标准

- 可发送新单、撤单、改单。
- 可打印请求和响应详情。
- 可识别 ExecutionReport、Reject、BusinessMessageReject。
- 必填参数缺失时给出清晰中文提示。
- 单元测试覆盖消息构造和响应匹配。

## 实现步骤

1. 定义订单命令请求 DTO。
2. 实现 side、order type、time in force 等枚举转换。
3. 实现三类订单消息构造。
4. 实现响应 matcher。
5. 接入 renderer。
6. 添加单元测试。

## 前置依赖

- 任务 02。
- 任务 03。

## 风险

- 不同网关对必填字段要求不同，第一期通过 profile 默认字段和 `--tag` 扩展解决。

