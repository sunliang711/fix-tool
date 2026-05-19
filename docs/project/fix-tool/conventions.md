# fix-tool 编码规范

## 1. 规范来源

本文档基于以下规则裁剪生成：

- `/Users/eagle/.codex/ai-rules-skills/rules/00-global.md`
- `/Users/eagle/.codex/ai-rules-skills/rules/04-go-backend.md`
- `/Users/eagle/.codex/ai-rules-skills/rules/05-go-security.md`
- `/Users/eagle/.codex/ai-rules-skills/rules/06-go-api-design.md`

本项目是 CLI 工具，不提供 HTTP API，不使用数据库。因此 Gin、GORM、REST API 路由和数据库连接池相关规则不适用。

## 2. 语言与沟通

- 文档、说明、计划和代码注释使用简体中文。
- 日志消息统一使用英文。
- 技术术语可保留英文原文，例如 FIX、MsgType、Profile、Session。

## 3. 项目结构

```text
cmd/fix-tool/
config/
internal/app/
internal/cli/
internal/config/
internal/dictionary/
internal/fixsession/
internal/message/
internal/render/
internal/scenario/
internal/trace/
internal/validate/
pkg/fixfield/
testdata/
```

- `cmd/fix-tool/main.go` 只负责应用启动。
- CLI 层只做参数绑定、校验入口和响应展示，不写业务逻辑。
- Service 或应用层负责业务编排。
- Adapter 层封装 QuickFIX/Go、文件导出、终端渲染等外部依赖。
- Domain 模型与配置模型分离。

## 4. 依赖规范

- FIX 引擎使用 `github.com/quickfixgo/quickfix`。
- CLI 使用 `github.com/spf13/cobra`。
- 配置使用 `github.com/spf13/viper`，但业务代码禁止散落调用 `viper.Get*()`。
- 日志使用 `github.com/rs/zerolog`。
- 依赖注入使用 `go.uber.org/fx`。
- 新增同类替代依赖前必须先说明理由并确认。

## 5. 配置规范

- 配置加载统一放在 `internal/config`。
- 加载顺序：内嵌 `config/default.toml` < `config.toml` < `private.toml` < 环境变量 < CLI flags。
- 配置必须反序列化到强类型结构后再注入业务模块。
- 新增、修改或删除配置字段时，必须同步更新 `config/default.toml`、`config-example.toml`、`fix-tool config example` 子命令输出、用户文档、配置规范、配置加载逻辑、校验逻辑和对应测试。
- 密码、Token、证书私钥不得写入提交到 Git 的示例配置。
- TLS 默认启用证书校验；允许通过 profile 显式修改，但必须输出英文风险提示。

## 6. 日志规范

- 日志通过 Fx 注入，禁止散落全局 logger。
- 使用 Zerolog 结构化字段，禁止字符串拼接。
- 错误日志必须携带 `err` 字段。
- 生产默认 INFO 级别。
- 禁止打印密码、Token、私钥、完整认证报文、完整签名数据。
- 调试级 raw 报文输出也必须走统一脱敏器。

## 7. 命名规范

- 包名全小写、简短、无下划线。
- 类型命名使用 `XxxService`、`XxxCommand`、`XxxConfig`、`XxxReq`、`XxxResp`。
- ID 使用全大写：`ClOrdID`、`OrderID`、`TraceID`。
- 导出常量使用 PascalCase，未导出常量使用 camelCase。
- 禁止魔法数字；FIX tag、默认超时、默认心跳间隔必须定义常量。

## 8. 错误处理

- 遵循 Go 惯例，`if err != nil` 立即处理。
- 禁止忽略 error 返回值，除非有明确注释说明原因。
- 错误包装使用 `fmt.Errorf("xxx: %w", err)`。
- 对用户展示的错误信息要避免泄露内部路径、认证信息和底层实现细节。
- 业务错误应定义可判断的错误值，便于命令层映射退出码。

## 9. 并发与生命周期

- goroutine 必须绑定 `context.Context` 或明确退出信号。
- FIX session、trace buffer、response matcher 等共享状态必须使用锁或 channel 保护。
- ticker、timer、连接、文件句柄必须及时释放。
- Fx 生命周期中统一启动和停止 session 管理器。

## 10. 安全规范

- 禁止在代码、示例配置和文档中写入真实账号、密码、Token、API Key。
- 敏感字段统一通过脱敏器处理。
- 自定义 tag 支持 `sensitive = true` 标记。
- 默认不关闭 TLS 主机名和证书校验。
- 如果测试环境必须关闭 TLS 校验，配置项名称必须明确表达风险，例如 `insecure_skip_verify`。
- 文件路径输入需要校验，避免路径穿越和误读敏感文件。

## 11. 测试规范

- 测试文件与被测文件同目录。
- 优先使用表驱动测试。
- 消息构造、报文解析、字段渲染、脱敏、断言逻辑必须覆盖单元测试。
- FIX session 相关逻辑通过接口隔离，单元测试中使用 fake session。
- 集成测试使用 mock acceptor 或 testdata 中的样例报文。

## 12. 项目特有约定

- SOH 字符在终端展示中默认渲染为 `|`，但 raw 发送时必须使用真实 SOH。
- 所有请求响应记录统一称为 `trace`。
- 所有连接配置统一称为 `profile`。
- 自定义字段元数据统一称为 `custom field definition`，配置键使用 `profile.custom_field_defs`。
- 需要随 Logon 发送的自定义 tag 统一放在 `profile.logon_tags`。
- P0 MsgType 模板必须优先保持参数稳定，后续新增字段通过 `--tag key=value` 补充。
