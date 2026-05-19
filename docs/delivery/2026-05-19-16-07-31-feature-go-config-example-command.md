# 任务交付：config example 子命令

## 任务背景

为 `fix-tool` 增加生成 `config-example.toml` 的 CLI 子命令，便于用户从二进制直接生成完整配置样例。

## 实现方案

- 新增根包嵌入 `config-example.toml`，保证命令输出来自仓库中的同一份示例文件。
- 在 `fix-tool config` 下新增 `example` 子命令。
- 默认输出到 `config-example.toml`。
- 支持 `--output` / `-o` 指定输出路径。
- 默认不覆盖已有文件，支持 `--force` 覆盖。

## 文件变更

- 新增 `config_example.go`。
- 修改 `internal/cli/root.go`。
- 修改 `internal/cli/root_test.go`。
- 更新 `README.md` 中的配置示例生成命令。

## 验证结果

- `go test ./ ./internal/cli ./internal/validate -count=1`
- `go test ./... -count=1`
- `go vet ./...`
- `make build VERSION=v0.1.0-test`
- `go run ./cmd/fix-tool config example --output <temp-file>` 后与 `config-example.toml` 内容一致。

## 风险与后续建议

- `config example --output` 会在该子命令下表示输出文件；根命令已有全局 `--output` 输出格式参数，使用时建议把该参数放在 `config example` 之后。
