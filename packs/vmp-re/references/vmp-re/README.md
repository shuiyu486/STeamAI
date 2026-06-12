# VMP RE 按需路由入口

> 目标：让 Claude Code 新会话只加载当前需要的信息，避免把历史长文档、CSV 大表或 tool output 灌入上下文。

## 常驻原则

- 先读项目内 `CLAUDE.local.md`，再按本文件路由。
- 不要整段读取 `captures/doc_archive/**`、大型 CSV、长 Markdown 结果；只读必要行范围或用脚本/CSV 统计。
- 当前项目状态以 `task-handoff.md` 为接手入口；权威数据以项目内 `captures/*.csv` 为准。
- 每完成一轮语义降低，都要更新 `task-handoff.md`，必要时把可复用经验沉淀到模板文档。

## 路由表

| 任务 | 读取文档 | 说明 |
|---|---|---|
| 新会话接手当前项目进度 | `task-handoff.md` | 当前 coverage、待处理 handler、下一步 checklist。 |
| 复用本流程到新的 VMProtect x64 样本 | `workflow-template.md` | 从样本基准、VMEnter、context probe 到 value-flow 的模板。 |
| 选择/复查已用、可用、应止损的工具链 | `toolchain-router.md` | 工具状态表、按任务路由、公开工具止损条件。 |
| 控制上下文膨胀、处理 `/compact` / unexpected EOF | `progressive-disclosure.md` | 文档层级、读取预算、归档与 Stop hook 收尾规则。 |
| 继续 singleton handler 复核 | `singleton-handler-review.md` | 1-count handler 的 focused instruction review 流程。 |
| 查看权威 opcode/role 数据 | `captures/vm_opcode_semantics_confirmed.csv`、`captures/vm_handler_roles_confirmed.csv` | 不复制进 Markdown；按 handler 过滤读取。 |
| 查看覆盖率与 top unknown | `captures/routine_ir.summary.csv`、`captures/routine_ir.events.csv` | 用脚本/CSV 统计，不整读大文件。 |

## 自动维护规则

每轮推进任务后执行以下最小维护：

1. 若修改 confirmed CSV，重建 `routine_ir.*` 与 `routine_ir.superinstructions.*`。
2. 更新 `task-handoff.md`：coverage、已合入项、剩余 top unknown、下一步。
3. 若发现可复用方法/坑点，更新对应模板：
   - 通用流程 → `workflow-template.md`
   - 工具适配/止损经验 → `toolchain-router.md`
   - 上下文/文档策略 → `progressive-disclosure.md`
   - singleton handler 复核经验 → `singleton-handler-review.md`
4. 不把大表粘进 Markdown；保留到 CSV/JSON/归档。
5. Stop hook 前验证文档存在、大小合理、关键脚本语法通过。
