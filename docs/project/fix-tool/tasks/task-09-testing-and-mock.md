# 任务 09：测试、mock acceptor、样例配置

## 任务目标

补齐单元测试、集成测试、mock acceptor 和样例配置，提升工具可验证性。

## 技术方案

- 单元测试覆盖 config、dictionary、message、render、trace、scenario。
- 集成测试使用 mock acceptor 模拟 Logon、Heartbeat、ExecutionReport、Reject。
- `testdata` 提供字典、raw 报文和场景样例。
- 示例配置不包含真实账号和密码。

## 验收标准

- `go test ./...` 通过。
- 关键模块有表驱动测试。
- mock acceptor 可支持基础交易链路测试。
- 样例配置可通过 `config validate`。

## 实现步骤

1. 梳理测试覆盖矩阵。
2. 为核心纯函数补充单元测试。
3. 实现 mock acceptor。
4. 增加集成测试。
5. 增加样例配置和样例场景。
6. 编写测试说明。

## 前置依赖

- 任务 01 至任务 08。

## 风险

- mock acceptor 行为不能代表真实网关。文档需要明确它只用于基础链路验证。

