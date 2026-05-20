# 任务交付：ordermatch 订单回放测试

## 测试目标

- 使用 `order` 子命令向 `~/Sync/chief/quickfixgo-examples` 的 `ordermatch` 服务端发送订单。
- 覆盖挂新限价单、吃单、非交叉限价单、市价单、撤单、撤不存在订单、改单。
- 将订单详情固化为可重放脚本，方便后续回归。

## 测试脚本

- `scripts/ordermatch-order-replay.sh`
- 默认连接已启动的 `127.0.0.1:5001` ordermatch。
- 如需脚本自动构建并启动 ordermatch：

```bash
START_ORDERMATCH=1 ./scripts/ordermatch-order-replay.sh
```

## 本次执行

```bash
bash -n scripts/ordermatch-order-replay.sh
shellcheck scripts/ordermatch-order-replay.sh
START_ORDERMATCH=1 CASE_TIMEOUT_SECONDS=8 ./scripts/ordermatch-order-replay.sh
go test ./...
```

结果目录：

- `tmp/ordermatch-order-replay-20260520063543/summary.tsv`
- `tmp/ordermatch-order-replay-20260520063543/ordermatch-server.log`

## 发现的问题

1. 非交叉限价单被错误撮合：买单 `10.00` 与卖单 `11.00` 成交，双方都收到 `ExecType=2`。
2. 市价单不被支持：`OrdType=1` 且无 `Price(44)` 时返回 `BusinessMessageReject`，文本为 `Conditionally Required Field Missing (44)`。
3. 撤单不带 `OrderID` 时，服务端实际返回取消回报，但 `fix-tool order cancel` 无法关联该回报并超时。
4. 撤不存在订单时，服务端无任何业务拒绝回报，客户端超时。
5. 改单 `G` 不被 ordermatch 支持，返回 `BusinessMessageReject`，文本为 `Unsupported Message Type`。
6. 成交回报中的 `AvgPx(6)` 保持 `0.00`，与实际成交价不一致。
