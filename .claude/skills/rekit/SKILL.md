---
name: rekit
description: Re-context-kits toolkit entrypoint. Use this skill when the user mentions rekit, re-context-kits, context kits, security research packs, VMProtect RE templates, attaching a case to a kit, syncing templates into a case, promoting case learnings back to the kit, or validating context-kit structure. Prefer this skill over manually running pack scripts.
disable-model-invocation: true
---

# rekit

`/rekit` 是 `re-context-kits` 的目录级入口。目标是让 kit 仓库 clone 后直接可用，并让每个 case 通过薄 shim 回到本仓库的 canonical runtime。

## 工作模式

- **kit 模式**：当前目录是 `re-context-kits` 仓库，直接使用本仓库的 canonical `/rekit`。
- **case 模式**：当前目录是具体 case，先读取 `.rekit/instance.yml`；若不存在则回退读取 `.re-template.yml`，取得 `templateRoot` 与 `templatePack`，再遵循 template root 中的 canonical `/rekit` 规则。
- **不默认安装用户级 skill**：canonical skill 跟随 git 仓库；case 只生成 `.claude/skills/rekit/SKILL.md` 薄 shim。

## 用户使用方式

产品方向是 Mission Control：优先让用户用自然语言指挥主 Agent，而不是手动记 `/rekit` 子命令。`/rekit` 是主 Agent、维护者、自动化和排障使用的 deterministic runtime API；底层 Go CLI 是 canonical runtime，`rekit.ps1` 只是迁移期 legacy façade，二者都不应作为日常入口展示。

可给用户展示的日常表达优先是：

```text
开始这个 case，目标是还原核心逻辑。
继续推进当前 mission。
总体怎么样？哪些 lane 卡住了？
让我进入 verifier lane 帮它纠错。
这个 lane 上下文污染了，生成接手包，让新会话接手。
把这次可复用经验整理成 promote 候选。
```

需要说明 runtime API 时，再展示 `/rekit` 形式：

```text
/rekit init -Target <caseRoot> -Pack vmp-re -ProjectName <caseName> -Apply
/rekit attach -Target <caseRoot> -Pack vmp-re -Apply
/rekit status
/rekit packs
/rekit overview
/rekit continue
/rekit start <name>
/rekit handoff
/rekit sync
/rekit promote
/rekit doctor
/rekit repair
/rekit note
```

底层 runtime 只作为 `/rekit` 的内部实现；除非用户明确要求排障，不在日常说明中展示。

## 命令语义

