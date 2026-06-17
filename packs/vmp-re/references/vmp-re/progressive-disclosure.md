# 渐进式披露与上下文预算

> 目的：避免刚 `/compact` 后又因为大 Markdown、tool output 或归档历史被读入而触发上下文膨胀、`unexpected EOF` 或无效消耗。

## Policy registry 路由

横切规范不在本文件复制全文，按需读取：

| 需求 | 通用 policy | VMP overlay / route |
|---|---|---|
| 上下文预算 / 渐进式披露 | `<templateRoot>/common/policies/context-budget.md` | `<templateRoot>/packs/vmp-re/policies/context-budget.overlay.md` |
| 子 agent / bounded parallelism | `<templateRoot>/common/policies/subagents.md` | `<templateRoot>/packs/vmp-re/manifest.yml:subagentRoutes` 与 `<templateRoot>/packs/vmp-re/policies/subagents.overlay.md` |
| review-first / 回流确认 | `<templateRoot>/common/policies/review-first.md` | `<templateRoot>/packs/vmp-re/policies/review-first.overlay.md` |
| 写入边界 | `<templateRoot>/common/policies/write-boundaries.md` | `<templateRoot>/packs/vmp-re/policies/review-first.overlay.md` |
| 验证标准 | `<templateRoot>/common/policies/verification.md` | `<templateRoot>/packs/vmp-re/policies/verification.overlay.md` |
| 大工具输出 | `<templateRoot>/common/policies/tool-output.md` | `<templateRoot>/packs/vmp-re/policies/context-budget.overlay.md` |
| 接手与连续性 | `<templateRoot>/common/policies/handoff.md` | `task-handoff.md` |
| 并行功能会话 | `<templateRoot>/common/policies/parallel-sessions.md` | `parallel-sessions.md` 与 `<templateRoot>/packs/vmp-re/policies/parallel-sessions.overlay.md` |

## VMP RE 快速规则

- 新会话接手：先读 `CLAUDE.local.md`，再读 `references/vmp-re/task-handoff.md`。
- 查当前 coverage：用 `routine_ir.summary.csv` 或脚本统计，不整读 `routine_ir.events.csv`。
- 查某个 handler：按 RVA 过滤 CSV/JSON，不读全部 round Markdown。
- 批量 handler / trace / value-flow 复核：按 manifest route `vmp-re:bounded-review` 分片给子 agent；细则见 `<templateRoot>/packs/vmp-re/policies/subagents.overlay.md`。
- 可复用经验回流：走 `/rekit promote` 的 review-first 流程；日常不手动执行底层 runtime。

## Delegation gate

开始批量只读分析前，先判断是否触发 `vmp-re:bounded-review`：

- 候选 handler / trace / value-flow / tooling diff 项目数量 `>= 4`。
- 需要 instruction-level review，或会读取长 `*_instr.json`、`*_memory.json`、`*value_flow*.csv`。
- 出现 `LOW_OCCURRENCE`、`POINTER_ALIAS`、`SOURCE_POINTER_ALIAS`、`NO_TEMPLATE_MATCH` 等 blocker。
- 主 agent 上下文已有明显压力，只需要短结论和少量证据。

触发后：

1. 用 `/rekit plan-subagents` 或等价手工计划生成固定分片。
2. 默认先走 L1 packet review；只有冲突项才升级到 L2 sidecar review 或 L3 deep tool review。
3. 子 agent 只读复核分片，不写文件，不粘贴长 trace/disasm。
4. 普通 batch 子 agent 不自行打开 IDA/调试器、不全量分析 rebuilt PE；需要重型工具时返回 `needs_l3`，由主 agent 显式窄范围后台升级。
5. 主 agent 只接收结构化短结论，并独占 CSV backup/write、验证和 handoff 更新。
6. 不能给出固定分片时，先脚本化聚合或缩小输入；不要启动无界子 agent。

## 禁止模式

- 不把完整 CSV、完整 disasm、完整 decompile、完整 trace 粘进 Markdown。
- 不整段读取 `captures/doc_archive/**`。
- 不在 `CLAUDE.local.md` 追加 round-by-round 长历史。
- 不把 tool 大输出原样带回主会话；用脚本统计后只返回摘要。
- 不启动无界子 agent 去“自行探索整批大文件”；必须给定明确分片和输出契约。
- 不把底层 runtime 或旧式 dry run / direct apply 作为常规回流路径。

## Stop hook 收尾规则

每次修改项目文件后，结束前至少做：

1. 若代码变更：运行相关 `py_compile` / rebuild / smoke test。
2. 若 confirmed CSV 变更：按 `<templateRoot>/packs/vmp-re/policies/verification.overlay.md` 重建 `routine_ir.*` 与 superinstruction 产物。
3. 若任务进度变化：更新 `task-handoff.md`。
4. 若产生新可复用经验：先按 `<templateRoot>/packs/vmp-re/policies/review-first.overlay.md` 分类；需要回流模板时运行 `/rekit promote` 的 review-first 流程。
5. 运行 `/rekit doctor`，验证 Markdown 大小不超预算。

## 自动维护触发点

| 事件 | 更新 |
|---|---|
| 完成一批 handler 复核 | `task-handoff.md` 的 coverage、已合入、待复核列表。 |
| 发现新的复核规则 | `singleton-handler-review.md` 或 `<templateRoot>/packs/vmp-re/policies/subagents.overlay.md`。 |
| 发现可复用项目流程 | `workflow-template.md`。 |
| 新增/重评工具或止损条件 | `toolchain-router.md` 或 `<templateRoot>/packs/vmp-re/tooling/recipes/*`。 |
| 文档再次变大 | 归档长内容，保留短摘要。 |
| 需要将 case 经验回流模板 | `/rekit promote` review-first；用户确认具体动作后才生成候选或写回。 |

## 新会话恢复提示

```text
读取项目 CLAUDE.local.md，然后按 references/vmp-re/README.md 路由。当前任务从 references/vmp-re/task-handoff.md 接手；不要读取 captures/doc_archive 或大 CSV，除非需要按 handler 过滤。
```
