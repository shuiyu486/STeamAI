# Batch implementation plan

## 读取指南

先读本节的 current milestone / current batch state / next candidates；旧批次只在需要考古、验证历史决策或 release/debug 溯源时按 Batch ID 搜索 `docs/batch-history.md`。不要默认从 Batch 0 顺序重读 350+ 个历史批次。产品方向以 `docs/mission-control-product-direction.md` 为准，持续执行方式见 `docs/autonomous-goal.md`。

## 实施摘要

Batch 359 后，Go-owned/no-fallback public command surface、durable lanes、显式 reconcile、typed autonomy preflight、Mission brief / executor action、bounded reviewer dispatch → strict intake → verification-before-decision writeback → post-validation 的本机闭环、pack-memory promote/reconsume package E2E，以及 authorized execution observation evidence + bounded adapter execution report strict intake（含 authorized stopCondition boundaryHits、status summary enforcement、workspace-relative 与 case-relative machine-readable handoff）已形成底座。当前阶段继续从 contract/inventory field increments 转向 replaceable session executor / reviewer orchestration、lane executor / tool-adapter live validation、pack-memory product UX、Windows 本机 product-path 稳定与真实 release verification。

## 执行清单

### Current milestone

**Mission Commander operational closure and truthful release readiness**：把 durable lane/reviewer/autonomy contract 串成实际可运行、可跨会话接手、可验证的产品闭环，并区分 inventory ready、本地 gate executed 与远程 CI green。当前用户短期只要求 Windows 本机稳定可用；远程 Linux/macOS/Windows CI 因 runner/billing blocker 继续记录为 known gap，不阻塞本机 Mission Control 闭环。

### Current batch state

### Batch 552：invalid reviewer packet recovery / exact retirement closure

状态：已完成implementation、focused/package validation、两轮独立审查修复、完整本地release minimum、implementation commit `26d9061 Retire invalid reviewer packets safely`/push与远程release-gate inspection。对应run `29981677697` completed failure；Linux/macOS/Windows jobs均`steps=[]`，仍为既有runner/billing blocker，不能声明remote CI green。

目标：关闭`reviewer-packet-integrity-invalid`只有“重新生成packet”提示、却没有durable且可审计地解除exact corrupted packet blocker的operational断点。让主Agent在strict sidecar仍提供可信lane provenance时显式WhatIf→Apply retirement，而不是删除、覆盖或手工修补packet/sidecar。

已完成内容：

- 新增`plan-subagents -RetireInvalidReviewerPacket -PacketPath ... -Lane ... -Actor ... -Reason ... -WhatIf/-Apply`。WhatIf返回exact invalid reason、packet/integrity hash与size、receipt path及携带expected hashes的Mission Commander apply action；Apply要求这两个preview hash，在canonical packet-path派生lock内重读并重验同一snapshot与strict integrity sidecar后写`packet.retirement.json`。
- receipt绑定repo/case/pack、packetId/lane/canonical paths、exact packet与integrity hash/size、actor/reason、RFC3339Nano timestamp以及no-delete/no-heavy/no-authority标志；相同snapshot与decision的Apply replay幂等，不同decision或forged existing receipt fail-closed。
- durable workstream只有在receipt strict JSON、attached metadata binding、timestamp、paths、hashes/sizes、provenance和boundary全部匹配当前bytes时才停止投影该invalid blocker；packet或sidecar bytes变化、receipt篡改、symlink/non-regular path会恢复`reviewer-packet-integrity-invalid`。
- JSON/text CLI与case-local product-path coverage验证WhatIf no-write、Apply closure、exact replay、receipt forgery与packet drift。missing/malformed integrity sidecar因缺少可信lane provenance继续拒绝retirement，只能重新生成canonical packet。

边界：retirement不删除、覆盖或修补packet/integrity，不spawn/stop/poll/monitor reviewer，不执行heavy-tool，不写facts/authority/confirmed。只支持strict sidecar可读但packet无效的exact snapshot recovery；禁止新增PowerShell runtime logic。

验证结果：focused `subagents` / `workstream` / `cli` tests与case-local CLI WhatIf/text/Apply product path已通过；完整本地release minimum（`release-check -Format json` ready、`status`、`packs`、`doctor`、`go test ./...`、`go vet ./...`、`git diff --check`）已通过。独立审查发现并修复semantic-invalid sidecar provenance、WhatIf未绑定exact snapshot/锁外stale result、workstream无界non-regular读取，以及core mutation API未强制preview hashes四项问题；复核通过。远程run `29981677697`三平台jobs均在执行任何step前失败，未提供新的代码失败信号。

上一批摘要：Batch 551已完成reviewer packet integrity / durable fail-closed closure，implementation commit `0d1f518 Attest durable reviewer packets`与release inspection commit `d01f105 Record Batch 551 release gate inspection`已推送；对应run `29979488713`的Linux/macOS/Windows jobs均completed failure且`steps=[]`，仍为既有runner/billing blocker。

### Batch 551：reviewer packet integrity / durable fail-closed closure

状态：已完成implementation、focused/package validation、独立审查修复、完整本地release minimum、implementation commit `0d1f518 Attest durable reviewer packets`/push与远程release-gate inspection。对应run `29979488713` completed failure；Linux/macOS/Windows jobs均`steps=[]`，仍为既有runner/billing blocker，不能声明remote CI green。

目标：关闭canonical reviewer packet在生成后被截断、篡改或与其durable bindings漂移时，workstream宽松decode/静默跳过会让Mission Commander误以为reviewer work消失的operational断点。让新packet携带exact-bytes integrity receipt，并在status/handoff/start/reconcile/continue共同复用的workstream入口中fail-closed为lane-scoped blocked repair action。

已完成内容：

- attached canonical planning写入`packet.integrity.json`，receipt绑定schema/kind/sha256、packetId、targetLane、canonical packet path、exact bytes hash与size；packet内只保存canonical algorithm/path reference，identity覆盖该reference，legacy/custom/out-of-case packet不强制新增sidecar。
- durable workstream在提升任何dispatch/collection/intake/adoption command前验证canonical sidecar与exact packet bytes/bindings；hash/size/path/id/lane drift、missing/unknown/trailing sidecar或sidecar存在但packet移除reference均进入`reviewer-packet-integrity-invalid`，不投影runnable commands。adoption/direct/batch intake与collection执行端也在WhatIf/Apply路径复用stable packet reader与同一integrity validation，Apply mutation lock内再次校验exact packet。
- packet被截断到无法decode时，workstream从strict sidecar保留packetId/targetLane provenance并生成同一blocked handoff，防止active reviewer work静默消失；Mission Commander queue阻止普通lane continuation并只建议重新生成canonical packet，不建议手工修补packet或receipt。
- fresh canonical packet在candidate目录尚未创建时仍保持typed candidate/collection capability；legacy无sidecar packet继续按既有direct/batch intake兼容路径投影。

边界：integrity receipt不是外部授权证明，只用于repo-local durable packet完整性；不自动修复或覆盖packet/receipt，不spawn/stop/poll/monitor reviewer，不执行heavy tool，不写facts/authority/confirmed；collection与intake仍分别要求WhatIf→Apply。禁止新增PowerShell runtime logic。

验证结果：focused `subagents` / `workstream` / `cli` package tests已通过，覆盖exact sidecar binding、fresh planning→durable projection、lane drift与malformed/truncated packet fail-closed、removed-reference downgrade拒绝、执行入口integrity enforcement、blocked Mission Commander action与legacy compatibility。独立审查发现的reference downgrade、lane filter ordering与execution-path bypass均已修复；完整本地release minimum（`release-check` ready、`status`、`packs`、`doctor`、`go test ./...`、`go vet ./...`、`git diff --check`）已通过。远程run `29979488713`三平台jobs均在执行任何step前失败，未提供新的代码失败信号。

上一批摘要：Batch 550已完成reviewer collection capability gating closure，implementation commit `b0ea8c7 Gate reviewer collection capabilities`与release inspection commit `b995975 Record Batch 550 release gate inspection`已推送；对应run `29977522917`的Linux/macOS/Windows jobs均completed failure且`steps=[]`，仍为既有runner/billing blocker。

### Batch 550：reviewer collection capability gating closure

状态：已完成implementation、focused/package validation、两轮独立审查修复、完整本地release minimum、implementation commit `b0ea8c7 Gate reviewer collection capabilities`/push与远程release-gate inspection。对应run `29977522917` completed failure；Linux/macOS/Windows jobs均`steps=[]`，仍为既有runner/billing blocker，不能声明remote CI green。