| 用户意图 | 行为 |
|---|---|
| `/rekit status` | 只读显示 kit / case 绑定状态；检测迁移但不修复；`-Format json` 输出机器可读 status envelope。 |
| `/rekit packs` | 维护者只读查看当前 kit 内 pack 矩阵：成熟度、schema、route、managed/tooling 和 authority lane 概览；`-Format json` 输出机器可读 inventory。 |
| `/rekit attach` | 将已有 case 绑定到当前 template root 和 pack。 |
| `/rekit repair` | 预览迁移后的 metadata 修复；用户确认后才写入。 |
| `/rekit init` / `/rekit bootstrap` | 初始化 case metadata、case-local shim 和模板文件。 |
| `/rekit overview` | 显示项目概览、主线/支线、共享事实统计、Mission Control brief、逐 lane executor action index、最近 verification/decision 和下一步建议；JSON `laneExecutorActions[]` 与文本直接展示 blocked/ready、typed blocker counts、requirements、resume/handoff command，next steps 先处理 reconcile / pending gate / open decision 且只为 ready lane 建议 continue；`authorized-gate` 作为 durable autonomy 已授权决策单独展示但不阻塞 lane；缺 `.rekit/board.json` 时由 Go 初始化 case-local board/facts/policy/default authority lane；`-Format json` 输出机器可读 overview envelope；只表示总览，不代表当前会话已选择工作线。 |
| `/rekit continue main` | 明确接手主线并整理相关状态；多工作线时无参数 `continue` 只列选择，不盲猜；`-WhatIf -Format json` 输出机器可读非写入 continue preview 和结构化 `missionBrief`；每轮 digest 记录 inputs/route/packet refs/Mission Control brief（含 pending gates 与非阻塞 authorized gates）/outputs/decisions/open risks。 |
| `/rekit continue <name>` | 明确接手某条功能支线，只整理该支线的 workspace/outbox、路由 request、刷新接续提示；`-WhatIf -Format json` 预览收集、路由、authority append 计划并输出结构化 `missionBrief` 与 `executorAction`；显式 `-Apply` 的 JSON envelope、run `status.json`、digest、lane `RESUME.md` 与 typed `checkpoints/latest.json` 记录 apply 后 `missionBrief` / lane-local Mission Control brief / executor action snapshot / gate snapshot，让 lane executor 直接看到 blocked/ready、基于 typed facts 的 pending gate / open intervention / open decision counts、blocker reasons、reconcile/pending-gate/open-decision requirements、resume/handoff command、pending gates 与非阻塞 authorized gates；apply 后若新产生 open decision，JSON `nextSteps` 与 text 只建议 lane-local blocker resolution，不再建议 continue；若该 lane 存在 effective open intervention，则 fail-closed，先要求 `/rekit reconcile <name> -InterventionId <eventId> -Apply`。 |
| `/rekit reconcile <name>` | 显式把 Human-in-the-Lane 干预 reconcile 到 durable lane state；`-WhatIf -Format json` 预览 resolution event、lane event、executor takeover、resume/checkpoint/board 刷新与 `executorAction`；显式 `-Apply` 只写 case-local interventions ledger、lane events、lane.json、RESUME/checkpoint 和 board，刷新 lane-local pending/authorized gate snapshot，并在 JSON/text 输出 apply 后 `executorAction`，让 lane executor 看到 intervention 是否清除、剩余 blocker counts、requirements 与 resume/handoff command；不执行 heavy-tool、不写 authority/confirmed。 |
| `/rekit start <name>` | 创建或进入功能支线，例如 `login`；`-WhatIf -Format json` 输出机器可读非写入 start preview 和结构化 `missionBrief` / `executorAction`，显式 `-Apply` 输出含 apply 后 `missionBrief` / `executorAction` 的 Go JSON envelope，text output 也显示 executor blocker counts、requirements 与 resume/handoff command；支线写自己的 workspace，第二天可用 `/rekit continue <name>` 或接续提示继续。 |
| `/rekit handoff` | 生成项目级接手索引 `.rekit/handovers/latest.md`；索引和 Go JSON envelope 都包含 Mission Control brief，汇总 ready/blocked lanes、pending gates、authorized gates、open decisions、interventions、next agent actions 与 escalations；项目 handoff JSON 还包含 `laneExecutorActions[]`，Markdown lane index 写出每条 lane 的 executor blocker counts、requirements 与 resume/handoff command；`-WhatIf -Format json` 输出机器可读 preview envelope（含结构化 `missionBrief`），显式 `-Apply` 输出 Go JSON envelope；不代表某个会话。 |
| `/rekit handoff <name>` | 生成指定工作线接手文档，例如 `/rekit handoff main` 或 `/rekit handoff login`；`-WhatIf -Format json` 输出指定工作线 preview（含结构化 `missionBrief` 与 `executorAction`），显式 `-Apply` 输出 Go JSON envelope；包含该工作线 Mission Control brief、executor action snapshot 与最近 verification/decision/gate 等轻量 ledger 摘要，且 pending gate、open intervention、open candidate/decision 与 overview 一样会标记该 lane blocked；blocked lane 的 JSON/Markdown 只列 lane-local blocker resolution action 并明确不要继续，ready lane 才建议自己的 continue；`authorized-gate` 单独展示授权 profile / decision 但不阻塞 lane。 |
| `/rekit sync` | 默认先生成 LLM 审查包；用户确认具体范围后才执行写入型 sync。 |
| `/rekit promote` | 默认先生成回流审查包；用户确认后才生成候选或写回 pack。 |
| `/rekit doctor` | 验证 kit / case 结构、文档预算和 policy registry；`-Format json` 输出机器可读验证 rows。 |
| `/rekit gate -WhatIf` / `/rekit gate -Apply` | Go backend 的 heavy-action authorization preflight；`-WhatIf` 默认经 façade 委托 Go，输出 gate decision plan、结构化 `missionBrief`、当前 `executorAction` 与写入该 request 后的 `wouldExecutorAction`，不写 ledger、不执行 full-trace/debug/inject/patch/dump；`-Apply` 默认经 façade 委托 Go，只 append pending-gate 或 authorized-gate request ledger decision，并输出 apply 后 `missionBrief` / `executorAction`；pending-gate 会在 executor action 中显示 pending-gate blocker 与 requirement，durable lane autonomy profile 完全覆盖时可记录非阻塞 `authorized-gate`，并在 overview、handoff、continue digest/status 与 `missionBrief.authorizedGates` 中可见；否则 fail-closed 为 pending/denied decision；实际 heavy-tool 仍由 lane executor / tool adapter 在授权范围内执行并写 evidence，不写 confirmed/authority；用户传入的 `-Risk` override 必须是受支持的小写 risk scalar（`medium`、`high`、`critical`），`-StopConditions` override 必须是逗号/分号/换行分隔的小写 slug/snake token。 |
| `/rekit plan-subagents` | 内部只读计划器：根据 pack manifest 的 `subagentRoutes` 生成子 agent 分片计划；默认经 Go façade 写 `.rekit/reviews/...` 审查产物；不启动 agent，不修改 managed/project source files。日常不要主动推荐给用户。 |
| `/rekit note` | 账本写入/查询入口：向 `.rekit/facts/*.jsonl` append observation/hypothesis/candidate/verification/decision/intervention/rollback/publication/request 事件，用于 heavy-tool gate 登记、decision/observation 手动落账。attached case 的 append 与 `-WhatIf` 默认经 Go façade并输出机器可读 JSON envelope：`-WhatIf` 返回当前 `executorAction` 和内存模拟 append 后的 `wouldExecutorAction`，实际 append 返回写入后的 `executorAction`，duplicate eventId 只返回未改变的当前 action；candidate、defer/open decision、open intervention 与 pending-gate request 会按 shared typed facts 投影对应 blocker。该路径只写 facts JSONL 或预览，不写 authority/confirmed；`-List` 进入只读查询模式，默认经 Go façade 按 `-Kind`/`-Lane` 过滤列出事件；`-List -Format json` 输出机器可读 ledger events。写入受 strict JSONL 与 schema 校验（confidence/decision/status 枚举、evidenceRefs 非空、lane 存在性）。 |

