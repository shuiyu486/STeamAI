# Unpack PE pack

`unpack-pe` 是面向授权 PE unpacking、loader triage、import recovery 与 unpack candidate review 的最小 Agent Team pack 骨架。它用于验证 `re-context-kits` 在二进制安全研究子领域的 pack-neutral 边界：工作线、packet、ledger、review gate、handoff、sync/promote 和 tooling adapter 契约应保持可审计、可交接、review-first。

当前不是自动脱壳器、样本执行器、动态调试平台、patch/dump 自动化引擎或 IOC/样本库；它只提供 case workspace 组织规则与工具经验入口。

## 路由表

| 任务 | 读取文档 | 说明 |
|---|---|---|
| 接手本 pack | `workflow-template.md` | scope、轻到重路线、candidate/review/confirmed 流程。 |
| 规划多 Agent 分片 | `agent-team.md` | subagent routes、packet 输出契约、review-first 合并边界。 |
| 选择工具或 adapter | `toolchain-router.md` | PE static triage、loader/unpack review、dynamic/gated action 边界。 |
| 查看通用规则 | `<templateRoot>/common/policies/agent-team.md` | 角色、packet、状态流和人工确认边界。 |
| 查看工具 adapter 规则 | `<templateRoot>/common/policies/tool-adapters.md` | capability card、sidecar 输出、heavy-tool gate。 |

## 常驻边界

- 不把样本、unpacked binary、dump、trace、memory snapshot、patch、完整 import table、section bytes、IOC、hash、客户上下文或绝对路径写入 pack。
- 动态调试、样本执行、dump、patch、解密/解压 payload、外部联网、自动修复 import 或写 unpacked 文件必须在授权范围内，并记录隔离、预算、止损和确认。
- 子 agent 输出 candidate / verification；main agent 负责 ledger、handoff、review merge 和 authority 确认。
- 大输出必须保存为 case-local sidecar，只在聊天和 Markdown 中引用摘要与路径。

## 维护规则

- 新工具先进入 `tooling/catalog.yml` 的 `candidate`、`auxiliary` 或 `cautious` 状态。
- 至少两个授权 case 重复出现的通用 PE unpacking 工作流规则，才考虑提升到 common policy 或 runtime。
- 本 pack 的目的首先是验证多安全领域 pack 架构，不要复制 `vmp-re` 的领域细节或把具体样本、hash、dump、trace、patch 和 unpack result 写入模板。