目标：关闭Batch 549 collection执行端只接受canonical case-local review namespace，但planning仍会为attached custom `-ReviewOutputDir` / explicit `-PacketPath`生成必然失败collection commands的真实性断点。让planning、collection runtime与durable workstream共享同一canonical geometry，并把direct/batch intake能力与collection mutation能力显式分离。

已完成内容：

- 新增共享canonical review namespace helper：只接受`<case>/.rekit/reviews/<单层review>/packet.json`，并严格派生`results/candidates/<shard>.json`与`results/<shard>.json`；planning与collection runtime复用同一定义，nested/custom packet不再被视为collection-capable。
- `reviewerOrchestration.summary`新增`collectionAvailable`，与`intakeAvailable`/`dispatchOnly`分离。attached canonical packet继续生成typed candidate与collection WhatIf/Apply；attached custom/noncanonical artifact保持strict direct与packet batch intake，但省略candidate/collection commands并改用direct-result guidance；out-of-case packet保持纯dispatch-only，attach/init后要求重新生成canonical packet。
- durable workstream以实际扫描到的packet path为权威，并重新绑定embedded packet path、result root、candidate/result paths和collection candidate path；任一geometry不一致即抑制collection command/state/summary，不让forged packet进入Mission Commander collection queue，同时保留legacy direct intake fallback。
- collection执行端要求单层canonical packet，并从case root检查packet、result、candidate与publication parent的完整祖先链；planning在创建canonical review artifact目录前拒绝symlink traversal，collection在首次读取packet前拒绝`.rekit`或review root symlink。现有strict candidate validation、WhatIf→Apply、exact bytes/no-overwrite publication与idempotent replay保持不变。

边界：不禁止custom review artifacts，不删除legacy direct/batch intake。runtime不spawn、stop、poll或monitor reviewer；collection不写facts、不执行heavy tool、不修改managed/project source、不写authority/confirmed；intake仍独立要求WhatIf→Apply。禁止新增PowerShell runtime logic。

验证结果：focused `reviewpath` / `subagents` / `workstream` / `cli` tests已通过，覆盖default/custom canonical正例、case-local/external/custom-name/nested/case-variant packet capability suppression、nested runtime rejection、forged downstream geometry suppression、forged collection command canonical rebuild、missing candidate directory capability保留、canonical review symlink pre-write拒绝与`.rekit` symlink collection WhatIf/Apply no-publish。完整本地release minimum（`release-check` ready、`status`、`packs`、`doctor`、`go test ./...`、`go vet ./...`、`git diff --check`）已通过；两轮独立审查发现的case-root symlink祖先、fresh packet candidate parent、case-sensitive packet name与embedded command trust缺口均已修复。远程run `29977522917`三平台jobs均在执行任何step前失败，未提供新的代码失败信号。

上一批摘要：Batch 549已完成reviewer Agent handoff / immutable result collection closure，implementation commit `ae6b8bd Close reviewer result collection handoff`与release inspection commit `7aa1a7d Record Batch 549 release gate inspection`已推送；对应run `29974679916`的Linux/macOS/Windows jobs均completed failure且`steps=[]`，仍为既有runner/billing blocker。

### Batch 549：reviewer Agent handoff / immutable result collection closure

状态：已完成 implementation、focused/package validation、独立审查修复、完整本地 release minimum、implementation commit `ae6b8bd Close reviewer result collection handoff`/push 与远程 release-gate inspection。对应run `29974679916` completed failure；Linux/macOS/Windows jobs均`steps=[]`，仍为既有runner/billing blocker，不能声明remote CI green。

目标：关闭 `plan-subagents` packet 已能描述 reviewer task、strict intake 已能消费 canonical result，但主 Agent仍需手工拼 Claude Code Agent调用并直接写 canonical `reviewerResultPath` 的 operational 断点。让 packet提供 typed Agent tool request与独立 candidate path，并由 Go-native collection WhatIf→Apply严格验证和不可覆盖地发布exact result bytes，再接续既有 packet-level batch intake。

已完成内容：

- 每个 shard handoff与 reviewer orchestration dispatch新增 typed `agentToolRequest`（Claude Code Agent、read-only reviewer、prompt、exact single-JSON output contract）、packet派生 `reviewerResultCandidatePath` 与 collection preview/apply commands；runtime仍不启动、停止、轮询或监控 reviewer。
- 新增 `plan-subagents -CollectReviewerResult -ShardId ... -WhatIf/-Apply`：strict重读immutable packet与candidate，绑定repo/case/pack/lane、packet/route/shard/items、route output、evidence/conflict/write/heavy/authority blockers；WhatIf不创建canonical result，Apply持有packet/shard lock后重验并以temporary file + no-replace hard link发布exact bytes。
- canonical target只能来自packet handoff；different existing bytes fail-closed，exact replay返回`already-collected`；candidate/canonical intermediate或leaf symlink、empty/directory/oversize/trailing/unknown JSON、forged binding均拒绝。collection完成后若owner stale则先要求packet adoption，否则提升packet-level ready-result batch intake preview。
- status/overview/handoff/start/reconcile/continue与durable resume/checkpoint/digest复用 reviewer dispatch handoff中的candidate、typed Agent metadata和collection commands；candidate missing继续要求主Agent dispatch，non-regular candidate显式invalid，regular candidate提升collection WhatIf，canonical到位后提升batch intake。所有open阶段继续阻止lane continuation mutation。
- Windows本机 nested case/no-target/no-pack product path覆盖 planning→candidate write→collection WhatIf no-write→Apply publication→batch intake WhatIf→Apply；legacy direct `-ReviewerResultPath`与旧packet fallback继续保留。

边界：reviewer只返回JSON、不写文件/ledger；主Agent保存candidate并显式执行collection WhatIf→Apply。collection不写facts、不执行heavy tool、不修改managed/project source、不写authority/confirmed；intake仍是独立WhatIf→Apply。禁止新增PowerShell runtime logic。

验证结果：focused `subagents` / `workstream` / `cli` packages与case-local product-path tests已通过；独立审查发现并修复collection可被重算packetId引向case review namespace外、WhatIf遗漏canonical collision/non-regular preflight、`-ShardId`在非collection batch Apply被静默忽略三项问题，并补充forged namespace、WhatIf/Apply collision、invalid canonical与CLI no-write guards。完整本地release minimum已通过`release-check -Format json`（`ready=true`）、`status`、`packs`、`doctor`、`go test ./...`、`go vet ./...`与`git diff --check`。implementation run `29974679916`的Linux/macOS/Windows jobs均completed failure且`steps=[]`，仍为既有runner/billing blocker。

上一批摘要：Batch 548已完成reviewer packet adoption / continuation preflight closure，implementation commit `cc451d0 Close reviewer packet continuation preflight`已推送；对应run `29971220679`的Linux/macOS/Windows jobs均completed failure且`steps=[]`，仍为既有runner/billing blocker。

### Batch 548：reviewer packet adoption / continuation preflight closure

状态：已完成 implementation、focused/package validation、独立审查修复、完整本地 release minimum、implementation commit `cc451d0 Close reviewer packet continuation preflight`/push 与远程 release-gate inspection。对应run `29971220679` completed failure；Linux/macOS/Windows jobs均`steps=[]`，仍为既有runner/billing blocker，不能声明remote CI green。

目标：关闭 reviewer packet 由旧 executor/generation 创建、reviewer result 到达后 lane 被 replacement executor takeover 时，status/handoff仍可能看似可继续、strict intake却因 stale owner fail-closed 的 operational 断点。保持 packet与result binding immutable，用 case-local durable adoption receipt显式转移 strict intake ownership，并让 waiting/ready/partial/stale/blocked reviewer work统一阻止 lane continuation mutation。

已完成内容：

