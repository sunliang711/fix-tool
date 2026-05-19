# 任务 08：scenario runner 与断言

## 任务目标

支持通过场景文件批量执行 FIX 联调步骤，并对响应消息做断言。

## 技术方案

- 使用 YAML 或 TOML 定义 scenario。
- 每个 step 包含 action、input、wait、assert。
- 复用 admin/order/raw service 执行步骤。
- 断言支持字段等于、不等于、存在、不存在、枚举集合。
- 输出整体执行结果和每步 trace。

## 验收标准

- `fix-tool run scenario.yaml` 可执行多步骤场景。
- 支持登录、新单、等待 ExecutionReport、撤单、登出。
- 断言失败时输出失败步骤、字段、期望值和实际值。
- 支持 JSON 导出结果。

## 实现步骤

1. 定义 scenario 配置模型。
2. 实现场景文件加载和校验。
3. 实现 step executor。
4. 实现 assertion engine。
5. 接入 trace 输出。
6. 添加样例场景和单元测试。

## 前置依赖

- 任务 04。
- 任务 05。
- 任务 07。

## 风险

- 场景语言过度设计会增加维护成本。第一期只支持顺序执行和基础断言。

