# 任务交付：内嵌文档 docs 命令

## 任务背景

安装脚本实际只安装 `fix-tool` 二进制，用户安装后无法直接访问 release 包中的 `README.md`、用户指南和 FAQ。为提升离线可发现性，新增内嵌文档命令。

## 实现方案

- 将 `docs/project/fix-tool/user-guide.md` 和 `docs/project/fix-tool/faq.md` 通过 `go:embed` 打进二进制。
- 新增 `fix-tool docs` 展示可用文档主题。
- 新增 `fix-tool docs user-guide` 和 `fix-tool docs faq` 输出对应 Markdown 原文。
- 安装脚本完成后提示 `fix-tool docs` 和配置样例生成命令。

## 文件变更

- 新增 `docs_embed.go`。
- 新增 `internal/cli/docs.go`。
- 更新 `internal/cli/root.go` 注册 docs 命令。
- 更新 `internal/cli/root_test.go` 覆盖 docs 命令。
- 更新 `install.sh` 安装后提示。
- 更新 `README.md` 和 `docs/project/fix-tool/user-guide.md` 说明内嵌文档入口。
- 重排 `docs/project/fix-tool/user-guide.md`，增加三分钟快速开始、安装方式差异、命令速查，并将开发构建内容收敛到附录。

## 配置与依赖变更

- 未新增配置项。
- 未新增 Go 依赖。

## 测试结果

- `go test ./... -count=1`：通过。
- `git diff --check`：通过。
- `go run ./cmd/fix-tool docs`：通过。
- `go run ./cmd/fix-tool docs faq`：通过。
- `go run ./cmd/fix-tool docs user-guide`：通过。

## 风险与后续建议

- 当前输出 Markdown 原文，不做分页和渲染；终端较小时需要用户自行滚动或管道到 pager。
- 后续如需要导出文件，可在 `docs` 命令上增加 `--output`。