如果用户没有显式给 `Target`，在 case 模式下使用当前工作目录；在 kit 模式下 `doctor/status` 作用于 kit 本身。`status` 只读检测迁移，不静默修复；`repair` 写入前必须得到用户确认。

## LLM-first review 规则

1. `/rekit sync` 与 `/rekit promote` 默认都是 **review-first**：先生成 `.rekit/reviews/<timestamp>-<command>/packet.json`、`summary.md` 和 bounded diff，再由 Claude 比较优劣、冲突与风险。
2. 用户确认前，不要执行会写入 managed files、pack docs、promote candidates、tooling candidates 或 state 的动作。
3. review 报告必须按大项说明：方向、变化、收益、风险、冲突、推荐动作，并给出可选择项。
4. `/rekit sync` 的确认选项优先使用：同步全部推荐项、只同步无冲突项、逐项选择、取消。
5. `/rekit promote` 的确认选项优先使用：用 `-CreateCandidates` 仅生成候选、只生成 tooling candidate、用 `-Apply` 按报告改写进模板、逐项选择、取消。
6. “继续”“好”“confirm”不能扩大授权；写入前必须确认具体动作、target、pack 与文件范围。
7. 内部 review 模式是强只读；旧式 dry run 不等同于 LLM review。

## 执行规则

