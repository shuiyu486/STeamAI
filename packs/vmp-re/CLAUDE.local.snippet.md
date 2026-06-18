<!-- BEGIN vmp-re-template:router v0.2.0 -->
## VMP RE 按需路由 / 渐进式披露

常驻只读本文件。遇到具体任务时，再读取 `references/vmp-re/README.md` 中对应文档。

| 任务 | 读取 |
|---|---|
| 新会话接手当前进度 | `references/vmp-re/task-handoff.md` |
| 复用到新的 VMProtect x64 样本 | `references/vmp-re/workflow-template.md` |
| 选择/复查工具链与止损条件 | `references/vmp-re/toolchain-router.md` |
| 控制上下文膨胀、子 agent 并行复核、避免 `/compact` / EOF 问题 | `references/vmp-re/progressive-disclosure.md` |
| 批量只读复核 / 子 agent 分片规划 | `references/vmp-re/progressive-disclosure.md` |
| 继续 singleton handler 复核 | `references/vmp-re/singleton-handler-review.md` |

规则：不要整段读取归档长文档；优先读 CSV、summary、当前任务文档和必要行范围。批量 handler/trace/value-flow 复核先过 delegation gate，由主 agent 或自动流程生成固定分片计划（必要时内部使用 `plan-subagents`）或固定分片子 agent。每轮推进后同步 `task-handoff.md`；发现可复用经验时通过 `/rekit promote` 的 review-first 流程回流模板，用户确认具体动作前不要写候选或 pack。
<!-- END vmp-re-template:router -->
