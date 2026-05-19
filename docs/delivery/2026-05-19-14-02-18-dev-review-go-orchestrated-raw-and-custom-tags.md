# 任务交付：raw send、inspect raw、自定义 tag overlay

## 任务背景

根据 `docs/project/fix-tool/tasks/task-07-raw-and-custom-tags.md`，补齐 raw message 发送、离线 raw 解析和自定义 tag overlay，满足非标准字段和临时联调需求。

## 编排方案

- 实现 Agent：负责 raw builder、raw service、CLI 接入、scenario raw 接入和测试。
- 验证 Agent：先整理 task07 验证清单，再基于最终代码做独立复审。
- 主 Agent：负责本地验证、阻断问题修复、进度文档、交付文档和最终提交。

## 实现方案

- 新增 `internal/raw`，用 QuickFIX `Message` 构造 raw 请求，由 QuickFIX 生成 BodyLength、CheckSum 和真实 SOH。
- `raw send` 支持 `--msg-type` 和多个 `--tag key=value`。
- `--tag` 拒绝 `8/9/10/34/35/49/52/56`，允许普通 body tag 如 `11/38/55`。
- `raw send` 发送前输出风险提示和参数校验结果，发送后输出 request/response trace。
- raw service 复用 `fixsession.Manager`，等待 Logon 后发送消息，并按超时等待 ExecutionReport、Reject、BusinessMessageReject 或 admin response。
- 新增 `inspect raw <message>`，支持 `|` 展示分隔符和真实 SOH，复用 trace/render 输出 BodyLength、CheckSum、字段名、类型、枚举和敏感标记。
- custom tag overlay 通过 `profile.custom_tags` 加载，渲染遵守 `output.redact_sensitive`。
- scenario runner 的 `raw` action 已接入 raw service，支持 `input.msg_type` 和 `input.tags`。
- renderer 的 table/json 字段视图补充 `Type`，满足 custom tag 类型展示要求。

## 文件与配置变更

- 新增 `internal/raw/`。
- 新增 `internal/cli/raw.go`。
- 修改 `internal/cli/root.go`、`internal/cli/root_test.go`、`internal/cli/scenario.go`。
- 修改 `internal/scenario/model.go`、`internal/scenario/runner.go` 和相关测试。
- 修改 `internal/render/types.go`、`internal/render/render.go`、`internal/render/render_test.go`，输出字段类型。
- 修改 `internal/dictionary/dictionary.go` 和测试，兼容 custom tag enum key 大小写。
- 新增 `internal/mockfix/raw_integration_test.go`。
- 更新 `testdata/dictionaries/custom-tags.toml`。
- 更新 `docs/project/fix-tool/PROGRESS.md`。

## 测试结果

- 实现 Agent：`go test ./... -count=1` 通过。
- 主 Agent 初验：`go test ./... -count=1`、`go test ./internal/raw -count=1`、`go test ./internal/mockfix -run TestRawServiceSendsMessageAgainstMockAcceptor -count=1`、`go run ./cmd/fix-tool raw send --help`、`go run ./cmd/fix-tool inspect raw --help`、`git diff --check` 通过。
- 验证 Agent 初审：发现 1 个阻断问题。
- 主 Agent 修复：renderer field view 增加 `Type`，table/json 均输出字段类型，并补充测试。
- 主 Agent 复验：`go test ./internal/render -count=1`、`go test ./... -count=1`、`git diff --check` 通过。
- 验证 Agent 复审：通过，无阻断问题。

## 评审问题清单与处理结果

- 阻断问题：custom tag overlay 未展示字段类型。
  - 处理结果：`fieldView` 增加 `Type`，table 增加 `Type` 列，JSON 输出 `fields[].type`，测试覆盖标准字段和 custom tag 类型。
- 非阻断建议：raw 响应匹配对无关联字段请求较宽。
  - 处理结果：当前按顺序执行和基础联调场景接受；后续如支持同 session 并发 raw send，可增加更细粒度 correlation 规则。
- 非阻断建议：`raw send` 的“校验结果”是参数级摘要，最终 wire message 的 BodyLength/CheckSum 在发送后的 request trace 中展示。
  - 处理结果：保留当前文案和 request trace 输出，后续如需要可增加发送前 dry-run 预览。

## 风险与后续建议

- raw send 灵活度高，仍可能发出业务层非法但协议层可编码的报文。
- 当前 raw response matcher 面向基础联调顺序执行，不适合同一 session 多个 raw 请求并发发送。
- mockfix 集成测试覆盖基础交易链路，不代表真实网关行为。
