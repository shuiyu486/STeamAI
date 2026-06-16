# 渐进式披露与上下文预算

> 目的：避免刚 `/compact` 后又因为大 Markdown、tool output 或归档历史被读入而触发上下文膨胀、`unexpected EOF` 或无效消耗。

## Policy registry 路由

横切规范不在本文件复制全文，按需读取：

| 需求 | 通用 policy | VMP overlay |
|---|---|---|
| 上下文预算 / 渐进式披露 | `<templateRoot>/common/policies/context-budget.md` | `../../policies/context-budget.overlay.md` |
| 子 agent / bounded parallelism | `<templateRoot>/common/policies/subagents.md` | `../../policies/subagents.overlay.md` |
| review-first / 回流确认 | `<templateRoot>/common/policies/review-first.md` | `../../policies/review-first.overlay.md` |
| 写入边界 | `<templateRoot>/common/policies/write-boundaries.md` | `../../policies/review-first.overlay.md` |
| 验证标准 | `<templateRoot>/common/policies/verification.md` | `../../policies/verification.overlay.md` |
| 大工具输出 | `<templateRoot>/common/policies/tool-output.md` | `../../policies/context-budget.overlay.md` |
| 接手与连续性 | `<templateRoot>/common/policies/handoff.md` | `task-handoff.md` |

## VMP RE 快速规则

- 新会话接手：先读 `CLAUDE.local.md`，再读 `references/vmp-re/task-handoff.md`。
- 查当前 coverage：用 `routine_ir.summary.csv` 或脚本统计，不整读 `routine_ir.events.csv`。
- 查某个 handler：按 RVA 过滤 CSV/JSON，不读全部 round Markdown。
- 批量 handler / trace / value-flow 复核：按 `../../policies/subagents.overlay.md` 分片给子 agent。
- 可复用经验回流：走 `/rekit promote` 的 review-first 流程；backend 排障时使用 `rekit.ps1 promote -Review`。

## 禁止模式

- 不把完整 CSV、完整 disasm、完整 decompile、完整 trace 粘进 Markdown。
- 不整段读取 `captures/doc_archive/**`。
- 不在 `CLAUDE.local.md` 追加 round-by-round 长历史。
- 不把 tool 大输出原样带回主会话；用脚本统计后只返回摘要。
- 不启动无界子 agent 去“自行探索整批大文件”；必须给定明确分片和输出契约。
- 不把裸 `rekit.ps1 promote` 或 `-WhatIf -> -Apply` 作为常规回流路径。

## Stop hook 收尾规则

每次修改项目文件后，结束前至少做：

1. 若代码变更：运行相关 `py_compile` / rebuild / smoke test。
2. 若 confirmed CSV 变更：按 `../../policies/verification.overlay.md` 重建 `routine_ir.*` 与 superinstruction 产物。
3. 若任务进度变化：更新 `task-handoff.md`。
4. 若产生新可复用经验：先按 `../../policies/review-first.overlay.md` 分类；需要回流模板时运行 `/rekit promote` 的 review-first 流程。
5. 运行 `/rekit doctor`，验证 Markdown 大小不超预算。

## 自动维护触发点

| 事件 | 更新 |
|---|---|
| 完成一批 handler 复核 | `task-handoff.md` 的 coverage、已合入、待复核列表。 |
| 发现新的复核规则 | `singleton-handler-review.md` 或 `../../policies/subagents.overlay.md`。 |
| 发现可复用项目流程 | `workflow-template.md`。 |
| 新增/重评工具或止损条件 | `toolchain-router.md` 或 `../../tooling/recipes/*`。 |
| 文档再次变大 | 归档长内容，保留短摘要。 |
| 需要将 case 经验回流模板 | `/rekit promote` review-first；用户确认具体动作后才生成候选或写回。 |

## 新会话恢复提示

```text
读取项目 CLAUDE.local.md，然后按 references/vmp-re/README.md 路由。当前任务从 references/vmp-re/task-handoff.md 接手；不要读取 captures/doc_archive 或大 CSV，除非需要按 handler 过滤。
```
