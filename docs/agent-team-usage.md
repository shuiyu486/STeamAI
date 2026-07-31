# Agent Team 使用与兼容指南

## 读取指南

如果你只是维护本仓库，先读根目录 `CLAUDE.md` 与 `docs/context-routing.md`，再按场景读取本文件顶部或其它路由入口；不要把本文件全文、README 和 vision 当成默认必读清单。

如果你正在具体安全 case 中工作（当前成熟示例是 `vmp-re` RE case），先在 case 目录用 `/rekit status` 确认绑定，再按本文件选择新 case、旧 case、主线或功能支线流程。

本文件只说明通用使用方式和兼容策略，不记录真实样本名、RVA/VA、trace/dump、artifact 路径或 case-specific 进度。

## 实施摘要

新方案不是替换旧 case 的大迁移，而是在现有 `/rekit`、pack、case shim、工作线机制上增加 Agent Team 的组织方式：

- 用户日常入口优先是自然语言主 Agent / Mission Commander；`/rekit` 仍是主 Agent、维护者、自动化和排障使用的 deterministic runtime API。
- 旧 case 可继续使用 `.re-template.yml`，也可以通过 `/rekit attach` / `/rekit repair` 补齐 `.rekit/instance.yml`。
- 主线和功能支线仍然保留，而且是新架构的核心协作单元。
- `sync` / `promote` 仍然 review-first，写入前需要确认具体范围。
- Mission Commander action queue 现在会为所有 current action 投影 `currentRunLoopStepId` / `currentActionRunLoop`，让 replacement executor 直接按 `inspect-current → apply-or-run-current / preview-current → refresh-state → follow-up-after-refresh` 接手；`currentDriverRequest.expectedReceipt.command` 保留当前应执行/预览的 driver command，`expectedReceipt.refreshStatusCommand` 在 status/handoff/daily runbook 与 replacement takeover package 中给出执行后必须运行的同源 refresh command，避免从相邻文本手工拼接；该 run-loop 是只读 handoff，不会改变 action 选择、不自动执行命令。
- `status` 顶层会输出统一 `missionControlRunbook`：复用 Mission Commander first-screen focus routing，把当前 focus、routingReasons、focused queue/currentDriverRequest、`quickstart`、`replacementExecutorTakeoverPackage`、`handoffPreviewCommand` / `handoffApplyCommand`、status refresh command 与 `inspect-first-screen → consume-focused-driver-request → refresh-after-focus-result → preview-handoff → write-handoff-for-takeover` 收敛成单一只读入口；`quickstart` 是日常第一屏消费包，直接镜像 focused `currentDriverRequest`、同源 `currentDriverReceipt`、`refreshStatusCommand`、target documents、runbook steps 与 acceptance checklist，让主 Agent / replacement executor 可按 `status → quickstart.currentDriverRequest → explicit outcome → quickstart.refreshStatusCommand` 执行，不需要从 terminal prose 拼命令；`replacementExecutorTakeoverPackage` 会把 currentDriverRequest、targetDocuments、runbookSteps 与 boundary 收敛成无旧聊天上下文也可消费的只读接手包；当 focused driver request 是不可执行的 `review-guidance` 时，顶层 runbook 会附带只读 `guidanceHandoff`，交代 target documents、acceptance checklist、expected receipt，以及 next-batch starter/candidate package，避免 replacement harness 把自然语言 guidance 当命令或从 terminal prose 推断完成；implementation-pending 本机验证 action 会暴露 `/rekit release-run -Format json` read-only driver request，让 harness 直接跑本机 release gate profile 后再记录验证 evidence；从 kit CWD 用 `status -Target <case>` 接手时，顶层 runbook 会把可执行 case request 补成 invocation-scoped `-Target <case>`，并把 bare `continue` 降级为 `-WhatIf -Format json` 预览，同时把 handoff preview 固定为 `/rekit handoff -Target <case> -WhatIf -Format json`，先预览再刷新/显式 Apply。`status` case mission 与 project/lane `handoff` 继续输出 `dailyMissionControlRunbook`，汇总当前 `currentDriverRequest`、status refresh command、handoff preview/apply command，以及 `inspect-status → consume-current-driver-request → refresh-after-driver → preview-handoff → write-handoff-for-takeover` 的日常主 Agent/harness 接手节奏；project/lane handoff JSON 与 durable Markdown 也输出同源 `replacementExecutorTakeoverPackage`，把 handoff 当前 driver request、target documents、runbookSteps 与 boundary 放到不依赖旧聊天上下文的接手包中；这些 runbook/package 都不 spawn session、不执行命令、不写 authority/confirmed。
- reviewer dispatch operator package 也会投影 ordered `runLoop[]` / `currentRunLoopStepId`，让 replacement executor 直接按 `verify-prompt → spawn-reviewer → record-dispatch → save-result-input → record-completion → source-capture → stage-candidate → collect-result → intake-results` 接手；Go runtime 仍只记录 dispatch/completion/intake，不 spawn/poll/stop reviewer session。
- authorized-gate adapter live validation 会在 contract、validation、status、overview、handoff 与 durable Markdown 中投影同源 `currentRunLoopStepId` / `runLoop`，按 `inspect-contract → record-dispatch → run-external-adapter → draft-or-write-report → validate-report → record-receipt → record-observation → review-recorded-evidence` 交接当前 adapter 步骤、dispatch command、owner/provenance 与 tooling lineage；记录 execution evidence 后，非 escalation 的 evidence review 会把 `acknowledgementReviewCommand`（accepted `note -Kind verification ... -WhatIf -Format json`）设为 Mission Commander current action / `currentDriverRequest.command`，review 后只执行 returned hash-bound `recordCommand` 关闭 review queue；`/rekit handoff <lane>` 仍作为 follow-up/provenance 保留。Go runtime 仍不执行 adapter/heavy tool。
- Agent Team 当前主要是 context、workflow、tooling、ledger、gate 的底座，不代表已经全自动脱壳、全自动逆向、自动漏洞挖掘、自动恶意样本分析或通用自动渗透。

推荐心智模型：

```text
kit 仓库 = runtime + packs + common policies + tooling 经验
case 目录 = 具体目标/样本/项目状态 + 工作线 + 证据 + 候选结论
主线 = 收敛、确认、长期 handoff
功能支线 = 专项探索、证据收集、候选结论
```

## 执行清单

### 新安全 case（当前以 `vmp-re` 为例）

1. 在 kit 仓库启动 Claude Code。
2. 使用 `/rekit init -Target <caseRoot> -Pack vmp-re -ProjectName <caseName> -Apply`。
3. 进入 case 目录启动 Claude Code。
4. 执行 `/rekit status` 和 `/rekit overview`。
5. 用 `/rekit continue main` 接手主线。
6. 需要专项分析时，用 `/rekit start <name>` 创建功能支线。
7. 每轮结束用 `/rekit handoff` 或 `/rekit handoff <name>` 生成接手文档。

### 旧 case

1. 在 kit 仓库或 case 目录确认当前绑定：`/rekit status`。
2. 如果还没有 `.rekit/instance.yml`，用 `/rekit attach -Target <caseRoot> -Pack vmp-re -Apply` 补齐 metadata 和 case-local shim。
3. 执行 `/rekit sync` 生成同步审查包。
4. Claude 复核冲突、收益和覆盖范围后，再由用户确认是否执行写入型 `sync -Apply`。
5. 执行 `/rekit doctor` 验证结构。
6. 执行 `/rekit overview`，再选择 `/rekit continue main` 或 `/rekit start <name>`。

### 日常工作

优先用自然语言让主 Agent 统筹 mission，而不是让用户手动记命令：

```text
继续推进当前 mission。
总体怎么样？哪些 lane 卡住了？
让我进入 verifier lane 帮它纠错。
这个 lane 上下文污染了，生成接手包，让新会话接手。
把这次可复用经验整理成 promote 候选。
```

主 Agent 会按需组合底层 runtime API：

