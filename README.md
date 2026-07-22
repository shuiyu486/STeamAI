# re-context-kits

`re-context-kits` 是面向网络安全研究与安全工程任务的 Claude Code **Agent Team Mission Control** 框架，用于把主 Agent 统筹、长期 member lane、可替换 Claude Code session executor、短命 tactical subagent、领域工具链、证据账本、验证门禁和可复用安全领域 pack 组织成可持续迭代的 case workspace。

当前阶段，它已经提供 `/rekit` case 管理、首个成熟 pack `vmp-re`、安全领域 pack 骨架 `web-security`、`malware-analysis`、`vuln-research`、`ctf`、`unpack-pe`、`ollvm`、`android-native` 与 `generic-binary-re`、工作线协同、handoff、sync/promote 和 tooling 经验沉淀；`vmp-re` 是验证框架的第一个重点领域，不是最终边界。长期目标是逐步扩展到逆向工程、恶意样本分析、漏洞研究、Web/API 安全评估、授权测试/靶场/CTF、Android native、OLLVM 等多类安全任务。

当前项目不是全自动脱壳器、自动逆向引擎、自动漏洞挖掘器、自动恶意样本分析平台或通用自动渗透平台；它优先提供可审计、可交接、review-first 的 Agent Team 底座。lane 文档/packet 只能表达授权意图；heavy action 的确定性预授权来自 strict durable autonomy profile 与 `authorized-gate` decision。

一句话：**用户主要指挥主 Agent / Mission Commander；主 Agent 调度 durable member lanes、可替换会话执行体和短命 tactical subagents；`overview` JSON/text 与 project handoff 提供逐 lane typed executor action index 和 blocker-aware 下一步，`start` / `gate` / `reconcile` envelope/text、`handoff` JSON/Markdown、`continue` envelope/run artifacts 与 lane-local `RESUME.md` / typed checkpoint 复用 Mission Control brief、基于 typed facts 的 executor action blocker counts、gate snapshot 与下一步动作；`/rekit`、Go CLI/backend 是背后的 canonical deterministic runtime/API，`rekit.ps1` 仅作为 retained compatibility façade，不承载业务 runtime，也没有 PowerShell 业务 fallback；默认路径继续向 PowerShell-free / Go-native / 跨平台收敛。**

## 项目路线（按需文档索引）

以下是文档索引，不是默认必读清单。新会话、上下文压缩后接手或维护文档时，先读 `docs/context-routing.md`，再按场景只读对应顶部区。

- 新会话与维护文档的按需路由入口：`docs/context-routing.md`
- 新架构使用与旧 case 兼容：`docs/agent-team-usage.md`
- 参考资料吸收映射：`docs/reference-absorption.md`
- 长期愿景与阶段实施方案：`docs/vision.md`
- Mission Control 最终产品方向：`docs/mission-control-product-direction.md`
- 当前架构说明：`docs/design.md`
- 后续批次计划：`docs/batch-plan.md`
- 长期自主 goal 与新会话接手指南：`docs/autonomous-goal.md`
- pack 编写指南：`docs/pack-authoring.md`（新 pack 可从 `packs/_template/` 复制；`packs/web-security/`、`packs/malware-analysis/`、`packs/vuln-research/`、`packs/ctf/`、`packs/unpack-pe/`、`packs/ollvm/`、`packs/android-native/` 与 `packs/generic-binary-re/` 是首批安全领域 pack 骨架）
- evidence / intervention 账本草案：`docs/evidence-ledger.md`
- 半自动 orchestration 计划：`docs/orchestration-plan.md`
- Agent Team rollout 计划：`docs/agent-team-rollout-plan.md`
- VMP/RE Agent Team 工作方式：`packs/vmp-re/references/vmp-re/agent-driven-re.md`
- sync/promote 机制：`docs/promote-sync.md`
- case 迁移说明：`docs/case-migration.md`
- Go backend 渐进迁移：`docs/go-runtime-migration.md`
- Go-first 收束与 release readiness 阶段计划：`docs/go-first-convergence-plan.md`
- 发布门禁与当前 release readiness checklist：`docs/release-readiness.md`（机器可读 inventory 与 release handoff：`go run ./cmd/rekit -- -Command release-check -Format json`；三平台 Go-native workflow 定义：`.github/workflows/release-gate.yml`；inventory ready 不等于远程 jobs 已实际运行并通过，远程 jobs `steps=[]` / `steps 为空` 会通过 `remoteReleaseGateDetail` 记录 run/job/boundary，并按 runner/billing blocker 而不是 remote CI green 处理）
- PowerShell-free / Go-native convergence roadmap：`docs/powershell-deprecation.md`

## 如果你在维护本仓库

本仓库本身不是具体安全 case，也不是具体 RE case。维护时先看根目录 `CLAUDE.md` 与 `docs/context-routing.md`，再按需路由到对应顶部章节；不要默认串读或扩写全部 durable docs。

- `/rekit` skill：`.claude/skills/rekit/SKILL.md`
- runtime：`rekit/rekit.ps1` façade、`cmd/rekit/**`、`internal/rekit/**`；legacy `rekit/lib/*.ps1` 已删除，历史语义以 Go runtime 为准。
- 领域 pack：`packs/<pack>/**`
- 通用 policy / prompt：`common/**`
- 设计与路线：`docs/**`

不要因为下面的 case 初始化示例而在本仓库内伪造 case state；只有验证 `init/attach/sync/promote` 行为时才创建临时 case。