1. 使用 canonical runtime 执行实际动作，但不要把底层命令作为用户日常入口展示。
2. `sync` 只做 `kit -> case`：更新 manifest 声明的 managed files 与 managed blocks，并为覆盖前文件创建 backup；template files 默认 create-if-missing，只有显式 `-Force` 才会备份并覆盖本地模板目标。
3. `sync` / `promote` 必须要求目标是已经 `attach/init` 的 case；不要对普通目录或拼错路径隐式创建 case 或生成回流候选。
4. `promote` 只做 `case -> kit` 的 review、`-CreateCandidates` 候选提取或 `-Apply` 显式写回；永不提升 `CLAUDE.local.md`、`task-handoff.md`、`tools.local.yml`、`captures/**`、`artifacts/**`。
5. `promote` 同时处理 tooling：从 case 的工具链文档抽象候选，供人工合入 `tooling/catalog.yml` 或 `tooling/recipes/*`。
6. 若 promote 命中绝对路径、样本名、trace/dump/artifact/capture 路径或明显地址快照，先阻止或生成候选报告，不要静默写回模板。
7. `sync` / `promote` 发现 case 路径迁移但 metadata 未修复时必须拒绝执行，提示用户确认后运行 `repair`。
8. 工作线必须持久、agent 可以短命；长期成员身份绑定 durable lane，不绑定旧 Claude Code session。旧会话上下文污染、模型硬切或用户要求重开时，新会话应通过 handoff / packet / evidence 接手同一 lane。跨工作线协同通过 `.rekit/facts/*.jsonl`、inbox/tasks 和 publication 完成，不要求用户手动合并普通事实。
9. 用户可随时进入任意 lane 打断、纠错、改向或硬切模型；lane 继续时应先 reconcile 用户干预并写入 status/outbox/intervention。当前 lane 文档、task packet 或 autonomy profile 明确预授权的 heavy/debug/patch/dump/hook/network/exploit-replay 等动作，可在 scope、预算、止损、输出路径和记录要求内自主执行；超出范围、出现新风险或需要 confirmed/authority/promote 时必须升级。
10. `/rekit continue <name>` 可以写 case-local `.rekit/board.json`、`.rekit/facts/**`、`.rekit/lanes/**`、`.rekit/runs/**`、`.rekit/handovers/**` 和所选支线 workspace；只有 candidate 同时满足 evidence、accepted verifier、confidence、schema、no-conflict、backup、diff、max rows 时，才允许自动 append authority CSV。
10. 覆盖/删除 authority、冲突、schema change、changesProjectBaseline、externalSideEffect、destructiveAction 必须停下来问用户；不要自动执行。
11. 新功能分析使用 `/rekit start <name>`；不要再建议用户使用旧的底层工作线命令。
12. `plan-subagents` 只作为内部只读计划器，不是日常用户入口；默认经 Go façade 写 review artifacts，但不自动 spawn agent；能由主 agent 或自动流程判断时，不要求用户手动调用。
13. Go façade 默认接管低风险只读命令：`status`、`packs`、kit/case `doctor/validate`，attached case 的 `overview` 文本/JSON 与缺 board 时的 case-local board/facts/policy/default authority lane 初始化，`note -List` 文本/table/tsv/JSON 只读查询，attached case 的 `note` append 与 `note -WhatIf` facts JSONL 写入/预览，`gate -WhatIf` 非写入 heavy-tool gate preview，`gate -Apply` pending-gate request ledger 写入，attached case 的 `start` / `handoff` JSON preview、explicit apply、文本 preview 与 bare/default 工作线 flow，`continue` JSON preview、explicit apply 与文本/default preview 的 case-local facts/routing/run digest/lane resume/checkpoint/board safe subset（存在 effective open intervention 时 fail-closed，需先 `reconcile`），边界清晰的 case lifecycle `attach`、`repair`、`init/bootstrap` 预览与显式 `-Apply`，`/rekit sync` review、`sync -Apply` 实际写入和 `sync -Apply -WhatIf -Format json` 非写入预览，`/rekit promote` review、review artifact 写入、`promote -CreateCandidates` 实际候选写入、`promote -CreateCandidates -WhatIf -Format json` 非写入预览、`promote -Apply` 实际 pack source 写入和 `promote -Apply -WhatIf -Format json` 非写入预览，`reconcile` 显式 resolution 写入与 lane executor/resume/checkpoint/board 刷新，以及 `plan-subagents` review artifact 写入。`release-check`、`status`、`packs`、`doctor/validate`、`attach/repair/init/bootstrap`、`sync/update`、`promote`、`overview`、`note`、`gate`、`start`、`handoff`、`continue`、`reconcile` 与 `plan-subagents` 已 no-fallback；`REKIT_GO_DISABLE=1` 不再让 Go-default command rows 回落到 PowerShell 业务实现。文本 `sync -Apply -WhatIf`、文本 `promote -CreateCandidates/-Apply -WhatIf`、case lifecycle fallback 与 workstream fallback 已 no-fallback；实际 heavy-tool 执行、authority/confirmed 写入和其它写入路径仍需显式 gate 或手动路径。
14. manifest 中所有文件路径必须是相对路径，并且不能越出 case root 或 pack root。
15. 所有写操作后都运行对应 doctor；失败时如实报告错误与下一步。

## 常用说明模板

对用户解释时优先这样说：

```text
第一次 clone 后，在 kit 仓库启动 Claude Code，然后用 /rekit init 或 /rekit attach。
以后在 case 里优先用 /rekit overview 看项目概览，再用 /rekit continue main 或 /rekit continue <name> 明确选择工作线；需要新功能支线时用 /rekit start <name>。
/rekit handoff 生成项目级索引，/rekit handoff main 或 /rekit handoff <name> 生成指定工作线接手文档。
sync/promote 仍会先生成审查报告，确认后才写入模板；底层脚本只是内部实现，不需要手动执行。
```