```text
/rekit overview              # 看项目总览，不选择工作线
/rekit continue main         # 接手主线
/rekit start unpacking       # 创建/进入功能支线；已有 lane 若仍有 pending gate / open decision 会直接显示 handoff
/rekit continue unpacking    # 继续功能支线；若存在 intervention / pending gate / open decision 会先返回 blocked handoff
/rekit reconcile unpacking   # 将用户干预显式 reconcile 到 durable lane state；若剩余 gate/decision blocker 会直接显示下一条 handoff
/rekit handoff               # 生成项目级接手索引
/rekit handoff main          # 生成主线接手文档
/rekit handoff unpacking     # 生成功能支线接手文档
/rekit                       # 无子命令时默认只读 status；attached case 中可省略 -Pack 并使用 metadata templatePack；kit-mode 会先显示 latest-batch project current action、focus routing explanation、project runbook 与 current action run-loop；pack-memory focus 会先显示 evidence shortlist（含 next missing proof current step、draft WhatIf / ExpectedProofSha256 Apply template），再回到 compact Mission Commander first-screen strip / queue current action 接手，空 mission 会先投影 onboarding current action
/rekit status -Format text    # 显式 status text；显示 packSource、kit/case status、manifest、shim readiness、Mission Commander queue/current action、currentRunLoopStepId/currentActionRunLoop、first-screen focus routing、pack-memory focus evidence shortlist（含 proof draft/apply/boundary）、latest-batch project current action/project runbook、project handoff current action queue、pending-gate review/WhatIf/request-decision handoff、authorized-gate execution report contract handoff、open decision source/list + note WhatIf/hash-bound recordCommand handoff、open intervention reconcile WhatIf/Apply handoff、execution evidence review summary、latest-batch validation/remote handoff 与 bounded repair/status next-step handoff；active reviewer/evidence/adapter review blocker 会优先于普通 handoff/continue follow-up
/rekit release-check -Format text # 只读显示 release inventory、latest-batch validation evidence、handoff signals 与 CI truthfulness boundary
/rekit release-run -Format text   # 执行本机 release minimum，汇总每步 exit code / duration / output tail，并输出只读 release inspection handoff（git clean/sync、latest-batch cadence、third-inspection guard）；不写 repo/case state，不联网读取远程 CI，不替代远程 CI green
/rekit next-batch -Domain <candidate-domain> -Closure <batch-title> -WhatIf -Format json # 接受 kit-mode next-batch guidance 前预览 docs/batch-plan.md 与 CHANGELOG.md planning receipt，并返回 expectedNextBatchPlanSha256
/rekit next-batch -Domain <candidate-domain> -Closure <batch-title> -ExpectedNextBatchPlanSha256 <hash> -Apply -Format json # 复核同一 WhatIf hash 后只写这两个 kit docs；不触碰 case state、不执行 heavy/reviewer/adapter/pack-memory/gate/sync/promote mutation、不 commit/push、不声明 remote CI green
/rekit overview -Format text # 只读显示 case Mission Commander action queue/current action、queue-aware next steps、lane/executor/evidence review summary/evidence/section handoff
/rekit note -Kind verification -Lane main -WhatIf -Format text # 预览 ledger event 与 would-action delta
/rekit note -Kind verification -Lane main -Format text # 写入 append-only fact event 后打印 Mission Commander handoff
/rekit doctor -Format text    # 只读显示 pack/case validation summary 与 rows
/rekit validate -Format text  # 只读显示 validate identity、summary 与 rows
/rekit packs -Format text     # 只读显示 pack inventory、manifest counts 与 heavy action names
/rekit attach -Target <caseRoot> -Pack <pack> -WhatIf -Format text # 旧 case 接入前预览 metadata/shim/state writes
/rekit attach -Target <caseRoot> -Pack <pack> -Apply -Format text  # 写入 binding metadata/shim/state，并打印 next steps
/rekit repair -Target <caseRoot> -Pack <pack> -WhatIf -Format text # case 移动后预览 metadata/shim refresh
/rekit repair -Target <caseRoot> -Pack <pack> -Apply -Format text  # 在 WhatIf preview 与显式确认后修复 moved case 绑定，并打印 handoff
/rekit init -Target <caseRoot> -Pack <pack> -WhatIf -Format text   # 新 case 初始化前预览 managed/template writes
/rekit init -Target <caseRoot> -Pack <pack> -Apply -Format text    # 初始化完整 case，并打印 doctor handoff
/rekit bootstrap -Target <caseRoot> -Pack <pack> -WhatIf -Format text # compat bootstrap 预览，保持 bootstrap identity
/rekit sync                  # kit -> case，默认只生成 JSON review
/rekit sync -Format text     # kit -> case review plan 的 terminal handoff
/rekit sync -Apply -WhatIf -Format text  # 写入前预览每个 write / backup / next step
/rekit sync -Apply -Format text          # 确认范围后写入，并打印 terminal handoff
/rekit update -Apply -WhatIf -Format text # sync/update 共享写入路径，但输出保持 update identity
/rekit promote               # case -> kit，默认只生成 JSON review
/rekit promote -Format text  # case -> kit review plan 的 terminal handoff
/rekit promote -CreateCandidates -WhatIf -Review -Format json # 生成 candidate review workspace（packet / bounded diff / sanitized preview），不生成候选
/rekit promote -CreateCandidates -Review -Format json # 生成候选及 durable candidate review workspace；仍不 merge/cleanup/reconsume
/rekit promote -PacketPath <candidate-review-packet> -CandidateDecisionPath <decisions.json> -WhatIf -Format json # 严格绑定 packet/candidate/target/evidence hashes，预览 reviewed accept/reject/superseded actions，不写入；`decisionRunbookSteps[]` 同屏提示 action/evidence review、accepted 后续 provisioning/verification 与 identical Apply
/rekit promote -PacketPath <candidate-review-packet> -CandidateDecisionPath <decisions.json> -Apply -Format json # 主 Agent复核同一 WhatIf 后显式执行 managed-doc merge 或 candidate cleanup；`decisionRunbookSteps[]` 按 accepted vs reject/superseded-only 指向 receipt、verification provision/command/proof 或 cleanup/index closure
/rekit promote -PacketPath <candidate-review-packet> -CandidateDecisionPath <decisions.json> -ProvisionCandidateVerificationCases -FreshCaseRoot <workspace/fresh> -AttachedCaseRoot <workspace/attached> -WhatIf -Format json # strict重验accepted decision并预览两个distinct no-overwrite case的完整写集；不创建workspace/case
/rekit promote -PacketPath <candidate-review-packet> -CandidateDecisionPath <decisions.json> -ProvisionCandidateVerificationCases -FreshCaseRoot <workspace/fresh> -AttachedCaseRoot <workspace/attached> -ExpectedProvisionSha256 <hash> -Apply -Format json # 复核WhatIf hash后在canonical source-case-local workspace写durable intent，exclusive init两个doctor-ready cases；exact/partial replay由同一intent授权，仍不写final verification proof
/rekit promote -PacketPath <candidate-review-packet> -CandidateDecisionPath <decisions.json> -VerifyCandidateDecision -FreshCaseRoot <workspace/fresh> -AttachedCaseRoot <workspace/attached> -WhatIf -Format json # provisioning后执行accepted candidate的pack/fresh/attached doctor与cleanup/hash binding只读验证；`verificationRunbookSteps[]` 提示先复核doctor/reconsume，再 identical Apply；三个case必须彼此不同
/rekit promote -PacketPath <candidate-review-packet> -CandidateDecisionPath <decisions.json> -VerifyCandidateDecision -FreshCaseRoot <fresh-case> -AttachedCaseRoot <attached-case> -Apply -Format json # 复核同一 WhatIf 后只写 repo-local verification proof；首次写入返回`mode=verified`，exact proof replay返回`mode=already-verified`/`replay=true`；返回envelope的`verificationRunbookSteps[]` 指向proof retention、retirementPreviewCommand WhatIf、expected-hash retirement Apply与status/release-check确认
/rekit promote -PacketPath <candidate-review-packet> -CandidateDecisionPath <decisions.json> -RetireCandidateVerificationWorkspace -WhatIf -Format json # final proof到位后strict绑定proof、provision intent/receipt与两个exact trees，预览确定性删除计划；不写intent/receipt、不删除workspace
/rekit promote -PacketPath <candidate-review-packet> -CandidateDecisionPath <decisions.json> -RetireCandidateVerificationWorkspace -ExpectedRetirementSha256 <hash> -Apply -Format json # 复核WhatIf hash后写repo-local durable retirement intent，删除exact provisioned trees并写receipt；crash resume与exact replay幂等；release/status只有在每个receipt action的strict cleanup proof与retirement receipt都到位后才清零，且会识别`-DraftReviewProof -ProofPath`写出的自定义`*.candidate-cleanup-proof.{md,json,txt}`
/rekit plan-subagents -PacketPath <packet> -AdoptReviewerPacket -Lane <lane> -Actor <actor> -Reason <reason> -WhatIf -Format json # packet owner被replacement executor takeover后，先预览exact packet/generation adoption；不修改immutable packet
/rekit plan-subagents -PacketPath <packet> -AdoptReviewerPacket -Lane <lane> -Actor <actor> -Reason <reason> -Apply -Format json # 复核同一WhatIf后写case-local strict adoption receipt；只转移intake ownership
/rekit plan-subagents -PacketPath <canonical-case-review-packet> -RecordReviewerDispatch -ShardId <shard> -ReviewerHarness <harness> -ReviewerSession <session-id> -Lane <lane> -Actor <actor> -WhatIf -Format json # 主Agent实际调用read-only Agent tool request后预览immutable dispatch receipt；绑定packet/route/shard/prompt SHA、harness/session与current owner generation，zero-write
/rekit plan-subagents -PacketPath <canonical-case-review-packet> -RecordReviewerDispatch -ShardId <shard> -ReviewerHarness <harness> -ReviewerSession <session-id> -Lane <lane> -Actor <actor> -ExpectedReviewerDispatchBindingSha256 <hash> -Apply -Format json # 仅在harness实际接受session后执行；canonical no-overwrite receipt，exact replay幂等，takeover后旧owner attempt不可完成
/rekit plan-subagents -PacketPath <canonical-case-review-packet> -RecordReviewerCompletion -ReviewerDispatchId <dispatch-id> -ReviewerOutcome succeeded -ReviewerExitStatus <status> -ReviewerResultInputPath <reviewerStagingCommands.sourceCaptureInput> -Lane <lane> -Actor <actor> -WhatIf -Format json # session结束且唯一JSON已保存后预览completion receipt；strict绑定dispatch receipt与exact input path/hash/bytes
/rekit plan-subagents -PacketPath <canonical-case-review-packet> -RecordReviewerCompletion -ReviewerDispatchId <dispatch-id> -ReviewerOutcome succeeded -ReviewerExitStatus <status> -ReviewerResultInputPath <reviewerStagingCommands.sourceCaptureInput> -Lane <lane> -Actor <actor> -ExpectedReviewerDispatchReceiptSha256 <dispatch-hash> -ExpectedReviewerResultInputSha256 <input-hash> -Apply -Format json # current owner下记录successful completion；failed/partial/stale-owner attempt不得进入source capture
/rekit plan-subagents -PacketPath <canonical-case-review-packet> -CaptureReviewerResultSource -ShardId <shard> -ReviewerResultInputPath <reviewerStagingCommands.sourceCaptureInput> -Lane <lane> -Actor <actor> -WhatIf -Format json # successful completion receipt到位后strict验证packet/route/shard/items/receipt lineage/evidence并返回input SHA-256，不写packet-derived source
/rekit plan-subagents -PacketPath <canonical-case-review-packet> -CaptureReviewerResultSource -ShardId <shard> -ReviewerResultInputPath <reviewerStagingCommands.sourceCaptureInput> -Lane <lane> -Actor <actor> -ExpectedReviewerResultInputSha256 <hash> -Apply -Format json # 锁内重读input与packet，no-overwrite发布exact bytes到packet-derived reviewerStagingCommands.sourcePath；input drift/different source fail-closed，exact replay幂等
/rekit plan-subagents -PacketPath <canonical-case-review-packet> -StageReviewerResult -ShardId <shard> -ReviewerResultSourcePath <packet-derived-source-json> -Lane <lane> -Actor <actor> -WhatIf -Format json # source capture完成后strict预览packet-derived source→candidate绑定，返回source SHA-256，不写candidate
/rekit plan-subagents -PacketPath <canonical-case-review-packet> -StageReviewerResult -ShardId <shard> -ReviewerResultSourcePath <packet-derived-source-json> -Lane <lane> -Actor <actor> -ExpectedSourceSha256 <hash> -Apply -Format json # 锁内重读packet/source并no-overwrite发布到packet-derived candidate；source drift/different candidate fail-closed，exact replay幂等
/rekit plan-subagents -PacketPath <canonical-case-review-packet> -CollectReviewerResult -ShardId <shard> -Lane <lane> -Actor <actor> -WhatIf -Format json # staging完成后strict预览candidate→canonical result绑定，不创建canonical result
/rekit plan-subagents -PacketPath <canonical-case-review-packet> -CollectReviewerResult -ShardId <shard> -Lane <lane> -Actor <actor> -Apply -Format json # 复核同一WhatIf后，以exact bytes/no-overwrite方式发布；custom/noncanonical packet改走direct/batch intake，不运行collection
/rekit plan-subagents -PacketPath <canonical-case-review-packet> -RecoverReviewerResult -ShardId <shard> -Lane <lane> -Actor <actor> -Reason <reason> -WhatIf -Format json # candidate正确但canonical result含不同regular bytes，或被Windows empty regular file/file symlink占据时，复核exact hash或typed object fingerprint与quarantine路径；directory/其它non-regular object仅typed识别并fail-closed，不覆盖、不写facts
/rekit plan-subagents -PacketPath <canonical-case-review-packet> -RecoverReviewerResult -ShardId <shard> -Lane <lane> -Actor <actor> -Reason <reason> -ExpectedCandidateSha256 <hash> -ExpectedReviewerResultSha256 <hash> -Apply -Format json # 显式隔离冲突bytes或Windows empty-file/file-symlink本体并写strict intent/receipt；obstruction路径以no-follow source handle、durable intent namespace guard和destination parent handle执行no-replace move，不打开或修改symlink target；directory/其它non-regular与非Windows平台不提升runnable recovery，中断后可确定性finalize，恢复后仍需collection WhatIf→Apply；direct recovery JSON/text 同步输出 runbookSteps / reviewer result recovery runbook，串起 WhatIf→hash-bound Apply→interrupted finalize→collection WhatIf
/rekit plan-subagents -PacketPath <packet> -ReadyReviewerResults -Lane <lane> -Actor <actor> -WhatIf -Format json # planning packet / Mission Commander queue 提供的 packet-level 命令；按 packet 顺序预览所有已到位 reviewer results，并返回 missionCommanderDriverReceipt，把本次 preview outcome、refreshed action queue/current run-loop step、下一跳 driver request 与 expectedReceipt.refreshStatusCommand 绑定到同一 envelope
/rekit plan-subagents -PacketPath <packet> -ReadyReviewerResults -Lane <lane> -Actor <actor> -Apply -Format json # 主 Agent检查 batch preview 后显式执行；按 packet 顺序 strict intake，遇首个 blocked/partial/error 即停；Apply result 同步返回 missionCommanderDriverReceipt，指向 apply 后刷新得到的下一跳 request，不代表 Go runtime spawn/poll/stop reviewer session、写 authority/confirmed 或执行 heavy tool
/rekit promote -Apply -WhatIf -Format text # 写回 pack 前预览 changed / blocked / write / next step
/rekit promote -Apply -Format text         # 确认 review scope 后写回 pack，并打印 validation / backup handoff
```

