# re-context-kits

`re-context-kits` 是面向网络安全研究与安全工程任务的 Claude Code **Agent Team Mission Control** 框架，用于把主 Agent 统筹、长期 member lane、可替换 Claude Code session executor、短命 tactical subagent、领域工具链、证据账本、验证门禁和可复用安全领域 pack 组织成可持续迭代的 case workspace。

当前阶段，它已经提供 `/rekit` case 管理、首个成熟 pack `vmp-re`、安全领域 pack 骨架 `web-security`、`malware-analysis`、`vuln-research`、`ctf`、`unpack-pe`、`ollvm`、`android-native` 与 `generic-binary-re`、工作线协同、handoff、sync/promote 和 tooling 经验沉淀；`vmp-re` 是验证框架的第一个重点领域，不是最终边界。长期目标是逐步扩展到逆向工程、恶意样本分析、漏洞研究、Web/API 安全评估、授权测试/靶场/CTF、Android native、OLLVM 等多类安全任务。

当前项目不是全自动脱壳器、自动逆向引擎、自动漏洞挖掘器、自动恶意样本分析平台或通用自动渗透平台；它优先提供可审计、可交接、review-first 的 Agent Team 底座。lane 文档/packet 只能表达授权意图；heavy action 的确定性预授权来自 strict durable autonomy profile 与 `authorized-gate` decision。

一句话：**用户主要指挥主 Agent / Mission Commander；主 Agent 把“开始 case”的自然语言显式收敛为 `Target` / `Pack` / `ProjectName` / opaque bounded `Goal` / `Actor` / `Executor` / `InitialLane`，再通过 public Go-owned/no-fallback `onboard` 发布 immutable mission intent；随后调度 durable member lanes、可替换会话执行体和短命 tactical subagents。`/rekit`、Go CLI/backend 是背后的 canonical deterministic runtime/API，`rekit.ps1` 仅作为 retained compatibility façade，不承载业务 runtime，也没有 PowerShell 业务 fallback。当前阶段先把骨架收敛成可真实日常使用的最低可用 Mission Control：开始 case、继续推进、状态总览、人工插手纠偏、新会话接手要顺畅、可记录、可恢复；外部 session harness可把strict observation写入canonical case-local inbox，fresh `status`只在存在唯一checkpoint-bound候选时给出可执行WhatIf接力，歧义或无效候选均fail-closed，陈旧候选只计数且不参与选择，成功Apply返回并由successor checkpoint保留one-shot processed receipt。底层 `status`、`overview`、handoff、`continue` artifacts、lane-local `RESUME.md`、typed checkpoint、reviewer/session handoff、authorized-gate adapter live validation 与 pack-memory flow继续作为支撑，边用边增强，而不是让用户在多份 JSON/Markdown 或命令细节间手工拼接。默认路径继续向 PowerShell-free / Go-native / 跨平台收敛，并保持 truthful release readiness。**

## 项目路线（按需文档索引）

以下是文档索引，不是默认必读清单。新会话、上下文压缩后接手或维护文档时，先在 `main` 分支确认 `main` 与 `origin/main` 同步且工作树干净，再读 `docs/context-routing.md`，并按场景只读对应顶部区。

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
- 发布门禁与当前 release readiness checklist：`docs/release-readiness.md`（机器可读 inventory 与 release handoff：`go run ./cmd/rekit -- -Command release-check -Format json`；三平台 Go-native workflow 定义：`.github/workflows/release-gate.yml`；inventory ready 不等于远程 jobs 已实际运行并通过；`commitRefs[]` 只记录 implementation commit refs，远程 jobs `steps=[]` / `steps 为空` 会通过 `remoteReleaseGateDetail` 记录 run/job/boundary，并按 runner/billing blocker 而不是 remote CI green 处理）
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

然后直接用自然语言告诉主 Agent：

```text
开始这个 case，目标是还原核心逻辑；使用 vmp-re pack，由当前 Mission Commander 会话接手主线。
```

主 Agent 会先把意图显式收敛为 `Target`、`Pack`、`ProjectName`、opaque bounded `Goal`、`Actor`、`Executor` 与 `InitialLane`，再运行：

```text
/rekit onboard -Target <workspaceRoot>\cases\<caseName> -Pack vmp-re -ProjectName <caseName> -Goal <opaqueGoal> -Actor <actor> -Executor <executor> -InitialLane <lane> -WhatIf -Format json
```

preview 是零写入的 immutable mission-intent 审查包，返回 exact `publicationStamp`、`onboardingPlanSha256` 和机器可读 `applyArgs[]`。主 Agent 复核后必须原样消费 `applyArgs[]`；不能手工重建或更换 identity/stamp/hash。Marker、hash 与 exact plan 来自同一 immutable ordinary snapshot，Apply 按 intent-first / commit-last 发布；intent 已发布后的 partial recovery 只消费受 canonical onboarding write contract 约束的 durable bounded exact envelope 与同一组参数，不重新读取可能已变化的 live kit/pack，也不能夹带 authority/confirmed、lane/board/ledger、heavy-tool 或伪造 case-local skill 写入；已 committed 的 exact replay 不重复写入。提交后先刷新 `/rekit status`，再执行 `/rekit overview`，最后按 committed `InitialLane` / `Executor` / `Actor` 运行 `/rekit start <lane>` 接手。

已有 case 仍可用兼容入口接入：

```text
/rekit attach -Target <workspaceRoot>\cases\<caseName> -Pack vmp-re -Apply
```

> `onboard` 不解析自然语言、不执行 heavy-tool、不写 authority/confirmed，也不 spawn、poll 或管理 session；自然语言到显式字段的收敛与 applyArgs 审核由主 Agent完成。这里不需要你手动执行底层脚本。

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

主 Agent 会把这些意图翻译成 `/rekit overview`、`continue`、`start`、`handoff`、`gate`、`note`、`sync`、`promote` 等底层 runtime 操作。claimed lane需要实际工作时，Mission Commander会先返回durable external member handoff，让外部Agent在限定result路径写bounded outputs与strict manifest；Go只记录`accepted/returned/failed` observation并验证owner generation与output hashes，验证通过后才进入既有continue→reviewer→complete路线。`run-current-loop`遇到member或reviewer外部会话边界时会保存剩余预算和strict checkpoint；status、replacement takeover与显式handoff统一给出绑定checkpoint及attempt的下一次external job。外部harness按result-first/submission-last返回后，status默认给出一次reviewed external-result turn，单次hash-bound Apply串联relay、strict observation intake、one-shot checkpoint claim与bounded resume；relay-only入口只用于恢复/诊断。member accepted后只允许returned或failed，returned进入intake-ready后不再暴露旧handoff。外部harness仍负责session生命周期，Go不spawn/poll/stop session。用户不需要把 `/rekit` 子命令当成主要交互界面；它们主要是主 Agent、维护者、自动化和排障使用的确定性 API。

