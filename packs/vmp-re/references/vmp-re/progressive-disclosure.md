# 渐进式披露与上下文预算

> 目的：避免刚 `/compact` 后又因为大 Markdown、tool output 或归档历史被读入而触发上下文膨胀、`unexpected EOF` 或无效消耗。

## 文档层级

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

## Stop hook 收尾规则

每次修改项目文件后，结束前至少做：

1. 若代码变更：运行相关 `py_compile` / rebuild / smoke test。
2. 若 confirmed CSV 变更：重建 `routine_ir.*` 与 superinstruction 产物。
3. 若任务进度变化：更新 `task-handoff.md`。
4. 若产生新可复用经验：更新对应 reference，但保持短；需要回流模板时运行 `/rekit promote` 或 `rekit/rekit.ps1 promote`。
5. 运行 `/rekit validate` 或 `rekit/rekit.ps1 validate`，验证 Markdown 大小不超预算。

## 自动维护触发点

| 事件 | 更新 |
|---|---|
| 完成一批 handler 复核 | `task-handoff.md` 的 coverage、已合入、待复核列表。 |
| 发现新的复核规则 | `singleton-handler-review.md`。 |
| 发现可复用项目流程 | `workflow-template.md`。 |
| 新增/重评工具或止损条件 | `toolchain-router.md`。 |
| 文档再次变大 | 归档长内容，保留短摘要。 |
| 需要将 case 经验回流模板 | 先预览 `/rekit promote -WhatIf`，确认安全后再 `-Apply`。 |

## 新会话恢复提示

```text
读取项目 CLAUDE.local.md，然后按 references/vmp-re/README.md 路由。当前任务从 references/vmp-re/task-handoff.md 接手；不要读取 captures/doc_archive 或大 CSV，除非需要按 handler 过滤。
```