### Adapter execution report handoff identity

`gate -ExecutionReportContract`、`gate -ValidateExecutionReport`、scaffold/draft live snapshot 与 recorded evidence snapshot 的 Mission Commander next-action/current-action 行会直接显示 `lane`、`label`、`gateEventId`、`actionId`。这些 contract、scaffold、draft、validation、record 与 status/handoff envelope 还会输出 `runbookSteps[]`（text 中为对应 runbook 行），并共享 adapter live-validation `currentRunLoopStepId` / `runLoop`（text 中为 live run-loop 行），replacement executor 应优先消费这些 typed fields、runbook 与 live run-loop 来确认当前步骤是 inspect contract、record dispatch、等待/写 report、validate、record receipt、record observation、review evidence 还是 repair：先确认 state/path/hash/owner/provenance，再运行当前 Mission Commander command；record 前必须先做 read-only validation，并只使用 validation/status 返回的 `-ExpectedExecutionReportSha256` hash-bound record Apply；record 后只进入 bounded observation evidence review。显式 `gate -ValidateExecutionReport` 与 `gate -Apply -ExecutionReportPath ...` result 还会返回 `missionCommanderDriverReceipt`，把本次 validation/record outcome、refreshed action queue/current run-loop step、下一跳 driver request 与 `expectedReceipt.refreshStatusCommand` 绑定到同一 envelope；status/handoff 的 live snapshot 仍只是只读 handoff，不伪造 explicit command receipt。不要把 contract 阶段的 handoff 当作已授权执行 adapter/heavy tool，也不要在缺少 matching `gateEventId`/`actionId`、current dispatch/report hash 或 current owner/provenance 时手工拼接 record。

### Batch 561 当前实施边界

Batch 561 为 `continue` executor-generation stale-writer guard，当前状态是 implementation in progress，不能当作已验证 runtime 行为。计划中的调用方 contract 是：选定lane后，`continue -WhatIf`与`continue -Apply`都提供当前`-Executor <executor-id>`和`-ExpectedExecutorGeneration <generation>`；runtime在创建run、追加facts、刷新lane `RESUME.md`/checkpoint或修改board前，strict比对durable `currentExecutor`与`executorGeneration`，Apply在mutation边界内再次比对。缺失、不匹配或takeover后的旧generation均fail-closed且zero-write。

该guard只防止旧executor继续写durable lane state。executor takeover仍通过显式`start <name> -Apply -Executor ...`或`reconcile <name> ... -Apply`记录；runtime不自动spawn、停止、轮询或管理session，不执行heavy action，不写authority/confirmed。在runtime与product-path验证完成前，不应把上述参数要求描述为已发布能力。

## 验证标准

