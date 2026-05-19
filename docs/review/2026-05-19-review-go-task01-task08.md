# task01-task08 Go 复查记录

## 审查范围

- `docs/project/fix-tool/tasks/task-01-project-skeleton.md` 至 `task-08-scenario-runner.md`
- `docs/project/fix-tool/PROGRESS.md`
- `docs/project/fix-tool/testing.md`
- `internal/config`、`internal/validate`、`internal/fixsession`、`internal/trace`、`internal/render`、`internal/admin`、`internal/message`、`internal/order`、`internal/shell`、`internal/raw`、`internal/scenario`

## 审查基线

- Go 分层、配置、错误处理、并发生命周期和测试规则
- task01-task08 的任务目标、验收标准和已交付代码

## 发现的问题

| 序号 | 位置 | 严重等级 | 问题描述 | 处理结果 |
|:---:|------|:---:|----------|----------|
| 1 | `internal/validate/config.go` | 警告 | `profile.custom_tags` 未逐项校验，非法 tag、空 name 或空 type 可通过配置校验，影响 task07 custom tag overlay 的可靠性 | 已补充 custom tag 校验，并新增单元测试 |
| 2 | `docs/project/fix-tool/testing.md` | 建议 | 测试说明仍写 task07 raw service 未实现，与当前 task07/task08 实现状态不一致 | 已更新为 raw action 已接入 |

## 验证结果

- `go test ./...` 通过
- `go vet ./...` 通过
- `git diff --check` 通过

## 残余风险

- `raw send` 的响应匹配仍以基础顺序联调为目标，不适合同一 session 内并发发送多个 raw 请求。
- mock acceptor 只覆盖基础链路，不代表真实券商或交易所网关行为。
- scenario 当前只支持 YAML，未实现 TOML 场景文件解析。