## 使用方式：把 kit 接入安全 case（当前以 `vmp-re` 为例）

### 1. 第一次 clone 后

进入 kit 仓库启动 Claude Code：

```text
cd <workspaceRoot>\kits\re-context-kits
claude
```

然后直接对 Claude 说：

```text
/rekit init -Target <workspaceRoot>\cases\<caseName> -Pack vmp-re -ProjectName <caseName> -Apply
```

或已有 case 接入：

```text
/rekit attach -Target <workspaceRoot>\cases\<caseName> -Pack vmp-re -Apply
```

> 这里不需要你手动执行底层脚本。`/rekit` 会调用内部 runtime。

### 2. 之后每天在 case 里

进入 case 启动 Claude Code：

```text
cd <workspaceRoot>\cases\<caseName>
claude
```

日常优先直接用自然语言指挥主 Agent，例如：

```text
开始这个 case，目标是还原核心逻辑。
继续推进当前 mission。
总体怎么样？哪些 lane 卡住了？
让我进入 verifier lane 帮它纠错。
这个 lane 上下文污染了，生成接手包，让新会话接手。
把这次可复用经验整理成 promote 候选。
```

主 Agent 会把这些意图翻译成 `/rekit overview`、`continue`、`start`、`handoff`、`gate`、`note`、`sync`、`promote` 等底层 runtime 操作。用户不需要把 `/rekit` 子命令当成主要交互界面；它们主要是主 Agent、维护者、自动化和排障使用的确定性 API。

排障或新会话接手时再用：

```text
/rekit          # 无子命令时默认只读 status，先确认 kit/case 绑定
/rekit doctor
```

## 目录模型

推荐 workspace 结构：

```text
<workspaceRoot>\
  kits\
    re-context-kits\              # 模板仓库；canonical /rekit + packs + tooling
  cases\
    <caseName>\                   # 具体 case
  tools\                          # 第三方工具
  shared-artifacts\               # 大文件/共享产物
```

`kits/` 和 `cases/` 是 sibling，不是包含关系。这样多个 case 可以复用同一套模板，同时避免样本、trace、dump、大文件混入模板仓库。

## 为什么第一次要在 kit 里启动 Claude Code

新 case 还没有：

```text
<caseRoot>\.claude\skills\rekit\SKILL.md
<caseRoot>\.rekit\instance.yml
<caseRoot>\.rekit\state.json
```

所以第一次需要在 kit 仓库里使用 canonical `/rekit` 完成 `init/attach`。

完成后，case 里会有 thin shim。以后你在 case 根目录或其下的 lane/workspace 子目录启动 Claude Code，也能直接使用 `/rekit`；未显式传 `-Target` 时，Go runtime 会向上寻找最近的 attached case root，再用 metadata 中的 `templateRoot` 定位 canonical runtime。

## Runtime/API 命令参考

这些命令是主 Agent 和维护者使用的确定性 runtime API，不是最终产品的主要 UX。普通日常使用优先通过自然语言让主 Agent 选择和组合这些动作。

