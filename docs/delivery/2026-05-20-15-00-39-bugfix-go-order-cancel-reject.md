# 任务交付：支持 OrderCancelReject(35=9)

## 问题背景

`ordermatch` 对撤不存在订单返回标准 `OrderCancelReject(35=9)`，但 `fix-tool order cancel` 未将该消息识别为订单响应，导致命令等待到超时。

## 根因分析

- `internal/order/service.go` 的订单响应匹配只包含 `ExecutionReport(8)`、`Reject(3)`、`BusinessMessageReject(j)`。
- `internal/raw/service.go` 的 raw 响应匹配也未覆盖 `OrderCancelReject(9)`。
- `internal/message/order.go` 没有定义 `35=9` 的消息类型常量。

## 修复方案

- 新增 `MsgTypeOrderCancelReject = "9"`。
- `order` 和 `raw` 响应匹配逻辑支持 `OrderCancelReject(9)`。
- 新增 service 层单元测试，覆盖 `CancelOrder` 收到 `35=9` 时正常返回。

## 验证结果

```bash
gofmt -w internal/message/order.go internal/order/service.go internal/raw/service.go internal/order/service_test.go
go test ./...
bash -n scripts/ordermatch-order-replay.sh
shellcheck scripts/ordermatch-order-replay.sh
START_ORDERMATCH=1 CASE_TIMEOUT_SECONDS=8 ./scripts/ordermatch-order-replay.sh
```

回放结果目录：

- `tmp/ordermatch-order-replay-20260520065941/summary.tsv`

关键结果：

- `11_cancel_unknown_order` 状态从超时变为 `0`。
- 捕获响应：`35=9, 11=FT-20260520065941-011, 39=8, 58=Unknown OrigClOrdID`。
