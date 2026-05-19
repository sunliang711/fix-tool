# 任务 07：raw send、inspect raw、自定义 tag overlay

## 任务目标

实现 raw message 发送、离线 raw 解析和自定义 tag overlay，满足非标准字段和临时联调需求。

## 技术方案

- `raw send` 支持 `--msg-type` 和多个 `--tag key=value`。
- `inspect raw` 支持解析用户输入的 raw FIX 字符串。
- 自定义 tag 元信息从 profile 或 dictionary 配置加载。
- 发送前统一进行 BodyLength、CheckSum、SOH 处理。

## 验收标准

- 可通过 `--tag` 补充任意自定义字段。
- `inspect raw` 能解析历史报文并输出字段说明。
- 自定义 tag 可展示字段名、类型、枚举和敏感标记。
- raw 发送时不会把展示用 `|` 当成真实 SOH。

## 实现步骤

1. 定义 custom tag 配置结构。
2. 实现 raw message builder。
3. 实现 `raw send` 命令。
4. 实现 `inspect raw` 命令。
5. 接入字段解释和脱敏。
6. 添加单元测试。

## 前置依赖

- 任务 03。
- 任务 05。

## 风险

- raw send 灵活度高，也更容易发出非法报文。命令需要在发送前输出校验结果和风险提示。