| 命令 | 方向 | 什么时候用 |
|---|---|---|
| `/rekit status` | 只读 | 看当前 case 绑定状态、case-local thin shim / canonical skill readiness 与 Mission Commander 接手第一屏；若目录被移动或 shim drift，只提示，不修复；case 模式会投影 `pendingGateHandoffs[]` 的 review / WhatIf / request-decision boundary、`authorizedGateHandoffs[]` 的 execution report contract boundary 及 compact adapter handoff（`defaultReportPath` / `reportPath`、`reportSummary`、`liveValidation` validate/record commands、authorized workspaces、adapter candidate / selected / sidecar guidance、contract error）、`openDecisionHandoffs[]` 的 source fact/list command 与 decision note WhatIf/record boundary、`interventionHandoffs[]` 的 reconcile WhatIf/Apply boundary，以及已记录 authorized execution observation evidence 的 compact `executionEvidenceReviewSummary`（ready/main escalation/duplicate/ref/boundary/action queue summary）和完整 `executionEvidenceReview[]`；`-Format json` 输出机器可读 status envelope 与 `caseShim` readiness。 |
| `/rekit packs` | 只读 | 维护者查看当前 kit 内所有 pack 的成熟度、schema、route、managed/tooling 和 authority lane 概览；`-Format json` 输出机器可读 inventory。 |
| `/rekit overview` | case-local 状态 | 显示项目概览、主线/支线、共享事实统计、Mission Control brief、逐 lane `laneExecutorActions[]` 和 blocker-aware 下一步建议；文本/JSON 直接展示 blocked/ready、typed blocker counts、requirements、resume/handoff command 以及 current executor / generation / last takeover 摘要，只有 ready lane 才进入 continue 建议；已记录 authorized execution observation evidence 时，同时显示 compact `executionEvidenceReviewSummary` 与完整 `executionEvidenceReview[]`，让替换 executor 先确认 ready/main escalation、duplicate、refs、latest review/handoff/current action 与 no-replay/no-authority boundary；`authorized-gate` 作为 durable autonomy 已授权决策单独展示但不阻塞 lane，并在 Mission brief 中带出 `requestedBudget`、`outputPaths`、`stopConditions`、`eventId` 与可复制的 `reportContract=/rekit gate -ExecutionReportContract -GateEventId ... -Format json` handoff；overview JSON/text 还直接投影 compact adapter handoff（`defaultReportPath` / `reportPath`、`reportSummary`、`liveValidation` validate/record commands、authorized workspaces 与 adapter sidecar guidance），让替换 executor 不必切回 status 或完整 contract 才能定位 safe validation/record handoff；缺 `.rekit/board.json` 时由 Go 初始化 case-local board/facts/policy/default authority lane；只表示总览，不代表当前会话已选择工作线。 |
| `/rekit continue main` | case-local 自动整理 | 明确接手主线并整理相关状态；多工作线时不要用无参数 `continue` 盲猜；维护自动化可用 `-WhatIf -Format json` 经默认 Go façade 消费非写入 continue 计划，JSON envelope 含结构化 `missionBrief`，其中包含 pending gates 与非阻塞 authorized gates。 |
| `/rekit continue <name>` | case-local 自动整理 | 明确接手某条功能支线，只整理该支线的 workspace/outbox 并刷新接续提示；`-WhatIf -Format json` 默认经 Go façade 预览收集、路由和 authority append 计划；显式 `-Apply` 的 JSON envelope、run `status.json` 与 `digest.md` 都包含同一 `missionBrief`，且 JSON/status/digest 会直接投影 compact authorized-gate adapter handoff、compact `executionEvidenceReviewSummary` 以及 execution evidence review follow-through outcome 的 `when` / `evidence`，让 lane executor 直接看到 pending gates、非阻塞 authorized gates、adapter report validation/record handoff、evidence review 接续条件与 no-replay/no-authority boundary。 |
| `/rekit start <name>` | case-local 状态 | 创建或进入一个功能支线，例如 `/rekit start login`；支线只写自己的工作区；当 `<name>` 解析到已有工作线（如 `main` 或 `feature-login`）时，start 会进入该 durable lane 而不是新建并行 lane；维护自动化可用 `-WhatIf -Format json` 消费非写入 start 计划和结构化 `missionBrief`，显式 `-Apply` 输出含 apply 后 `missionBrief` 的 Go JSON envelope；start JSON/text 会投影 lane-local compact authorized-gate adapter handoff（default/current report path、`reportSummary`、`liveValidation` validate/record commands、authorized workspace 与 no-heavy/no-authority boundary），让替换 executor 在 preview 或 takeover apply 第一屏即可定位 safe validation/record handoff；需要登记/接管当前会话时由主 Agent显式传 `-Executor <session>`、`-Actor <actor>`、`-Reason <reason>`，例如 `start main -Apply -Executor <new-session>` 可让替换会话接手主线并刷新 lane resume/checkpoint/events；runtime 只写 durable executor metadata，不自动 spawn 或管理 session。 |
| `/rekit handoff` | case-local 状态 | 生成项目级接手索引 `.rekit/handovers/latest.md`；索引和 Go JSON envelope 都包含 Mission Control brief，汇总 ready/blocked lanes、pending gates、authorized gates、compact authorized-gate adapter handoff、open decisions、interventions、next agent actions 与 escalations；维护自动化可用 `-WhatIf -Format json` 消费同一结构化 `missionBrief` 与写入预览，显式 `-Apply` 输出 Go JSON envelope；不代表某个会话。 |
| `/rekit handoff <name>` | case-local 状态 | 生成指定工作线接手文档，例如 `/rekit handoff main` 或 `/rekit handoff login`；lane handoff 的 Markdown 与 Go JSON `missionBrief` 使用 overview 同一 blocker 语义，pending gate、open intervention、open candidate/decision 都会让该 lane 显示为 blocked；`authorized-gate` 单独展示授权 profile / decision 但不阻塞 lane，并带出 requested budget、authorized output paths、stop conditions、gate `eventId`、可复制 report contract command 与 compact adapter handoff，供替换 executor 接手 actual heavy action 前核对边界并读取 default/current report path、report summary、live validation validate/record commands 与 authorized workspace；已记录 execution observation evidence 时，lane handoff JSON/Markdown 同时输出 compact review summary 与完整 review item，避免逐条扫描 refs/follow-through/action queue；`-WhatIf -Format json` 可预览目标工作线 handoff 写入计划，显式 `-Apply` 输出 Go JSON envelope。 |
| `/rekit reconcile <name>` | case-local intervention | 显式处理 lane-local effective open intervention；`-WhatIf` 预览 resolution、lane event、executor takeover、resume/checkpoint/board refresh，`-Apply` 只写这些 case-local durable state，不执行 heavy-tool、不写 authority/confirmed。reconcile JSON/text 会和 start 一样投影 lane-local compact authorized-gate adapter handoff，让替换 executor 在 intervention resolution 前后都能看到同一 safe validate/record handoff 与 no-replay/no-authority boundary。 |
| `/rekit gate -WhatIf` / `/rekit gate -Apply` | case-local gate/evidence | Go backend 的 heavy-action authorization preflight；`-WhatIf -Format json` 输出 gate decision plan 和当前 `missionBrief`，不写 ledger、不执行 heavy-tool；`-Apply -Format json` 默认 append pending-gate 或 authorized-gate request ledger decision，并输出 apply 后 `missionBrief`；`missionBrief.authorizedGates` 会直接显示 requested budget、authorized output paths、stop conditions、gate event id 与可复制 report contract command；对已授权 gate 可先用 `-ExecutionReportContract -GateEventId <authorized-gate-event-id> -Format json` 读取只读 adapter execution report contract（在 case-local / authorized output workspace cwd 中可省略 `-Target`），供 lane executor / tool adapter 在执行前先看 compact `reportSummary` 判断 state、report/default path、validation/record readiness、current action、repair/main-escalation flags、allowed counts 与 no-heavy/no-authority boundary，再按需消费完整 action、budget、output paths、default report path、status、stop conditions、boundary/escalation requirements、validation failure taxonomy / `validationRepairHints[]`、`liveValidation` handoff（`authorizedWorkspaces[]`、`reportFileName`、`caseRelativeReportPath`、sidecar template、workspace-relative 与 case-relative validate/record command strings + args、从 pack `tooling/catalog.yml` 投影的 `adapterCandidates[]`、默认 `selectedAdapter` / sidecar `adapterId` guidance、replay behavior；record handoff 运行前需替换 `<executor-id>`，重复 `RecordArgs` / `CaseRelativeRecordArgs` 返回 `duplicate eventId` 且不追加 observations）和 sidecar 规则；adapter 写出 sidecar 后，可用 `-ValidateExecutionReport -GateEventId <authorized-gate-event-id> -ExecutionReportPath <path> -Format json` 做只读 strict validation preflight（在 case-local / authorized output workspace cwd 中可省略 `-Target`，report path 可相对当前 workspace），输出 `isMutation=false` / `applied=false` 且 `valid=true` 或 `valid=false` envelope；validation JSON/text 同样投影 compact `reportSummary`，让替换 executor 直接看到 valid/recordReady/recordBlocked、repair hints counts、report status/adapter id/actualBudget、refs/boundary hits、failure code/stage 与下一条安全命令，并在可用时携带 `adapterContext.candidates[]` / `adapterContext.selected` 且 text 输出 adapter candidate / selected adapter provenance（invalid sidecar 含 `error`、`errors[]`、`failureCode`、`failureStage`、带 `evidence[]` / `boundary[]` 的 `repairHints[]`、`reportPath`、可用时的 partial report 与 contract boundaries），且不写 observations ledger；传入 `-GateEventId <authorized-gate-event-id>` 与 `-ExecutionStatus` 时改为记录授权动作后的 observation execution evidence 到 `.rekit/facts/observations.jsonl`，包含 actual budget、output refs、evidence refs、boundary hits 与 escalation；也可传入 `-ExecutionReportPath` 读取 lane executor / tool adapter 写在 authorized output paths 下的 bounded `adapter-execution-report` JSON sidecar，并可在 case-local / authorized output workspace cwd 中省略 `-Target`、用当前 workspace 相对 sidecar path 记录 evidence，重复记录同一 sidecar 返回 `duplicate eventId` 且不重复 append observations，校验 action/status/gateEventId/budget/refs/boundary/escalation 后嵌入 evidence，并在 `execution.adapterContext` 中保留匹配到的 concrete tooling candidate；output refs、evidence refs 与 report path 必须落在 authorized gate 的 output paths 内；sidecar 若声明 `boundary-hit` / `escalated` 或实际预算越界，必须自带 `boundaryHits` 或 `escalation` marker，`boundaryHits` 必须被本次 authorized gate `stopConditions` 覆盖，`failed` / `boundary-hit` / `escalated` / `aborted` sidecar 必须包含 bounded summary；durable lane autonomy profile 完全覆盖时可记录 `authorized-gate`，并在 overview、handoff、continue digest/status 与 `missionBrief.authorizedGates` 中可见；否则 fail-closed 为 pending/denied decision；实际 heavy-tool 仍由 lane executor / tool adapter 在授权范围内执行，`/rekit` 只记录 request/evidence，不写 confirmed/authority。 |
| `/rekit sync` | kit -> case | 默认生成同步审查包；确认后才用 `-Apply` 写入 managed docs / managed block。 |
| `/rekit promote` | case -> kit | 默认生成回流审查包；确认后才用 `-CreateCandidates` 生成候选或用 `-Apply` 写回 pack。 |
| `/rekit doctor` | 只读 | 排障时详细验证结构；日常不必主动运行；维护自动化可用 `-Format json` 消费验证 rows。 |
| `/rekit repair` | case metadata | 迁移目录后先预览修复；确认后由 Claude 调用 backend `-Apply`。 |

