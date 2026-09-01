# Template tooling

本目录保存新 pack 的声明式 tool capability、recipes 与经审查的候选经验；不包含 adapter runtime、可执行脚本或命令调度器。

## 内容

| 路径 | 用途 |
|---|---|
| `catalog.yml` | 工具状态、输入输出、side effects、确认要求和止损。 |
| `recipes/*.md` | 按任务阶段记录可复用工具流程。 |
| `candidates/` | 经脱敏审查但尚未回流的 tooling candidate。 |

## 原则

- catalog 的 `entry` 或示例文本不得被解释为可执行命令。
- 优先记录 passive/read-only 路线；heavy action 只记录风险、确认条件和止损。
- 不硬编码本机路径；使用 `<caseRoot>`、`<toolsRoot>`、`<target>`。
- 不保存真实样本、trace、dump、凭据、客户信息或完整工具输出。
