# 任务交付：报文捕获、解析、字段名映射、脱敏渲染

## 任务背景

根据 `docs/project/fix-tool/tasks/task-03-packet-rendering.md`，实现 FIX 报文 trace 记录、raw 解析、字段名映射、BodyLength/CheckSum 校验、统一脱敏和多格式输出。

## 编排方案

- 实现 Agent：负责 `internal/trace`、`internal/dictionary`、`internal/render` 和 `testdata/messages` 的功能实现与测试。
- 验证 Agent：先整理 task03 验证清单，再基于最终代码做独立审查和测试验证。
- 主 Agent：负责基线测试、结果集成、进度文档和最终交付。

## 实现方案

- 在 `internal/trace` 定义 trace 模型、raw parser、BodyLength/CheckSum 校验和线程安全 recorder。
- 在 `internal/dictionary` 提供 P0 常用 FIX 字段、枚举解释、自定义 tag 合并和敏感字段识别。
- 在 `internal/render` 提供 table、json、raw 输出，并默认对敏感字段脱敏。
- 在 `testdata/messages` 增加 Logon、NewOrderSingle、ExecutionReport 样例报文。

## 文件与配置变更

- 新增 `internal/trace/`。
- 新增 `internal/dictionary/`。
- 新增 `internal/render/`。
- 新增 `testdata/messages/`。
- 更新 `docs/project/fix-tool/PROGRESS.md`。
- 未新增第三方依赖。

## 测试结果

- 实现 Agent：`go test ./...` 通过。
- 主 Agent：`go test ./...` 通过。
- 验证 Agent：`go test ./...` 通过；单独运行 `go test ./internal/trace -run TestParseRawValidatesBodyLengthAndCheckSum -count=1 -v` 通过。

## 评审问题清单与处理结果

- 阻断问题：无。
- 非阻断建议：后续可补充真实 SOH 输入、CheckSum 末尾字段约束、更多内置敏感 tag 的边界测试。

## 风险与后续建议

- 标准字段字典当前覆盖 P0 和常用字段，不是 FIX 全量字典。
- task03 只提供基础模块，尚未接入 CLI、session event 和 `inspect raw` 命令；后续任务可继续集成。
