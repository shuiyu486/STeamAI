# Template pack route

> 这是新 pack 的按需路由入口模板。复制为 `references/<pack>/README.md` 后再按领域改写。

## 常驻原则

- 先读 case 的 `CLAUDE.local.md`，再按本文件路由。
- 不读取大 trace、大 CSV、完整反汇编或完整反编译；只读必要行范围或 sidecar 摘要。
- 当前进度保存在 case-local handoff；pack reference 只保存可复用流程。

## 路由表

| 任务 | 读取文档 | 说明 |
|---|---|---|
| 了解本 pack 工作流 | `workflow-template.md` | 领域主流程、验证路线和升级条件。 |
| 规划 Agent Team 分片 | `agent-team.md` | 默认 subagent routes、packet 输出契约和 review-first 合并边界。 |
| 选择工具 | `toolchain-router.md` | 工具状态、适用阶段、止损条件和重型工具门禁。 |
| 查看通用 Agent Team 规则 | `<templateRoot>/common/policies/agent-team.md` | 角色、packet、状态流和人工确认边界。 |
| 查看工具 adapter 规则 | `<templateRoot>/common/policies/tool-adapters.md` | capability card、sidecar 输出和 heavy-tool gate。 |

## 维护规则

- 新增可复用流程时，优先更新本 pack reference。
- 新增工具经验时，优先更新 `tooling/catalog.yml` 或 `tooling/recipes/*`。
- 横切规则再考虑提升到 `common/policies/*`。