- 新增显式 reviewer packet owner adoption WhatIf→Apply product path，在`.rekit/reviewer-adoptions/<packetId>.json`写strict receipt，绑定exact packet SHA-256、repo/case/pack/lane、dispatched/adopted executor generation、actor/reason与no-spawn/no-heavy/no-authority boundary；packet bytes及既有reviewer result `packetId` binding保持不变，后续再次takeover会使receipt失效。
- status/overview、project/lane handoff、start/reconcile、continue、lane`RESUME.md`/checkpoint复用同一reviewer-aware Mission Commander queue；stale adoption、result symlink/attach repair、ready intake preview与waiting state按packet identity去重和排序，start/reconcile本次bounded Apply与execution evidence main escalation仍保持更高优先级。
- active reviewer work使continue WhatIf/Apply fail-closed：不创建run directory、不写facts、不刷新resume/checkpoint/board；partial verification-before-decision写回仍保持open并可通过deterministic event id重试。start/reconcile takeover先刷新board再生成durable resume/checkpoint，避免新executor owner快照滞后；handoff/resume prose不再把reviewer-blocked lane描述为ready continue。
- owner stale判定覆盖packet创建时unassigned/generation 0、之后首次claim executor的场景；status侧receipt validator strict拒绝unknown/trailing JSON及错误case/pack/path/hash/owner/boundary，adoption与intake lock路径拒绝metadata root/intermediate/leaf symlink。

边界：adoption只转移strict intake ownership，不修改原packet或reviewer result、不spawn/monitor reviewer、不执行heavy tool、不写authority/confirmed。reviewer intake仍由主Agent显式WhatIf后Apply；queue只排序和投影typed actions。禁止新增PowerShell runtime logic。

验证结果：focused adoption、stale owner/re-adoption history、effective owner writeback、forged receipt、symlink lock/path、queue priority、多packet identity、partial intake与case-local product-path coverage，以及`go test ./internal/rekit/workstream ./internal/rekit/subagents ./internal/rekit/overview ./internal/rekit/cli -count=1`均通过。独立审查发现并修复authoritative intake receipt contract弱于status validator、第二次takeover无法再次adopt、ledger仍记录旧owner三项问题。完整本地release minimum已通过`release-check -Format json`（`ready=true`）、`status`、`packs`、`doctor`、`go test ./...`、`go vet ./...`与`git diff --check`。implementation run `29971220679`的Linux/macOS/Windows jobs均completed failure且`steps=[]`，仍为既有runner/billing blocker。

上一批摘要：Batch 547已完成authorized adapter Mission Commander action queue closure，implementation commit `889c421 Close authorized adapter action queue`已推送；对应run `29967797366`的Linux/macOS/Windows jobs均completed failure且`steps=[]`，仍为既有runner/billing blocker。

### Batch 547：authorized adapter Mission Commander action queue closure

状态：已完成 implementation、focused/package validation、三轮独立审查修复、完整本地 release minimum、implementation commit `889c421 Close authorized adapter action queue`/push 与远程 release-gate inspection。对应 run `29967797366` completed failure；Linux/macOS/Windows jobs均`steps=[]`，仍为既有 runner/billing blocker，不能声明 remote CI green。

目标：关闭canonical adapter live state虽已进入status/handoff/continue独立字段、但replacement executor仍需把repair/record/escalation与普通lane continue手工拼接优先级的operational断点。把missing validation、invalid repair、valid record-ready、recorded evidence review与recorded main escalation收口到统一`MissionCommanderNextActions[]` / `MissionCommanderActionQueue`及durable handoff。

已完成内容：

- authorized-gate handoff保留contract/live validation产生的typed Mission Commander actions，并通过共享builder与lane/evidence actions合并；`GateEventID`进入typed action identity与去重键，多个gate的recorded/escalated/repair/record动作不再因同lane、同command互相吞并。
- exact recorded evidence逐gate去重adapter record动作；changed sidecar可替换同gate旧evidence；missing contract-only sidecar不会误删已有evidence。main escalation evidence排在普通evidence review之前成为current，其它gate adapter actions继续可见但blocked，并停止自主continue。
- status/overview、project/lane handoff、start、reconcile、continue preview/apply/blocked path、lane`RESUME.md`/checkpoint与durable handoff均复用统一queue；invalid sidecar repair与valid sidecar显式record可成为review-owned current action，而不是被普通lane continue抢占。start/reconcile显式preview Apply保持command-local priority，blocked lane takeover不会被adapter action抢占。
- product-path与unit coverage锁定invalid repair current、valid record-ready current、recorded evidence去重、same-path changed sidecar、多gate identity/main escalation priority、missing sidecar evidence preservation、blocked start takeover priority，以及status/handoff/continue只读状态不写`.rekit`。

边界：queue只排序并投影已有typed commands；不执行adapter/heavy-tool、不自动record observation、不写authority/confirmed。record仍要求主Agent显式复核并执行；不新增PowerShell runtime logic。

验证结果：focused adapter product-path、Mission Commander multi-gate identity/main escalation、missing sidecar preservation、blocked start takeover priority tests，以及`go test ./internal/rekit/mission ./internal/rekit/workstream ./internal/rekit/cli -count=1`均通过；三轮独立审查发现并修复same-path changed sidecar误去重、跨gate actions消失、start/reconcile preview优先级、多gate identity去重与main escalation current排序。完整本地release minimum已通过`release-check -Format json`（`ready=true`）、`status`、`packs`、`doctor`、`go test ./...`、`go vet ./...`与`git diff --check`。implementation run `29967797366`的Linux/macOS/Windows jobs均completed failure且`steps=[]`，仍为既有runner/billing blocker。

上一批摘要：Batch 546已完成canonical adapter sidecar live Mission Commander closure，implementation commits `fb1bd07 Close adapter sidecar live handoff`、`7038f04 Align recorded adapter handoff state`与release inspection commit `ca976d4 Record Batch 546 release gate inspection`已推送；最终implementation run `29964742726`的Linux/macOS/Windows jobs均completed failure且`steps=[]`，仍为既有runner/billing blocker。

### Batch 546：canonical adapter sidecar live Mission Commander closure

状态：已完成 implementation、focused/package validation、独立审查修复、完整本地 release minimum、implementation commits/push 与远程 release-gate inspection；最终 implementation commit `7038f04 Align recorded adapter handoff state` 已推送。对应 run `29964742726` completed failure；Linux/macOS/Windows jobs均`steps=[]`，仍为既有runner/billing blocker，不能声明remote CI green。

目标：关闭 adapter sidecar已写入canonical/default path后，replacement executor仍需显式运行另一条validation命令并手工判断是否可record、是否已record的operational断点。让只读`status`、`handoff`与`continue`自动复用strict validation，投影missing / invalid repair / valid record-ready / exact evidence already-recorded状态，同时保持record显式、无adapter/heavy replay。

已完成内容：

- authorized-gate compact handoff在canonical sidecar存在时复用`ValidateAdapterExecutionReport`，把真实`reportSummary`、selected adapter、repair hints、next steps与validation error投影到status、overview、handoff、start/reconcile、continue及durable resume/checkpoint/digest；sidecar缺失时保持contract-only状态。
- exact recorded detection绑定gate/report path和sidecar完整执行身份（adapter/status/budget/refs/boundary/escalation/summary），旧observation不会把同路径新内容误判为已记录；exact evidence到位后切换为`evidence-already-recorded`并只推荐evidence handoff/review，不再推荐record/replay。
- strict reader拒绝leaf或intermediate symlink、directory与其它non-regular sidecar，open后重新`f.Stat`并绑定同一file identity，避免只读status跟随symlink、阻塞特殊文件或在read窗口接受替换对象；新增`report-not-regular`failure/repair taxonomy。
- focused/package与nested case-local product-path coverage锁定invalid→repair、valid→record-ready、recorded→evidence review、same-path changed report、status/handoff/continue no-write及symlink fail-closed。

边界：live projection只读取并strict validate已存在canonical sidecar；不执行adapter/heavy-tool、不自动record observation、不写authority/confirmed。record仍要求主Agent复核`valid=true`后显式执行`gate -Apply ... -Actor ...`；不新增PowerShell runtime logic。

验证结果：focused gate identity/symlink/malformed-arrival/recorded-boundary tests、`go test ./internal/rekit/gate ./internal/rekit/workstream ./internal/rekit/overview ./internal/rekit/cli -count=1`、三轮独立审查及其record/replay suppression、record-ready queue replacement、recorded main-escalation preservation、malformed presence、text repair handoff修复均通过；完整本地release minimum已通过`release-check -Format json`（`ready=true`）、`status`、`packs`、`doctor`、`go test ./...`、`go vet ./...`与`git diff --check`。implementation commits `fb1bd07 Close adapter sidecar live handoff`、`7038f04 Align recorded adapter handoff state` 已推送；最终implementation run `29964742726` 的Linux/macOS/Windows jobs均completed failure且`steps=[]`，仍为既有runner/billing blocker。

