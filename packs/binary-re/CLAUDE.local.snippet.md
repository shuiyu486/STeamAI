<!-- BEGIN binary-re-template:router v0.4.0 -->
## Binary RE 按需路由

`binary-re` 提供通用 static triage、function/API behavior review、bounded sidecar inspection，以及 review-first 的 VMProtect trace/devirtualization 参考。先读 `references/binary-re/README.md`，只按当前任务选择一个入口。

| 任务 | 读取 |
|---|---|
| 通用 binary/function/API 分析 | `references/binary-re/general-analysis.md`、`general-workflow.md` |
| 团队分工与独立复核 | `references/binary-re/general-agent-team.md` |
| 通用工具或 sidecar 选择 | `references/binary-re/general-toolchain-router.md` |
| VMProtect x64 trace/devirtualization | `references/binary-re/workflow-template.md` |
| VMP/IDA 工具、handler 或 trace 复核 | `references/binary-re/toolchain-router.md`、`progressive-disclosure.md` |

规则：样本、完整二进制、hash、IOC、trace、dump、patch、完整函数体、客户信息和绝对路径只留在 case-local artifact/evidence。Commander 按需创建 durable member；每个问题一名 owner、最多一名 verifier；Reviewer 只读 evidence/finding 并只写 review。heavy action 必须取得 exact case scope 的用户具体确认和 Claude Code 工具权限，并遵守隔离、预算、输出与 stop conditions。
<!-- END binary-re-template:router -->