`validate` 和 `plan-subagents` 仍是 backend/内部命令，不是日常主入口；`plan-subagents` planning mode 默认经 Go façade 生成 review artifacts，reviewer-intake mode 由主 Agent显式执行 strict WhatIf/Apply 与 post-validation，但 runtime 不自动 spawn agent；`packs` 是维护者/排障入口，用于多 pack 发现和矩阵验证；`note -List` 文本/table/tsv 与 `note -List -Format json` 默认经 Go façade 只读查询 ledger events；`note -WhatIf` JSON envelope 输出当前 `executorAction` 与内存模拟 append 后的 `wouldExecutorAction`，实际 append 输出写入后的 `executorAction`，duplicate eventId 只返回未改变的当前 action 且不写 ledger。note 仍只写 facts JSONL 或预览，不写 authority/confirmed。

## 日常工作流

### 1. 看当前项目状态

```text
/rekit overview
```

它会展示：

- 当前主线和功能支线；
- 共享事实、request、candidate、publication 统计；
- Mission Control brief：ready/blocked lanes、pending gates、authorized gates（含 execution boundaries、eventId 与 report contract handoff）、open decisions、interventions、next agent actions 与 escalations；
- 逐 lane executor action index：blocked/ready、pending gate / open intervention / open decision counts、requirements、resume/handoff command 与 current executor / generation / last takeover 摘要；
- blocker-aware 下一步：`start` / `handoff` / `continue` / `reconcile` 的 apply JSON、text 与 handoff Markdown 使用 lane-local executor actions；blocked lane 只建议 reconcile / pending gate / open decision 处理，ready lane 才建议自己的 continue，paused/closed/unready lane 回到 handoff/read-only；
- 未决 candidate、pending-gate、authorized-gate、最近 verification / decision 等 review loop 摘要；
- 需要人工确认的事项；
- 推荐下一步。