上一批摘要：Batch 545已完成terminal decision receipt attestation，implementation commit `a7681b4 Attest terminal candidate decisions` 与 release inspection commit `482e19f Record Batch 545 release gate inspection` 已推送；implementation run `29961478940` 的Linux/macOS/Windows jobs均completed failure且`steps=[]`，仍为既有runner/billing blocker。

### Batch 545：pack-memory terminal decision receipt attestation closure

状态：已完成；implementation commit `a7681b4 Attest terminal candidate decisions` 已推送。对应 release-gate run `29961478940` completed failure；Linux/macOS/Windows jobs均 `steps=[]`，仍为既有 runner/billing blocker，不能声明 remote CI green。

目标：关闭 Batch 544 tooling reject/superseded无需reconsume时仅靠 receipt形状与非空 committed marker即可清除release blocker的真实性断点。让 release/status在把terminal receipt视为closed前，重新绑定exact packet/decision、transaction journal、committed result、candidate backup和decision evidence，确保 replacement executor接手的是可审计完成态。

已完成内容：

- release scanner对 `verificationPending=false` receipt执行strict terminal attestation：重算packet/decision SHA-256，strict decode decision/transaction/committed result，逐项绑定repo/case/pack、backup/index、counts与actions。
- 每个terminal action重新绑定decision outcome、candidate hash、reason/actor、evidence path/hash和staged candidate backup；decision/evidence/backup/transaction/marker drift均fail-closed。
- pending accepted receipt继续由Batch 543 strict verification proof路径收口，避免在proof产生前重复要求case-local packet artifact必须持续可读；proof仍绑定receipt hash与完整actions。
- `docs/agent-team-usage.md`补齐mixed与tooling-only WhatIf→Apply/terminal receipt语义；README、根/项目CLAUDE、pack reference、manifest/config与case-shim均无需改动，因为公共入口、schema、pack配置和shim边界未变化。

边界：本批只加强repo-local release/status只读attestation，不执行candidate decision、init/sync/reconsume、tooling merge或heavy action，不写authority/confirmed，不新增PowerShell runtime logic。

验证结果：focused terminal receipt valid/canonical path/decision drift/backup drift/forged tooling accept与existing pending proof测试、独立两轮审查、`go test ./...`、`go vet ./...`、`git diff --check`、`release-check -Format json`（`ready=true`）、`status`、`packs`与`doctor`均通过。implementation run `29961478940` 的Linux/macOS/Windows jobs均 completed failure且`steps=[]`，仍为既有runner/billing blocker。

上一批摘要：Batch 544已完成mixed candidate decision closure，implementation commit `1f36471 Close mixed candidate decisions` 与 release inspection commit `99f5474 Record Batch 544 release gate inspection` 已推送；implementation run `29959376352` 的Linux/macOS/Windows jobs均 completed failure且`steps=[]`，仍为既有runner/billing blocker。

### Batch 544：pack-memory mixed candidate decision closure

状态：已完成；implementation commit `1f36471 Close mixed candidate decisions` 已推送。对应 release-gate run `29959376352` completed failure；Linux/macOS/Windows jobs均 `steps=[]`，仍为既有 runner/billing blocker，不能声明 remote CI green。

目标：关闭 Batch 543 strict receipt consumer暴露的合法多决策断点：tooling-only reject/superseded packet没有 managed index时不能Apply，managed accept与 tooling reject混合 receipt在verification/release scanner中可能因candidate root不同而 fail-closed。让 replacement executor可在同一 reviewed packet内完成不同kind/outcome，并在无需reconsume时自动清除release阻塞。

已完成内容：

- candidate planning允许packet没有 managed pending item时使用 canonical missing index；WhatIf在全新 pack上不创建 candidate root/index/lock，Apply才安全建立 canonical candidate lock root；receipt始终记录canonical index path。
- receipt/verification按 action kind选择 managed `promote-candidates` 或 `tooling/candidates` root；混合 managed accept + tooling reject仍严格绑定 transaction、backup、decision hash、evidence与 accepted reconsume proof；tooling auto-accept继续拒绝。
- tooling-only reject生成非 pending receipt，清理candidate后不要求无意义的 fresh/attached verification，release/status scanner接受该完整receipt并保持ready。
- focused coverage新增混合 accept/reject product path、tooling-only reject和 completed non-verification receipt release handoff。

边界：所有decision仍必须来自exact durable packet并先WhatIf后Apply；tooling candidate只能reject/superseded或由人工另行review/merge，runtime不自动accept或写tooling catalog，不执行heavy-tool、不写authority/confirmed、不新增PowerShell runtime logic。

验证结果：focused promote mixed/tooling-only WhatIf→Apply与releasecheck completed/forged receipt测试、`go test ./...`、`go vet ./...`、`git diff --check`、`release-check -Format json`（`ready=true`）、`status`、`packs`与`doctor`均通过。implementation run `29959376352` 的 Linux/macOS/Windows jobs均 completed failure且 `steps=[]`；这是既有 GitHub Actions runner/billing blocker，不是远程测试执行失败。

上一批摘要：Batch 543已完成 candidate decision receipt/reconsume verification closure，implementation commit `8458ec7 Close candidate decision verification` 与 release inspection commit `12ab902 Record Batch 543 release gate inspection` 已推送；implementation run `29958441694` 的 Linux/macOS/Windows jobs均 completed failure且 `steps=[]`，仍为既有 runner/billing blocker。

### Batch 543：pack-memory candidate decision receipt / reconsume verification closure

状态：已完成；implementation commit `8458ec7 Close candidate decision verification` 已推送。对应 release-gate run `29958441694` completed failure；Linux/macOS/Windows jobs均 `steps=[]`，仍为既有 runner/billing blocker，不能声明 remote CI green。

目标：补齐 Batch 542 Apply后 candidate/index消失导致 replacement executor与 release/status丢失后续 doctor/reconsume工作的 operational断点。让 reviewed decision留下 durable receipt，并提供严格、显式、可幂等的 pack/fresh/attached verification product path；release/status在 candidate residue清零后仍能交付 pending verification，proof完成后自动闭环。

已完成内容：

- candidate decision Apply在 canonical candidate root的 `review-artifacts` 写单一 durable receipt，绑定 repo/case/pack、packet/decision hashes、transaction backup/index、actions/evidence与 verification proof path；accepted receipt给出包含 `<fresh-case>` / `<attached-case>` 必需输入的 concrete WhatIf命令。receipt写失败仍走既有 target/index/candidate rollback。
- 新增 `-VerifyCandidateDecision -FreshCaseRoot ... -AttachedCaseRoot ... -WhatIf/-Apply`：严格重读 packet/decision/receipt/transaction/committed marker，检查 receipt、transaction与 marker的 action/count/path bindings，reviewed candidate backup仍匹配 decision hash，accepted target与 staged reviewed candidate backup一致，candidate缺失、index不再引用 candidate、source/fresh/attached roots互异，并运行 pack/fresh/attached doctor和 fresh/attached accepted managed-content hash验证。WhatIf no-write；Apply持有 decision lock且在proof commit前重验状态，只写 repo-local durable proof，相同验证结果幂等 replay。
- release/status扫描 strict repo-local decision receipts；candidate/index已清理但 accepted verification尚无 proof时继续投影 open work、pending count、concrete command和 proof path。proof必须严格绑定 schema/kind、pack、packet/decision hashes、source/fresh/attached roots、receipt/proof paths、doctor rows、mutation/applied/ready状态和完整 actions；malformed、trailing、unknown-field或错误绑定均 fail-closed，合法 proof到位后该 receipt不再阻塞。CLI text与 JSON同步输出 receipt/pending/completed/verification状态。
- package/CLI coverage验证 receipt命令与 durable path、verification WhatIf no-write、Apply/replay、missing/same/source roots、stale fresh content、target drift、candidate/index重新出现、existing proof drift、malformed/wrong-hash/wrong-receipt/wrong-actions proof、release handoff pending/complete转换，以及 case-local CLI decision→fresh/attached init→verification全闭环。

边界：runtime不创建或初始化 fresh/attached cases，不执行 sync、heavy-tool或 authority/confirmed写入；调用者必须先准备两个不同的 attached cases，并先审查 WhatIf再Apply。verification只读取 pack/cases并写 repo-local proof；tooling candidate仍不允许自动accept；不新增 PowerShell runtime logic。

