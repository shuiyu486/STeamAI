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




### Batch 813：canonical external session observation inbox discovery and one-shot checkpoint resume handoff

状态：已完成。

目标：让外部member/reviewer harness只需把strict envelope写入canonical case-local inbox，fresh status即可唯一发现并生成checkpoint-bound WhatIf接力；成功Apply返回one-shot processed receipt并由successor checkpoint恢复，不再要求replacement executor记住文件路径。

实现：`.rekit/external-session-observations/inbox/*.json`由case-root anchored no-follow/reparse-safe bounded枚举读取，并复用既有strict envelope decode、checkpoint/attempt/capability currentness校验。Fresh status只在恰好一个候选匹配latest ready checkpoint及exact member/reviewer attempt时返回typed `observationInbox.selectedDriverRequest`；多个匹配、任一invalid entry或namespace异常均fail-closed，旧checkpoint候选只计数且不参与选择。WhatIf与Apply继续绑定exact path/bytes SHA/source checkpoint/nested+outer hashes，Apply前再次确认同一路径/SHA仍是唯一current strict候选；legacy member/reviewer templates继续以underlying resume request为基底，不混入互斥的observation-path参数。成功Apply返回processed`observationReceipt`；successor checkpoint以可选kind/actor及既有source/path/SHA lineage供fresh status恢复完整typed receipt，旧Batch 812 path/SHA-only checkpoint仍可读取。Receipt独立投影于top-level current-loop operator，因此successor已不再等待observation时，status/replacement takeover/handoff仍可恢复处理结果。

边界：inbox discovery只读，不删除/移动文件、不claim checkpoint、不自动Apply；source checkpoint one-shot claim阻断replay。Runtime不spawn/poll/stop外部session、不执行heavy-tool、不写authority/confirmed；不新增PowerShell runtime logic，旧Batch 812 checkpoint schema保持兼容。

验证结果：focused regressions覆盖unique discovery、path-only preview、hash-bound Apply、processed receipt、fresh status recovery、stale/ambiguous/invalid fail-closed、anchored directory enumeration、旧legacy resume与member/reviewer campaign；受影响`fs/currentloop/mission/workstream/cli`包与完整`go test ./... -count=1`通过（CLI 347.100秒），`go vet ./...`、`status`、`packs`、`doctor`和`git diff --check`通过。独立correctness/security审查发现的exact attempt qualification、legacy template参数互斥、typed receipt完整性与Apply前唯一性重检问题均已关闭，终复核无高置信Critical/Important。完成态`release-check -Format json`返回`ready=true` / `summary=release gate inventory ok`；普通batch不等待或声明remote CI green。

### Batch 812：unified external session observation envelope intake for checkpoint-bound campaign resume

状态：已完成。

目标：让外部member/reviewer harness只写一次case-local strict observation envelope，Mission Commander或replacement executor即可从durable status/handoff执行checkpoint-bound WhatIf与hash-bound Apply，不再手工拼装attempt、actor和多组observation flags。

实现：Status、quickstart、replacement takeover、handoff JSON/Markdown/text为member/reviewer accepted/returned/failed alternatives返回绑定exact checkpoint与attempt的`observationEnvelopeTemplate`和统一`observationPathCommand`。`run-current-loop -CurrentLoopObservationPath`以case-local、symlink-free、≤64 KiB stable read和strict JSON intake一次observation；WhatIf把absolute path与exact bytes SHA纳入outer plan identity并返回path-only Apply，Apply在resume claim前和nested mutation前重读同一文件。Envelope展开后复用既有member/reviewer runner，并重新验证member owner/state/capability与reviewer attempt currentness；successor checkpoint把observation path/SHA纳入可重算segment identity。Legacy flags仍兼容，但不能与envelope或`-Actor`混用。PowerShell façade只透传path/hash，不解析envelope。

边界：Go runtime不spawn/poll/stop member或reviewer session，不执行heavy-tool、不写authority/confirmed；direct reviewer result write保持既有fresh refresh路径，不用predecessor envelope；不放宽member manifest/output、reviewer result、checkpoint one-shot/currentness或lane owner generation guards，不新增PowerShell runtime logic。

验证结果：focused `currentloop/cli/workstream`回归、member accepted/returned E2E、reviewer accepted/returned/failed campaign E2E、strict unknown/trailing/oversize/outside-case与参数互斥、exact-byte drift、checkpoint identity tamper、status/handoff projections及PowerShell façade smoke通过。受影响四包完整测试通过（CLI 195.179秒）；修复审查项后完整`go test ./... -count=1`通过（CLI 193.498秒），`go vet ./...`、`status`、10-pack `packs`、`doctor`、façade smoke与`git diff --check`通过。独立correctness/security审查确认祖先目录namespace TOCTOU与claim后reopen烧毁remaining budget两项Important；改用pinned `os.Root` anchored no-follow/reparse-safe reader，并让claim后只消费已审核immutable materialization，定向复核确认两项关闭且无新的高置信Critical/Important。完成态`release-check -Format json`返回`ready=true` / `summary=release gate inventory ok`；implementation commit/push待统一记录，普通batch不等待或声明remote CI green。

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
