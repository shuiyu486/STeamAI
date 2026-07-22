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

产品方向是 Mission Control：优先让用户用自然语言指挥主 Agent，而不是手动记 `/rekit` 子命令。`/rekit` 是主 Agent、维护者、自动化和排障使用的 deterministic runtime API；底层 Go CLI 是 canonical runtime，`rekit.ps1` 只是 retained compatibility façade，无业务 runtime 或 PowerShell fallback；二者都不应作为日常入口展示。

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
| `/rekit` / `/rekit status` | 无子命令时默认走只读 status；attached case 中未显式传 `-Pack` 时使用 case metadata 的 `templatePack`；第一屏显示当前 pack 及来源（显式 `-Pack`、case metadata 或 repo default），case 模式还显示当前 pack 是否匹配 metadata `templatePack` 及不一致诊断；pack mismatch、moved metadata 或 case-local thin shim drift/missing 时输出 bounded next steps（status recheck 或 `repair -WhatIf -Format text` preview，`repair -Apply` 仍需显式确认）；case 模式还把已记录 `pending-gate` 的 lane handoff、`gate -WhatIf` preview、request-decision `gate -Apply` command、decision/continue boundary 与 requested evidence，已记录 `authorized-gate` 的可复制 execution report contract command、validation boundary、`valid=true` 后 bounded evidence record boundary 与 authorized output/stop evidence，open candidate/defer decision 的 source fact/list command、lane handoff、`note -WhatIf` decision preview、append-only decision record command、decision/continue boundary 与 evidence refs，以及 open intervention 的 lane handoff、`reconcile -WhatIf` preview、case-local `reconcile -Apply` command、decision/continue boundary 与 intervention evidence 投影到第一屏；kit 模式 project handoff 还直接显示最新 batch 的 local validation readiness、`release-check ready=true` evidence、remote release-gate state/detail（run refs、jobs、empty-steps、can-claim-green 与 no-green boundary）、validation evidence 与 next action；同时显示 kit / case 绑定状态与 case-local thin shim / canonical skill readiness；检测迁移或 shim drift 但不修复；`-Format json` 输出机器可读 status envelope、`packSource`、case pack diagnostic、case `nextSteps`、case `pendingGateHandoffs[]` / `authorizedGateHandoffs[]` / `openDecisionHandoffs[]`、`caseShim` readiness 与 project handoff。 |
| `/rekit packs` | 维护者只读查看当前 kit 内 pack 矩阵：成熟度、schema、route、managed/tooling 和 authority lane 概览；`-Format json` 输出机器可读 inventory。 |
| `/rekit attach` | 将已有 case 绑定到当前 template root 和 pack。 |
| `/rekit repair` | 预览迁移后的 metadata 修复；用户确认后才写入。 |
| `/rekit init` / `/rekit bootstrap` | 初始化 case metadata、case-local shim 和模板文件。 |
| `/rekit overview` | 显示项目概览、主线/支线、共享事实统计、Mission Control brief、逐 lane executor action index、最近 verification/decision 和下一步建议；JSON `laneExecutorActions[]` 与文本直接展示 blocked/ready、typed blocker counts、requirements、resume/handoff command，next steps 先处理 reconcile / pending gate / open decision 且只为 ready lane 建议 continue；`authorized-gate` 作为 durable autonomy 已授权决策单独展示但不阻塞 lane，并在 Mission brief 中带出 `requestedBudget`、`outputPaths`、`stopConditions`、`eventId` 与可复制的 `reportContract=/rekit gate -ExecutionReportContract -GateEventId ... -Format json` handoff；缺 `.rekit/board.json` 时由 Go 初始化 case-local board/facts/policy/default authority lane；`-Format json` 输出机器可读 overview envelope；只表示总览，不代表当前会话已选择工作线。 |
| `/rekit continue main` | 明确接手主线并整理相关状态；多工作线时无参数 `continue` 只列选择，不盲猜；`-WhatIf -Format json` 输出机器可读非写入 continue preview 和结构化 `missionBrief`；每轮 digest 记录 inputs/route/packet refs/Mission Control brief（含 pending gates 与非阻塞 authorized gates）/outputs/decisions/open risks，并在存在 execution evidence review 时直接列出 follow-through outcome 的 `when` / `evidence`。 |
| `/rekit continue <name>` | 明确接手某条功能支线，只整理该支线的 workspace/outbox、路由 request、刷新接续提示；`-WhatIf -Format json` 预览收集、路由、authority append 计划并输出结构化 `missionBrief` 与 `executorAction`；显式 `-Apply` 的 JSON envelope、run `status.json`、digest（含 execution evidence review follow-through outcome 的 `when` / `evidence`）、lane `RESUME.md` 与 typed `checkpoints/latest.json` 记录 apply 后 `missionBrief` / lane-local Mission Control brief / executor action snapshot / gate snapshot，让 lane executor 直接看到 blocked/ready、基于 typed facts 的 pending gate / open intervention / open decision counts、blocker reasons、reconcile/pending-gate/open-decision requirements、resume/handoff command、pending gates 与非阻塞 authorized gates；apply 后若新产生 open decision，JSON `nextSteps` 与 text 只建议 lane-local blocker resolution，不再建议 continue；若该 lane 存在 effective open intervention，则 fail-closed，先要求 `/rekit reconcile <name> -InterventionId <eventId> -Apply`。 |
| `/rekit reconcile <name>` | 显式把 Human-in-the-Lane 干预 reconcile 到 durable lane state；`-WhatIf -Format json` 预览 resolution event、lane event、executor takeover、resume/checkpoint/board 刷新与 `executorAction`；显式 `-Apply` 只写 case-local interventions ledger、lane events、lane.json、RESUME/checkpoint 和 board，刷新 lane-local pending/authorized gate snapshot，并在 JSON/text 输出 apply 后 `executorAction`，让 lane executor 看到 intervention 是否清除、剩余 blocker counts、requirements 与 resume/handoff command；不执行 heavy-tool、不写 authority/confirmed。 |
| `/rekit start <name>` | 创建或进入功能支线，例如 `login`；当 `<name>` 解析到已有工作线（如 `main` 或 `feature-login`）时进入该 durable lane，不新建并行 lane；`-WhatIf -Format json` 输出机器可读非写入 start preview 和结构化 `missionBrief` / `executorAction`，显式 `-Apply` 输出含 apply 后 `missionBrief` / `executorAction` 的 Go JSON envelope，text output 也显示 executor blocker counts、requirements 与 resume/handoff command；主 Agent 可用 `start main -Apply -Executor <new-session> -Actor <actor> -Reason <reason>` 让替换会话接手主线并刷新 lane resume/checkpoint/events；runtime 只写 durable executor metadata，不自动 spawn 或管理 session。 |
| `/rekit handoff` | 生成项目级接手索引 `.rekit/handovers/latest.md`；索引和 Go JSON envelope 都包含 Mission Control brief，汇总 ready/blocked lanes、pending gates、authorized gates、open decisions、interventions、next agent actions 与 escalations；项目 handoff JSON 还包含 `laneExecutorActions[]`，Markdown lane index 写出每条 lane 的 executor blocker counts、requirements 与 resume/handoff command；`-WhatIf -Format json` 输出机器可读 preview envelope（含结构化 `missionBrief`），显式 `-Apply` 输出 Go JSON envelope；不代表某个会话。 |
| `/rekit handoff <name>` | 生成指定工作线接手文档，例如 `/rekit handoff main` 或 `/rekit handoff login`；`-WhatIf -Format json` 输出指定工作线 preview（含结构化 `missionBrief` 与 `executorAction`），显式 `-Apply` 输出 Go JSON envelope；包含该工作线 Mission Control brief、executor action snapshot 与最近 verification/decision/gate 等轻量 ledger 摘要，且 pending gate、open intervention、open candidate/decision 与 overview 一样会标记该 lane blocked；blocked lane 的 JSON/Markdown 只列 lane-local blocker resolution action 并明确不要继续，ready lane 才建议自己的 continue；`authorized-gate` 单独展示授权 profile / decision 但不阻塞 lane，并带出 requested budget、authorized output paths、stop conditions、gate `eventId` 与可复制 report contract command，供替换 executor 接手 actual heavy action 前核对边界并读取 adapter execution report contract。 |
| `/rekit sync` | 默认先生成 LLM 审查包；用户确认具体范围后才执行写入型 sync。 |
| `/rekit promote` | 默认先生成回流审查包；用户确认后才生成候选或写回 pack。 |
| `/rekit doctor` | 验证 kit / case 结构、文档预算和 policy registry；`-Format json` 输出机器可读验证 rows。 |
| `/rekit gate -WhatIf` / `/rekit gate -Apply` | Go backend 的 heavy-action authorization preflight；`-WhatIf` 默认经 façade 委托 Go，输出 gate decision plan、结构化 `missionBrief`、当前 `executorAction` 与写入该 request 后的 `wouldExecutorAction`，不写 ledger、不执行 full-trace/debug/inject/patch/dump；`-Apply` 默认经 façade 委托 Go，append pending-gate 或 authorized-gate request ledger decision，并输出 apply 后 `missionBrief` / `executorAction`；`missionBrief.authorizedGates` 会直接显示 requested budget、authorized output paths、stop conditions、gate event id 与可复制 report contract command；对已授权 gate 可先用 `-ExecutionReportContract -GateEventId <authorized-gate-event-id> -Format json` 读取只读 adapter execution report contract（在 case-local / authorized output workspace cwd 中可省略 `-Target`），供 lane executor / tool adapter 执行前先看 compact `reportSummary` 判断 state、report/default path、validation/record readiness、current action、repair/main-escalation flags、allowed counts 与 no-heavy/no-authority boundary，再按需消费完整 action、budget、output paths、default report path、status、stop conditions、boundary/escalation requirements、validation failure taxonomy / `validationRepairHints[]`、`liveValidation` handoff（`authorizedWorkspaces[]`、`reportFileName`、`caseRelativeReportPath`、sidecar template、workspace-relative 与 case-relative validate/record command strings + args、从 pack `tooling/catalog.yml` 投影的 `adapterCandidates[]`、默认 `selectedAdapter` / sidecar `adapterId` guidance、replay behavior；record handoff 运行前需替换 `<executor-id>`，重复 `RecordArgs` / `CaseRelativeRecordArgs` 返回 `duplicate eventId` 且不追加 observations）和 sidecar 规则；adapter 写出 sidecar 后，可用 `-ValidateExecutionReport -GateEventId <authorized-gate-event-id> -ExecutionReportPath <path> -Format json` 做只读 strict validation preflight（在 case-local / authorized output workspace cwd 中可省略 `-Target`，report path 可相对当前 workspace），输出 `isMutation=false` / `applied=false` 且 `valid=true` 或 `valid=false`，并用 compact `reportSummary` 直接显示 valid/recordReady/recordBlocked、repair hints counts、report status/adapter id/actualBudget、refs/boundary hits、failure code/stage 与下一条安全命令，并在可用时携带 `adapterContext.candidates[]` / `adapterContext.selected` 且 text 输出 adapter candidate / selected adapter provenance（invalid sidecar 含 `error`、`errors[]`、`failureCode`、`failureStage`、带 `evidence[]` / `boundary[]` 的 `repairHints[]`、`reportPath`、可用时的 partial report 与 contract boundaries），且不写 observations ledger；传入 `-GateEventId <authorized-gate-event-id>` 与 `-ExecutionStatus` 时，`-Apply` 只记录 post-action observation execution evidence 到 `.rekit/facts/observations.jsonl`，包含 actual budget、output refs、evidence refs、boundary hits 与 escalation；也可传入 `-ExecutionReportPath` strict intake lane executor / tool adapter 写在 authorized output paths 下的 bounded `adapter-execution-report` JSON sidecar，并可在 case-local / authorized output workspace cwd 中省略 `-Target`、用当前 workspace 相对 sidecar path 记录 evidence，重复记录同一 sidecar 返回 `duplicate eventId` 且不重复 append observations，把 adapter sidecar provenance 与匹配到的 concrete tooling candidate `adapterContext` 嵌入 evidence；拒绝 pending gate、越出 authorized output paths 的 output refs/evidence refs/report path、schema/action/status/gateEventId 不匹配的 report、未被本次 authorized gate `stopConditions` 覆盖的 `boundaryHits`、缺少 bounded summary 的 failed/boundary/escalated/aborted report，以及缺少 `boundaryHits` / `escalation` marker 的 boundary/escalated 或预算越界 report；pending-gate 会在 executor action 中显示 pending-gate blocker 与 requirement，durable lane autonomy profile 完全覆盖时可记录非阻塞 `authorized-gate`，并在 overview、handoff、continue digest/status 与 `missionBrief.authorizedGates` 中可见；否则 fail-closed 为 pending/denied decision；实际 heavy-tool 仍由 lane executor / tool adapter 在授权范围内执行，`/rekit` 只记录 request/evidence，不写 confirmed/authority；用户传入的 `-Risk` override 必须是受支持的小写 risk scalar（`medium`、`high`、`critical`），`-StopConditions` override 必须是逗号/分号/换行分隔的小写 slug/snake token。 |
| `/rekit plan-subagents` | 内部 bounded reviewer runtime：planning mode 根据 pack manifest 的 `subagentRoutes` 写 `.rekit/reviews/...` packet/summary/diff，并输出每个 shard 的 read-only dispatch prompt、strict `reviewerResultContract`、decision mapping、conflict handling、`reviewerIntakeCommands`（含 `repairGuidance[]` 预修复 taxonomy / boundary）与 writeback sequence；runtime 不自动启动 reviewer。planning JSON 的 `reviewerOrchestration.summary` 与 text / `summary.md` summary lines 直接显示 mode、target lane、reviewer/dispatch counts、owner binding、intakeAvailable/dispatchOnly、action queue counts、first dispatch、current action、next actions 与 no-spawn/no-heavy/no-authority boundary。主 Agent实际调度 reviewer后，把单个 JSON object 放到 `reviewerResultPath`，再用 reviewer-intake `-WhatIf/-Apply` 完成 packet/route/shard/items/evidence/output contract 校验、verification-before-decision facts 写回、幂等重试与 overview/handoff/doctor post-validation；reviewer-intake top-level `summary` / text summary lines 直接显示 writeback status、dispatch progress、blocked/repair counts、postValidation totals、reviewer writeback count、current/next action 与 no-heavy/no-authority boundary；postValidation 同时提供 compact `summary` / text summary lines，直接显示 verification/decision totals、doctor row count、lane/executor action state、reviewer writeback count、current action、next actions 与 no-heavy/no-authority boundary；写回后的 status/handoff/continue、lane `RESUME.md`、checkpoint 与 digest 还提供 compact `reviewerWritebackSummary` / reviewer writeback summary lines，汇总 verification/decision counts、latest reviewer session/result/shard、owner binding、risks/conflicts/route output flags、latest evidence refs 与 no-heavy/no-authority/no-spawn boundary；blocked / event-id-collision / post-validation failed intake 会返回 `repairGuidance[]` 并在 text 输出 repair action/evidence/boundary。reviewer 不写文件/ledger，intake 不写 authority/confirmed、不执行 heavy-tool、不修改 managed/project source files。日常不要主动推荐给用户。 |
| `/rekit note` | 账本写入/查询入口：向 `.rekit/facts/*.jsonl` append observation/hypothesis/candidate/verification/decision/intervention/rollback/publication/request 事件，用于 heavy-tool gate 登记、decision/observation 手动落账。attached case 的 append 与 `-WhatIf` 默认经 Go façade并输出机器可读 JSON envelope：`-WhatIf` 返回当前 `executorAction` 和内存模拟 append 后的 `wouldExecutorAction`，实际 append 返回写入后的 `executorAction`，duplicate eventId 只返回未改变的当前 action；candidate、defer/open decision、open intervention 与 pending-gate request 会按 shared typed facts 投影对应 blocker。该路径只写 facts JSONL 或预览，不写 authority/confirmed；`-List` 进入只读查询模式，默认经 Go façade 按 `-Kind`/`-Lane` 过滤列出事件；`-List -Format json` 输出机器可读 ledger events。写入受 strict JSONL 与 schema 校验（confidence/decision/status 枚举、evidenceRefs 非空、lane 存在性）。 |

