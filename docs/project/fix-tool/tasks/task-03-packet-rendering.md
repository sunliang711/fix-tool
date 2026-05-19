# 任务 03：报文捕获、解析、字段名映射、脱敏渲染

## 任务目标

实现 FIX 报文 trace 记录、raw 解析、字段名映射、BodyLength/CheckSum 校验和多格式输出。

## 技术方案

- 在 `internal/trace` 定义 `MessageTrace` 和 recorder。
- 在 `internal/dictionary` 加载标准字典和 custom field definition。
- 在 `internal/render` 实现 table、raw、json 输出。
- 默认将 SOH 展示为 `|`。
- 实现统一脱敏器，支持内置敏感 tag 和 custom field definition。

## 验收标准

- 能展示 raw FIX、tag、字段名、值、枚举含义。
- 能校验 BodyLength 和 CheckSum。
- 敏感字段默认脱敏。
- 输出格式支持 table 和 json。
- 单元测试覆盖解析、校验、脱敏和渲染。

## 实现步骤

1. 定义 trace 模型。
2. 实现 raw FIX parser。
3. 实现 BodyLength 和 CheckSum 校验。
4. 实现字典字段名映射。
5. 实现脱敏规则。
6. 实现 table/json renderer。
7. 添加 testdata 和单元测试。

## 前置依赖

- 任务 01。
- 任务 02。

## 风险

- 自定义字典和标准字典冲突时需要明确优先级：custom field definition 优先覆盖展示信息，但不修改标准 tag 的协议语义。