排障或新会话接手时再用：

```text
/rekit          # 无子命令时默认只读 status，先确认 kit/case 绑定，并先看 compact Mission Commander first-screen strip / focus routing / queue current action 接手；kit-mode 会先投影 latest-batch project current action 与 project runbook，pack-memory focus 会先显示 evidence shortlist（含 next missing proof draft WhatIf / ExpectedProofSha256 Apply template），再回到 case/reviewer/pack-memory first-screen strip，空 case mission 会先投影 onboarding current action。status 顶层还会输出统一 missionControlRunbook，把 first-screen focus、routingReasons、focused queue/currentDriverRequest、currentDriverReceipt、replacementExecutorTakeoverPackage、handoffPreviewCommand / handoffApplyCommand 与 refresh 节奏放在单一机器可读入口中；currentDriverReceipt 只记录本次 status refresh 后的 refreshedCurrentDriverRequest 与 queue summary，不证明 Go runtime 已执行原 request；replacementExecutorTakeoverPackage 会把 currentDriverRequest、targetDocuments、runbookSteps 与 boundary 收敛成无旧聊天上下文也可消费的只读接手包；project/lane handoff JSON 与 durable Markdown 也会输出同源 replacementExecutorTakeoverPackage，指向 handover/currentDriverRequest/target documents；若已有成功current-loop segment，status与explicit handoff JSON还会strict检查并投影`currentLoopSegment`，仅`state=ready`携带可恢复的typed continuation/remaining budget，stale/invalid/terminal/status-unavailable均不暴露continuation；guidance-only current action 会附带只读 guidanceHandoff，给 replacement harness 明确 target documents、acceptance checklist、expected receipt 与 next-batch starter/candidate package；implementation-pending 本机验证 action 会暴露 `/rekit release-run -Format json` Git-local receipt driver request；从 kit CWD 用 `status -Target <case>` 接手时，顶层 runbook 会把可执行 case request 约束为带 `-Target` 的 `-WhatIf -Format json` 预览，并把 handoff preview 约束为 `/rekit handoff -Target <case> -WhatIf -Format json`，先预览再刷新/显式 Apply。case mission 继续输出 daily Mission Control runbook，把 inspect status、consume currentDriverRequest、refresh after result、preview handoff 与 apply handoff 的只读接手节奏放在同一个 envelope 中。
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

所以第一次需要在 kit 仓库里使用 canonical `/rekit`：新 Mission Control case 走 review-first `onboard`，已有 case 走 `attach`；`init` 仅保留为兼容/维护初始化入口。

完成后，case 里会有 thin shim。以后你在 case 根目录或其下的 lane/workspace 子目录启动 Claude Code，也能直接使用 `/rekit`；未显式传 `-Target` 时，Go runtime 会向上寻找最近的 attached case root，再用 metadata 中的 `templateRoot` 定位 canonical runtime。

## Runtime/API 命令参考

这些命令是主 Agent 和维护者使用的确定性 runtime API，不是最终产品的主要 UX。普通日常使用优先通过自然语言让主 Agent 选择和组合这些动作。Mission Commander `currentDriverRequest.expectedReceipt` 会保留当前应执行/预览的 `command`，并在 status/handoff/daily runbook 与 replacement takeover package 中提供同源 `refreshStatusCommand`；接手者执行 driver 后应直接运行该 refresh command 重建 durable state，不再从相邻文本手工拼接。

Adapter execution report lifecycle 的 contract、dispatch、scaffold、draft、validation、receipt、record 与 status/handoff 投影会输出 `runbookSteps[]` 或对应 text runbook 行；replacement executor 应优先按这些步骤确认 state/path/hash。durable lane 且当前 action 有 tooling catalog candidate 时，先按 `liveValidation.dispatchCommand` 记录 immutable dispatch，再等外部 adapter/harness 写出 bounded report；contract、validation、`status`、`overview`、project/lane `handoff` 与 durable Markdown 会共享 `currentRunLoopStepId` / `runLoop`，按 `inspect-contract → record-dispatch → run-external-adapter → draft-or-write-report → validate-report → record-receipt → record-observation → review-recorded-evidence` 显示当前步骤、命令、owner/provenance 与 boundary。随后用 validation 返回的 receipt preview 记录 current executor generation、external harness/session、catalog/report/artifact hashes 与 outcome/exit status，再使用 validation/status 返回的 report+receipt 双 hash-bound record Apply；record 后只进入 evidence review。已记录 execution evidence 且无需 main escalation 时，`status`/`handoff`/`continue` 的 Mission Commander current action 与 `currentDriverRequest.command` 会直接指向 `acknowledgementReviewCommand`（accepted `note -Kind verification ... -WhatIf -Format json`），review 后再执行该 WhatIf 返回的 hash-bound `recordCommand` 关闭 review queue；`/rekit handoff <lane>` 仍保留为 follow-up/provenance，而不是当前 primary。installed case-local `/rekit` 可从 nested lane/workspace cwd 完成同一路径；takeover 后旧 executor/generation/session receipt 会 fail-closed，acknowledgement 后的 handoff、`RESUME.md` 与 continue digest 仍保留 receipt/owner/harness/catalog/artifact lineage。Go runtime 不执行 adapter/heavy tool，也不从 contract/report/receipt 推断 authority/confirmed。

| 命令 | 方向 | 什么时候用 |
|---|---|---|
| `/rekit status` | 只读 | 看当前 case 绑定状态、case-local thin shim / canonical skill readiness，并在 case shim 后先显示 compact Mission Commander first-screen strip；daily status使用lightweight project handoff读取latest batch、known gaps、pack-memory与validation routes，不执行PowerShell/public façade全仓release audit，完整release inventory仍由`release-check`持有；kit-mode project handoff 会同步投影 latest-batch release-run transient retry evidence / validation warning；若目录被移动或 shim drift，只提示，不修复；case 模式会投影 `pendingGateHandoffs[]` 的 review / WhatIf / request-decision boundary、`authorizedGateHandoffs[]` 的 execution report contract boundary 及 compact adapter handoff（`defaultReportPath` / `reportPath`、`reportSummary`、`liveValidation` validate/record commands、authorized workspaces、adapter candidate / selected adapter detail（entry/purpose/sideEffects/reportGuidance/evidenceGuidance/stopConditionHints）/ sidecar guidance、contract error）、`openDecisionHandoffs[]` 的 source fact/list command 与 decision note WhatIf/hash-bound recordCommand boundary（terminal `note -Kind decision -Related <candidateEventId>` 会关闭对应 open candidate blocker）、`interventionHandoffs[]` 的 reconcile WhatIf/Apply boundary，以及已记录 authorized execution observation evidence 的 compact `executionEvidenceReviewSummary`（ready/main escalation/duplicate/ref/boundary/action queue summary、latest adapter context）和完整 `executionEvidenceReview[]`（含 `acknowledgementReviewCommand`、accepted/rejected acknowledgement previews、adapterContext entry/purpose/sideEffects/reportGuidance/evidenceGuidance/stopConditionHints；终结性的 related verification / decision notes 会关闭 review queue，并阻止 exact `evidence-already-recorded` current action 回流）；非 escalation 的 evidence review current driver request 会直接指向 acknowledgement note preview，不再把 `/rekit handoff <lane>` 当作当前 primary；`-Format json` 输出机器可读 status envelope 与 `caseShim` readiness。 |
| `/rekit packs` | 只读 | 维护者查看当前 kit 内所有 pack 的成熟度、schema、route、managed/tooling 和 authority lane 概览；`-Format json` 输出机器可读 inventory。 |
| `/rekit release-check` | 只读 | 维护者查看 release inventory、gateProfile、latest-batch handoff 与 CI truthfulness boundary；只枚举门禁，不执行测试。latest-batch handoff 会识别明确记录的 `release-run` 7/7 成功结果，并在 completed release cadence 后把 current action 交棒给 next-batch selection；目标、计划、待执行、失败或历史叙述不会被当作成功证据。已记录的 transient retry 仍作为 evidence / validation warning 暴露，但不替代完整本机 release minimum，也不代表远程 CI green。 |
| `/rekit release-run` | Git-local 本机验证 | 顺序执行 `release-check` 的 local release minimum，汇总 exit code / duration / attempts / output tail。成功且 latest batch 待提交时，会在 Git metadata 写 strict receipt，绑定 baseline HEAD、完整 gate profile 与当前 exact implementation artifacts；该 receipt 不进入 commit。提交后仅当 HEAD 是 baseline 后唯一 direct commit，且 changed set、mode、bytes/hash 完全匹配，status 才恢复 post-push cadence。缺失、篡改、验证后编辑或额外 commit 均 fail-closed；不写 tracked repo/case state、不查询远程 CI、不执行 heavy tool、不写 authority/confirmed。 |
| `/rekit next-batch` | kit review-first planning receipt | 接受 `status` / `release-check` 的 next-batch guidance 时使用；`-WhatIf -Format json` 预览 `docs/batch-history.md`、`CHANGELOG.md` 与 `docs/batch-plan.md` 三份 exact writes，先归档更早 active batch，再返回 `expectedNextBatchPlanSha256`；`-Apply` 必须带同一 hash，并可从已完成的 exact write prefix 幂等恢复，随后要求刷新 `/rekit status -Format json`。不触碰 case state，不执行 reviewer/adapter/pack-memory/gate/sync/promote mutation，不 commit/push，也不声明 remote CI green。 |
| `/rekit onboard` | new-case review-first mission intent | 新 case 的 public Go-owned/no-fallback Mission Control 入口。主 Agent先把自然语言显式映射为 `Target` / `Pack` / `ProjectName` / opaque bounded `Goal` / `Actor` / `Executor` / `InitialLane`；`-WhatIf -Format json` 零写入返回 immutable mission intent、exact `publicationStamp` / `onboardingPlanSha256` 与机器可读 `applyArgs[]`。`-Apply` 必须消费同一 stamp/hash 和 identity，按 intent-first / commit-last 发布；partial publication 用 exact Apply 恢复，committed exact replay 幂等。提交后先 `status`，再 `overview`，最后按 committed lane/executor/actor `start`；后续继续使用 public `note` / `reconcile` / `handoff` / `complete` / `reopen`。Runtime 不解析自然语言、不创建 board/lane、不执行 heavy-tool、不写 authority/confirmed，也不 spawn/poll session。 |
| `/rekit run-current-step` | case-local unified review-first runner | 主 Agent/harness推进当前 focused Mission Commander request 的首选单步入口。每次 `-WhatIf -Format json` 都从 refreshed `missionControlRunbook.scope/currentDriverRequest` 自动选择 case 或 reviewer route，返回对应 nested runner plan；有 deterministic nested step 时同时返回绑定 route、current request 与 nested hash 的 `expectedCurrentStepPlanSha256`，复核后用 `-Apply -ExpectedCurrentStepPlanSha256 <hash>` 执行一步并读取 refreshed receipt。reviewer spawn/result wait 等外部动作只返回 typed handoff，不生成可 Apply hash；lane/reviewer各自原有lease、packet、artifact、candidate与intake锁保持不变。runtime不调用Agent tool、不管理session、不执行heavy-tool、不写authority/confirmed；必须显式`-Target`且只支持JSON。 |
| `/rekit run-current-loop` | case-local bounded review-first loop | 主 Agent/harness在同一初始 route/lane 上连续推进最多 `-MaxSteps 1..20` 个 deterministic current steps。先运行 `-WhatIf -Format json`，再以返回的 `expectedCurrentLoopPlanSha256` 和相同 `MaxSteps` 显式 Apply；每步都刷新 durable status、重建 exact nested plan并留下 receipt。route/lane漂移、fresh Human-in-the-Lane reconcile和external reviewer stop在预算尚有剩余时都会返回统一 `kind=current-loop-campaign-continuation`：`segmentRoute/segmentLane → expectedRoute/expectedLane`、按本段已执行步数扣减后的 `remainingMaxSteps` 与fresh WhatIf command；external reviewer另带单一typed `attempt`：稳定绑定packet/route/shard/prompt/owner/current executor与dispatch receipt，提供`attemptSnapshotSha256`、唯一`selectedAction`及checkpoint-bound `durableContinuationDriverRequest`。外部harness接受session后生成绑定immutable dispatch ID的新attempt；session accepted、managed result与failed observation必须携带fresh snapshot guard，stale值在preview前zero-write拒绝。direct result按typed target外部写入后刷新到successor attempt再intake，不复用predecessor snapshot；replacement executor只消费fresh status/handoff当前attempt。成功Apply还会把单段provenance按严格递增sequence写为immutable `.rekit/runs/current-loop-segments/<sequence>.json` checkpoint，并用前一个exact artifact SHA组成无gap/fork链；新会话`status.missionControlRunbook.currentLoopSegment`与`handoff.currentLoopSegment`仅在完整chain、canonical exact bytes、case/pack、重算的outer plan/request-receipt lineage及refreshed current request完全匹配时暴露typed continuation、remaining budget和可执行`resumeDriverRequest`，wall-clock回退不改变latest，tamper、chain gap/break、symlink、unknown entry、stale request、terminal或status refresh failure均fail-closed且不回退旧artifact。主Agent/harness可直接执行该request中的`-ResumeCurrentLoop -ExpectedCurrentLoopCheckpointSha256 <artifact>` fresh WhatIf；external member/reviewer alternative同时返回checkpoint/attempt-bound `observationEnvelopeTemplate`与统一`observationPathCommand`，harness将一次accepted/returned/failed observation写入case-local symlink-free bounded strict JSON后只传`-CurrentLoopObservationPath`。Preview返回exact file SHA，Apply命令只携带path、expected observation SHA、checkpoint SHA及outer/nested hashes；bytes/checkpoint/attempt/state/capability漂移或与legacy flags混用均fail-closed。Runtime从strict checkpoint派生remaining budget并复核expected route/lane/current request。Apply再次要求同一source artifact仍是latest ready checkpoint，并在任何nested mutation前durably one-shot claim该source；claim后source立即为`consumed`，并发、崩溃、nested/publication failure均不恢复其预算。成功后的新segment checkpoint把`resumeSourceSha256`绑定到immediate predecessor；旧source/plan hash重试zero-write失败。JSON中的旧unbound WhatIf仅以`legacyUnboundWhatIfCommand`保留诊断兼容，唯一可执行/推荐恢复入口是`resumeDriverRequest.command`。旧segment plan hash与receipts不能跨界复用或累计；没有ready durable checkpoint时不能声称恢复旧预算。fresh status还会从latest ready checkpoint派生统一typed `externalSessionJob`，固定job/checkpoint/attempt/owner或reviewer packet-route-shard身份、submission-last路径与允许outcome。外部harness先写member outputs或ReviewerResult，再最后写strict `submission.json`；status随后把`selectedDriverRequest`切换为一次reviewed external-result turn：WhatIf零写入绑定exact job/submission/source/destination/relay artifacts、observation、checkpoint与nested resume hashes；Apply以exclusive no-overwrite、exact-prefix recovery顺序生成member manifest+outputs或canonical reviewer relay source、publication receipt，最后发布Batch 813兼容inbox envelope，再从durable filesystem strict intake、one-shot claim并继续bounded loop。若后半段因Human intervention或其它currentness drift拒绝，已提交relay保持truthful，checkpoint不会被误消费，fresh status可路由reconcile或relay-only recovery。无submission时旧member/reviewer handoff继续可用；invalid submission或ambiguous/invalid inbox fail-closed。managed packet把`reviewerResultDropPath`标为canonical input destination，relay生成与drop/input/source分离的case-local immutable source后仍进入既有save/completion/capture/stage/collect/intake链；无managed input-save capability的direct packet则把drop path标为direct result destination。guidance/blocker、no-progress、step limit或nested error仍停止且不扩容预算；已成功步骤不回滚。runtime不自动跨lane/route Apply、不调用Agent tool、不管理session、不执行heavy-tool、不写authority/confirmed；必须显式`-Target`且只支持JSON。 |
| `/rekit run-driver-step` | case-local review-first runner | 主 Agent/harness 消费唯一 focused、case-scoped `missionControlRunbook.currentDriverRequest` 时使用。外层 `-WhatIf -Format json` 允许当前 `start`、`continue` 或 `reconcile` preview request，直接调用对应 Go preview handler并返回 typed Apply request与 `expectedDriverStepPlanSha256`，不写 case；复核后外层 `-Apply -ExpectedDriverStepPlanSha256 <hash> -Format json` 会重新构建同一 preview plan，hash drift 时 fail-closed，只调用一个 matching Go Apply handler，随后刷新 status并返回 typed runner receipt。三类写入都会在 lane mutation lease 内重验 preview currentness；必须显式 `-Target`。runner 不调用 shell、不递归调用 public runtime、不 spawn/poll/stop session、不执行 reviewer/adapter/heavy-tool、不写 authority/confirmed，也不接受 missing-board onboarding、gate、note、handoff、sync、promote、next-batch 等 request。 |
| `/rekit run-reviewer-step` | case-local reviewer review-first runner | 主 Agent/harness 消费 `caseMission.reviewerDispatchIntakeSummary.operatorPackage.currentDriverRequest` 时使用。spawn reviewer 与 ReviewerResult JSON 生成保持外部动作：缺少真实 harness/session 或结果路径时返回 typed `externalHandoff`；提供 `-ReviewerHarness/-ReviewerSession/-Actor` 或 `-ReviewerResultInputSourcePath/-Actor` 后，runner 直接调用现有 Go reviewer preview handler，返回 typed Apply request与 `expectedReviewerStepPlanSha256`。hash-bound `-Apply` 只执行当前 dispatch receipt、result-input save、completion receipt、source capture、staging、collection 或 intake 一步，随后刷新 reviewer status并返回 receipt；同一packet仍有running/open shard时优先继续该shard，不提前batch intake，collection还会在锁内复验WhatIf返回的candidate hash。replacement executor takeover 会让旧 packet owner路径 fail-closed，显式 adoption 后才可生成新 dispatch plan。runtime 不调用 Agent tool、不 spawn/poll/stop reviewer、不伪造 reviewer output、不执行 heavy-tool、不写 authority/confirmed；必须显式 `-Target` 且只支持 JSON。 |
| `/rekit overview` | case-local 状态 | 显示项目概览、主线/支线、共享事实统计、Mission Control brief、逐 lane `laneExecutorActions[]` 和 blocker-aware 下一步建议；文本/JSON 直接展示 blocked/ready、typed blocker counts、requirements、resume/handoff command 以及 current executor / generation / last takeover 摘要，只有 ready lane 才进入 continue 建议；已记录 authorized execution observation evidence 时，同时显示 compact `executionEvidenceReviewSummary` 与完整 `executionEvidenceReview[]`（含 recorded adapter context detail），让替换 executor 先确认 ready/main escalation、duplicate、refs、latest review/handoff/current action、adapter entry/guidance/stop conditions 与 no-replay/no-authority boundary；`authorized-gate` 作为 durable autonomy 已授权决策单独展示但不阻塞 lane，并在 Mission brief 中带出 `requestedBudget`、`outputPaths`、`stopConditions`、`eventId` 与可复制的 `reportContract=/rekit gate -ExecutionReportContract -GateEventId ... -Format json` handoff；overview JSON/text 还直接投影 compact adapter handoff（`defaultReportPath` / `reportPath`、`reportSummary`、`liveValidation` validate/record commands、authorized workspaces 与 adapter sidecar guidance），让替换 executor 不必切回 status 或完整 contract 才能定位 safe validation/record handoff；缺 `.rekit/board.json` 时由 Go 初始化 case-local board/facts/policy/default authority lane；只表示总览，不代表当前会话已选择工作线。 |
| `/rekit continue main` | case-local 自动整理 | 明确接手主线并整理相关状态；多工作线时不要用无参数 `continue` 盲猜；维护自动化可用 `-WhatIf -Format json` 经默认 Go façade 消费非写入 continue 计划，JSON envelope 含结构化 `missionBrief`，其中包含 pending gates 与非阻塞 authorized gates。 |
| `/rekit continue <name>` | case-local 自动整理 | 明确接手某条功能支线，只整理该支线的 workspace/outbox 并刷新接续提示；`-WhatIf -Format json` 默认经 Go façade 预览收集、路由和 authority append 计划；显式 `-Apply` 的 JSON envelope、run `status.json` 与 `digest.md` 都包含同一 `missionBrief`，且 JSON/status/digest 会直接投影 compact authorized-gate adapter handoff、compact `executionEvidenceReviewSummary`、recorded execution evidence adapter context 以及 execution evidence review follow-through outcome 的 `when` / `evidence`，让 lane executor 直接看到 pending gates、非阻塞 authorized gates、adapter report validation/record handoff、adapter entry/guidance/stop conditions、evidence review 接续条件与 no-replay/no-authority boundary；若该 lane 因 effective open intervention、pending-gate request 或 open candidate/decision 阻塞，blocked `continue` 的 JSON/text 会直接输出 `reconcileHandoffs[]`、`pendingGateHandoffs[]` 或 `openDecisionHandoffs[]` 以及 `continue reconcile/pending gate/open decision handoff` 的 concrete reconcile/gate WhatIf/Apply 或 note WhatIf/hash-bound recordCommand、boundary 与 evidence，且保持 zero-write，不创建 run 或刷新 lane artifacts。 |
| `/rekit start <name>` | case-local 状态 | 创建或进入一个功能支线，例如 `/rekit start login`；支线只写自己的工作区；当 `<name>` 解析到已有工作线（如 `main` 或 `feature-login`）时，start 会进入该 durable lane 而不是新建并行 lane；维护自动化可用 `-WhatIf -Format json` 消费非写入 start 计划和结构化 `missionBrief`，显式 `-Apply` 输出含 apply 后 `missionBrief` 的 Go JSON envelope；start JSON/text 会投影 lane-local `pendingGateHandoffs[]` / `openDecisionHandoffs[]`（含 gate WhatIf/Apply、note WhatIf/hash-bound recordCommand、boundary 与 evidence）以及 compact authorized-gate adapter handoff（default/current report path、`reportSummary`、`liveValidation` validate/record commands、authorized workspace 与 no-heavy/no-authority boundary），让替换 executor 在 preview 或 takeover apply 第一屏即可定位 gate/decision 与 safe validation/record handoff；需要登记/接管当前会话时由主 Agent显式传 `-Executor <session>`、`-Actor <actor>`、`-Reason <reason>`，例如 `start main -Apply -Executor <new-session>` 可让替换会话接手主线并刷新 lane resume/checkpoint/events；runtime 只写 durable executor metadata，不自动 spawn 或管理 session。mission-complete 后普通 start 不会隐式重开 mission。 |
| `/rekit complete <name>` | case-local review-first completion | 审核 lane 的 case-local evidence 后先运行 `-Actor ... -Reason ... -EvidenceRefs ... -WhatIf -Format json`；preview hash 绑定 evidence exact bytes、blockers 与写集，Apply 必须使用返回的 `-ExpectedCompletePlanSha256`。未解决 intervention/gate/decision/task/reviewer/execution-review/adapter blocker 时拒绝，main 必须最后关闭；feature 完成后路由下一条 open lane，全部 lane 的 intent/receipt/board/resume/checkpoint 一致时 status 返回 typed `mission-complete`。completion 不写或推断 authority/confirmed，不执行 heavy-tool；closed/pending-completion lane 拒绝 start/continue/gate/note/reviewer mutation。 |
| `/rekit reopen <name>` | case-local review-first completion supersession | 误完成、补充证据或事后发现新工作时使用。`-WhatIf`把actor/reason、case-local evidence、被supersede的exact completion receipt、effective targets和写集绑定到`reopenPlanSha256`；Apply必须携带返回的`-ExpectedReopenPlanSha256`。terminal feature复开会在同一个compound operation显式纳入已失效的main aggregate completion，final operation commit是共同生效点；中断恢复只消费immutable per-lane intent，pending/invalid operation期间handoff与ordinary lane mutation均fail-closed。复开清空旧executor、递增generation且不恢复旧session/current-loop budget；历史completion/reopen artifacts保持append-only，不写authority/confirmed、不执行heavy-tool。 |
| `/rekit handoff` | case-local 状态 | 生成项目级接手索引 `.rekit/handovers/latest.md`；索引和 Go JSON envelope 都包含 Mission Control brief，汇总 ready/blocked lanes、pending gates、authorized gates、compact authorized-gate adapter handoff、open decisions、interventions、next agent actions 与 escalations。`-WhatIf -Format json` 零写入返回 `publicationPlanSha256`、`publicationStamp` 与 exact Apply request；Apply 必须携带同一 hash/stamp。发布完成后 `latest-generation.json` 才作为整组 RESUME/checkpoint/handoff/takeover 的最终 commit point；status 只在整组 exact bytes匹配时信任 durable takeover，`mixed-generation` 时必须刷新 handoff，不得拼用混合文件；不代表某个会话。 |
| `/rekit handoff <name>` | case-local 状态 | 生成指定工作线接手文档，例如 `/rekit handoff main` 或 `/rekit handoff login`；lane handoff 的 Markdown 与 Go JSON `missionBrief` 使用 overview 同一 blocker 语义，pending gate、open intervention、open candidate/decision 都会让该 lane 显示为 blocked；`authorized-gate` 单独展示授权 profile / decision 但不阻塞 lane，并带出 requested budget、authorized output paths、stop conditions、gate `eventId`、可复制 report contract command 与 compact adapter handoff，供替换 executor 接手 actual heavy action 前核对边界并读取 default/current report path、report summary、live validation validate/record commands 与 authorized workspace；已记录 execution observation evidence 时，lane handoff JSON/Markdown 同时输出 compact review summary 与完整 review item（含 recorded adapter context detail），避免逐条扫描 refs/follow-through/action queue 或回退 observations ledger/contract 才能确认 adapter entry/guidance/stop conditions。该命令与项目级 handoff 使用同一 hash/stamp Apply 和 lane-local generation commit；中途失败不会推进 latest generation，replacement executor 只能消费 status 标记为 fresh 的 committed takeover。 |
| `/rekit reconcile <name>` | case-local intervention | 显式处理 lane-local effective open intervention；`-WhatIf` 预览 resolution、lane event、executor takeover、resume/checkpoint/board refresh，`-Apply` 只写这些 case-local durable state，不执行 heavy-tool、不写 authority/confirmed。reconcile JSON/text 会和 start 一样投影 lane-local `pendingGateHandoffs[]` / `openDecisionHandoffs[]` 与 compact authorized-gate adapter handoff；当 intervention resolution 后仍有 gate/decision blocker 时，替换 executor 可在 reconcile 结果第一屏直接拿到下一条 gate/note handoff 与 safe validate/record handoff，而不必回查 status/handoff。 |
| `/rekit gate -WhatIf` / `/rekit gate -Apply` | case-local gate/evidence | Go backend 的 heavy-action authorization preflight；`-WhatIf -Format json` 输出 gate decision plan 和当前 `missionBrief`，不写 ledger、不执行 heavy-tool；`-Apply -Format json` 默认 append pending-gate 或 authorized-gate request ledger decision，并输出 apply 后 `missionBrief`；`missionBrief.authorizedGates` 会直接显示 requested budget、authorized output paths、stop conditions、gate event id 与可复制 report contract command；对已授权 gate 可先用 `-ExecutionReportContract -GateEventId <authorized-gate-event-id> -Format json` 读取只读 adapter execution report contract（在 case-local / authorized output workspace cwd 中可省略 `-Target`），供 lane executor / tool adapter 在执行前先看 compact `reportSummary` 判断 state、report/default path、validation/record readiness、current action、repair/main-escalation flags、allowed counts 与 no-heavy/no-authority boundary，再按需消费完整 action、budget、output paths、default report path、status、stop conditions、boundary/escalation requirements、validation failure taxonomy / `validationRepairHints[]`、`liveValidation` handoff（`authorizedWorkspaces[]`、`reportFileName`、`caseRelativeReportPath`、sidecar template、workspace-relative 与 case-relative validate/record command strings + args、从 pack `tooling/catalog.yml` 投影的 `adapterCandidates[]`、默认 `selectedAdapter` / sidecar `adapterId` guidance、replay behavior；managed adapter attempt 必须在外部执行前先运行 runtime 返回的 `-RecordAdapterExecutionDispatch` preview，再消费其 exact expected-binding-hash Apply command写入 immutable `dispatch.json`；sidecar template会携带dispatch ID/path/SHA，report-first事后补造dispatch会fail-closed；dispatch已记录但report缺失时，status/handoff会等待external harness，或仅在harness outcome已知时用`-DraftExecutionReport -ExecutionStatus failed|aborted`记录dispatch-bound terminal report；takeover后stale dispatch只能走distinct reauthorization，不能由新owner采用；record handoff 运行前需替换 `<executor-id>`，重复 `RecordArgs` / `CaseRelativeRecordArgs` 返回 `duplicate eventId` 且不追加 observations）和 sidecar 规则；adapter 写出 sidecar 后，可用 `-ValidateExecutionReport -GateEventId <authorized-gate-event-id> -ExecutionReportPath <path> -Format json` 做只读 strict validation preflight（在 case-local / authorized output workspace cwd 中可省略 `-Target`，report path 可相对当前 workspace），输出 `isMutation=false` / `applied=false` 且 `valid=true` 或 `valid=false` envelope；validation JSON/text 同样投影 compact `reportSummary`，让替换 executor 直接看到 valid/recordReady/recordBlocked、repair hints counts、report status/adapter id/actualBudget、refs/boundary hits、failure code/stage 与下一条安全命令，并在可用时携带 `adapterContext.candidates[]` / `adapterContext.selected` 且 text 输出 adapter candidate / selected adapter provenance（invalid sidecar 含 `error`、`errors[]`、`failureCode`、`failureStage`、带 `evidence[]` / `boundary[]` 的 `repairHints[]`、`reportPath`、可用时的 partial report 与 contract boundaries），且不写 observations ledger；传入 `-GateEventId <authorized-gate-event-id>` 与 `-ExecutionStatus` 时改为记录授权动作后的 observation execution evidence 到 `.rekit/facts/observations.jsonl`，包含 actual budget、output refs、evidence refs、boundary hits 与 escalation；也可传入 `-ExecutionReportPath` 读取 lane executor / tool adapter 写在 authorized output paths 下的 bounded `adapter-execution-report` JSON sidecar，并可在 case-local / authorized output workspace cwd 中省略 `-Target`、用当前 workspace 相对 sidecar path 记录 evidence，重复记录同一 sidecar 返回 `duplicate eventId` 且不重复 append observations，校验 action/status/gateEventId/budget/refs/boundary/escalation 后嵌入 evidence，并在 `execution.adapterContext` 中保留匹配到的 concrete tooling candidate；output refs、evidence refs 与 report path 必须落在 authorized gate 的 output paths 内；sidecar 若声明 `boundary-hit` / `escalated` 或实际预算越界，必须自带 `boundaryHits` 或 `escalation` marker，`boundaryHits` 必须被本次 authorized gate `stopConditions` 覆盖，`failed` / `boundary-hit` / `escalated` / `aborted` sidecar 必须包含 bounded summary；durable lane autonomy profile 完全覆盖时可记录 `authorized-gate`，并在 overview、handoff、continue digest/status 与 `missionBrief.authorizedGates` 中可见；否则 fail-closed 为 pending/denied decision；实际 heavy-tool 仍由 lane executor / tool adapter 在授权范围内执行，`/rekit` 只记录 request/evidence，不写 confirmed/authority。 |
| `/rekit sync` | kit -> case | 默认生成同步审查包；确认后才用 `-Apply` 写入 managed docs / managed block。当前显式 attached case 的 status/handoff 也会发现 completed verified pack-memory change；选择一个 `changeId` 后先审核 selected WhatIf 返回的 producer authority、单一 managed path、source/target/state hashes、backup/receipt和 exact plan hash，再执行原样 hash-bound Apply。成功后 case-local receipt 与 fresh status/handoff 证明已消费；runtime 不扫描其它 case、不自动 sync。 |
| `/rekit promote` | case -> kit | 默认生成回流审查包；确认后才用 `-CreateCandidates` 生成候选或用 `-Apply` 写回 pack。 |
| `/rekit doctor` | 只读 | 排障时详细验证结构；日常不必主动运行；维护自动化可用 `-Format json` 消费验证 rows。 |
| `/rekit repair` | case metadata | 迁移目录后先预览修复；确认后由 Claude 调用 backend `-Apply`。 |

`validate` 和 `plan-subagents` 仍是 backend/内部命令，不是日常主入口；`plan-subagents` planning mode 默认经 Go façade 生成 review artifacts，reviewer-intake mode 由主 Agent显式执行 strict WhatIf/Apply 与 post-validation，但 runtime 不自动 spawn agent；`packs` 是维护者/排障入口，用于多 pack 发现和矩阵验证；`note -List` 文本/table/tsv 与 `note -List -Format json` 默认经 Go façade 只读查询 ledger events；`note -WhatIf` JSON envelope 输出当前 `executorAction`、内存模拟 append 后的 `wouldExecutorAction`、`eventSha256` 与可重放时的 hash-bound `recordCommand`；该 command 带 `-CreatedAt`、`-EventId` 与 `-ExpectedNoteEventSha256`，record 时若 event body drift 会 fail-closed 且不写 ledger。实际 append 输出写入后的 `executorAction`，duplicate eventId 只返回未改变的当前 action 且不写 ledger；含 reviewer-intake 内部字段等不可 CLI 重放 event 时只输出 hash、不输出 misleading record command。note 仍只写 facts JSONL 或预览，不写 authority/confirmed。

## 日常工作流

维护 onboarding、status quickstart、continue/reconcile、handoff 或 replacement executor takeover 路线时，可用一条跨平台 Go-native smoke 覆盖完整日常闭环：

```text
go test ./internal/rekit/cli -run '^TestRunDailyMissionControlRouteSmokeProductPath$' -count=1
```

具体场景与安全边界见 `rekit/tests/README.md`；正常 case 用户不需要手动运行该维护命令。

### 1. 看当前项目状态

```text
/rekit overview
```

它会展示：

- 当前主线和功能支线；
- 共享事实、request、candidate、publication 统计；
- Mission Control brief：ready/blocked lanes、pending gates、authorized gates（含 execution boundaries、eventId 与 report contract handoff）、open decisions、interventions、next agent actions 与 escalations；
- 逐 lane executor action index：blocked/ready、pending gate / open intervention / open decision counts、requirements、resume/handoff command 与 current executor / generation / last takeover 摘要；
- blocker-aware 下一步：`continue` 的 apply JSON、text 与 handoff Markdown 使用 lane-local executor actions；blocked lane 只建议 reconcile / pending gate / open decision 处理，ready lane 才建议自己的 continue，paused/closed/unready lane 回到 handoff/read-only；
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

`-CreateCandidates` 的 JSON `reviewPlan.reviewSummary` 与文本 `promote candidates review summary...` 会先给出 candidate/tooling/index、review/cleanup/reconsume artifact、Mission Commander next action、no-merge/no-cleanup/no-heavy/no-authority boundary，以及 terminal `proofSummary`：expected proof total/present/missing、decision/cleanup/reconsume missing counts、proof progress、current stage、next missing proof type/path/candidate/pack target、compact `nextMissingProof` detail（stage/proofType/path/candidatePath/packTarget/when/action/format/evidence/boundary）与 proof boundary。`status` / `release-check` 的 pack-memory handoff 也输出 compact review/proof summary、expected proof paths、present/missing counts 与同类 next-missing proof detail；当存在 next missing proof 时，还会投影 `currentRunLoopStepId` / `runLoop`，把 `inspect-proof-gap → bind-review-packet → draft-proof-whatif → apply-proof-with-expected-hash → refresh-pack-memory-status → continue-review-cleanup-reconsume` 作为只读 operator workflow。缺 case-local packet 时 current action 停在 `bind-review-packet`，case-local status 绑定 packet/evidence 后才进入 `draft-proof-whatif`，便于 replacement executor 刚预览或生成候选后不扫描完整候选列表、`reviewArtifacts[]` 或 proof 目录，就判断是否还需先补 packet、decision proof、cleanup proof 或 reconsume proof，以及该 proof 应记录什么 evidence / boundary。

`promote` 很保守：若 managed docs 含真实绝对路径、样本名、RVA/VA、ctx/round 快照、artifact/capture/trace/dump 路径，会阻止直接回流。工具链经验只有在脱敏后不再命中 deny pattern 时才写 sanitized preview；候选由你审查后合入正式 tooling 文档。合入 `tooling/catalog.yml` 或 `tooling/recipes/*` 后，后续 init/attached fresh case 通过 `templateRoot` + `templatePack` 读取同一 pack tooling 资产；`sync` 不把 tooling files 复制进 case managed docs，避免把候选经验静默混入 case 私有路由。

`promote` 只允许作用于已经 `attach/init` 的 case，避免从普通目录误回流到 pack。

## 内部状态模型

日常不用理解这些文件，但排障或 review 时可能会看到：

| 路径 | 内容 |
|---|---|
| `.rekit/board.json` | 项目概览的机器状态。 |
| `.rekit/lanes/<id>/` | 每条工作线的事件、任务、inbox/outbox、`prompts/RESUME.md`、`checkpoints/latest.json` 与append-only completion lifecycle；resume/checkpoint记录lane-local blockers/gates，completion/reopen generations保留历史closure与supersession evidence，便于替换executor接手。 |
| `.rekit/facts/*.jsonl` | append-only ledger：observation、hypothesis、candidate、verification、decision、intervention、rollback、publication、request；Go runtime 通过 shared facts path/read/append helper 访问，避免各命令自行拼路径、读取或追加 JSONL。 |
| `.rekit/runs/<run-id>/digest.md` | `/rekit continue` 每轮摘要，记录 inputs、route、packet refs、Mission Control brief、outputs、decisions、open risks；存在 execution evidence review 时直接列出 follow-through outcome 的 `when` / `evidence`，供 replacement executor 从 digest 接手。 |
| `.rekit/handovers/latest.md` | 项目级接手索引。 |
| `.rekit/handovers/<laneId>-latest.md` | 指定工作线接手文档。 |

字段名中仍保留 `lane`，这是内部 schema 名称；用户层统一称“工作线 / 主线 / 功能支线”。

## 高级/内部：子 agent 分片计划

`/rekit plan-subagents` 是内部 tactical reviewer planning/intake 入口。planning mode 按 manifest route 生成 `packet.json`、`summary.md`、read-only shard handoff、strict reviewer result contract、owner binding 与 writeback guidance；它不自动启动 reviewer。planning result / packet 的 `reviewerOrchestration.summary` 以及 terminal / `summary.md` lines 会直接给出 mode、target lane、reviewer/dispatch counts、owner binding、intakeAvailable/dispatchOnly、Mission Commander queue counts、first dispatch、current action、next actions 与 no-spawn/no-heavy/no-authority boundary，让 replacement executor 不必解析 nested dispatches / lifecycle / action queue 才能接续。主 Agent调用 shard 的 read-only Agent tool request 后，先运行 `-RecordReviewerDispatch -WhatIf`，仅在 harness 实际接受该 session 后消费返回的 hash-bound Apply；session 结束后运行 `-RecordReviewerCompletion -WhatIf` 并消费其 hash-bound Apply。immutable receipts strict 绑定 packet/route/shard/prompt SHA、harness/session、current lane owner generation、completion outcome 与 exact result input hash/bytes；只有 current owner 下的 successful completion 才可进入 source capture。reviewer 产出单个 contract-compliant JSON object（包含 dispatch receipt 绑定的 `reviewerSession`）后，主 Agent显式传入 `PacketPath`、`ReviewerResultPath`、`Lane` 与 `Actor`，先用 WhatIf 校验 packet/route/shard/items、receipt lineage、route output、evidence refs、conflicts、blocked actions 与 lane executor owner binding，再用 Apply 按 verification-before-decision 顺序写 case-local facts；reviewer intake JSON / text 的 `summary` 会直接压缩 status、dispatch progress、compact `orchestrationProgress`（dispatch index/total、completed/open、current/next/remaining shards）、blocked/repair counts、compact `repairGuidanceSummary`、postValidation totals、reviewer writeback count、compact `reviewerWritebackSummary`、current/next actions 与 no-heavy/no-authority boundary。写回后的 downstream status/handoff/continue、lane `RESUME.md`、checkpoint 与 digest 还会输出 compact `reviewerWritebackSummary` / reviewer writeback summary lines，直接汇总 verification/decision counts、latest shard/reviewer session/result/packet/route、owner binding / risks / conflicts / route output flags、latest evidence refs 与 no-heavy/no-authority/no-spawn boundary，replacement executor 不必逐条扫描 `reviewerWritebacks[]` 才能复核 reviewer provenance。deterministic event IDs 支持相同 intake 的安全重试。写后返回完整 overview、lane handoff 与 doctor validation，并在 `postValidation.summary` / text summary lines 中直接给出 verification/decision totals、doctor row count、lane/executor action state、reviewer writeback count、compact `reviewerWritebackSummary`、current action、next actions 与 no-heavy/no-authority boundary。PowerShell fallback 已退休，即使设置 `REKIT_GO_DISABLE=1` 也不会回落到 PowerShell 业务实现。

在 reviewer result 写回前，case `status`、`handoff`、`continue`、continue run `status.json` / `digest.md`、lane `RESUME.md` 与 checkpoint 也会投影 open `reviewerDispatchIntakeHandoffs` / compact summary：直接显示每个 shard 的 reviewer result 目标路径、Agent tool request、dispatch/completion receipt command 与 provenance、staging/collection/WhatIf/Apply intake command、ready-for-dispatch / running-unknown / completion-preview / failed / stale-owner / source-capture-ready / staging-ready / collection-ready / intake-ready / dispatch-only state、packet-level progress（dispatchTotal / completed / open / nextOpen / remaining）、`nextActionRunbookSteps[]` / per-shard `runbookSteps[]` 与 no-spawn/no-heavy/no-authority boundary，便于 replacement executor 不打开完整 `packet.json` / `summary.md` 或扫描 reviewer writebacks 即可从 live command 或 durable artifact 接续。实际执行 `-CaptureReviewerResultSource`、`-StageReviewerResult` 或 `-CollectReviewerResult` 后，返回 JSON 的 `runbookSteps[]` 与 text 的 `reviewer result ... runbook` 行也会再次给出当前 command、hash-bound Apply 纪律、下一步 WhatIf 和 capture/staging/collection/packet-level intake 分离边界，避免会话切换时重新拼 reviewer writeback 顺序。若 collection preview 发现 canonical reviewer result 已被不同 bytes、empty-file 或 symlink obstruction 占据，会以 `status=recovery-required` 返回 canonical kind/hash/bytes 与独立 `-RecoverReviewerResult -WhatIf` handoff；collection Apply 仍拒绝覆盖，recovery 必须走自己的 WhatIf→hash-bound Apply；direct recovery JSON 的 `runbookSteps[]` 与 text 的 `reviewer result recovery runbook` 行会串起 WhatIf→hash-bound Apply→interrupted finalize→collection WhatIf。

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
- 不默认 commit / push；只有当前用户 goal/session 明确授权具体仓库和分支时才执行。已授权的普通 batch 以 Windows 本机 focused tests 与完整 release minimum 为完成门槛，只做一次 implementation commit/push；推送成功后立即继续下一批，不轮询或等待远程 workflow，也不创建专门的 release inspection commit。远程 Linux/macOS/Windows CI 保留为异步信号，仅在正式发布、跨平台专项或周期复审时等待并记录。
