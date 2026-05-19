# 任务交付：打包发布与使用文档

## 任务背景

根据 `docs/project/fix-tool/tasks/task-10-release-docs.md`，补齐 release 前需要的版本信息、二进制打包、checksum、CI 验证和用户文档。

## 实现方案

- 新增 `internal/version` 包，提供可通过 `go build -ldflags` 注入的 `version`、`commit`、`build_time`。
- 调整 `fix-tool version`，输出稳定的三行版本信息。
- 新增 `Makefile`，支持本地构建、测试、漏洞检查、macOS/Linux 交叉编译、release tar 包和 SHA-256 checksum。
- 新增 GitHub Actions CI，执行 Go 测试、`govulncheck` 和交叉编译。
- 新增 README、用户指南和 FAQ，覆盖安装、配置、下单、场景脚本、脱敏策略、TLS 配置风险和排障。
- 更新项目进度文档，标记任务 10 完成。

## 文件变更

- 新增 `.github/workflows/ci.yml`。
- 新增 `.gitignore`。
- 新增 `Makefile`。
- 新增 `README.md`。
- 新增 `internal/version/version.go`。
- 修改 `go.mod`。
- 修改 `internal/cli/root.go`。
- 修改 `internal/cli/root_test.go`。
- 新增 `docs/project/fix-tool/user-guide.md`。
- 新增 `docs/project/fix-tool/faq.md`。
- 修改 `docs/project/fix-tool/PROGRESS.md`。
- 新增本交付文档。

## 配置与依赖变更

- 未新增 Go 运行时依赖。
- Go 工具链要求从 `1.25.0` 调整为 `1.25.10`，用于避开 govulncheck 报告的标准库漏洞。
- CI 和 Makefile 使用 `golang.org/x/vuln/cmd/govulncheck@latest` 做漏洞检查。
- release 包默认包含二进制、README、默认配置、用户指南、FAQ、样例场景和 custom tag 样例。

## 测试结果

- `go test ./... -count=1` 通过。
- `make vuln` 在 `go1.25.10` 下通过，未发现当前代码可达漏洞。
- `make build VERSION=v0.1.0-test` 通过。
- `./dist/bin/fix-tool version` 输出 `version`、`commit`、`build_time`。
- `make release VERSION=v0.1.0-test` 通过，生成 macOS/Linux amd64/arm64 tar 包。
- `cd dist/release && shasum -a 256 -c checksums.txt` 通过。
- 抽查 `fix-tool_v0.1.0-test_linux_amd64.tar.gz`，确认包含二进制、README、默认配置、用户指南、FAQ、样例场景和 custom tag 样例。

## 风险与后续建议

- 当前 Makefile 默认只生成 macOS/Linux 的 amd64 和 arm64 包，Windows 仍在待确认事项中。
- release 包可在本机验证构建产物和 checksum，但真实 macOS/Linux 运行兼容性仍建议在对应平台各执行一次冒烟测试。
- CI 依赖 GitHub Actions 网络拉取 `govulncheck`，如网络受限会导致漏洞检查步骤失败。
