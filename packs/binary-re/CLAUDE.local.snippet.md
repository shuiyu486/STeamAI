<!-- BEGIN binary-re-template:router v0.3.0 -->
## Binary RE 按需路由

`binary-re` 是唯一 active 二进制逆向 pack：通用 static triage、function/API behavior review 是基线能力，VMProtect x64 trace-based devirtualization 与 IDA sidecar inspection 是成熟专项能力。先读 `references/binary-re/README.md`，只按当前任务选择一个入口。

| 任务 | 读取 |
|---|---|
| 新会话接手当前进度 | `references/binary-re/task-handoff.md` |
| 通用 binary/function/API 分析 | `references/binary-re/general-analysis.md`、`references/binary-re/general-workflow.md` |
| 通用 Agent Team 分片与复核 | `references/binary-re/general-agent-team.md` |
| 通用工具或 sidecar 选择 | `references/binary-re/general-toolchain-router.md` |
| VMProtect x64 trace-based devirtualization | `references/binary-re/workflow-template.md` |
| VMP/IDA 工具链、handler 或长 trace 复核 | `references/binary-re/toolchain-router.md`、`references/binary-re/progressive-disclosure.md` |

规则：样本、完整二进制、hash、IOC、trace、dump、patch、完整函数体、客户信息和绝对路径只留在 case-local artifact/sidecar。子 agent 默认只读或仅写自己的 lane workspace；main agent 负责 ledger、handoff、review merge 和 authority 写入。heavy action 必须在 exact scope 内通过 fresh gate；`gate -Apply` 只记录 decision，不执行动作。current `.steamai` 项目使用 `/steamai`；legacy-only `.rekit` 项目才使用 `/rekit` compatibility。
<!-- END binary-re-template:router -->
