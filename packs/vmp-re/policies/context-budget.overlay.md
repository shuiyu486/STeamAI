# VMP capture and trace context budget overlay

Extends: `common/policies/context-budget.md`

## VMP RE 读取预算

| 层级 | 文件 | 预算 | 规则 |
|---|---|---:|---|
| 常驻 | `CLAUDE.local.md` | `< 8KB` | 只放授权、反调试、当前状态、路由表。 |
| 接手 | `references/vmp-re/task-handoff.md` | `< 12KB` | 当前任务进度、下一步、必要命令。 |
| 专项 | `references/vmp-re/*.md` | `< 16KB/文件` | 只在相关任务读取。 |
| 机器数据 | `captures/*.csv/json` | 不进常驻 | 按 handler/行范围过滤读取。 |
| 归档 | `captures/doc_archive/**` | 不主动读取 | 只在人类要求追溯历史时读取片段。 |

## 读取策略

- 新会话接手：先读 `CLAUDE.local.md`，再读 `references/vmp-re/task-handoff.md`。
- 查当前 coverage：用 `routine_ir.summary.csv` 或脚本统计，不整读 `routine_ir.events.csv`。
- 查某个 handler：按 RVA 过滤 CSV/JSON，不读全部 round Markdown。
- 查工具链选择：读 `toolchain-router.md`，不要重新搜索所有公开工具。
- 查历史原因：优先读当前 reference；只有必要时读取归档片段。

## 禁止模式

- 不把完整 CSV、完整 disasm、完整 decompile、完整 trace 粘进 Markdown。
- 不整段读取 `captures/doc_archive/**`。
- 不在 `CLAUDE.local.md` 追加 round-by-round 长历史。
- 不把 tool 大输出原样带回主会话；用脚本统计后只返回摘要。