- `/rekit`（无子命令，默认 status）和 `/rekit status` 都能正确显示 kit/case 绑定与 pack 来源；attached case 或 nested lane workspace 中未显式传 `-Pack` 时，应使用 case metadata 的 `templatePack`，并在 status 中显示 `packSource=case-metadata`；显式 `-Pack` 显示 `packSource=explicit`，kit-mode 默认 pack 显示 `packSource=repo-default`；case 模式还应显示 pack 是否匹配 metadata `templatePack`，显式 pack 不一致时输出诊断但仍保持显式 `-Pack` 优先，并为 pack mismatch、moved metadata 或 shim drift/missing 输出 bounded status/repair preview next step。authorized-gate 的 default/canonical adapter sidecar 若已存在，status/overview/handoff/start/reconcile/continue 及 durable handoff 会只读复用 strict validation并将`repair-adapter-report`、`ready-to-record-evidence`或exact`evidence-already-recorded`合并到统一Mission Commander queue：typed actions按`gateEventId`逐gate去重，invalid repair或valid显式record优先于普通lane continue，同gate已有exact recorded execution evidence时不重复加入record，changed sidecar会重新进入repair/record，missing sidecar不误删旧evidence；多gate boundary/escalation evidence成为current并停止自主continue，其它adapter action保留但blocked，start/reconcile的显式bounded Apply仍优先于adapter action；但runtime不会自动record或replay。同路径sidecar内容改变后必须重新视为待record，leaf/intermediate symlink或non-regular sidecar必须fail-closed。`status`本身仍只读，`repair -Apply`与adapter evidence record仍需显式确认。
- `gate -ScaffoldExecutionReport` / `gate -DraftExecutionReport` 只提供 sidecar/template repair 或 draft handoff；其中的裸 record template 仅为兼容占位，主 Agent 或 replacement executor 记录 adapter execution observation 前必须先运行 read-only `gate -ValidateExecutionReport`（或消费 `status` / handoff 的 liveValidation），在 `valid=true` 且 `reportSha256` / `recordExpectedReportSha256` 对齐后使用包含 `-ExpectedExecutionReportSha256 <hash>` 的 hash-bound record command。若 validation 后 sidecar bytes 改变，`gate -Apply -ExecutionReportPath ... -ExpectedExecutionReportSha256 ...` 必须 fail-closed 且 `.rekit` zero-write；validation/status 仍只读，record 只写 bounded observation evidence，不执行 adapter/heavy tool，不写 authority/confirmed，不新增 PowerShell runtime logic。
- pack-memory candidate residue 出现在 `release-check` / `status` 时，`packMemoryCandidates` 必须提供只读 `missionCommanderNextActions[]` 与 `missionCommanderActionQueue`，text/default 第一屏同步打印 pack-memory action queue/current action/next action reason/boundary；主 Agent 或 replacement executor 优先消费 queue 的 current action，而不是手工遍历 `packs[]` 拼命令。Case-local `/rekit` 默认 status 与 `status -Format json/text` 都应在发现 `promote -CreateCandidates -Review` 的 packet-derived review workspace 后重算 queue，使 missing proof current action 使用 concrete `-PacketPath ...` draft command；若只能从 release-only handoff 推断，则保持 `<packet.json>` 占位和 review-required boundary。该 queue 仅是 handoff，不 merge/cleanup/provision/verify/retire、不写 proof/facts/authority/confirmed、不执行 heavy tool、不新增 PowerShell runtime logic。
- `/rekit doctor` 通过，且 managed docs、policy、tooling 文件预算未超限。
- 旧 case 同步前先看到 `.rekit/reviews/<timestamp>-sync/summary.md`、`packet.json` 和 bounded diff。
- `release-check -Format text` 应直接输出 release-check summary、CI release gate inventory + `inventory-ready-not-remote-ci-green` boundary、required commands/docs、heavy action inventory、Go-native public surface command groups/profiles/boundaries/policies/facade prerequisites、PowerShell deprecation command/module/freeze/blocked/retired/public façade/reference inventory、public façade removal prerequisites/plan/deletion gates/execution/boundary/recovery/docs/impact migration targets、case shim readiness、public default docs readiness、release handoff readFirst/signals/latest batch（含 localValidationReady、releaseCheckReady、remoteReleaseGate、evidence、nextAction）/release notes/pack maturity/validation/known gaps/next actions 与 full known-gap detail，保持 default/table/tsv compatibility、JSON inventory compatibility、read-only/no authority/confirmed/no-heavy/no PowerShell runtime logic，并明确 `ciReleaseGate.ready` 只表示 workflow/inventory ready，不能替代真实远程 CI green。
- `status -Format text` 应直接输出 mutation/mode/targetProvided/pack/target/runtimeRoot/templateRoot、manifest summary、case metadata 与 case shim readiness counts/warnings；`doctor` / `validate -Format text` 应直接输出 command/mutation/valid/mode/pack/target/row count/summary 与逐 row file/bytes/limit；`packs -Format text` 应直接输出 pack count、逐 pack manifest/maturity/schema/managed/template/local/promote/tooling/prompts/routes/heavyToolGates/authority/version/description 与 heavy action names，保持 default/table/tsv compatibility、JSON inventory compatibility、read-only/no authority/confirmed/no-heavy/no PowerShell runtime logic。
- `attach` / `repair` / `init` / `bootstrap` 的 `-WhatIf/-Apply -Format text` 应直接输出 lifecycle summary、逐 write path/kind/action/source/target/backup、blocked actions 与 nextSteps，保持 default JSON compatibility、case write semantics、case durable schema、no authority/confirmed/no-heavy/no PowerShell runtime logic；`bootstrap` 兼容入口必须保持 bootstrap command identity。
- `sync -Format text` 与 `promote -Format text` 应直接输出 review plan summary、逐 item/tooling item action/risk/recommendation、paths、hashes、deny violations 与 replacement counts，保持 non-apply review-first、default JSON compatibility、review artifacts 行为不变、no authority/confirmed/no-heavy/no PowerShell runtime logic。
- `sync -Apply/-WhatIf -Format text` 应直接输出 mutation/applied/writes/backupRoot、逐 write path/kind/action/source/target/backup 与 nextSteps，保持 kit -> case review-first、default JSON compatibility、no authority/confirmed/no-heavy/no PowerShell runtime logic；`update -Apply/-WhatIf` 复用同一写入路径时，JSON/text command identity、line prefix 与 WhatIf nextStep 必须保持 `update`。
- `promote -Apply/-WhatIf -Format text` 应直接输出 mutation/applied/changed/blocked/skipped/writes/requiresReview/cleanup/backupRoot、逐 write path/kind/action/source/target/backup/reason、pack validation rows、denied actions 与 nextSteps，保持 case -> kit review-first、default JSON compatibility、candidate workflow 不变、no authority/confirmed/no-heavy/no PowerShell runtime logic。pack-memory final proof完成后应返回canonical workspace retirement preview；retirement必须是独立strict WhatIf→`ExpectedRetirementSHA256` Apply，绑定decision authority、final proof、provision intent/receipt及fresh/attached exact trees，保留repo-local durable intent/receipt并支持crash resume/exact replay。missing/extra/different tree object、proof/provision drift、forged receipt或receipt后workspace重现必须fail-closed且不得自动再次删除。release/status 的 closure 判断还必须继续要求每个 receipt action 的 strict `candidate-cleanup-proof`；`-DraftReviewProof -ProofPath` 写出的自定义 `*.candidate-cleanup-proof.{md,json,txt}` 可以被 strict discovery 识别，但 final retirement receipt 不能替代 per-action cleanup proof。不写facts/authority/confirmed、不执行heavy tool、不新增PowerShell runtime logic。
- `overview` 能显示主线、功能支线、共享事实统计和 Mission Control brief；brief 必须让主 Agent 不读完整 ledger 也能看到 ready/blocked lanes、pending gates、authorized gates、open decisions、interventions、next agent actions 与 escalations。overview JSON/text 还应暴露逐 lane `laneExecutorActions[]`、`missionCommanderActions[]` action index（state/primary/follow-up/boundary）、compact `executionEvidenceReviewSummary`、顶层 `executionEvidenceReview[]`（含 followThrough outcome when/evidence；终结性的 related verification / decision notes 会关闭 review queue，并阻止 exact `evidence-already-recorded` current action 回流）与主 Agent 可直接按序消费的 `missionCommanderNextActions[]`（source/blocked/requiresReview/reasons/boundary/command），以及把 current/unblocked/blocked/review/follow-up 直接汇总给替换 executor 的 `missionCommanderActionQueue`，并让 next steps 与 next actions 先处理 execution evidence review、reconcile、pending gate、open decision，只为 ready lane 建议 continue，避免 blocker summary、evidence review queue 与继续建议冲突。overview、start/continue/handoff/gate/reconcile JSON envelope、continue run artifacts、project handoff、lane handoff、lane `RESUME.md` 与 typed `checkpoints/latest.json` 应使用同一 Go mission snapshot / blocker 语义：只有 pending gate、effective open intervention、open candidate/decision 会让对应 lane blocked；start/gate/reconcile JSON envelope/text、continue JSON envelope/run artifacts、overview/project/lane handoff JSON/Markdown 与 lane-local resume/checkpoint 还应持久化或投影同一 lane Mission Control brief 与 executor action snapshot，包含 blocked/ready、基于 typed facts 的 pending gate / open intervention / open decision counts、blocker reasons、reconcile/pending-gate/open-decision requirements、resume/handoff command、next actions、escalations 与 `missionCommanderAction` state/prompt/primary/follow-up/boundary；project handoff 文本、lane handoff 与 `RESUME.md` 必须让主 Agent或替换 executor 不回查 JSON 也能直接看到 commander primary/follow-up/boundary；`gate -WhatIf` 应同时暴露当前 executor action 与写入 request 后的 would executor action，并像 `start` / `reconcile` 一样在 normal request path 输出 top-level `missionCommanderAction` 与 `missionCommanderNextActions[]`：pending-gate preview 投影 `needs-gate-apply` bounded request-ledger write，authorized-gate preview 投影 `needs-authorized-gate-apply` durable authorization decision write，preview follow-up 在 apply/refreshed state 前保持 blocked/requiresReview；pending-gate apply 后投影 refreshed `needs-gate-decision` handoff/continue-WhatIf，authorized-gate apply 后投影 `ready-for-execution-report-contract` 并用实际 `GateEventId` 给出 report contract handoff；CLI text 也应打印 normal gate request `mission commander next action` lines；`authorized-gate` 只是已记录的 durable autonomy authorization decision，应在 `missionBrief.authorizedGates`、overview、handoff、continue digest/status、gate executor action 与 lane-local resume/checkpoint gate snapshot 中可见，并直接携带 execution boundaries、gate `eventId` 与可复制 report contract handoff，让替换 executor 不重扫 request ledger 即可读取 `gate -ExecutionReportContract`；`gate -ExecutionReportContract` 与 `gate -ValidateExecutionReport` JSON 还应暴露 compact `reportSummary`（state、gate/action/lane、report/default path、reportPresent、valid/recordReady/recordBlocked、validation/repair/main-escalation flags、allowed status/output/stop/candidate counts、repair/record-blocked/escalation hint counts、outcome/next-action/review counts、current action、report status/adapter id/actualBudget、refs/boundary hits、failure code/stage、no-heavy/no-authority boundary）、adapter sidecar `missionCommanderAction`（validation/record/repair state、primary/follow-up commands、read-only/no-heavy/no-authority boundary）、`missionCommanderNextActions[]`（read-only validate、valid=true 后 record、handoff、repair hints、rerun validation 的 source/reasons/boundary/command）与 `missionCommanderActionQueue`（summary/counts/current/unblocked/blocked/reviewRequired/followUp buckets）与 `authorizedExecutionFollowThrough`（contract 的 write-and-validate/valid-record/invalid-repair outcomes、validation valid record 或 invalid/missing repair outcomes、record normal/boundary/duplicate evidence review outcomes，并携带 actionQueue/boundary），并在 text/default 输出中同步打印 adapter report contract/validation identity、case-relative validate/record command、liveValidation handoff（invocation cwd、authorized workspaces、sidecar template、pack tooling adapter candidates、read-only/strict-record notes）、Mission Commander action、action queue summary/counts/current、follow-through outcome lines（含 outcome when/evidence）与 `mission commander next action` lines，让主 Agent 先确认 `valid=true` 再记录 bounded observation evidence，contract 阶段不得把 blocked record handoff 当作可立即执行，invalid/missing sidecar validation 不得推荐 `-Apply` record，且 validation text valid sidecar handoff 应直接输出 normalized report identity、actualBudget、output/evidence refs、boundaryHits、escalation、summary 与 adapterContext candidate/selected adapter provenance，validation text repair hints 应直接输出 code/stage/fields/allowedValues/allowedOutputPaths/allowedStopConditions/maxBytes/escalateToMain/detail/evidence/boundary，并把 same repair evidence/boundary 投影到 Mission Commander next-action reasons/boundary，避免 review valid sidecar 或修 sidecar 时回查 JSON；`gate -Apply -GateEventId ...` 的 execution evidence record result 也应暴露 evidence-specific `missionCommanderAction`，区分 `ready-for-evidence-review`、`evidence-already-recorded` 与 `needs-main-escalation`，并同步输出 compact `executionEvidenceReviewSummary`、`executionEvidenceReview[]`、`missionCommanderNextActions[]` 与 `missionCommanderActionQueue` / CLI text action queue summary/counts/current 和 `mission commander next action` lines，让主 Agent 在刚记录 bounded observation evidence 后不用再跑 overview/handoff/continue/resume/checkpoint 就能消费 evidence-first handoff/overview/continue ordering；record result CLI text 还应直接输出 observation subject/summary/target、recordRequired、executionReportPath、actualBudget、outputRefs/evidenceRefs、boundaryHits 与 escalation，避免 evidence review 时回查 JSON；duplicate execution evidence 必须把 `evidence-already-recorded` 与 `duplicate record did not append observation evidence` boundary 投影到 review queue 和 next actions，只保留 review handoff/overview，不推荐 `/rekit continue ... -WhatIf` 或 lane-level `missionCommanderActions` continue；且先 review output/evidence refs、保持 no-heavy/no-authority/confirmed boundary；但它不作为 pending-gate blocker；project/lane handoff JSON、project/lane handoff Markdown、`/rekit continue` status/digest、lane `RESUME.md` 与 typed `checkpoints/latest.json` 还应投影已记录 authorized execution observation evidence 的 compact `executionEvidenceReviewSummary` 与 `executionEvidenceReview[]`/review checklist，包含 gate eventId、status、action、outputRefs/evidenceRefs、review/handoff command、`missionCommanderAction` state/primary/follow-up、`followThrough` state/outcomes/actionQueue、outcome when/evidence、no-replay/no-authority boundary，并让 normal review、boundary-hit/escalated/main escalation 与 duplicate replay evidence 明确映射为 recorded/boundary/duplicate follow-through outcome，boundary-hit/escalated evidence 明确进入 main review 且不推荐 autonomous continue；`promote -CreateCandidates` 可与 `-Review` / `-ReviewOutputDir` / `-PacketPath` / `-DiffPath` 组合，在同一次 case-local product-path 调用中写出 durable candidate review workspace：packet 必须同时封装完整 `candidateResult.reviewPlan` 与 generic promote `reviewInput`，workspace 必须包含 managed-doc bounded diff、tooling sanitized preview、短 `summary.md` 与 combined diff，JSON/text terminal 返回 workspace paths；replacement executor 应能从 packet 直接继续 decision/cleanup/reconsume review，而不重新运行另一套 generic promote review。workspace 只记录 review evidence，不 merge/cleanup candidate、不更新 pack source、不运行 doctor/init/reconsume、不创建 expected proof、不写 authority/confirmed、不执行 heavy-tool；candidate review workspace 还应支持主 Agent提供 strict decisions JSON 后运行同一 packet 的 `-WhatIf` → `-Apply` 决策闭环：decision 必须绑定 exact packet SHA-256、candidate SHA-256、accepted managed target SHA-256，以及每个 evidence ref 的 path + SHA-256；runtime 必须验证 canonical candidate/tooling roots、canonical index、manifest managed target、packet review item/create-candidate write/index mapping和全路径 symlink boundary。`accept` 只允许 managed-doc，写 pack target 前先 stage candidate/target/index backups；`reject` / `superseded` 只清理 reviewed candidate与对应 index entry；Apply中途失败必须 rollback或返回明确 recovery envelope，tooling candidate仍只允许人工 catalog/recipe merge。`promote -CreateCandidates` 的 `reviewPlan` 还应暴露 compact `reviewSummary` 以及 pack-memory candidate `missionCommanderAction`、`decisionChecklist[]`、`decisionFollowThrough[]`、`mainAgentExecutionPlan[]`、`missionCommanderNextActions[]` 与 `missionCommanderActionQueue`，让主 Agent 可先从 summary 确认 candidate/tooling/index、review/cleanup/reconsume artifact、current/next action 与 boundary，再区分 WhatIf preview、actual candidate review 与 blocked/no-op plan，并按 per-candidate review/accept/reject/superseded/cleanup/verification follow-through 处理；`decisionFollowThrough[]` 应将 accepted、rejected、superseded、blocked 与 not-needed outcome 直接映射到 actions、cleanupActions、verificationCommands、expected/evidence/boundary，accepted tooling outcome 必须包含 doctor、fresh case 与 attached case reconsume；`cleanupTargets[]` 应携带 candidate cleanup action 与 `indexPath` 处理，`mainAgentExecutionPlan[]` 应把 materialize、review decisions、cleanup、pack doctor、fresh-case reconsume 与 attached-case reconsume 收口为 bounded commands/expected/evidence/boundary 且明确 runtime 不执行 merge/cleanup/init/doctor，`missionCommanderNextActions[]` 应把 decisionChecklist review、candidate cleanup、pack doctor、fresh-case init/doctor 与 attached-case doctor 投影为 source/reasons/boundary/command 的可消费 next-action list，`missionCommanderActionQueue` 应直接给出 current/review/cleanup/reconsume action queue，CLI text（包括 WhatIf preview 与 actual create-candidates）也应输出 promote candidates reviewItems、decisionChecklist、mainAgentExecutionPlan、decision follow-through、decision outcome when/evidence、follow-through boundaries、cleanup/reconsume、cleanup action detail（path/candidatePath/indexPath/action）、top-level reconsume commands/boundaries、reconsume check command/boundary detail、reconsume check when/evidence、action queue 和 `mission commander next action` lines；`reconsume.verificationChecklist[]` 应在 accepted tooling merge 后列出 doctor、fresh case 与 attached case reconsume 的 commands/expected/evidence/boundary，并在 text path 直接暴露每个 check 的 when/evidence、command detail 与 boundary detail；被 `resolvesEventId` resolution 关闭的 intervention 不应继续阻塞。
- `overview -Format text` 应直接输出 overview summary、lane summaries、fact counts、Mission brief、逐 lane executor action、Mission Commander action queue/current/buckets、Mission Commander next actions、execution evidence review、section counts/events 与 next steps，保持 table/tsv legacy render、JSON inventory compatibility、read-only/no authority/confirmed/no-heavy/no PowerShell runtime logic，并让 case-local/nested cwd replacement executor 不回查 JSON 或 prose render 也能确定当前 action、ready/blocked lanes、evidence review 与 ledger section state。
- `note -Format text` 应直接输出 append summary、target、event fields/evidence refs/related refs、Mission brief、当前 executor action、当前 Mission Commander next actions，以及 `-WhatIf` would executor action / would Mission Commander next actions；actual append 仍写 append-only fact event，`-WhatIf` 不写入，默认空 format 保持 JSON compatibility，`note -List` 的 table/text/tsv legacy summary 与 JSON event inventory 不变，并保持 strict duplicate eventId/lane guard、no authority/confirmed/no-heavy/no PowerShell runtime logic。
- attached canonical `plan-subagents` packet应生成同目录`packet.integrity.json`，strict绑定sha256、packetId、targetLane、canonical packet path与exact bytes；status/overview/handoff/start/reconcile/continue在提升reviewer命令前必须验证receipt。missing/drift/unknown/trailing或truncated packet必须保留lane/packet provenance并投影blocked `reviewer-packet-integrity-invalid`；当sidecar仍可读但packet本体unreadable/malformed时，handoff evidence还应保留具体read/decode error，避免replacement executor只能看到泛化decode failed；同一status/handoff/continue reviewer dispatch handoff还应直接投影`packetRetirementPreviewCommand` / `nextActionPacketRetirementPreviewCommand`与text `packetRetirementPreview=`，让主Agent在判定该invalid packet已废弃时可先运行`plan-subagents -RetireInvalidReviewerPacket -PacketPath ... -Lane ... -Actor ... -Reason ... -WhatIf`复核exact invalid packet/integrity hash与size，再执行返回的、携带`-ExpectedPacketSha256`与`-ExpectedIntegritySha256`的显式`-Apply`命令写同目录`packet.retirement.json`；不静默丢失active reviewer work、不自动修补packet/receipt；legacy无sidecar packet继续兼容。若strict sidecar仍可读并提供可信packetId/targetLane，retirement preview/Apply仍只关闭该exact invalid packet snapshot。receipt必须strict绑定attached repo/case/pack、packet/lane/canonical paths、exact packet与sidecar hash/size、actor/reason/timestamp和no-delete/no-heavy/no-authority边界；exact replay幂等，forged receipt、symlink/non-regular path或任一bytes drift均恢复blocker。retirement不得删除、覆盖或修补packet/sidecar；missing/malformed sidecar没有可信lane provenance时必须拒绝retirement并重新生成canonical packet。
- `plan-subagents` planning result / packet / `summary.md` / `-Format text` 应直接输出 compact reviewer orchestration summary（mode、target lane、reviewer/dispatch counts、owner binding、intakeAvailable/collectionAvailable/dispatchOnly、action queue counts、first dispatch、current action、next actions、boundary），避免 replacement executor 在 dispatch 前解析 nested `reviewerOrchestration.dispatches[]`、lifecycle、action queue 或 shard handoffs 才能接续；`plan-subagents` reviewer-intake JSON / `-Format text` 应直接输出 compact intake progress summary（writeback status、ready/applied、dispatch progress、blocked/repair counts、postValidation presence/totals、reviewer writebacks、action queue counts、current/next actions、boundary），避免 preview/blocked/complete/already-complete 后手工拼 `orchestrationSnapshot`、`blockedReasons`、`repairGuidance`、`postValidation.summary` 和 action queue；写回前的 status/handoff/continue、lane `RESUME.md`、checkpoint 与 digest 应投影 open `reviewerDispatchIntakeHandoffs` / compact summary，并直接给出 `nextActionRunbookSteps[]` 与 per-shard `runbookSteps[]`：按 waiting → capture reviewer JSON input WhatIf/expected-input-hash Apply → staging WhatIf/expected-source-hash Apply → collection WhatIf/Apply → batch/single intake WhatIf/Apply，以及 owner adoption、prompt repair、result recovery等 blocker状态列出最小可执行步骤，使 replacement executor 不必打开 packet、summary 或 nested dispatch JSON 才能接续；写回后的 status/handoff/continue、lane `RESUME.md`、checkpoint 与 digest 应进一步提供 compact `reviewerWritebackSummary` / reviewer writeback summary lines（verification/decision counts、lane count、latest reviewer session/result/shard/packet/route、owner binding、risks/conflicts/route output flags、latest evidence refs、no-heavy/no-authority/no-spawn boundary），避免 replacement executor 逐条扫描 `reviewerWritebacks[]` 才能复核 reviewer provenance；reviewer-intake text 在 verification / decision writeback preview、apply recovery 与 already-complete 路径中应直接复用 note text handoff，输出 nested `note.AppendResult` 的 current/would/post commander delta、event fields、evidence refs、duplicate reason 与 note path；post-validation 还应输出 compact summary（overview verification/decision totals、doctor rows、lane/executor action state、reviewer writebacks、current action、next actions、boundary），且 reviewer-intake 总 summary 与 postValidation summary 的 current/next action 必须保留 lane、label、action id、reasons 与 boundary 并在 text path 直接打印，避免 replacement executor 解析 nested overview/handoff/doctor JSON 或完整 action queue 才能接续；保持 planning/reviewer-intake JSON compatibility、verification-before-decision writeback、partial retry boundary、no auto-spawn/no authority/confirmed/no-heavy/no PowerShell runtime logic。
- overview 与 project/lane handoff 的 top-level `nextSteps[]` 应直接提升 `executionEvidenceReview[]` 的 commander guidance：先 review gateEventId 对应 outputRefs/evidenceRefs，再执行 evidence handoff；如果 review queue 中存在 boundary-hit/escalated/escalation evidence，则必须显示 main-review stop guidance，并抑制 autonomous `continue` 建议，即使 authorized-gate 本身仍是非阻塞且 lane executor action 为 ready。overview text/JSON、project/lane handoff JSON/Markdown、`/rekit continue` JSON/run artifacts、lane `RESUME.md` 与 typed `checkpoints/latest.json` 的 `missionCommanderNextActions[]` / `Mission Commander next actions` 投影还应把 evidence review commander primary/follow-up 与 lane commander primary/follow-up actions 收口到单一 next-action list；overview 还应用 `missionCommanderActionQueue` / `Mission Commander action queue` 将当前 action、unblocked/blocked/review-required/follow-up counts 和 action buckets 直接投影给主 Agent / replacement executor：evidence review source 排在前面；lane primary action 后继续列出 `missionCommanderActions.followUp`，ready lane follow-up 保留 handoff，blocked lane follow-up 保持 blocked/requiresReview 并携带先解决 blocker、continue 只能 `-WhatIf` 的 reason 与 do-not-continue boundary；blocked mission 或 evidence main-review 时过滤 evidence `continue` follow-up，保留 review/reconcile command、reason 与 no-replay/no-authority boundary，避免主 Agent 或替换 executor 从 `executionEvidenceReview[]`、`missionCommanderActions[]`、`laneExecutorActions[]`、`executorAction` 和 `nextSteps[]` 多处手工拼接执行顺序。project handoff Markdown 的逐 lane 行、lane handoff 新会话开场、continue run digest 与 lane-local resume/checkpoint 也必须同步这一优先级：先列 evidence next action，把 ready lane continue 降级为 review 后候选，并在 continue run digest 与 lane-local resume 中先展示 compact execution evidence review summary，再在 execution evidence follow-through outcome 中直接展示 when/evidence，明确当前不要执行 autonomous continue。
- `note -List` 与 note append strict duplicate eventId 去重 / lane guard、gate lane guard、doctor JSONL validation、workstream board snapshot、shared facts 写入/读取路径、facts path/read/append、continue facts promotion 与 duplicate eventId scan、handoff/continue facts snapshot、gate duplicate eventId、note ledger kind 顺序、workstream lane/workspace local JSONL 文件集合与 path helpers、facts JSONL / fact-file mapping、JSONL append、board 读取、open-lane 过滤和 known-lane 诊断应复用同一 Go helper source，避免 note、overview、doctor、handoff、start/continue/gate 对 ledger kind/file/path、workstream local JSONL file/path、board JSON、JSONL/facts reader/append 或 lane guard 维护并行实现。note 写路径还应返回 shared typed action / Mission Commander action delta：`-WhatIf` 输出当前 `executorAction` / `missionCommanderAction` / `missionCommanderNextActions[]` 与内存模拟 append 后的 `wouldExecutorAction` / `wouldMissionCommanderAction` / `wouldMissionCommanderNextActions[]`，actual append 输出 post action 与 post `missionCommanderNextActions[]`，duplicate eventId 只输出未改变的 current action 与 current commander projection，不生成 misleading would guidance；candidate、defer/open decision、open intervention 与 pending-gate request 应分别投影 open-decision、intervention 与 pending-gate blocker，非 blocker kind 保持 readiness 不变，malformed ledger 必须在投影或写入前 fail-closed。
- `continue main` 与 `continue <name>` 明确接手不同工作线；无参数 `continue` 不应在多工作线时盲猜。`start` / `handoff` / `continue` / `reconcile` apply 后的 `nextSteps`、CLI text 与 handoff Markdown 必须使用 lane-local executor actions：blocked lane 不得推荐 continue 或泄漏其它 ready lane 的 continue，ready lane 才建议自己的 continue，paused/closed/unready lane回到 handoff/read-only。`start -WhatIf/-Apply -Format json` 还应像 handoff/continue 一样输出 top-level `missionCommanderAction` 与 `missionCommanderNextActions[]`；创建 lane、进入已有 lane 或 executor claim/takeover 的 preview 必须把 start apply primary command 标记为 review-owned，并把 continue/handoff follow-up 保持 blocked/requiresReview，直到 apply 成功并刷新后的 executor action 仍 ready；start apply 后则直接投影 ready/blocked lane 的 current commander next actions。`reconcile -WhatIf/-Apply -Format json` 同样应输出 top-level `missionCommanderAction` 与 `missionCommanderNextActions[]`：preview 使用 selected intervention 的 actual eventId 生成 concrete apply command，并保留适用的 executor/actor/reason，primary reconcile apply action 是 review-owned bounded write，continue/handoff follow-up 在 apply 成功并刷新后的 executor action 仍 ready 前保持 blocked/requiresReview；apply 后直接投影 refreshed ready/blocked lane 的 current commander next actions。blocked `continue -WhatIf/-Apply` 遇到 effective open intervention 时也必须 zero-write 返回 `reconcileHandoffs[]`，CLI text 同步打印 `continue reconcile handoff` / boundary / evidence，使 replacement executor 不必切回 status/handoff 才能拿到 concrete reconcile WhatIf→Apply handoff。start/continue/handoff/reconcile JSON/text、lane `RESUME.md`、typed checkpoint、continue run status/digest 与 project/lane handoff Markdown 还应同步输出 `missionCommanderActionQueue` / `Mission Commander action queue`，直接给出 summary/counts/current 与 unblocked/blocked/review/follow-up buckets，让替换 executor 不需要从 nested `executorAction.missionCommanderAction`、`missionCommanderNextActions[]` 与 `nextSteps[]` 手工拼接 start/takeover/continue/handoff/reconcile 顺序。CLI text 同步打印 `mission commander next action` lines。
- 功能支线只写自己的 workspace、outbox、candidate/request，不直接写 confirmed CSV、routine IR 或长期 handoff。
- 长期成员身份绑定 lane，不绑定旧 session；旧会话上下文污染或用户希望重开时，新会话应读取 handoff / packet / evidence 接手同一 lane。需要显式登记或接管当前 lane executor 时，主 Agent使用 `/rekit start <name> -Apply -Executor <session-id> -Actor <actor> -Reason <reason>` 写入 `currentExecutor`、`executorGeneration` 与 takeover metadata；当 `<name>` 解析到已有工作线（例如 `main`、`feature-login` 或 `login`）时，start 进入该 durable lane 而不是新建并行 lane，因此 replacement session 可用 `/rekit start main -Apply -Executor <new-session> ...` 接手主线并刷新 resume/checkpoint/events；runtime 只记录和投影该声明，不自动 spawn、停止或监控 session。
- 用户可随时进入 lane 打断、纠错、改向或硬切模型；lane 继续时要用 `/rekit reconcile <name> -InterventionId <eventId> -Apply` 将干预写成 append-only resolution event，并刷新 durable lane executor/resume/checkpoint/board state。
- `plan-subagents` planning mode 只写 review artifacts，不自动 spawn reviewer；`packet.json` / `summary.md` 的 `ownerBinding`、`shardHandoffs[]` 与 `reviewerOrchestration` 提供 target lane current executor / generation / last takeover snapshot、read-only dispatch prompt、strict `reviewerResultContract`、decision/conflict mapping、`reviewerIntakeCommands`、`reviewerIntakeCommands.repairGuidance[]`、blocked repair boundaries、writeback sequence、多 reviewer dispatch/result root、lifecycle 与 result/intake completion criteria。planning result、packet `reviewerOrchestration` 与 summary 还应输出 top-level / nested `missionCommanderAction`、`missionCommanderNextActions[]` 与 `missionCommanderActionQueue`（summary/counts/current/unblocked/blocked/review/follow-up buckets），把 reviewer dispatch、result collection、intake preview/apply ordering、dispatch-only unattached target 与 empty-plan replan guidance 收口为主 Agent 可直接消费的 source/blocked/requiresReview/reasons/boundary/command 列表；`plan-subagents -Format text` 也应打印同一 commander action、action queue、next-action lines、reviewer orchestration lifecycle（scope、packet/result identity、owner、inputs、mustPass、runtime boundary、completion criteria）与 `shardHandoffs[]` terminal handoff（reviewerResultPath/items/expectedOutput、owner binding/requiredForIntake/current executor/generation/runtime session boundary/takeover provenance、reviewer writeback、main-agent next action、read-only boundary、dispatch prompt、strict `reviewerResultContract`、可复制 reviewer result JSON skeleton、routeOutput required field hints、`reviewerIntakeCommands` preview/apply handoff、preview checks/blocked outputs、intake checklist、decision mappings、conflict handling、writeback sequence `mustPass[]`/command binding source、post-review merge、completion/failure criteria），写入的 `summary.md` review artifact 也必须在 shard handoff 区域保留 reviewerResultPath、可复制 reviewer result JSON skeleton、routeOutput field hints 和 packet/route/shard binding guidance；且生成给 short-lived read-only reviewer 的 packet shard prompt / `dispatchPrompt` 本身也必须要求返回单个 `ReviewerResult` JSON object、内嵌 reviewer result skeleton、routeOutput field hints 与 top-level decision/confidence/evidenceRefs 绑定说明，明确不要只返回 routeOutput alone；这样可避免 terminal replacement executor 或 reviewer 为 reviewer dispatch / intake path 构造 strict reviewer result、解析 JSON packet 或打开 summary。attached-case intake preview/apply next actions 必须保持 blocked/requiresReview，直到 reviewer JSON 已通过 capture/staging/collection 到 canonical result path、preview 返回 `readyForWriteback=true` 且 evidenceRefs 已被主 Agent复核。每个 shard handoff提供 typed `agentToolRequest`；只有严格位于`.rekit/reviews/<单层review>/packet.json`且result/candidate geometry一致的canonical case-local packet才同时提供`reviewerResultCandidatePath`与`reviewerCollectionCommands`。主 Agent使用`agentToolRequest`调度短命read-only reviewer，harness实际接受session后运行`-RecordReviewerDispatch -WhatIf`→returned expected-hash Apply；reviewer只返回包含同一`reviewerSession`的单个JSON object且不写文件/ledger。session结束并保存exact JSON input后运行`-RecordReviewerCompletion -WhatIf`→returned expected-dispatch/input-hash Apply；只有current owner generation下的successful completion receipt才允许canonical packet用 `-CaptureReviewerResultSource` 将 exact JSON input 发布到 packet-derived source，再用 `-StageReviewerResult` 发布到 candidate path，随后运行 collection WhatIf 复核 packet/route/shard/items、case-local evidence、route output 和 blocked 边界，再显式 Apply；collection只向 immutable packet 派生的 canonical `reviewerResultPath` 发布 exact bytes，不覆盖不同结果，相同 replay 幂等。若candidate正确但canonical result已存在不同regular bytes，durable handoff提升typed result recovery WhatIf；主Agent必须复核exact candidate/result hashes，再显式Apply把冲突bytes移动到hash-addressed quarantine并写strict durable intent/receipt。中断于quarantine与receipt之间时，同一WhatIf→expected-hash Apply从exact intent+quarantine补齐receipt；forged path/hash、changed bytes或已有verification/decision均fail-closed。recovery不生成verdict、不写或撤销facts，完成后仍须重新执行collection WhatIf→Apply和独立intake。attached custom/noncanonical packet不产生collection command，仍可将JSON直接放到`reviewerResultPath`后运行strict direct或packet batch intake；out-of-case packet只dispatch，attach/init后必须重新生成canonical packet。canonical result到位后再调用packet-level reviewer intake `-WhatIf/-Apply`；runtime 严格绑定 packet/route/shard/items，验证 lane owner binding、case-local evidence 与 route output contract，若 packet 要求的 executor generation 已被 takeover 则写 facts 前 fail-closed，按 verification-before-decision 顺序写 facts，并返回 top-level `missionCommanderAction` / `missionCommanderNextActions[]` / `missionCommanderActionQueue`、`orchestrationSnapshot`、overview/handoff/doctor post-validation；reviewer-intake `-Format text` 应同步打印 intake status、reviewer result identity/decision/confidence/session/recommendedVerdict、summary/items/evidenceRefs/risks/conflicts、sorted routeOutput key/value、blocked reason、repair guidance/action/evidence/boundary、verification/decision writeback checkpoint（applied/eventId）、verification/decision nested note event detail（kind/eventId/applied、subject/target/lane/confidence/evidenceRefs、summary、verdict/decision/reason、packet/route/shard、reviewerSession、owner binding）、verification/decision/post-validation、postValidation overview totals、handoff lane/queue/current、post-validation next actions、commander action、action queue 与 next actions；blocked / event-id-collision / post-validation failed repair guidance 必须让主 Agent 不解析 JSON 也能看到修复动作、证据和不要 apply / no-heavy / no-authority 边界；partial writeback recovery 必须让主 Agent 不解析 JSON 也能看到已落账 verification、未落账 decision 与同一 apply command 重试边界；nested verification / decision `note.AppendResult` 也保留 note-level current/would/post commander delta。verification / decision events 与 lane handoff 会记录 reviewer session provenance、owner binding snapshot、reviewer decision/recommendedVerdict、risks/conflicts 与 normalized routeOutput；这些字段也会进入 downstream `reviewerWritebacks[]`、overview detail、status/handoff/continue、lane `RESUME.md`、checkpoint 与 digest，并在 status/handoff/continue、lane `RESUME.md`、checkpoint 与 digest 通过 `reviewerWritebackSummary` / reviewer writeback summary lines 压缩 latest reviewer session/result/shard、counts、evidence refs 与 no-heavy/no-authority/no-spawn boundary，让 replacement executor 不必重开 reviewer result JSON 或逐条扫描 writebacks 即可复核 reviewer provenance。reviewer 不写文件或 ledger；intake 不写 authority/confirmed、不执行 heavy-tool，最终 merge decision 仍由主 Agent拥有。多个 shard 的 result 已放入 packet 指定路径后，attached-case planning result、`packet.json`、`summary.md` 与 Mission Commander action queue 会直接提供 packet-level `batchPreviewCommand` / `batchApplyCommand`，主 Agent无需逐 shard拼接 intake 命令；status/handoff/continue 与 durable resume/checkpoint/digest 也会在 ready result 出现时提升同一 batch preview，旧 packet 无 batch fields 时回退 single-result preview，out-of-case dispatch-only packet 不生成 runnable batch commands。若packet创建后lane executor发生takeover，status/handoff会先提升`reviewer-packet-owner-adoption-required`，主Agent必须使用同一packet运行`-AdoptReviewerPacket -WhatIf`再显式`-Apply`；case-local receipt绑定exact packet hash、repo/case/pack/lane及新旧owner generation，保持packet与reviewer result identity immutable，再次takeover会使receipt失效。任何waiting/ready/partial/stale/blocked reviewer packet仍open时，continue WhatIf/Apply都fail-closed且不创建run、不写facts、不刷新resume/checkpoint/board。主 Agent仍必须显式执行 batch WhatIf、复核 ready/waiting/evidence/blocked state 后才 Apply：runtime 只发现非空、非 symlink ready results，要求 result `shardId` 绑定当前 packet handoff，按 packet shard handoff顺序复用同一 strict single-result intake，保持每 shard verification-before-decision 与幂等重试，并在首个 blocked、partial writeback、event collision、post-validation failure 或 strict intake error 处停止；strict error 返回非成功命令状态并保留 recovery envelope，missing/empty paths 仅计入 waiting 且不会建议继续 lane，不自动 spawn、轮询或监控 reviewer。
- confirmed / authority 写入仍需要更严格 gate；lane 文档/packet 只表达授权意图，动态调试、注入、patch、dump、hook、full trace、网络、exploit replay 等外部副作用只有在 strict durable autonomy profile + `authorized-gate` decision 完全覆盖 action、exact target、typed budget、stop conditions、output paths、record/notify 和 grant/expiry 时才可由 executor 执行，否则必须升级。