验证结果：focused candidate promote/releasecheck/CLI测试、`go test ./...`、`go vet ./...`、`git diff --check`、`release-check -Format json`（`ready=true`）、`status`、`packs`与`doctor`均通过。implementation run `29958441694` 的 Linux/macOS/Windows jobs均 completed failure且 `steps=[]`；这是既有 GitHub Actions runner/billing blocker，不是远程测试执行失败。

上一批摘要：Batch 542已完成 reviewed candidate decision/cleanup product path，implementation commit `882a553 Close pack memory candidate decisions` 与 release inspection commit `32650b9 Record Batch 542 release gate inspection` 已推送；implementation run `29953603900` 的 Linux/macOS/Windows jobs均 completed failure且 `steps=[]`，仍为既有 runner/billing blocker，不能声明 remote CI green。

### Batch 542：pack-memory reviewed candidate decision / cleanup product-path closure

状态：已完成 implementation、focused/package validation、完整本地 release minimum、implementation commit/push 与远程 release-gate inspection；implementation commit `882a553 Close pack memory candidate decisions` 已推送到 `main`。远程 release-gate run `29953603900` completed failure，Linux/macOS/Windows jobs 均 completed failure 且 `steps=[]`，仍是既有 GitHub Actions runner/billing blocker，不能声明 remote CI green。按 cadence 只记录 implementation commit触发的远程 run；不为本 release inspection commit自身触发的 CI追加第三个记录提交，除非出现不同于既有 `steps=[]` blocker的新信号。

目标：解决 Batch 538 durable candidate review workspace 之后仍需主 Agent手工复制 candidate到 pack target、删除 candidate并编辑 index 的 operational 断点。提供一个 strict、review-first、Go-native `promote -PacketPath ... -CandidateDecisionPath ... -WhatIf/-Apply` 闭环，让 replacement executor可从同一 durable packet预览并执行 reviewed accept/reject/superseded，同时保持 tooling merge、authority/confirmed与 heavy action边界。

已完成内容：

- 新增 strict candidate decision schema和 CLI入口；decision绑定 exact packet SHA-256、attached repo/case/pack、canonical candidate/tooling roots与 index、manifest managed target、packet pending review item、`create-candidate` write、managed index mapping、candidate/target hashes，以及每个 evidence ref 的 path + SHA-256；拒绝 unknown fields、trailing JSON、hash drift、forged roots/items/writes/index、path escape和 symlink traversal。
- `accept` 仅允许 managed-doc并写 exact packet/manifest target；tooling candidate auto-accept fail-closed，继续要求人工 catalog/recipe review。`reject` / `superseded` 只删除 reviewed candidate并移除对应 index entry。
- Apply在 canonical candidate root持有 pack-scoped exclusive lock，mutation前重新验证 packet/decision/index/candidate/target/evidence，在经过 symlink检查的随机 transaction目录中 exclusive stage candidate/target/index backups并同步写 durable journal；namespace中若存在其它 unfinished decision会 fail-closed，使用原 packet+decision重试时先严格校验 journal/backup/target绑定并执行 deterministic rollback。正常错误也会逆序恢复 target/index和已删除 candidate。CLI通过 typed error保留 `failedAction`、`rolledBack`、`recoveryRequired`、backup paths与 recovery actions；Windows路径不依赖 `os.Rename` 提供 crash-atomic保证；journal/recovery明确覆盖进程中断恢复，Unix额外同步 transaction目录项，系统断电级保证仍受平台文件系统语义限制。
- package/CLI coverage验证 WhatIf no-write、managed accept/backup/cleanup、case-local nested cwd product path、candidate/evidence hash drift、canonical root/manifest target/packet write/Apply recovery伪造、tooling auto-accept拒绝、candidate/backup-root symlink拒绝、live/stale/malformed lock、进程中断journal recovery与 cleanup failure rollback。

边界：本批只执行已由主 Agent在 durable packet上明确记录的 reviewed candidate decision；必须先 WhatIf再显式 Apply。runtime不自动决定 accept/reject/superseded，不自动 accept tooling candidate，不运行 doctor/init/reconsume，不创建 expected proof，不写 authority/confirmed、不执行 heavy-tool、不新增 PowerShell runtime logic。

验证结果：focused `go test ./internal/rekit/promote ./internal/rekit/cli -run "TestApplyCandidateDecisions|TestRunPromoteCandidateDecision" -count=1` 与 package `go test ./internal/rekit/promote -count=1` 已通过；完整本地 release minimum 已通过 `go run ./cmd/rekit -- -Command release-check -Format json`（ready=true / release gate inventory ok）、`go run ./cmd/rekit -- -Command status -Format text`、`go run ./cmd/rekit -- -Command packs -Format text`、`go run ./cmd/rekit -- -Command doctor -Format text`、`go test ./...`、`go vet ./...`、`git diff --check`（仅 LF→CRLF working-copy warning，无 whitespace error）。implementation commit `882a553 Close pack memory candidate decisions` 已推送；远程 release-gate run `29953603900` 已检查，Linux/macOS/Windows jobs均 completed failure且 `steps=[]`，仍符合既有 runner/billing blocker，不能声明 remote CI green。

上一批摘要：Batch 541 已完成 reviewer batch Mission Commander / durable handoff closure，implementation commit `5a128f9 Close reviewer batch commander handoff` 与 release inspection commit `ff077a8 Record Batch 541 release gate inspection` 已推送；implementation run `29949565383` completed failure，Linux/macOS/Windows jobs均 `steps=[]`，仍是既有 GitHub Actions runner/billing blocker，不能声明 remote CI green。

### Batch 541：reviewer batch Mission Commander / durable handoff closure

状态：已完成 implementation、focused/package validation、完整本地 release minimum、implementation commit/push 与远程 release-gate inspection；implementation commit `5a128f9 Close reviewer batch commander handoff` 已推送到 `main`。远程 release-gate run `29949565383` completed failure，Linux/macOS/Windows jobs 均 completed failure 且 `steps=[]`，仍是既有 GitHub Actions runner/billing blocker，不能声明 remote CI green。按 cadence 只记录 implementation commit 触发的远程 run；不为本 release inspection commit 自身触发的 CI 追加第三个记录提交，除非出现不同于既有 `steps=[]` blocker 的新信号。

目标：补齐 Batch 539 的真实 operational handoff：runtime 已支持 `-ReadyReviewerResults`，但 planning packet、Mission Commander action queue 与 downstream durable handoff 仍主要暴露逐 shard `-ReviewerResultPath ... -WhatIf/-Apply`，replacement executor 仍需手工发现并拼接 batch intake。让 attached-case reviewer planning、action queue、status/handoff/continue 与 durable artifacts 直接交付 packet-level batch preview/apply，同时保留旧 packet和 out-of-case安全语义。

已完成内容：

- attached-case `plan-subagents` 的 `reviewerOrchestration` / compact summary 新增 `batchPreviewCommand` 与 `batchApplyCommand`；`packet.json`、`summary.md` 和 CLI text 同步输出可复制的 `-ReadyReviewerResults` commands。out-of-case dispatch-only planning 保持 batch commands 为空。
- Mission Commander action queue 保留每个 shard 的 read-only dispatch，但将每 shard preview/apply 收口为 packet-level batch preview/apply；两 shard queue 从 6 个 action 收敛为 4 个，batch Apply 仍 blocked/requiresReview，直到 reviewer JSON、WhatIf 与 evidence review通过。
- downstream reviewer dispatch intake handoff / summary 现在携带 batch commands，status/handoff/continue、run status/digest、lane `RESUME.md` 与 checkpoint 可直接接手；ready 判定与 strict batch intake 共享 non-empty regular-file classifier，empty/directory 保持 waiting、symlink 显式 blocked；存在 ready shard 时 summary 选择该 ready packet 的 batch preview，不被最后一个 waiting shard或 5-row detail cap 覆盖。legacy packet无 batch fields时回退 single-result preview。
- package/CLI coverage锁定 attached/out-of-case command生成、action queue、summary artifact/text、ready+waiting packet选择、empty/directory/symlink classification、detail-cap ready preservation、legacy fallback，以及 status/continue/durable projection。

边界：本批只增强 reviewer orchestration command handoff 与 Mission Commander ordering；不自动 spawn、轮询或监控 reviewer，不创建 result，不绕过 strict batch intake、packet order、waiting/evidence/blocker review 或 verification-before-decision；不执行 heavy-tool、不写 authority/confirmed、不新增 PowerShell runtime logic。

