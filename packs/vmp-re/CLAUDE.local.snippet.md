<!-- BEGIN vmp-re-template:router v0.2.0 -->
## VMP RE 按需路由 / 渐进式披露

常驻只读本文件。遇到具体任务时，再读取 `references/vmp-re/README.md` 中对应文档。

| 任务 | 读取 |
|---|---|
| 新会话接手当前进度 | `references/vmp-re/task-handoff.md` |
| 复用到新的 VMProtect x64 样本 | `references/vmp-re/workflow-template.md` |
| 选择/复查工具链与止损条件 | `references/vmp-re/toolchain-router.md` |
| 控制上下文膨胀、避免 `/compact` / EOF 问题 | `references/vmp-re/progressive-disclosure.md` |
| 继续 singleton handler 复核 | `references/vmp-re/singleton-handler-review.md` |

规则：不要整段读取归档长文档；优先读 CSV、summary、当前任务文档和必要行范围。每轮推进后同步 `task-handoff.md`；发现可复用经验时通过 `/rekit promote` 回流模板。
<!-- END vmp-re-template:router -->
