# Shell Order Stream 开发与评审交付

## 任务背景

为 shell 模式增加测试用的定时发单能力，支持按指定时间间隔持续发送 `order new`，并支持 ClOrdID 递增或随机生成，以及部分订单参数按序列变化。

## 编排方案

- 实现 Agent：负责实现 `order stream start|stop|status`、后台任务管理和测试。
- Review Agent：独立审查 shell/order 并发、context 取消、命令解析兼容性和测试覆盖。
- 主 Agent：集成补丁、修复评审问题、执行验证并汇总交付。

## 实现方案

- 新增 shell 命令：
  - `order stream start`
  - `order stream stop`
  - `order stream status`
- `start` 复用 `order new` 参数，并新增控制参数：
  - `--interval`
  - `--count`
  - `--cl-ord-id-prefix`
  - `--cl-ord-id-mode`
  - `--start-seq`
  - `--side-mode`
  - `--symbol-seq`
  - `--qty-seq`
  - `--price-seq`
- 新增 `orderStream` 管理后台 goroutine，同一 shell 实例只允许一个 stream 运行。
- shell 退出和 `logout` 时自动停止 stream，避免后台 goroutine 泄漏。
- 前台 admin/order 命令和后台 stream 共享互斥锁，避免多个命令并发消费同一个 FIX event stream。
- TUI Heartbeat 区增加常驻 Stream 状态行，通过 1s tick 自动刷新 `running/stopped`、sent、ok、failed 和 last_error。
- TUI Command 输入框增加轻量 Vim 模式，默认 insert，`Esc` 进入 normal，normal 支持 `i/h/l/w/b/e/d/Enter`。

## 文件与配置变更

- 修改 `internal/shell/parser.go`
- 修改 `internal/shell/runner.go`
- 新增 `internal/shell/stream.go`
- 修改 `internal/shell/parser_test.go`
- 修改 `internal/shell/runner_test.go`
- 新增 `internal/shell/stream_test.go`
- 修改 `internal/shell/tui.go`
- 修改 `internal/shell/tui_test.go`

无配置项和依赖变更。

## 测试结果

- `GOCACHE=/Users/eagle/Sync/chief/fix-tool/tmp/go-build-cache go test ./internal/shell -count=1`：通过。
- `GOCACHE=/Users/eagle/Sync/chief/fix-tool/tmp/go-build-cache go test -race ./internal/shell -count=1`：通过。
- `GOCACHE=/Users/eagle/Sync/chief/fix-tool/tmp/go-build-cache go test ./internal/order ./internal/admin -count=1`：通过。
- `GOCACHE=/Users/eagle/Sync/chief/fix-tool/tmp/go-build-cache go test ./... -count=1`：受本地沙箱限制失败，失败点为 mock acceptor 监听 `127.0.0.1:0` 被拒绝，和本次 shell stream 改动无关。
- `git diff --check`：通过。

## 评审问题与处理结果

- 问题：后台 stream 与前台 admin 命令可能并发消费同一个 `manager.Events()`，导致响应被错误命令抢走。
  - 处理：将 `logon/logout/heartbeat/test-request` 与前台 order、后台 stream 纳入同一把 `orderMu` 串行。
  - 验证：新增 `TestRunnerAdminCommandWaitsForActiveStreamOrder`，并通过 `go test -race ./internal/shell`。
- 问题：`order stream start` 的 help 只罗列 flag，用户难以理解必填项、默认值和 `*-seq` 覆盖关系。
  - 处理：将 help 改为 `required`、`optional order flags`、`stream controls`、`variation`、`examples` 分组。
  - 验证：补充 help 文案断言，并保持 `TestShellHelpTextUsesShortLines` 通过。
- 问题：启动前校验需要和 help 的覆盖规则一致。
  - 处理：允许 `--symbol-seq` 替代 `--symbol`、`--qty-seq` 替代 `--qty`、`--price-seq` 替代 limit 单 `--price`；`--side` 仍必填。
  - 验证：补充 seq 替代必填项、空 start、缺 side、market 单无 price 的测试。
- 问题：TUI 中缺少 stream 是否运行的常驻提示。
  - 处理：在 Heartbeat 面板增加 `Stream:` 状态行，并通过 `Runner.streamStatus()` 安全读取状态快照。
  - 验证：补充 stopped/running 状态渲染测试，并通过 `go test -race ./internal/shell`。
- 问题：TUI Command 输入框缺少简单 Vim 操作模式，且 `Esc` 会直接退出程序。
  - 处理：新增独立 Command 输入模式；Command pane 中 `Esc` 切 normal，不退出；`Ctrl+C` 仍退出。
  - 验证：补充模式切换、光标移动、删除、提交、退出键和非 Command pane `Esc` 回归测试。
- 问题：`w` 在分隔符位置可能跳过紧邻单词。
  - 处理：修正 `moveCommandCursorNextWord`，当前为分隔符时从当前分隔符开始寻找下一个 word。
  - 验证：补充 `TestTUIModelCommandNormalNextWordFromSeparator`。

## 风险与后续建议

- `Stop` / `StopIfRunning` 当前等待后台任务退出，无额外停止超时；现有 `OrderService.NewOrder` 有内部 timeout，风险可接受。
- `order stream start` 的必填业务字段仍复用下游 order/message 校验；后续可在启动前增加更早的校验，避免持续失败的 stream。

## 结论

独立 Review Agent 复审结论为：无阻断问题，通过复审。
