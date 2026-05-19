# 任务 02：QuickFIX/Go session adapter 与 profile 加载

## 任务目标

封装 QuickFIX/Go initiator，支持按 profile 创建、启动、停止 FIX session，并处理基础会话事件。

## 技术方案

- 在 `internal/fixsession` 定义 `Manager`、`Session`、`Application`。
- 将强类型 profile 转换为 QuickFIX/Go settings。
- 支持 BeginString、SenderCompID、TargetCompID、SocketConnectHost、SocketConnectPort、HeartBtInt、DataDictionary、TLS 相关配置。
- 通过 channel 或 callback 输出收发消息事件。
- 生命周期由 Fx 管理。

## 验收标准

- 可根据 profile 创建 QuickFIX/Go initiator。
- session 可启动和停止。
- Logon、Logout、ToAdmin、FromAdmin、ToApp、FromApp 事件可被捕获。
- TLS 默认校验开启；配置关闭时输出英文风险提示。
- 单元测试覆盖 profile 到 settings 的映射。

## 实现步骤

1. 定义 session manager 接口。
2. 实现 QuickFIX/Go application 回调。
3. 实现 profile 到 QuickFIX settings 的转换。
4. 接入 TLS 配置。
5. 接入 trace 事件出口。
6. 添加单元测试。

## 前置依赖

- 任务 01。

## 风险

- QuickFIX/Go settings 字段较多，必须优先覆盖 P0，避免一次性暴露过多配置。