## 风险与注意事项

- 不要把 `docs/vision.md` 中的长期目标理解为当前已经具备全自动脱壳、全自动逆向、自动漏洞挖掘、自动恶意样本分析或通用自动渗透能力。
- 不要在 kit 仓库里创建真实 case state；验证 `init/attach/sync/promote` 时只用临时 case。
- 不要将真实样本、客户信息、RVA/VA、trace/dump、artifact 路径或绝对 case 路径写回 pack 模板。
- `attach` 只绑定 metadata 和 shim，不会直接同步 managed docs；旧 case 接入后还需要 `/rekit sync` review。
- `sync -Apply` 会写 managed docs / managed block，必须先 review 再确认。
- `promote -Apply` 会写回 pack，风险更高；优先使用 `promote` review 或 `-CreateCandidates`。
- 多 Agent 可以并行读和产出候选，但不要并发写同一个 IDB、confirmed CSV、handoff 或 authority 文件。

## 1. 新架构如何使用

### 1.1 维护 kit 仓库

维护者主要改五层：

| 层 | 路径 | 什么时候改 |
|---|---|---|
| Skill UI | `.claude/skills/rekit/SKILL.md` | 调整用户可见 `/rekit` 语义和确认规则 |
| Runtime | `cmd/rekit/**`、`internal/rekit/**`、retained façade `rekit/rekit.ps1` | 调整确定性命令、状态、review、sync/promote 行为；不新增 PowerShell runtime logic |
| Pack | `packs/<pack>/**` | 新增领域流程、tooling、reference、manifest 规则 |
| Common | `common/**` | 多 pack 共享 policy / prompt |
| Docs | `README.md`、`docs/**` | 使用说明、设计、路线、迁移和验证 |

