# 任务交付：config-example.toml 路径调整

## 重构背景

将仓库内的配置样例文件收敛到 `config/` 目录下，避免根目录同时承载源码入口和配置样例文件。

## 重构方案

- 将 `config-example.toml` 移动到 `config/config-example.toml`。
- 更新根包 `go:embed` 路径，保证 `fix-tool config example` 仍从同一份样例内容生成文件。
- 更新 release 打包脚本，使发布包内同样保留 `config/config-example.toml`。
- 更新配置校验测试、README 和配置维护约定中的仓库路径引用。

## 行为保持策略

- `fix-tool config example` 的默认输出文件名仍为 `config-example.toml`。
- `fix-tool config example --output <file>` 的输出内容保持不变。
- 配置加载顺序、配置结构和校验规则不变。

## 测试与验证结果

- `go test ./... -count=1`：通过。
- `git diff --check`：通过。
- `go run ./cmd/fix-tool config example --output <temp-file>` 后与 `config/config-example.toml` 内容一致。
- `make release VERSION=v0.0.0-test DIST_DIR=<temp-dir>/dist`：通过，release tar 包包含 `config/config-example.toml`。

## 风险与后续建议

- 历史 delivery 文档中保留了旧路径描述，作为历史记录未回写。
- 外部用户如果直接引用仓库根目录的 `config-example.toml`，需要改为 `config/config-example.toml`；CLI 生成配置样例不受影响。
