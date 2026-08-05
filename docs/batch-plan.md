# Batch implementation plan

## 读取指南

先读本节的 current milestone / current batch state / next candidates；旧批次只在需要考古、验证历史决策或 release/debug 溯源时按 Batch ID 搜索 `docs/batch-history.md`。不要默认从 Batch 0 顺序重读数百个历史批次。产品方向以 `docs/mission-control-product-direction.md` 为准，持续执行方式见 `docs/autonomous-goal.md`。

## 实施摘要

Batch 359 后，Go-owned/no-fallback public command surface、durable lanes、显式 reconcile、typed autonomy preflight、Mission brief / executor action、bounded reviewer dispatch → strict intake → verification-before-decision writeback → post-validation 的本机闭环、pack-memory promote/reconsume package E2E，以及 authorized execution observation evidence + bounded adapter execution report strict intake（含 authorized stopCondition boundaryHits、status summary enforcement、workspace-relative 与 case-relative machine-readable handoff）已形成底座。当前阶段继续从 contract/inventory field increments 转向 replaceable session executor / reviewer orchestration、lane executor / tool-adapter live validation、pack-memory product UX、Windows 本机 product-path 稳定与真实 release verification。

## 执行清单

### Current milestone

**最低可用 Mission Control 收敛 + truthful release readiness**：当前阶段从继续打磨底层零件切到尽快真实用起来。优先让用户能用自然语言开始 case、继续推进、查看状态、人工插手纠偏、新会话接手；允许半自动，但必须顺畅、可记录、可恢复。2026-08-01 已恢复真实 Linux/macOS/Windows release green；当前重新聚焦 Mission Commander 长期 run-loop，在不放宽每段 WhatIf/hash-bound Apply 与 Human-in-the-Lane review 边界的前提下，减少主 Agent 跨 lane/reviewer segment 持续推进时的手工预算与路由编排。

### Next candidates / goal guardrails

用户仍希望继续用短 goal 长期推进；goal 只负责启动自主推进，不承载完整路线和停止条件。压缩上下文后优先按 `docs/autonomous-goal.md` 顶部的“先真实用起来”阶段方向选择下一批：围绕开始 case、继续推进、查看状态、人工插手纠偏、新会话接手、reviewer/subagent 接手或 pack-memory 复用做一个 Windows 本机可验证产品闭环；不要继续把 adapter provenance / live validation / summary / projection 拆成单字段微调。

首选候选：

1. **最低可用 Mission Control 路线**：把开始 case、继续推进、状态总览、人工插手纠偏、新会话接手串成一条可复制、可验证、可恢复的日常路线。
2. **Mission Commander run loop MVP**：主 Agent/harness 实际驱动 ready lane 或 reviewer session 的最小 run loop；Go runtime 只记录 request/receipt/state，不 spawn/poll/stop Claude Code 进程。
3. **Reviewer/session orchestration UX**：把 ready/running/failed/stale/completed/source-capture/intake 的 operator next step 做成一条可复制、可恢复、可验证路径。
4. **Pack-memory product UX**：把 promote/reconsume 从 proof chain 推进为跨 case 可消费的 review-first workflow。
5. **嵌入式可维护性收敛**：只在上述 slice 中拆巨型 CLI/projection/test 或类型化 action source/state，不单独做大重构批。

### Current batch state







### Batch 817：machine-bound local validation receipt and post-push autonomous continuation closure

状态：已完成。

目标：关闭完整 Windows 本机验证和 implementation push 已真实完成，但 fresh `status` 因自然语言验证句式未命中而继续停在 `implementation-pending`、`next-batch` 无法接手的长期自主推进断点。完整链为成功 `release-run` → exact pre-commit artifact receipt → direct implementation commit/push → clean synchronized status → strict receipt rebuild → post-push receipt → next-batch selection。

实现方向：成功 `release-run` 为已完成且待 implementation commit 的 latest batch 写一份 Git-local strict canonical v2 validation receipt，绑定 baseline HEAD、catalog gate profile完整有序步骤、全部success exit/attempt、latest Batch ID、exact changed/untracked artifact path/state/mode、验证时工作树SHA-256/bytes与经Git clean filter计算的blob OID；tracked工作树不因此变化。Fresh release handoff仅在当前HEAD是validated baseline之后唯一direct commit，且commit changed set、file status/mode/tree blob OID与receipt完全一致时，才用machine receipt恢复`LocalValidationReady/ReleaseCheckReady`并继续既有post-push reconciliation。旧v1 receipt与prose readiness对Batch 817+均fail-closed；若首个已推送implementation commit暴露receipt runtime缺陷，只允许fresh v2 receipt精确绑定的单个同批repair commit接力，不放宽其它extra commit。

边界：receipt不进入commit、不写case、不执行heavy-tool、不写authority/confirmed、不查询远程CI；missing/旧schema/malformed/tampered/stale receipt，验证后任意编辑，未绑定额外/压缩/merge commit，path/mode/blob/deletion漂移均fail-closed。普通batch仍默认只做一次implementation commit/push且不等待remote workflow；本批因首个已推送commit真实暴露`core.autocrlf`误拒绝，仅使用上述strict同批repair路径，不改写已推送历史。