验证结果：已通过 `gofmt`、`go test ./internal/rekit/subagents ./internal/rekit/workstream ./internal/rekit/cli -count=1`；完整本地 release minimum 已通过 `go run ./cmd/rekit -- -Command release-check -Format json`（ready=true / release gate inventory ok）、`go run ./cmd/rekit -- -Command status -Format text`、`go run ./cmd/rekit -- -Command packs -Format text`、`go run ./cmd/rekit -- -Command doctor -Format text`、`go test ./...`、`go vet ./...`、`git diff --check`（仅 LF→CRLF working-copy warning，无 whitespace error）。implementation commit `5a128f9 Close reviewer batch commander handoff` 已推送；远程 release-gate run `29949565383` 已检查，Linux/macOS/Windows jobs 均 completed failure 且 `steps=[]`，仍符合既有 runner/billing blocker，不能声明 remote CI green。

上一批摘要：Batch 540 已完成 release handoff current-batch truthfulness repair，implementation commit `74327c3 Fix current batch release handoff` 与 release inspection commit `d4e65f2 Record Batch 540 release gate inspection` 已推送；implementation run `29946738172` completed failure，Linux/macOS/Windows jobs 均 `steps=[]`，仍是既有 GitHub Actions runner/billing blocker，不能声明 remote CI green。

### Batch 540：release handoff current-batch truthfulness repair

状态：已完成 implementation、focused/package validation、完整本地 release minimum、implementation commit/push 与远程 release-gate inspection；implementation commit `74327c3 Fix current batch release handoff` 已推送到 `main`。远程 release-gate run `29946738172` completed failure，Linux/macOS/Windows jobs 均 completed failure 且 `steps=[]`，仍是既有 GitHub Actions runner/billing blocker，不能声明 remote CI green。按 cadence 只记录 implementation commit 触发的远程 run；不为本 release inspection commit 自身触发的 CI 追加第三个记录提交，除非出现不同于既有 `steps=[]` blocker 的新信号。

目标：修复真实 product-path truthfulness 断点：Batch 539 已完成并发布后，`release-check` / kit `status.projectHandoff` 仍报告 Batch 537。根因是 latest-batch parser 遍历整个 active document 并选择最后一个历史 `### Batch` section；这会让 replacement executor/release maintainer看到错误的 goal、validation、run 和 cadence evidence。

已完成内容：

- `latestBatchSummary` 现在选择 `docs/batch-plan.md` 中第一个 `### Batch` section，即 `Current batch state` 的 active batch，不再被后续历史 section 覆盖。
- `releaseCheckReady` 解析同时识别 `release-check ready=true` 与当前文档使用的 ``release-check -Format json`（ready=true）` 表述。
- regression fixture 包含 current Batch 539 与 previous Batch 538，锁定 current title/ID/goal/validation、local/release-check readiness 与 remote `steps=[]` gate；真实 `status -Format text` 已确认 latestBatch=Batch 539、releaseCheckReady=true、run=29945764199。

边界：本批只修复 repo-local release handoff 文档解析与只读 status/release-check truthfulness；不执行远程 CI、不改变 release policy、case runtime、heavy-tool、authority/confirmed、PowerShell façade 或 public removal gate。

验证结果：已通过 `gofmt -w internal/rekit/releasecheck/release_handoff.go internal/rekit/releasecheck/release_handoff_test.go internal/rekit/cli/cli_test.go`、`go test ./internal/rekit/releasecheck ./internal/rekit/cli -count=1`，以及 `go run ./cmd/rekit -- -Command status -Format text` 真实 product-path 检查；完整本地 release minimum 已通过 `go run ./cmd/rekit -- -Command release-check -Format json`（ready=true / release gate inventory ok）、`go run ./cmd/rekit -- -Command status -Format text`、`go run ./cmd/rekit -- -Command packs -Format text`、`go run ./cmd/rekit -- -Command doctor -Format text`、`go test ./...`、`go vet ./...`、`git diff --check`（仅 LF→CRLF working-copy warning，无 whitespace error）。implementation commit `74327c3 Fix current batch release handoff` 已推送；远程 release-gate run `29946738172` 已检查，Linux/macOS/Windows jobs 均 completed failure 且 `steps=[]`，仍符合既有 runner/billing blocker，不能声明 remote CI green。

上一批摘要：Batch 539 已完成 multi-reviewer ready-result batch intake，implementation commit `e368a0d Add multi-reviewer batch intake` 与 release inspection commit `226a2c3 Record Batch 539 release gate inspection` 已推送；implementation run `29945764199` completed failure，Linux/macOS/Windows jobs 均 `steps=[]`，仍是既有 runner/billing blocker，不能声明 remote CI green。

### Batch 539：multi-reviewer ready-result batch intake vertical slice

状态：已完成 implementation、focused/package 验证、完整本地 release minimum、implementation commit/push 与远程 release-gate inspection；implementation commit `e368a0d Add multi-reviewer batch intake` 已推送到 `main`。远程 release-gate run `29945764199` completed failure，Linux/macOS/Windows jobs 均 completed failure 且 `steps=[]`，仍是既有 GitHub Actions runner/billing blocker，不能声明 remote CI green。按 cadence 只记录 implementation commit 触发的远程 run；不为本 release inspection commit 自身触发的 CI 追加第三个记录提交，除非出现不同于既有 `steps=[]` blocker 的新信号。

目标：解决 multi-reviewer packet 的多个 result path 已到位后，Mission Commander 仍需逐 shard 手工拼接并重复执行 `-ReviewerResultPath ... -WhatIf/-Apply` 的 operational 断点。新增单一 Go-native case-local batch intake 入口，按 durable packet order 发现 ready results 并复用既有 strict single-result intake，而不是新增并行 writeback 实现或继续投影 summary 字段。

已完成内容：

- `plan-subagents -ReadyReviewerResults` 要求显式 `-PacketPath`、`-Lane`、`-Actor` 和 `-WhatIf` 或 `-Apply`；拒绝 planning scope flags，因为 intake scope 完全由 durable packet 决定；从 packet `shardHandoffs[]` 读取 reviewer result paths，只将存在、非目录、非 symlink 且非空的结果视为 ready，missing/empty 计入 waiting。
- ready shard 按 packet 顺序逐个复用 `IntakeReviewerResult`，并额外绑定当前 handoff expected shard，保留 packet/route/shard/items、owner binding、evidence、route output strict validation，以及每 shard verification-before-decision、post-validation 和 `already-complete` 幂等语义。
- batch 在首个 blocked、partial writeback、event-id collision、post-validation failure 或 strict intake error 处停止；strict error 返回非成功命令状态，同时 CLI 仍输出 recovery envelope；此前完整 shard 保留已完成写回，后续 shard 不处理。存在 waiting shard 时不会建议继续 lane。JSON/text 输出 totals、ready/waiting/processed/completed/alreadyComplete、stop shard/reason、逐 shard status/progress、next steps 与 no-spawn/no-heavy/no-authority boundary。
- package coverage 验证两 shard WhatIf no-write、Apply 写入两 verification + 两 decision、幂等 replay、path-to-shard binding、partial ready waiting guidance，以及第二 shard blocker 时第一 shard完整落账、第二 shard和后续不写；CLI nested cwd / no `-Target` / no `-Pack` product-path coverage验证 JSON/text preview、Apply、planning scope guard 与 malformed result strict error envelope。

边界：本批不自动 spawn、轮询或监控 reviewer，不创建 reviewer result，不绕过 strict single-result validation，不并发写 ledger；不写 authority/confirmed、不执行 heavy-tool、不新增 PowerShell runtime logic。最终 merge decision 仍由主 Agent拥有。

验证结果：已通过 `gofmt -w internal/rekit/subagents/intake.go internal/rekit/subagents/intake_test.go internal/rekit/cli/cli.go internal/rekit/cli/reviewer_intake_test.go`、focused `go test ./internal/rekit/subagents ./internal/rekit/cli -run "TestIntakeReadyReviewerResults|TestRunPlanSubagentsReadyReviewerResults|TestRunPlanSubagentsReviewerIntake" -count=1`、package `go test ./internal/rekit/subagents ./internal/rekit/cli -count=1`；完整本地 release minimum 已通过 `go run ./cmd/rekit -- -Command release-check -Format json`（ready=true / release gate inventory ok）、`go run ./cmd/rekit -- -Command status -Format text`、`go run ./cmd/rekit -- -Command packs -Format text`、`go run ./cmd/rekit -- -Command doctor -Format text`、`go test ./...`、`go vet ./...`、`git diff --check`（仅 LF→CRLF working-copy warning，无 whitespace error）。implementation commit `e368a0d Add multi-reviewer batch intake` 已推送；远程 release-gate run `29945764199` 已检查，Linux/macOS/Windows jobs 均 completed failure 且 `steps=[]`，仍符合既有 runner/billing blocker，不能声明 remote CI green。

