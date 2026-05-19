# fix-tool 进度跟踪

## 当前阶段

- 阶段：任务 09 已完成，任务 07 尚未开始。
- 当前版本：规划 V0.1。
- 最后更新：2026-05-19。

## 已确认决策

- 使用 Go 开发单二进制 CLI 工具。
- FIX 引擎使用 QuickFIX/Go。
- CLI 框架使用 Cobra，配置使用 Viper。
- MVP MsgType 覆盖常用交易链路，不做标准 FIX 全量消息。
- TLS 默认开启证书校验，但允许通过配置显式修改。
- 文档目录使用 `docs/project/fix-tool/`。
- 子任务拆分沿用 10 个任务。

## 任务状态

| 任务 | 状态 | 说明 |
|---|---|---|
| 01 项目骨架、配置模型、日志、Fx 组装 | 已完成 | 已完成 Go module、Cobra、Viper、Zerolog、Fx、配置校验和基础测试 |
| 02 QuickFIX/Go session adapter 与 profile 加载 | 已完成 | 已完成 QuickFIX/Go initiator 封装、profile settings 映射、事件捕获、Fx 生命周期和测试 |
| 03 报文捕获、解析、字段名映射、脱敏渲染 | 已完成 | 已完成 trace、raw 解析、字段字典、BodyLength/CheckSum 校验、脱敏、table/json/raw 渲染和测试 |
| 04 admin 命令 | 已完成 | 已完成 logon、logout、heartbeat、test-request 命令、admin service、发送/等待/超时逻辑和测试 |
| 05 order 命令 | 已完成 | 已完成 order new/cancel/replace 命令、订单消息构造、响应匹配、custom tag、trace 输出和测试 |
| 06 交互式 shell | 已完成 | 已完成 shell 命令、有限命令解析、共享 session、trace list、错误不中断、退出关闭和测试 |
| 07 raw send、inspect raw、自定义 tag overlay | 未开始 | 依赖任务 03、05 |
| 08 scenario runner 与断言 | 已完成 | 已完成 YAML scenario runner、顺序执行、基础断言、JSON 结果导出、样例场景和测试；raw action 等待任务 07 接入 |
| 09 测试、mock acceptor、样例配置 | 已完成 | 已完成测试覆盖矩阵、mock acceptor、集成测试、安全样例配置、字典样例、场景样例和测试说明 |
| 10 打包发布与使用文档 | 未开始 | 依赖任务 01-09 |

## 待确认事项

- 是否需要 Windows 交付包。
- 是否需要内置某个交易所或券商的默认字典样例。
- mock acceptor 已在任务 09 提供基础链路验证，仍不代表真实网关行为。

## 下一步

根据安排补做 `task-07-raw-and-custom-tags.md`，或继续进入 `task-10-release-docs.md`。