### 2. 选择并继续某条工作线

```text
/rekit continue main
/rekit continue login
```

`overview` 只是项目总览，不代表当前会话已经选择主线或支线。多条 open 工作线时，无参数 `/rekit continue` 只会列出选择，不会盲目推进。需要自动化预览时可先运行 `/rekit continue login -WhatIf -Format json` 获取只读计划和结构化 `missionBrief`。明确选择后，它会自动整理对应工作线的 case-local 状态：收集 outbox/workspace 事件、发布低风险共享事实、路由 request、验证候选并刷新接续提示；显式 `-Apply` 后 JSON、run status 与 digest 会记录 apply 后 Mission Control brief，便于 lane executor 直接判断 open decision / pending gate / authorized gate / intervention 状态。

安全边界：candidate 同时满足 evidence、accepted verifier、confidence 阈值、CSV schema、无冲突、backup、diff、max rows 时，只代表可进入 authority review。`continue -Apply` 不写 authority/confirmed；authority 写入、覆盖/删除、冲突、schema change、外部副作用或破坏性动作必须经过独立 gate 和显式用户确认。

### 3. 开一个功能支线

```text
/rekit start login
```

这会创建一个功能支线，例如 `feature-login`。需要自动化预览时可先运行 `/rekit start login -WhatIf -Format json` 获取只读写入计划和当前 `missionBrief`；显式 `/rekit start login -Apply` 由 Go façade 创建或进入工作线并输出含 apply 后 `missionBrief` 的 JSON envelope。若当前 Claude Code 会话要登记为该 lane 的 executor，主 Agent可在 start apply 时传 `-Executor <session-id> -Actor <actor> -Reason <reason>`；runtime 只记录 `currentExecutor` / `executorGeneration` / takeover metadata 并刷新 RESUME、checkpoint、board、overview 和 handoff，不负责创建、停止或监控会话。功能支线用于专项分析、证据收集、候选结论和 request；它默认不能写 confirmed CSV、`routine_ir.*` 或 `references/vmp-re/task-handoff.md`。

主线/支线不是级别高低，而是写入权限不同：

| 类型 | 职责 | 可写 |
|---|---|---|
| 主线 | 维护最终结论、验证和长期 handoff | canonical 文件 |
| 功能支线 | 分析某个功能、收集证据、提出候选和 request | 自己的 workspace |

### 4. 换新会话

项目级索引用：

```text
/rekit handoff
```

指定工作线接手文档用：

```text
/rekit handoff main
/rekit handoff login
```

显式 `/rekit handoff ... -Apply` 由 Go façade 写入 handoff 文件并输出 JSON envelope；不带 `-Apply` 的文本命令也由 façade 明确请求 Go text output，不再进入 PowerShell fallback。

它会生成：

```text
.rekit/handovers/latest.md                 # 项目级索引
.rekit/handovers/devirt-main-latest.md     # 主线接手
.rekit/handovers/feature-login-latest.md   # 功能支线接手
```

新会话应先明确接哪条工作线，例如：

```text
按 .rekit/handovers/devirt-main-latest.md 接手，然后 /rekit continue main。
```

工作线接手文档会附带本工作线的 workspace packet、最近 verification、decision、pending-gate、authorized-gate、intervention 和 rollback 摘要，便于新会话看到 reviewer verdict、main decision 与 durable autonomy gate decision 的状态。

这些接手文档只引用 `references/vmp-re/task-handoff.md`，不会覆盖它。

### 5. 同步模板更新到当前 case

在 case 里：

```text
/rekit sync
```

默认只生成 `.rekit/reviews/<timestamp>-sync/packet.json`、`summary.md` 和 bounded diff。Claude 复核冲突与收益、你确认具体范围后，才执行写入型同步（backend 为 `sync -Apply`）。

写入型同步会同步：

```text
references/vmp-re/README.md
references/vmp-re/agent-driven-re.md
references/vmp-re/workflow-template.md
references/vmp-re/progressive-disclosure.md
references/vmp-re/toolchain-router.md
references/vmp-re/singleton-handler-review.md
references/vmp-re/lane-collaboration.md
CLAUDE.local.md 中的 managed router block
```

不会覆盖：

```text
references/vmp-re/task-handoff.md
tools.local.yml
captures/**
artifacts/**
CLAUDE.local.md 中 block 外的 case 私有内容
```

默认 `sync -Apply` 不会覆盖已存在的本地 template files；只有显式 `-Force` 才会在写入前备份并覆盖 manifest 声明的本地模板目标。

`sync` 只允许作用于已经 `attach/init` 的 case。若目标目录拼错或还未绑定，会直接失败，不会静默创建假 case。

### 6. 回流可复用经验

在 case 里：

```text
/rekit promote
```

默认只生成 `.rekit/reviews/<timestamp>-promote/packet.json`、`summary.md`、bounded diff 和安全的脱敏 preview。Claude 复核后，你再选择明确写入动作：