维护本仓库时不需要运行 `/rekit init`。只有需要验证 case 行为时，才创建临时 case。

### 1.2 接入新安全 case（当前以 `vmp-re` 为例）

从 kit 仓库启动 Claude Code，然后：

```text
/rekit init -Target <workspaceRoot>\cases\<caseName> -Pack vmp-re -ProjectName <caseName> -Apply
```

`init` 会创建：

```text
<caseRoot>\.rekit\instance.yml
<caseRoot>\.rekit\state.json
<caseRoot>\.claude\skills\rekit\SKILL.md
<caseRoot>\references\vmp-re\...
<caseRoot>\CLAUDE.local.md 中的 managed router block
```

之后进入 case 目录，每天只用 `/rekit`：

```text
/rekit overview
/rekit continue main
/rekit start <feature>
/rekit handoff
```

### 1.3 接入已有 case

已有 case 不建议直接 `init` 覆盖。先用：

```text
/rekit attach -Target <caseRoot> -Pack vmp-re -Apply
```

`attach -Apply` 只做四件事：

1. 写 `.rekit/instance.yml`。
2. 写 legacy `.re-template.yml`。
3. 写/更新 `.rekit/state.json`。
4. 写 case-local `/rekit` thin shim。

它不会覆盖已有 references、handoff 或工具链文档。随后用：

