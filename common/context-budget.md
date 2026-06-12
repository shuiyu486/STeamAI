# Context budget

建议预算：

| 文件类型 | 预算 |
|---|---:|
| `CLAUDE.local.md` 常驻 | 8KB |
| `task-handoff.md` 接手文档 | 12KB |
| 单个 reference 文档 | 16KB |
| 归档文档 | 不主动读取 |

规则：大 CSV、trace、disasm、decompile 不进入常驻上下文；只按任务读取片段或用脚本统计。
