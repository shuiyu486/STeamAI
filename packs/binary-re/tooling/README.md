# binary-re tooling

本目录保存通用 binary/function/API passive analysis、已有 IDA sidecar 的 bounded review，以及 VMProtect trace/devirtualization 的声明式 recipe。它不包含 runtime、adapter host 或命令调度器。

## 内容

| 路径 | 用途 |
|---|---|
| `catalog.yml` | capability、状态、输入输出、side effects、确认要求和止损。 |
| `recipes/static-binary-triage.md` | passive metadata/section/import/string sidecar。 |
| `recipes/function-behavior-review.md` | saved function/API summary 的只读复核。 |
| 其余 `recipes/*.md` | VMP/IDA、trace、value-flow 与成员协同。 |
| `schemas/*.json` | case-local sidecar 的数据形状。 |
| `patches/*.md` | 第三方工具补丁/适配经验。 |
| `candidates/` | 经脱敏审查、尚未回流的 tooling candidate。 |

## 边界

- catalog 只描述能力，永远不作为可解释执行命令。
- passive sidecar、bounded query 和 tool capability 可以进入 pack；真实样本、hash、IOC、完整函数、dump、trace、patch bytes、凭据与绝对路径不得进入。
- actual heavy action 需要 exact case scope 的用户具体确认、Claude Code 工具权限、隔离、预算、输出、rollback 和 stop conditions。
- learning 只从 accepted finding/review 提炼，用户确认完整 exact patch 前不写 canonical pack。
