# VMP RE 按需路由入口

> 目标：让 Claude Code 新会话只加载当前需要的信息，避免把历史长文档、CSV 大表或 tool output 灌入上下文。

## 常驻原则

- 先读项目内 `CLAUDE.local.md`，再按本文件路由。
- 不要整段读取 `captures/doc_archive/**`、大型 CSV、长 Markdown 结果；只读必要行范围或用脚本/CSV 统计。
- 当前项目状态以 `task-handoff.md` 为接手入口；权威数据以项目内 `captures/*.csv` 为准。
- 每完成一轮语义降低，都要更新 `task-handoff.md`；可复用经验通过 `/rekit promote` 的 review-first 流程回流模板。

## 路由表

| 任务 | 读取文档 | 说明 |
|---|---|---|
| 新会话接手当前项目进度 | `task-handoff.md` | 当前 coverage、待处理 handler、下一步 checklist。 |
| 复用本流程到新的 VMProtect x64 样本 | `workflow-template.md` | 从样本基准、VMEnter、context probe 到 value-flow 的模板。 |
| 选择/复查已用、可用、应止损的工具链 | `toolchain-router.md`、`<templateRoot>/packs/vmp-re/tooling/README.md` | 工具状态表、按任务路由、公开工具止损条件和可复用 tooling 资产。 |
| 控制上下文膨胀、处理 `/compact` / unexpected EOF | `progressive-disclosure.md` | 文档层级、读取预算、policy overlay、子 agent 并行复核、归档与 Stop hook 收尾规则。 |
| Agent Team 工作方式 | `agent-driven-re.md` | 主 agent、功能支线、reviewer、tooling agent、人类确认者的职责、packet、candidate→confirmed 流程和人工门禁。 |
| 主线与功能支线协同推进 | `lane-collaboration.md` | `/rekit overview/continue/start/handoff`、功能支线 workspace、续接提示、request/candidate、standalone 规则。 |
| 继续 singleton handler 复核 | `singleton-handler-review.md` | 1-count handler 的 focused instruction review 流程。 |
| 查看通用 policy | `<templateRoot>/common/policies/README.md` | 子 agent、review-first、写入边界、验证、tool output 等跨 pack 规范。 |
| 查看 VMP policy overlay | `<templateRoot>/packs/vmp-re/policies/README.md` | VMP handler 分片、captures 预算、经验回流和 routine IR 验证 overlay。 |
| 查看权威 opcode/role 数据 | `captures/vm_opcode_semantics_confirmed.csv`、`captures/vm_handler_roles_confirmed.csv` | 不复制进 Markdown；按 handler 过滤读取。 |
| 查看覆盖率与 top unknown | `captures/routine_ir.summary.csv`、`captures/routine_ir.events.csv` | 用脚本/CSV 统计，不整读大文件。 |

## 自动维护规则

每轮推进任务后执行以下最小维护：

1. 若修改 confirmed CSV，重建 `routine_ir.*` 与 `routine_ir.superinstructions.*`。
2. 更新 `task-handoff.md`：coverage、已合入项、剩余 top unknown、下一步。
3. 若发现可复用方法/坑点，先按 `<templateRoot>/packs/vmp-re/policies/review-first.overlay.md` 分类：common policy、pack overlay、reference doc、tooling recipe 或 case-only。
4. 不把大表粘进 Markdown；保留到 CSV/JSON/归档。
5. Stop hook 前运行 `/rekit doctor`，验证文档存在、大小合理、关键结构正常。
