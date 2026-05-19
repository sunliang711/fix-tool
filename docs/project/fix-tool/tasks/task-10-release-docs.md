# 任务 10：打包发布与使用文档

## 任务目标

完成二进制打包、版本信息、安装说明、使用手册和常见问题文档。

## 技术方案

- 使用 Go build 生成 macOS/Linux 二进制。
- 编译期注入 version、commit、build time。
- 文档覆盖安装、配置、常用命令、场景脚本、脱敏策略、TLS 配置风险。
- CI 中执行测试、漏洞检查和交叉编译。

## 验收标准

- 可生成 `fix-tool` 二进制。
- `fix-tool version` 输出版本、commit、构建时间。
- README 或 docs 中包含从配置到下单的完整示例。
- release 包包含 checksum。

## 实现步骤

1. 增加版本信息包。
2. 增加 build 脚本或 Makefile。
3. 增加 CI 配置。
4. 编写用户文档。
5. 编写常见问题和排障指南。
6. 验证 release 包。

## 前置依赖

- 任务 01 至任务 09。

## 风险

- 不同平台终端显示差异可能影响 table 输出。发布前需要在 macOS 和 Linux 各验证一次。