```text
/rekit sync
```

生成 review 包。确认无误后，再让 Claude 执行写入型 sync；Batch 106 起 `sync -Apply` 默认由 Go backend 处理，Batch 228 起 `sync` / `update` PowerShell fallback 已退休。

## 2. 主线和功能支线是否还能用

能，而且新架构更依赖它们。

| 工作线 | 典型命令 | 主要职责 | 默认可写 |
|---|---|---|---|
| 主线 | `/rekit continue main` | 收敛结论、验证 candidate、维护长期 handoff、处理 authority review；JSON envelope/run artifacts 暴露 apply 后 `missionBrief`，含 pending gates 与非阻塞 authorized gates；若存在 pending gate / open decision / intervention blocker，continue 先 zero-write 返回 handoff | canonical 文件、主线 workspace、`.rekit/**` |
| 功能支线 | `/rekit start <name>`、`/rekit continue <name>` | 围绕一个功能点/阻塞点做探索、收集 evidence、提出 candidate/request；start/reconcile/continue preview/apply `missionBrief` 让 lane executor 看到全局 ready/blocked 状态、pending gates 与非阻塞 authorized gates；start/reconcile 在接管或清除 intervention 后会直接投影剩余 gate/decision handoff；按 handoff 记录 terminal `note -Kind decision -Related <candidateEventId>` 后会关闭对应 open candidate blocker；blocked continue 不写 lane artifacts，先给 gate/decision/reconcile handoff | 自己的 lane workspace、outbox、candidate/request |
| 项目级索引 | `/rekit handoff` | 生成跨工作线接手索引，并在顶部 Markdown 与 Go JSON `missionBrief` 汇总 ready/blocked lanes、pending gates、authorized gates、open decisions、interventions、next agent actions 与 escalations | `.rekit/handovers/latest.md` |

