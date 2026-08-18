# Binary RE 按需路由入口

> `binary-re` 是唯一 active 二进制逆向 pack。通用 static triage、function/API behavior review 是基线；VMProtect x64 trace-based devirtualization 与 IDA sidecar inspection 是同一 pack 内的成熟专项能力。

## 常驻原则

- 先读项目内 `CLAUDE.local.md` 与 fresh `/steamai status`，再从下表选择一个任务入口；不要默认串读全部 reference。
- current `.steamai` 项目使用 `/steamai`；legacy-only `.rekit` 项目才使用 `/rekit` compatibility。
- 当前状态以 `task-handoff.md` 和 typed lane state 为入口；原始证据保存在 case-local sidecar、CSV 或 artifact，不复制进 pack 文档。
- 子 agent 只提交 observation、request、candidate 或 verification；main agent 承担合并、ledger、handoff 与 authority 写入。

## 路由表

| 任务 | 读取文档 | 说明 |
|---|---|---|
| 接手当前项目 | `task-handoff.md` | 当前 scope、lane、pending gate、证据与下一步。 |
| 通用 binary/function/API 分析 | `general-analysis.md`、`general-workflow.md` | 从 passive sidecar 到独立 reviewer 的轻到重路线。 |
| 通用 Agent Team 分片 | `general-agent-team.md` | `binary-analysis` lane、bounded review 与 packet contract。 |
| 通用工具或 adapter | `general-toolchain-router.md`、`<templateRoot>/packs/binary-re/tooling/README.md` | static triage、saved summary review 与 heavy action gate。 |
| VMProtect x64 专项 | `workflow-template.md` | VMEnter、context probe、trace、handler value-flow 与 routine IR。 |
| VMP/IDA 工具选择 | `toolchain-router.md` | 公开工具止损、IDA sidecar、debug/trace 边界。 |
| VMP Agent Team 协作 | `agent-driven-re.md`、`lane-collaboration.md` | devirt mainline、feature lane、candidate→confirmed。 |
| 批量 handler/trace 复核 | `progressive-disclosure.md`、`singleton-handler-review.md` | 固定分片、读取预算与低样本 focused review。 |
| 通用 policy | `<templateRoot>/common/policies/README.md` | Agent Team、adapter、review-first、证据和写入边界。 |
| VMP 专项 overlay | `<templateRoot>/packs/binary-re/policies/README.md` | 只补充 VMP trace/devirtualization 规则。 |

## 写入与执行边界

- 不把样本、完整二进制、hash、IOC、dump、trace、memory snapshot、patch、完整函数体、符号表、客户上下文或绝对路径写入 pack。
- 大输出保存为 case-local sidecar；聊天和 Markdown 只引用脱敏 alias、row id、摘要和相对证据位置。
- debug、trace、inject、dump、patch、network、symex、批量反编译、rename/comment 或 database writeback 必须有 exact target、隔离、预算、stop conditions、strict durable profile 与 fresh `authorized-gate`。
- `gate -Apply` 只记录 gate decision；actual adapter/executor 才执行动作并留下 observation/evidence。
- 可复用经验通过 `/steamai promote` review-first 回流；用户确认 exact candidate 与 hash 前不写 pack。
