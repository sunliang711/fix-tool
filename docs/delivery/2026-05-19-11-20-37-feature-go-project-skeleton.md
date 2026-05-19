# 任务交付：项目骨架、配置模型、日志、Fx 组装

## 任务背景

根据 `docs/project/fix-tool/tasks/task-01-project-skeleton.md`，建立 `fix-tool` Go CLI 项目基础结构，完成 Cobra root command、Viper 配置加载、Zerolog 日志、Fx 依赖组装和默认配置文件。

## 实现方案

- 使用 `fix-tool` 作为 Go module path。
- 以 `cmd/fix-tool/main.go` 作为唯一入口。
- 通过 `internal/app` 组装 Fx module、CLI 命令和默认 logger。
- 通过 `internal/cli` 提供 root command、全局 flags、`version` 和 `config validate`。
- 通过 `internal/config` 统一加载默认值、默认配置文件、用户配置、私有配置、环境变量和 CLI flags。
- 通过 `internal/logging` 基于强类型配置创建 Zerolog logger。
- 通过 `internal/validate` 对强类型配置做 fail fast 校验。

## 文件变更

- 新增 `go.mod`、`go.sum`。
- 新增 `cmd/fix-tool/main.go`。
- 新增 `config/default.toml`。
- 新增 `internal/app`、`internal/cli`、`internal/config`、`internal/logging`、`internal/validate`。
- 更新 `docs/project/fix-tool/PROGRESS.md`。

## 配置与依赖变更

- 新增依赖：Cobra、Viper、Zerolog、Uber Fx、Validator。
- 新增环境变量前缀：`FIX_TOOL`。
- 支持加载顺序：内置默认值 < `config/default.toml` < `config.toml` < `private.toml` < 环境变量 < CLI flags。

## 测试结果

- `go test ./...`：通过。
- `go run ./cmd/fix-tool --help`：通过。
- `go run ./cmd/fix-tool config validate`：通过。

## 风险与后续建议

- 当前 profile 仅覆盖 P0 骨架字段，任务 02 接入 QuickFIX/Go 时需要继续补充 session store、dictionary 和连接生命周期配置。
- 当前 logger 已支持 `json` 和 `console` 两种格式，后续业务组件接入时可继续通过 Fx 注入复用。