推荐流程：

```text
/rekit overview
/rekit continue main
# 主线判断需要专项探索
/rekit start vm-entry
# 在 vm-entry 支线收集证据和候选
/rekit continue vm-entry
# 回主线复核并决定是否确认
/rekit continue main
/rekit handoff
```

功能支线不是“低级 agent”，而是隔离写入和上下文的单位。它可以由同一个 Claude 会话推进，也可以由后续子 agent 或新会话接手。

## 3. 旧 case 如何兼容

### 3.1 旧 metadata

旧 case 可能只有 `.re-template.yml`。新 runtime 会优先读 `.rekit/instance.yml`，缺失时回退 `.re-template.yml`。

建议逐步补齐：

```text
/rekit status
/rekit attach -Target <caseRoot> -Pack vmp-re -Apply
/rekit sync
/rekit doctor
```

如果 case 被移动过：

```text
/rekit status
/rekit repair
确认修复，执行 repair -Apply
/rekit doctor
```

### 3.2 旧文档和 handoff

- `references/vmp-re/task-handoff.md` 是 local file，不会被 sync 覆盖。
- `CLAUDE.local.md` 的 managed router block 会被 sync 管理，但 block 外私有内容保留。
- `tools.local.yml`、`captures/**`、`artifacts/**` 不会被 promote。
- 旧 handoff 可继续保留；新 handoff 会优先写 `.rekit/handovers/**`。

### 3.3 旧命令习惯

旧 wrapper 仍保留，但日常不推荐直接使用脚本：

| 旧习惯 | 新入口 |
|---|---|
| 手动跑 `bootstrap.ps1` | `/rekit init` |
| 手动跑 `update.ps1` | `/rekit sync` |
| 手动跑 `validate.ps1` | `/rekit doctor` |
| 手动维护单一 task handoff | `/rekit handoff` 与 `/rekit handoff <name>` |
| 直接把 case 经验复制回 pack | `/rekit promote` review-first |

## 4. 这套架构的后续优化空间

### 4.1 短期优化

- 补 `docs/agent-team-usage.md` 到 case managed docs 或 reference 路由中，让 case 内也能直接看到本指南。
- 让 `/rekit overview` 更清楚地区分“未接手工作线”和“已接手工作线”。
- 增加旧 case 检测摘要：缺 `.rekit/instance.yml`、只有 `.re-template.yml`、缺 managed docs、缺 handoff 索引时给出明确下一步。
- smoke test 已有维护指南 `rekit/tests/README.md` 和 pack smoke matrix；后续可继续推进 CI 检查，避免只靠人工临时命令。

### 4.2 中期优化

- 将 evidence ledger 从文档草案推进到 runtime 支持的 append-only JSONL。
- 将 heavy-tool gate / lane autonomy profile 做成可复用 packet、授权和记录流程，支持预授权范围内自主执行与越界升级。
- 继续完善 `plan-subagents` 的 tactical reviewer dispatch、多 shard intake 与跨会话接手收敛；当前已支持 planning review artifacts、planning compact orchestration summary、capture-first reviewer result source→staging→collection→ready intake 链路、每个 reviewer result writeback execution envelope 的 `runbookSteps[]` / text runbook 行，以及 collection preview 遇 canonical 冲突时的 `recovery-required` → `-RecoverReviewerResult -WhatIf` 恢复 handoff，后续重点是 bounded orchestration，而不是重复实现单 reviewer intake 或恢复手动 note 落账。
- 为 `packs/_template` 增加最小验证命令，降低新 pack 作者出错概率。

### 4.3 长期优化

- 拆出更多安全领域 pack；`web-security`、`malware-analysis`、`vuln-research`、`ctf`、`unpack-pe`、`ollvm`、`android-native` 与 `generic-binary-re` 已有 skeleton，后续按真实需求继续推进其它领域 pack。
- 引入工具 adapter 层，把 IDA/x64dbg/trace/unicorn/symex、Web/API 测试、样本分析等能力先 recipe 化，再稳定成 adapter。
- 建立 candidate -> review -> confirmed 的机器可验证 gate。
- 将多 Agent 编排与证据账本结合，支持可回放、可审计、可回滚的安全研究工作流。

## 5. 推荐使用决策表

| 你现在的情况 | 推荐动作 |
|---|---|
| 只维护本仓库 | 读 `CLAUDE.md` 与 `docs/context-routing.md`，再按场景读本文件顶部或其它入口；不要 init case |
| 新建安全 case（当前成熟示例：`vmp-re` RE case） | 在 kit 仓库用 `/rekit init -Target ... -Pack vmp-re -Apply` |
| 已有 case 接入新架构 | 用 `/rekit attach`，再 `/rekit sync` review |
| 旧 case 移动了目录 | `/rekit status` -> `/rekit repair` -> 确认后 `repair -Apply` -> `/rekit doctor` |
| 想看项目全局状态 | `/rekit overview` |
| 想继续主线 | `/rekit continue main`；自动化可用 `-WhatIf/-Apply -Format json` 读取 `missionBrief`、`missionCommanderNextActions[]` 与 `missionCommanderActionQueue` |
| 想做专项探索 | `/rekit start <name>`，之后 `/rekit continue <name>`；start JSON/text、reconcile JSON/text 与 continue run status/digest 会记录 Mission Control brief / executor action snapshot / Mission Commander action queue；已有 blocker 时 start/reconcile 也会直接显示 gate/decision handoff |
| 想换会话 | `/rekit handoff` 或 `/rekit handoff <name>`；JSON/text/Markdown 会直接投影 Mission Commander action queue |
| 想把 kit 更新同步到 case | `/rekit sync`，确认后才 apply |
| 想把 case 经验回流到 kit | `/rekit promote`，优先生成 candidate |
