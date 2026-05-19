# 任务 01：项目骨架、配置模型、日志、Fx 组装

## 任务目标

建立 Go CLI 项目基础结构，完成 Cobra root command、Viper 配置加载、Zerolog 日志、Fx 依赖组装和默认配置文件。

## 技术方案

- 创建 `cmd/fix-tool/main.go` 作为唯一入口。
- 创建 `internal/app` 注册 Fx module。
- 创建 `internal/cli` 定义 root command、全局 flags、版本命令。
- 创建 `internal/config` 统一加载内嵌默认配置、用户配置、私有配置、环境变量和 flags。
- 创建 `config/default.toml` 作为内嵌默认配置源文件。
- 创建 `internal/validate` 对强类型配置做 fail fast 校验。

## 验收标准

- `go test ./...` 通过。
- `fix-tool --help` 可展示根命令帮助。
- `fix-tool config validate` 可校验默认配置。
- 配置加载后业务层不直接依赖 Viper。
- 日志输出为结构化英文消息。

## 实现步骤

1. 初始化 Go module。
2. 添加 Cobra、Viper、Zerolog、Fx、Validator 依赖。
3. 定义 `AppConfig`、`ProfileConfig`、`TLSConfig`、`OutputConfig`。
4. 实现配置加载顺序和环境变量前缀。
5. 实现 logger provider。
6. 实现 root command 和 `config validate`。
7. 补充基础单元测试。

## 前置依赖

无。

## 风险

- 配置模型过早复杂化会拖慢后续开发。第一期只实现 P0 必需字段，扩展字段预留结构。