1. `-CreateCandidates`：生成 managed docs 候选或 tooling candidate。
2. `-Apply`：按已确认内容写回 pack。

`-CreateCandidates` 的 JSON `reviewPlan.reviewSummary` 与文本 `promote candidates review summary...` 会先给出 candidate/tooling/index、review/cleanup/reconsume artifact、Mission Commander next action、no-merge/no-cleanup/no-heavy/no-authority boundary，以及 terminal `proofSummary`：expected proof total/present/missing、decision/cleanup/reconsume missing counts、proof progress、current stage、next missing proof type/path/candidate/pack target 与 proof boundary。`status` / `release-check` 的 pack-memory handoff 也输出 compact review/proof summary、expected proof paths 与 present/missing counts，便于 replacement executor 刚预览或生成候选后不扫描完整候选列表、`reviewArtifacts[]` 或 proof 目录，就判断是否还需先补 decision proof、cleanup proof 或 reconsume proof。

`promote` 很保守：若 managed docs 含真实绝对路径、样本名、RVA/VA、ctx/round 快照、artifact/capture/trace/dump 路径，会阻止直接回流。工具链经验只有在脱敏后不再命中 deny pattern 时才写 sanitized preview；候选由你审查后合入正式 tooling 文档。合入 `tooling/catalog.yml` 或 `tooling/recipes/*` 后，后续 init/attached fresh case 通过 `templateRoot` + `templatePack` 读取同一 pack tooling 资产；`sync` 不把 tooling files 复制进 case managed docs，避免把候选经验静默混入 case 私有路由。

`promote` 只允许作用于已经 `attach/init` 的 case，避免从普通目录误回流到 pack。

## 内部状态模型

日常不用理解这些文件，但排障或 review 时可能会看到：

| 路径 | 内容 |
|---|---|
| `.rekit/board.json` | 项目概览的机器状态。 |
| `.rekit/lanes/<id>/` | 每条工作线的事件、任务、inbox/outbox、`prompts/RESUME.md` 与 `checkpoints/latest.json`；resume/checkpoint 会记录 lane-local pending gates 与非阻塞 authorized gates，便于替换 executor 接手。 |
| `.rekit/facts/*.jsonl` | append-only ledger：observation、hypothesis、candidate、verification、decision、intervention、rollback、publication、request；Go runtime 通过 shared facts path/read/append helper 访问，避免各命令自行拼路径、读取或追加 JSONL。 |
| `.rekit/runs/<run-id>/digest.md` | `/rekit continue` 每轮摘要，记录 inputs、route、packet refs、Mission Control brief、outputs、decisions、open risks；存在 execution evidence review 时直接列出 follow-through outcome 的 `when` / `evidence`，供 replacement executor 从 digest 接手。 |
| `.rekit/handovers/latest.md` | 项目级接手索引。 |
| `.rekit/handovers/<laneId>-latest.md` | 指定工作线接手文档。 |

字段名中仍保留 `lane`，这是内部 schema 名称；用户层统一称“工作线 / 主线 / 功能支线”。

## 高级/内部：子 agent 分片计划

`/rekit plan-subagents` 是内部 tactical reviewer planning/intake 入口。planning mode 按 manifest route 生成 `packet.json`、`summary.md`、read-only shard handoff、strict reviewer result contract、owner binding 与 writeback guidance；它不自动启动 reviewer。planning result / packet 的 `reviewerOrchestration.summary` 以及 terminal / `summary.md` lines 会直接给出 mode、target lane、reviewer/dispatch counts、owner binding、intakeAvailable/dispatchOnly、Mission Commander queue counts、first dispatch、current action、next actions 与 no-spawn/no-heavy/no-authority boundary，让 replacement executor 不必解析 nested dispatches / lifecycle / action queue 才能接续。reviewer 产出单个 contract-compliant JSON object（包含主 Agent提供的 `reviewerSession`）后，主 Agent显式传入 `PacketPath`、`ReviewerResultPath`、`Lane` 与 `Actor`，先用 WhatIf 校验 packet/route/shard/items、route output、evidence refs、conflicts、blocked actions 与 lane executor owner binding，再用 Apply 按 verification-before-decision 顺序写 case-local facts；reviewer intake JSON / text 的 `summary` 会直接压缩 status、dispatch progress、compact `orchestrationProgress`（dispatch index/total、completed/open、current/next/remaining shards）、blocked/repair counts、compact `repairGuidanceSummary`、postValidation totals、reviewer writeback count、compact `reviewerWritebackSummary`、current/next actions 与 no-heavy/no-authority boundary。写回后的 downstream status/handoff/continue、lane `RESUME.md`、checkpoint 与 digest 还会输出 compact `reviewerWritebackSummary` / reviewer writeback summary lines，直接汇总 verification/decision counts、latest shard/reviewer session/result/packet/route、owner binding / risks / conflicts / route output flags、latest evidence refs 与 no-heavy/no-authority/no-spawn boundary，replacement executor 不必逐条扫描 `reviewerWritebacks[]` 才能复核 reviewer provenance。deterministic event IDs 支持相同 intake 的安全重试。写后返回完整 overview、lane handoff 与 doctor validation，并在 `postValidation.summary` / text summary lines 中直接给出 verification/decision totals、doctor row count、lane/executor action state、reviewer writeback count、compact `reviewerWritebackSummary`、current action、next actions 与 no-heavy/no-authority boundary。PowerShell fallback 已退休，即使设置 `REKIT_GO_DISABLE=1` 也不会回落到 PowerShell 业务实现。

