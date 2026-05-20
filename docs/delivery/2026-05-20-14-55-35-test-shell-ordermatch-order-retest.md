# 任务交付：ordermatch 订单回放复测

## 本次执行

```bash
bash -n scripts/ordermatch-order-replay.sh
shellcheck scripts/ordermatch-order-replay.sh
START_ORDERMATCH=1 CASE_TIMEOUT_SECONDS=8 ./scripts/ordermatch-order-replay.sh
go test ./...
cd ~/Sync/chief/quickfixgo-examples && go test ./...
```

结果目录：

- `tmp/ordermatch-order-replay-20260520065441/summary.tsv`
- `tmp/ordermatch-order-replay-20260520065441/ordermatch-server.log`

## 复测结论

- 非交叉限价单已不会错误成交，买价 `10.00` 和卖价 `11.00` 都保留在订单簿。
- 市价单已可吃单，`OrdType=1` 无 `Price(44)` 时成交价为对手方挂单价 `12.00`。
- 撤单不带 `OrderID` 已能返回并被 `fix-tool` 关联到取消回报。
- 改单 `G` 已能返回 `ExecType=5` / `OrdStatus=5`，订单簿中保留新 `ClOrdID` 和新价格。
- 成交回报 `AvgPx(6)` 已随成交价更新。

## 剩余问题

1. 撤不存在订单时，ordermatch 已返回 `OrderCancelReject(35=9)`，内容包含 `OrdStatus=8` 和 `Text=Unknown OrigClOrdID`。
2. `fix-tool order cancel` 目前未把 `35=9` 识别为订单响应类型，因此该场景仍然等到超时才结束。