如果用户没有显式给 `Target`，在 case 模式下使用当前目录向上找到的最近 attached case root；如果没有显式给 `Pack`，attached case 使用 metadata `templatePack`，kit 模式仍使用默认 pack。`status` 会投影 `packSource`，用于区分显式参数、case metadata 与 repo default；case 模式还会投影当前 pack 是否匹配 metadata `templatePack`，显式 pack 不一致时只给诊断，不静默改写 pack。`status` 只读检测迁移与 shim drift，并在需要时给出 bounded next steps；`repair -WhatIf` 只是预览，`repair -Apply` 写入前必须得到用户确认。

## LLM-first review 规则

1. `/rekit sync` 与 `/rekit promote` 默认都是 **review-first**：先生成 `.rekit/reviews/<timestamp>-<command>/packet.json`、`summary.md` 和 bounded diff，再由 Claude 比较优劣、冲突与风险。
2. 用户确认前，不要执行会写入 managed files、pack docs、promote candidates、tooling candidates 或 state 的动作。
3. review 报告必须按大项说明：方向、变化、收益、风险、冲突、推荐动作，并给出可选择项。
4. `/rekit sync` 的确认选项优先使用：同步全部推荐项、只同步无冲突项、逐项选择、取消。
5. `/rekit promote` 的确认选项优先使用：用 `-CreateCandidates` 仅生成候选、只生成 tooling candidate、用 `-Apply` 按报告改写进模板、逐项选择、取消；`-CreateCandidates` 的 `reviewPlan.reviewSummary` / text summary 只压缩候选 review、cleanup、reconsume 与 boundary，不代表已合入、已清理或已验证。
6. “继续”“好”“confirm”不能扩大授权；写入前必须确认具体动作、target、pack 与文件范围。
7. 内部 review 模式是强只读；旧式 dry run 不等同于 LLM review。