在 reviewer result 写回前，case `status`、`handoff`、`continue`、continue run `status.json` / `digest.md`、lane `RESUME.md` 与 checkpoint 也会投影 open `reviewerDispatchIntakeHandoffs` / compact summary：直接显示每个 shard 的 reviewer result 目标路径、dispatch command、WhatIf/Apply intake command、waiting / intake-ready / dispatch-only state、packet-level progress（dispatchTotal / completed / open / nextOpen / remaining）与 no-spawn/no-heavy/no-authority boundary，便于 replacement executor 不打开完整 `packet.json` / `summary.md` 或扫描 reviewer writebacks 即可从 live command 或 durable artifact 接续。

生成的 `packet.json` / `summary.md` 会标出 route 选择原因、目标 lane 的 `ownerBinding`（current executor / generation / last takeover snapshot）、每个 shard 的初始 `planned` 状态、`shardHandoffs[]` read-only dispatch prompt、spawn/merge 责任、`reviewerResultContract`、`intakeChecklist[]`、`reviewerDecisionMappings[]`、`conflictHandling[]`、`reviewerIntakeCommands`、`writebackSequence[]` / `commandBindings[]` 和 post-review merge guidance。若 packet 要求 owner binding 且当前 lane executor/generation 已被 takeover，reviewer intake 会在写 facts 前 fail-closed；planning 阶段的 `packet.json` / `summary.md` / text handoff 会通过 `reviewerIntakeCommands.repairGuidance[]` 预先列出会导致 blocked intake 的原因、修复动作、证据字段与 no-apply / no-heavy / no-authority 边界；blocked / event-id-collision / post-validation failed intake 也会返回结构化 `repairGuidance[]`，top-level `summary.repairGuidanceSummary` 与 terminal text 会直接给出 total、primary reason/action、evidence、boundary 和下一条 safe command，让主 Agent 不必解析完整 JSON 或人工拼 blocked reason 才能修复后重跑 WhatIf。verification 与 decision events 会记录 `reviewerSession`、`ownerExecutor`、`ownerGeneration`、`ownerBindingMode`、`ownerBindingTarget`、reviewer `decision` / `recommendedVerdict`、`risks[]` / `conflicts[]` 与 normalized `routeOutput`；这些字段也会通过 downstream `reviewerWritebacks[]`、status/overview/handoff/continue、lane `RESUME.md`、checkpoint 与 digest 投影，replacement executor 不必重开 reviewer result JSON 即可复核 reviewer provenance。evidence-ref validation 只证明引用为 packet ID、已知 ledger event ID 或存在的 case-local 文件，证据内容是否足够仍由主 Agent审查。reviewer 本身不写文件或 ledger；runtime 不自动 spawn agent、不写 authority/confirmed、不执行 heavy-tool，也不修改 managed docs 或项目源文件；`sync`/`promote` 继续 review-first。

## 工具经验保存在哪里

工具经验不会只留在当前 case。现在分两层：

| 层级 | 路径 | 内容 |
|---|---|---|
| 通用 tooling 资产 | `packs/vmp-re/tooling/` | 工具 catalog、recipes、脚本模板化清单、补丁/止损经验；fresh case 通过 pack reference/tooling 路径重新消费，不复制成 case-local managed docs。 |
| 当前 case 状态 | `<caseRoot>/references/vmp-re/toolchain-router.md` | 当前样本具体脚本、路径、工具结论和状态。 |

通用 tooling 资产包括：

```text
packs/vmp-re/tooling/catalog.yml
packs/vmp-re/tooling/recipes/public-tool-triage.md
packs/vmp-re/tooling/recipes/lane-collaboration.md
packs/vmp-re/tooling/recipes/vmenter-context-probe.md
packs/vmp-re/tooling/recipes/unicorn-trace.md
packs/vmp-re/tooling/recipes/focused-handler-review.md
packs/vmp-re/tooling/recipes/value-flow-mining.md
packs/vmp-re/tooling/recipes/ida-x64dbg-mcp.md
packs/vmp-re/tooling/scripts/README.md
packs/vmp-re/tooling/patches/vmpimportfixer-timeout-and-quiet-log.md
```

原则：具体样本名、RVA、ctx、coverage 留在 case；可复用工具路线、脚本接口、短测/止损经验进 tooling。

## 迁移已有 case 到新目录

推荐流程：**先复制，确认修复 metadata，再验证新目录，最后归档旧目录**。

### 1. 复制 case 目录

先关闭正在使用该 case 的 Claude Code、IDA、x64dbg、trace 脚本等进程：

```text
robocopy <oldCaseRoot> <newCaseRoot> /E
```

### 2. 在新目录检查状态

```text
cd <newCaseRoot>
claude
```

然后：

```text
/rekit status
```

如果 `.rekit/instance.yml` 里的旧 `projectRoot` 和当前目录不一致，`status` 只会提示，不会静默修改。

### 3. 确认后修复 metadata

确认这是你预期的迁移后：

```text
/rekit repair
```

`repair` 默认只预览。需要写入时，直接告诉 Claude：

```text
确认修复，执行 repair -Apply
```

`repair -Apply` 会更新：

```text
.rekit/instance.yml
.re-template.yml
.claude/skills/rekit/SKILL.md
```

### 4. 排障验证

```text
/rekit doctor
```

### 5. 检查旧绝对路径

迁移后还要搜索只属于旧 case 根目录的绝对路径：