验证结果：临时真实Git repo覆盖pre-run snapshot、Git-local零tracked污染、`core.autocrlf=true`下工作树bytes→clean-filter blob→direct commit acceptance、Batch 817+无receipt/prose-complete/旧schema/tamper/stale/artifact drift/unbound extra commit fail-closed、strict同批repair acceptance及Batch 816 legacy prose兼容；Windows原子替换成功与失败保留旧receipt均有专项回归。首轮完整`release-run`以7/7通过后，implementation commit `6c680c4`已推送；fresh post-push检查真实暴露旧v1 validator把工作树bytes与normalized Git blob直接比较的Windows误拒绝，现已改为v2双绑定且不force-push。修复后focused与受影响四包完整回归通过（releasecheck 36.013秒、CLI 227.542秒），全仓`go test ./... -count=1`通过（CLI 230.267秒），`go vet ./...`、status、10-pack packs、doctor、canonical skill预算32759/32768 bytes与`git diff --check`通过。独立终审发现并关闭worktree bytes复验、Windows tracked executable mode、validated HEAD精确绑定和含空白Git路径NUL解析四项Important，终复核无剩余高置信Critical/Important。repair commit/push与最终post-push receipt待本轮完成。普通batch不等待或声明remote CI green。

### Batch 816：daily mission campaign orchestration and replacement takeover closure

状态：已完成。

目标：关闭日常 Mission Commander campaign 仍需跨多个手工步骤拼接 external relay、checkpoint resume、reviewed completion和replacement takeover的断点。完整链为natural-language onboard/start → bounded current loop → external member/reviewer handoff → result-first/submission-last → one reviewed external-result turn → relay + strict intake + checkpoint claim + bounded resume → accepted reviewer lineage → evidence-derived lane completion → next lane/Human/external boundary → mission-complete → replacement executor durable takeover。

实现：统一external-result turn在WhatIf中零写入绑定checkpoint、job、submission、relay artifacts、observation和nested resume；member/reviewer planned result snapshot只用于构建reviewed plan，Apply清除overlay并从durable relay filesystem重建。Apply先完成exact-prefix可恢复relay，再strict intake、one-shot claim和bounded resume；relay后若Human intervention/currentness漂移，relay保持truthful且claim在同一project lease内fail-closed。Reviewer replacement result一次Apply走完save input→completion→source→stage→collect→intake六步并产生accepted verification/decision lineage。Status从current owner的intake-ready member manifest调用唯一`CompletePreview` owner派生completion request，replacement handoff保留同一request；current-step不把complete误派为新member attempt，driver与completion plan双hash绑定。全部lane关闭后current-loop以typed`mission-complete`停止且无Apply request。Member latest inspection只把canonical intent或intent→handoff exact prefix视为pending，commit/observation/result/额外artifact缺前件均作为corruption向上报错。

边界：Go runtime不spawn/poll/stop member或reviewer session，不调用shell/Agent tool，不执行heavy-tool，不写或推断authority/confirmed。PowerShell façade仅验证参数组合并逐字透传；WhatIf零写入，Apply按exact hash/currentness/owner/lease guards fail-closed。External turn明确non-transactional：已提交relay在后续拒绝时保留为可验证恢复事实。普通batch不等待或声明remote CI green。

验证结果：member/reviewer composite turn、reviewer snapshot path/SHA/bytes drift、Apply filesystem-only、relay后Human intervention claim门禁、pending dispatch corruption、completion discovery/replacement takeover/double hash与typed mission-complete focused回归通过；受影响`currentloop/mission/memberexecution/externalsession/subagents/workstream/cli`完整包测试最终通过（CLI 209.641秒）。独立终审发现relay非前缀补齐与pending handoff非exact两项Important，已增加全写集preflight、canonical dispatch重建及反例并由原审查者逐项复核关闭，无剩余高置信Critical/Important。修复后完整`go test ./... -count=1`通过（CLI 210.838秒），`go vet ./...`、status、10-pack packs、doctor、PowerShell façade smoke、canonical skill预算32742/32768 bytes与`git diff --check`通过。完成态`release-check -Format json`返回`ready=true` / `summary=release gate inventory ok`；implementation commit/push待记录，普通batch不等待或声明remote CI green。

## 活动文档维护规则

- 本文件只保留当前批次与最近完成批次，常态最多出现 2 个 `### Batch N` 段落；没有进行中批次时只保留最近完成批次。
- 规划新批次时，`next-batch` 先把更早的 active batch 原样追加到 `docs/batch-history.md`，再写入新的当前批次；历史已存在的同一批次必须内容一致，否则停止并要求人工复核。
- 阶段方向变化只更新本文件顶部 Current milestone / Next candidates 与 `docs/autonomous-goal.md` 顶部实施区；不要把逐轮日志、完整验证输出或旧候选继续堆入长期方向区。
- 历史事实不删除；考古时按 Batch ID 搜索 `docs/batch-history.md`，日常接手不要读取归档全文。

## 验证标准

每个 active batch 记录实际执行过的命令及结果；`release-check`/`ciReleaseGate.ready` 只算 inventory readiness，不能替代本地命令执行或远程 job conclusions。优先保持 coherent vertical slice，不用逐字段 metadata batch 维持连续推进。

历史上普通 batch 曾使用两次 push（implementation + release inspection）；自 Batch 792 起，日常节奏改为 Windows 本机完整验证后只做一次 implementation push 并立即继续，远程 workflow 异步、非阻塞。旧批次中的 inspection 记录保留为历史事实，不作为当前 cadence。

## 风险与注意事项

- `docs/batch-plan.md` 是 active/next 的 durable source，不是已完成批次日志。
- `docs/batch-history.md` 是历史归档；不要把它重新并回活动文档，也不要在默认 handoff/read-first 中要求全文读取。
- `CHANGELOG.md` 只记录必要的用户可见变化和边界；逐步实现细节留在 batch history。
- 只有当前用户 goal/session 明确授权时才 commit/push 指定分支。

## 历史批次归档

完整历史已拆到 `docs/batch-history.md`。除非要查旧 batch 细节、验证历史决策或做 release/debug 溯源，不要默认读取历史归档全文。