## 执行规则

1. 使用 canonical runtime 执行实际动作，但不要把底层命令作为用户日常入口展示。
2. `sync` 只做 `kit -> case`：更新 manifest 声明的 managed files 与 managed blocks，并为覆盖前文件创建 backup；template files 默认 create-if-missing，只有显式 `-Force` 才会备份并覆盖本地模板目标。
3. `sync` / `promote` 必须要求目标是已经 `attach/init` 的 case；不要对普通目录或拼错路径隐式创建 case 或生成回流候选。
4. `promote` 只做 `case -> kit` 的 review、`-CreateCandidates` 候选提取或 `-Apply` 显式写回；永不提升 `CLAUDE.local.md`、`task-handoff.md`、`tools.local.yml`、`captures/**`、`artifacts/**`。
5. `promote` 同时处理 tooling：从 case 的工具链文档抽象候选，供人工合入 `tooling/catalog.yml` 或 `tooling/recipes/*`；`promote -CreateCandidates`、`status` 与 `release-check` 的 pack-memory review summary 只读投影 candidate/tooling/index、decision/cleanup/reconsume artifact 与 next action counts，不能替代人工 accept/reject、cleanup、doctor 或 fresh/attached reconsume proof。
6. 若 promote 命中绝对路径、样本名、trace/dump/artifact/capture 路径或明显地址快照，先阻止或生成候选报告，不要静默写回模板。
7. `sync` / `promote` 发现 case 路径迁移但 metadata 未修复时必须拒绝执行，提示先运行 `repair -WhatIf -Format text` 预览 metadata 与 thin-shim refresh，再显式确认 `repair -Apply`。
8. 工作线必须持久、agent 可以短命；长期成员身份绑定 durable lane，不绑定旧 Claude Code session。旧会话上下文污染、模型硬切或用户要求重开时，新会话应通过 handoff / packet / evidence 接手同一 lane。跨工作线协同通过 `.rekit/facts/*.jsonl`、inbox/tasks 和 publication 完成，不要求用户手动合并普通事实。
9. 用户可随时进入任意 lane 打断、纠错、改向或硬切模型；当前 runtime 在 `continue` 时对 effective open intervention fail-closed，要求先显式 `reconcile`，再把 resolution、executor takeover、resume/checkpoint 和 board 写回 durable state。lane 文档或 task packet 只能表达预授权意图；确定性执行依据是 strict validated `.rekit/lanes/<lane>/autonomy.json` 加 `gate` 记录的 `authorized-gate` decision。executor 仅可在 action、exact target、typed budget、stop conditions、output paths、record/notify 边界完全覆盖时不逐步询问地执行 heavy action；越界、新风险或需要 confirmed/authority/promote 时必须升级。
10. `/rekit continue <name>` 可以写 case-local `.rekit/board.json`、`.rekit/facts/**`、`.rekit/lanes/**`、`.rekit/runs/**`、`.rekit/handovers/**` 和所选支线 workspace；candidate 满足 evidence、accepted verifier、confidence、schema、no-conflict、backup、diff、max rows 时只代表可进入 authority review。`continue -Apply` 不写 authority/confirmed；这类写入必须由主 Agent 在独立 gate 和显式用户确认后处理。
11. 覆盖/删除 authority、冲突、schema change、changesProjectBaseline、externalSideEffect、destructiveAction 必须停下来问用户；不要自动执行。
12. 新功能分析使用 `/rekit start <name>`；不要再建议用户使用旧的底层工作线命令。
13. `plan-subagents` 不是日常用户入口。planning mode 只写 review artifacts 且不自动 spawn agent；reviewer intake 必须由主 Agent显式提供 `PacketPath`、`ReviewerResultPath`、`Lane`、`Actor`，并先用 WhatIf 审查，再用 Apply 让 runtime 写 verification/main decision。intake 写回后的 downstream `reviewerWritebacks[]`、overview detail、status/handoff/continue、lane `RESUME.md`、checkpoint 与 digest 会投影 reviewer decision/recommendedVerdict、risks/conflicts 与 normalized routeOutput；其中 status/handoff/continue、lane `RESUME.md`、checkpoint 与 digest 还通过 `reviewerWritebackSummary` / reviewer writeback summary lines 汇总 latest reviewer session/result/shard、counts、evidence refs 与 no-heavy/no-authority/no-spawn boundary，replacement executor 不必重开 reviewer result JSON 或逐条扫描 writebacks 即可复核 reviewer provenance。不要要求用户手动调用，也不要把该路径误述为 reviewer 自行写 ledger 或 runtime 自动 dispatch。
14. Go façade 默认接管低风险只读命令：`status`、`packs`、kit/case `doctor/validate`，attached case 的 `overview` 文本/JSON 与缺 board 时的 case-local board/facts/policy/default authority lane 初始化，`note -List` 文本/table/tsv/JSON 只读查询，attached case 的 `note` append 与 `note -WhatIf` facts JSONL 写入/预览，`gate -WhatIf` 非写入 heavy-tool gate preview，`gate -Apply` pending/authorized request ledger 写入与 authorized execution observation evidence 写入，attached case 的 `start` / `handoff` JSON preview、explicit apply、文本 preview 与 bare/default 工作线 flow，`continue` JSON preview、explicit apply 与文本/default preview 的 case-local facts/routing/run digest/lane resume/checkpoint/board safe subset（存在 effective open intervention 时 fail-closed，需先 `reconcile`），边界清晰的 case lifecycle `attach`、`repair`、`init/bootstrap` 预览与显式 `-Apply`，`/rekit sync` review、`sync -Apply` 实际写入和 `sync -Apply -WhatIf -Format json` 非写入预览，`/rekit promote` review、review artifact 写入、`promote -CreateCandidates` 实际候选写入、`promote -CreateCandidates -WhatIf -Format json` 非写入预览、`promote -Apply` 实际 pack source 写入和 `promote -Apply -WhatIf -Format json` 非写入预览，`reconcile` 显式 resolution 写入与 lane executor/resume/checkpoint/board 刷新，以及 `plan-subagents` planning review artifact 写入和显式 reviewer intake 的 WhatIf 预览、verification/main-decision facts writeback 与 overview/handoff/doctor post-validation。`release-check`、`status`、`packs`、`doctor/validate`、`attach/repair/init/bootstrap`、`sync/update`、`promote`、`overview`、`note`、`gate`、`start`、`handoff`、`continue`、`reconcile` 与 `plan-subagents` 已 no-fallback；`REKIT_GO_DISABLE=1` 不再让 Go-default command rows 回落到 PowerShell 业务实现。文本 `sync -Apply -WhatIf`、文本 `promote -CreateCandidates/-Apply -WhatIf`、case lifecycle fallback 与 workstream fallback 已 no-fallback；实际 heavy-tool 执行、authority/confirmed 写入和其它写入路径仍需显式 gate 或手动路径。
15. manifest 中所有文件路径必须是相对路径，并且不能越出 case root 或 pack root。
16. 所有写操作后都运行对应 doctor；失败时如实报告错误与下一步。

## 常用说明模板

对用户解释时优先这样说：

```text
第一次 clone 后，在 kit 仓库启动 Claude Code，然后用 /rekit init 或 /rekit attach。
以后在 case 里优先用 /rekit overview 看项目概览，再用 /rekit continue main 或 /rekit continue <name> 明确选择工作线；需要新功能支线时用 /rekit start <name>。
/rekit handoff 生成项目级索引，/rekit handoff main 或 /rekit handoff <name> 生成指定工作线接手文档。
sync/promote 仍会先生成审查报告，确认后才写入模板；底层脚本只是内部实现，不需要手动执行。
```