```text
CLAUDE.local.md
.re-template.yml
references/vmp-re/task-handoff.md
自写脚本中的 PROJECT_ROOT / workdir / output path
```

目标样本路径如果没有变化，不需要改。

## 后端脚本什么时候用

正常情况下不用。

这些入口只是为了自动化、按需 CI、排障或旧流程兼容：

```text
cmd/rekit/main.go                  # Go-native backend CLI，CI workflow / 维护自动化入口
rekit/rekit.ps1                    # retained compatibility façade；无业务 runtime、无 PowerShell fallback，默认 CI 不依赖
rekit/tests/README.md              # smoke 维护指南与验证选择表
rekit/tests/catalog.json            # smoke 机器可读导航目录（非自动执行器）
rekit/tests/catalog-smoke.ps1       # smoke catalog 输出契约自测
rekit/tests/facade-smoke.ps1       # façade 委托回归 smoke
rekit/tests/pack-smoke-lib.ps1     # 多安全领域 pack smoke 共享 helper
rekit/tests/pack-smoke-matrix.ps1  # 多安全领域 pack smoke 串行矩阵 runner，支持 -Format json 与 -DiscoveryOnly
rekit/tests/pack-smoke-matrix-selftest.ps1 # pack smoke matrix 输出契约自测
packs/vmp-re/scripts/bootstrap.ps1
packs/vmp-re/scripts/update.ps1
packs/vmp-re/scripts/validate.ps1
packs/vmp-re/scripts/promote.ps1
```

如果 README 前面能用 `/rekit` 表达，就不要让用户手动跑脚本。

## 架构边界

- `/rekit` 是用户入口。
- `rekit/rekit.ps1` 是 retained PowerShell compatibility façade，只负责参数兼容、Go delegation、no-fallback guard 与错误透传；它不承载业务 runtime，也没有 PowerShell 业务 fallback。
- Go backend 位于 `cmd/rekit/**` 与 `internal/rekit/**`；低风险只读命令 `status`、`packs`、`doctor/validate`，attached case 的 `overview` 文本/JSON 与缺 board 时的 case-local board/facts/policy/default authority lane 初始化，`note -List` 文本/table/tsv/JSON 只读查询，attached case 的 note append / `note -WhatIf` facts JSONL 写入或预览，`gate -WhatIf` 非写入 heavy-action authorization preflight，`gate -Apply` pending-gate / authorized-gate request ledger decision 写入，attached case 的 `start` / `handoff` JSON preview、explicit apply、文本 preview 与 bare/default 工作线 flow，`continue` JSON preview、explicit apply 与文本/default preview 的 case-local facts/routing/run digest/lane resume/checkpoint/board safe subset（存在 effective open intervention 时 fail-closed，需先 `reconcile`），边界清晰的 case lifecycle 命令 `attach`、`repair`、`init/bootstrap` 的预览与显式 `-Apply`，`/rekit sync` review、`sync -Apply` 实际写入和 `sync -Apply -WhatIf -Format json` 非写入预览，以及 `/rekit promote` review、review artifact 写入、promote `-CreateCandidates` 实际候选写入、promote `-CreateCandidates -WhatIf -Format json` 非写入预览、promote `-Apply` 实际 pack source 写入和 promote `-Apply -WhatIf -Format json` 非写入预览、`reconcile` 显式 resolution 写入与 lane executor/resume/checkpoint/board 刷新默认委托 Go。`release-check`、`status`、`packs`、`doctor/validate`、`attach/repair/init/bootstrap`、`sync/update`、`promote`、`overview`、`note`、`gate`、`start`、`handoff`、`continue`、`reconcile` 与 `plan-subagents` 已 no-fallback；`REKIT_GO_DISABLE=1` 不再让 Go-default command rows 回落到 PowerShell 业务实现。`plan-subagents` review artifacts 只写 review packet/summary/combined diff 路径、不自动 spawn agent；actual heavy-tool 执行、authority/confirmed 写入和非 note/gate/continue/reconcile apply 的其它 ledger 写入命令仍不由默认 Go façade执行；文本 `sync -Apply -WhatIf`、文本 promote what-if、case lifecycle fallback 与 workstream fallback 已 no-fallback；Go `continue -Apply` 存在 effective open intervention 时 fail-closed 并要求先 `reconcile`，且不写 authority/confirmed；`gate -Apply` 只记录授权决策，不执行 heavy-tool、不写 authority/confirmed；authority/confirmed 仍需显式用户确认。
- legacy `rekit/lib/B3.*.ps1` 工作流 runtime 已在 Batch 240 删除；工作线、ledger、gate、handoff 与 Mission Control 状态以 Go-owned `internal/rekit/**` runtime 为准。
- `packs/<pack>/manifest.yml` 是 managed/local/tooling/budget/promote 规则的单一事实源。
- case-local `.claude/skills/rekit/SKILL.md` 只是 thin shim，不维护业务逻辑。
- `.re-template.yml` 只保留兼容旧入口；新状态看 `.rekit/instance.yml`。
- 不默认安装用户级 skill。
- 不默认 commit / push；只有当前用户 goal/session 明确授权具体仓库和分支时才执行。已授权 batch 正常最多两次 push：implementation commit 覆盖代码、测试、文档、本地验证；release inspection commit 只记录 implementation commit 的远程 run。不要为 release inspection commit 自己触发的 CI 再追加第三个记录提交，除非出现不同于既有 `steps=[]` runner/billing blocker 的新信号。