上一批摘要：Batch 538 已完成 pack-memory durable candidate review workspace，implementation commit `a00c605 Add durable candidate review workspace` 与 release inspection commit `3ce9362 Record Batch 538 release gate inspection` 已推送；implementation run `29943533595` completed failure，Linux/macOS/Windows jobs 均 `steps=[]`，仍是既有 GitHub Actions runner/billing blocker，不能声明 remote CI green。

### Batch 538：pack-memory durable candidate review workspace vertical slice

状态：已完成 implementation、focused/package 验证、完整本地 release minimum、implementation commit/push 与远程 release-gate inspection；implementation commit `a00c605 Add durable candidate review workspace` 已推送到 `main`。远程 release-gate run `29943533595` completed failure，Linux/macOS/Windows jobs 均 completed failure 且 `steps=[]`，仍是既有 GitHub Actions runner/billing blocker，不能声明 remote CI green。按 cadence 只记录 implementation commit 触发的远程 run；不为本 release inspection commit 自身触发的 CI 追加第三个记录提交，除非出现不同于既有 `steps=[]` blocker 的新信号。

目标：解决 `promote -CreateCandidates` 生成 candidate reviewPlan 后，replacement executor 若需要 inspectable managed-doc diff、sanitized tooling preview 与 durable packet，必须额外重新运行 generic `promote -Review`，且该 generic packet 又不包含 candidate-specific decision/cleanup/reconsume plan 的 operational 断点。把 candidate generation/WhatIf、generic review inputs 与完整 candidate review plan 收口为同一次 case-local Go-native product-path 调用和一个跨会话可接手 workspace，而不是再增加一个 summary 字段。

已完成内容：

- `promote -CreateCandidates` 现在可与 `-Review`、`-ReviewOutputDir`、`-PacketPath`、`-DiffPath` 组合；CLI 在 candidate result 生成后写 durable candidate review workspace，并在 JSON `reviewWorkspace` / text 输出中返回 root、packet、summary 与 combined diff paths。
- workspace `packet.json` 封装完整 `candidateResult.reviewPlan` 和已写入 artifact paths 的 generic promote `reviewInput`，让 replacement executor 在一个 packet 中同时获得 candidate decisions/follow-through/proof handoff、managed-doc bounded diff 与 tooling sanitized preview。
- workspace `summary.md` 保持短执行区，直接给出 candidate totals、pending review/proof stage、packet/diff paths、接手 checklist、验证标准与 no-merge/no-cleanup/no-heavy/no-authority boundary。
- focused CLI coverage 验证 explicit workspace/packet/diff paths、WhatIf candidate no-write、managed-doc bounded diff、tooling sanitized preview 去 case path、packet/summary durable handoff与 text output；既有 nested cwd / no `-Target` / no `-Pack` pack-memory product-path smoke 已升级为 actual candidate generation + durable review workspace。

边界：本批只写 case-local review workspace 和既有 candidate roots；不自动 merge/cleanup candidate，不更新 pack source，不运行 doctor/init/reconsume，不创建 expected decision/cleanup/reconsume proof，不写 authority/confirmed、不执行 heavy-tool，不新增 PowerShell runtime logic。`-WhatIf` 仍不创建 candidate/index，但允许显式 `-Review` 写 review artifacts，这与既有 generic sync/promote review artifact 语义一致。

验证结果：已通过 `gofmt -w internal/rekit/promote/promote.go internal/rekit/cli/cli.go internal/rekit/cli/cli_test.go`、focused `go test ./internal/rekit/promote ./internal/rekit/cli -run "TestCreateCandidates|TestRunPromoteCreateCandidatesWritesDurableReviewWorkspace|TestRunPromoteCreateCandidatesCaseLocalProductPathUsesMetadataRuntime" -count=1`、package `go test ./internal/rekit/promote ./internal/rekit/cli -count=1`；完整本地 release minimum 已通过 `go run ./cmd/rekit -- -Command release-check -Format json`（ready=true / release gate inventory ok）、`go run ./cmd/rekit -- -Command status`、`go run ./cmd/rekit -- -Command packs`、`go run ./cmd/rekit -- -Command doctor`、`go test ./...`、`go vet ./...`、`git diff --check`（仅 LF→CRLF working-copy warning，无 whitespace error）。implementation commit `a00c605 Add durable candidate review workspace` 已推送；远程 release-gate run `29943533595` 已检查，Linux/macOS/Windows jobs 均 completed failure 且 `steps=[]`，仍符合既有 runner/billing blocker，不能声明 remote CI green。

上一批摘要：Batch 537 已完成 installed entrypoint product-path vertical slice closure，implementation commit `8e3c2fc Add installed entrypoint product slice` 与 release inspection commit `3b5205b Record Batch 537 release gate inspection` 已推送；implementation run `29938234343` completed failure，Linux/macOS/Windows jobs 均 `steps=[]`，仍是既有 GitHub Actions runner/billing blocker，不能声明 remote CI green。

### Batch 537：installed entrypoint product-path vertical slice closure

状态：已完成 runtime/CLI/test/docs implementation、focused/package validation、完整本地 release minimum、implementation commit/push 与远程 release-gate inspection；已提交并推送 implementation commit `8e3c2fc Add installed entrypoint product slice`。远程 release-gate run `29938234343` completed failure，Linux/macOS/Windows jobs 均 completed failure 且 `steps=[]`，仍是既有 GitHub Actions runner/billing blocker，不能声明 remote CI green。本批按 release inspection cadence 只记录 implementation commit 的远程 run；不再为 release inspection commit 自身触发的 CI 追加第三个记录提交，除非出现不同于既有 `steps=[]` blocker 的新信号。用户本轮要求完成本批次后停止，因此 Batch 537 完成并记录 release inspection 后不自动开启下一批。

目标：把 Windows 本机 installed case-local `/rekit` 第一屏、replacement executor takeover、reviewer dispatch/intake/writeback durable handoff 与 pack-memory candidate/proof handoff 串成同一个真实 product-path vertical slice。验证标准不是新增某个 JSON 字段，而是新会话在 case-local nested cwd 中只运行 `/rekit` / 默认 `status`，即可确认 pack 来源、thin shim readiness、installed shim 是否匹配 template、Mission Commander queue/current action、reviewer writeback / reviewer dispatch intake 状态、pack-memory candidate review/proof 下一步，以及可接手的 durable artifacts。

边界：只增强 case-local installed entrypoint 的只读 first-screen / durable handoff 与 CLI product-path coverage；case-local shim 仍是 metadata-only thin shim，回到 kit 仓库 canonical runtime；不复制 runtime logic，不新增 PowerShell runtime logic，不安装或修改用户级 `~/.claude/skills`，不展示 retained façade 或低层 `go run` / PowerShell 命令作为产品入口；reviewer intake 仍由主 Agent 显式 WhatIf → Apply，保持 verification-before-decision；pack-memory 仍只生成 candidates / proof guidance，不 merge、不 cleanup、不 reconsume、不运行 doctor/init、不执行 heavy-tool、不写 authority/confirmed。

已完成内容：

- `status.caseShim.entrypoint` 新增 installed case-local first-screen handoff：`caseLocalFirstScreenCommand=/rekit`、explicit fallback status command、installed/canonical shim path、metadata paths、durable artifacts、first-screen checks 与 thin-shim boundary；default/status text 同屏输出同一信息，且 `status case shim` 行禁止泄漏 retained façade / low-level backend details。
- case-local shim template 与 readiness guard 新增 first-screen / durable-artifact 接手短语，要求新会话先用 `/rekit` 确认 `status case shim ready=true` 与 `installedShimMatchesTemplate=true`，再按 Mission Commander queue/current action 与 durable artifacts 接手；shim drift 仍必须 repair preview-first。
- `TestRunInstalledCaseShimProductPathStatusAndRefresh` 从单一 shim-refresh smoke 扩展为真实 installed product-path vertical slice：`init -Apply` 后进入 nested case cwd，无 `-Target` / 无 `-Pack` 运行 status/start/plan-subagents/reviewer intake/continue/promote/status/doctor。
- 同一测试覆盖 replacement executor lane takeover、multi-reviewer planning、reviewer result WhatIf/Apply strict intake、reviewer writeback summary、remaining reviewer dispatch intake summary、lane `RESUME.md`、typed checkpoint 与 continue run `digest.md`，确保 durable artifacts 不需要重开 packet/result JSON 即可接手。
- 同一测试覆盖 pack-memory candidate generation 与 downstream status first screen：managed-doc candidate、blocked deny-pattern item、sanitized tooling candidate、open candidate inventory、review summary、proof summary 与 next missing proof 在 case-local default status 中可见；测试清理 pack candidate/tooling candidate residue，避免污染 pack source。

