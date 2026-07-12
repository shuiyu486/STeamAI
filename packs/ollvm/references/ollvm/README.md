# OLLVM pack

`ollvm` 是面向授权 OLLVM / native obfuscation analysis、control-flow flattening triage、opaque predicate review、MBA simplification 与 deobfuscation candidate review 的最小 Agent Team pack 骨架。它用于验证 `re-context-kits` 在 native 混淆分析子领域的 pack-neutral 边界：工作线、packet、ledger、review gate、handoff、sync/promote 和 tooling adapter 契约应保持可审计、可交接、review-first。

当前不是自动反混淆引擎、patch 平台、符号恢复器、样本执行器或函数级大规模改写系统；它只提供 case workspace 组织规则与工具经验入口。

## 路由表

| 任务 | 读取文档 | 说明 |
|---|---|---|
| 接手本 pack | `workflow-template.md` | scope、轻到重路线、candidate/review/confirmed 流程。 |
| 规划多 Agent 分片 | `agent-team.md` | subagent routes、packet 输出契约、review-first 合并边界。 |
| 选择工具或 adapter | `toolchain-router.md` | CFG triage、MBA simplification review、dynamic/gated action 边界。 |
| 查看通用规则 | `<templateRoot>/common/policies/agent-team.md` | 角色、packet、状态流和人工确认边界。 |
| 查看工具 adapter 规则 | `<templateRoot>/common/policies/tool-adapters.md` | capability card、sidecar 输出、heavy-tool gate。 |

## 常驻边界

- 不把样本、反混淆后二进制、patch、trace、dump、完整 CFG、函数体 raw dump、符号表、IOC、hash、客户上下文或绝对路径写入 pack。
- 动态执行、调试、trace、dump、patch、批量反编译、自动重命名、自动写注释、自动导出反混淆二进制或外部联网必须在授权范围内，并记录隔离、预算、止损和确认。
- 子 agent 输出 candidate / verification；main agent 负责 ledger、handoff、review merge 和 authority 确认。
- 大输出必须保存为 case-local sidecar，只在聊天和 Markdown 中引用摘要与路径。

## 维护规则

- 新工具先进入 `tooling/catalog.yml` 的 `candidate`、`auxiliary` 或 `cautious` 状态。
- 至少两个授权 case 重复出现的通用 native obfuscation 工作流规则，才考虑提升到 common policy 或 runtime。
- 本 pack 的目的首先是验证多安全领域 pack 架构，不要复制 `vmp-re` 的领域细节或把具体样本、hash、function body、full CFG、patch 和 deobfuscation result 写入模板。