验证结果：已通过 `gofmt -w internal/rekit/cli/cli_test.go`、focused `go test ./internal/rekit/cli -run "TestRunInstalledCaseShimProductPathStatusAndRefresh" -count=1`、combined focused `go test ./internal/rekit/cli -run "TestRunInstalledCaseShimProductPathStatusAndRefresh|TestRunPlanSubagentsReviewerOrchestrationE2E" -count=1`、package `go test ./internal/rekit/caseshim ./internal/rekit/cli -count=1`；完整本地 release minimum 已通过 `go run ./cmd/rekit -- -Command release-check -Format json`（release-check ready=true / release gate inventory ok）、`go run ./cmd/rekit -- -Command status -Format text`、`go run ./cmd/rekit -- -Command packs -Format text`、`go run ./cmd/rekit -- -Command doctor -Format text`、`go test ./...`、`go vet ./...`、`git diff --check`（仅 LF→CRLF working-copy warning，无 whitespace error）。implementation commit `8e3c2fc Add installed entrypoint product slice` 已推送；远程 release-gate run `29938234343` 已检查，run completed failure，Linux/macOS/Windows jobs 均 completed failure 且 `steps=[]`。该远程失败符合既有 runner/billing blocker，不能声明 remote CI green；`release-check`/`ciReleaseGate.ready` 仍不能替代实际远程 job conclusion。

上一批摘要：Batch 536 已完成 execution evidence adapter context downstream handoff closure，implementation commit `6e2fb2a Add execution evidence adapter context handoff` 与 release inspection commit `e8f6a76 Record Batch 536 release gate inspection` 已推送；implementation run `29931195286` completed failure，Linux/macOS/Windows jobs 均 completed failure 且 `steps=[]`，仍是既有 GitHub Actions runner/billing blocker，不能声明 remote CI green；Batch 536 已归档到 `docs/batch-history.md`。

### Post-Batch 537：documentation routing / goal handoff cleanup

状态：已完成文档上下文治理收尾，commit `04ebab1 Improve documentation context routing handoff` 已推送到 `main`。该收尾不是新的 product batch；它响应用户对上下文污染、新会话交接、按需路由/渐进披露和 goal 语句过长的反馈，补齐 durable docs、reference、配置说明与示例入口的路由边界，并将 `docs/autonomous-goal.md` 中给新会话的接手语句和 goal 语句压缩为更短的中大型 vertical slice 导向。

补充纠偏：用户指出近期多个 batch 连续把某个字段、summary、handoff detail 或 text line 从 A 投影到 B，单批合理但连续会退化为内部 contract 可见性微调。已将该反模式固化到根 `CLAUDE.md`、`docs/context-routing.md` 与 `docs/autonomous-goal.md`：后续先找用户 / Mission Commander / replacement executor / reviewer / lane executor / pack-memory 的真实 operational 断点，再把字段或文本作为中大型 vertical slice 的支撑；不要为了继续推进而不断寻找下一个 `latestX` / `summaryX` / `contextX`。

交接判断：当前适合切到新会话继续。新会话先按 `docs/context-routing.md` 读取最小上下文，确认本文件 current state、`CHANGELOG.md` 顶部与真实 git/验证状态；正式 goal 使用 `docs/autonomous-goal.md` 顶部给出的短 goal 语句。下一轮若继续长期推进，应从下面 Next candidates 中选择中大型 product-path vertical slice，不再把本轮文档收尾延伸成连续文档微批次。

验证结果：已通过 `go run ./cmd/rekit -- -Command release-check -Format json`、`go run ./cmd/rekit -- -Command status -Format text`、`go run ./cmd/rekit -- -Command doctor -Format text`、`go run ./cmd/rekit -- -Command packs -Format text`、`go test ./...`、`go vet ./...`、`git diff --check`（仅 LF→CRLF working-copy warning，无 whitespace error）。

### Next candidates

1. **Lane/tool-adapter live validation residuals（如仍有缺口）**：Batch 421/424/432 已覆盖 adapter contract/validation/report action queue、follow-through 与 contract liveValidation text，Batch 458-462 已补齐 authorizedExecutionFollowThrough / evidence review follow-through text，Batch 504 已补齐 recorded adapter sidecar path、actualBudget 与 adapter provenance，Batch 518 已补齐 contract/validation compact `reportSummary`；后续仅在 Windows 本机 product-path 仍存在 validation/record/evidence review handoff 到 replacement executor 的真实断点时推进；不新增 adapter/heavy-tool execution。
2. **Reviewer orchestration E2E residuals（如仍有缺口）**：Batch 489/499/505/514-516/523-525/530-532 已覆盖 reviewer intake product path、reviewer dispatch/intake downstream/durable/progress handoff、reviewer writeback identity、reviewer result provenance summary、reviewer intake terminal summary/postValidation summary 的 compact reviewer writeback provenance、blocked reviewer intake repair guidance compact summary，以及 reviewer intake terminal compact orchestration progress；后续仅在 multi-reviewer 接续仍要求解析 nested JSON、打开 reviewer result artifact 或手工拼 route output 才能接续时推进，不自动 spawn reviewer、不执行 heavy-tool。
3. **Pack-memory downstream UX residuals（如仍有缺口）**：Batch 500 已把 candidate decision/cleanup/reconsume expected evidence 收口为 `reviewArtifacts[]`，Batch 501 已把 open candidate residue 投影到 release/status handoff，Batch 502/503 已补齐 case-local status/default path、candidate identity、index mapping 与 derived review artifact visibility，Batch 517/522/526 已补齐 compact review summary、proof presence 与 proof stage/next-missing handoff；后续仅在 accepted/rejected 人工流程、cleanup/reconsume 或 evidence review downstream UX 仍需跨 envelope 手工拼接时推进，不重复做字段微批次。
4. **Cross-platform product-path E2E（降优先级）**：在本地 CLI/case E2E 已覆盖 nested cwd / case shim 的基础上，仅保持可在 runner 可用时执行的三平台 matrix 候选和 known gap 记录；不要在 GitHub runner/billing blocker 未解除前让它阻塞 Windows 本机迭代。
5. **Retained public façade decision**：只有真实 release-gate-green、public references、case shim、smoke retirement 与恢复计划均满足后，才执行独立 removal batch；否则明确保留期限和 blocker。

### Escalation / stopping conditions

产品方向变化、runtime/policy durable schema 迁移、confirmed/authority 策略变化、未授权外部副作用、公共入口删除门禁不完整或真实 release gate 无法验证时升级。完成单个 batch、inventory ready、push 成功或工作树干净都不是长期 goal 完成。

## 验证标准

每个 active batch 记录实际执行过的命令及结果；`release-check`/`ciReleaseGate.ready` 只算 inventory readiness，不能替代本地命令执行或远程 job conclusions。优先保持 coherent vertical slice，不用逐字段 metadata batch 维持连续推进。

Batch 推送节奏默认收敛为最多两次 push：先用 implementation commit 覆盖代码、测试、文档与本地验证，再用 release inspection commit 只记录 implementation commit 的远程 run。不要继续为 release inspection commit 自己触发的 CI 追加第三个记录提交；除非出现不同于既有 `steps=[]` runner/billing blocker 的新远程信号，否则保持该 blocker 为已记录 known gap。

## 风险与注意事项

- `docs/batch-plan.md` 是 active/next 的 durable source，不只是一份已完成批次日志。
- `docs/batch-history.md` 是历史归档；不要把它重新并回 `docs/batch-plan.md`，也不要在默认 handoff/read-first 中要求全文读取。
- `CHANGELOG.md` 记录必要的用户可见变化和边界；逐步 plumbing 留在 batch history。
- 只有当前用户 goal/session 明确授权时才 commit/push 指定分支。

## 历史批次归档

完整历史已拆到 `docs/batch-history.md`。除非要查旧 batch 细节、验证历史决策或做 release/debug 溯源，不要默认读取历史归档全文。
