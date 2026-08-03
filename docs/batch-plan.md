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

### Batch 810：active batch plan bounded rotation and durable history archival

状态：已完成 active batch plan bounded rotation、durable history archival、三文档 exact-prefix 恢复与独立终审；implementation commit/push 待当前统一完成。本批解决 `docs/batch-plan.md` 已重新积累 271 个旧批次、上下文压缩后接手会被历史细节淹没的问题；将迁移前实际存在的 Batch 537–808 section 原样迁入 `docs/batch-history.md`，并让后续 `next-batch` 在规划新批次时自动把更早的 active batch 归档，防止活动文档再次膨胀。

目标：`docs/batch-plan.md` 顶部只承载 current milestone、next candidates、当前批次和最近完成批次；旧批次可按 Batch ID 在 `docs/batch-history.md` 找回。`docs/autonomous-goal.md` 明确方向变化写回顶部实施区、聊天 goal 保持简短，并通过仓库测试锁定 active batch 数量上限。

边界：历史内容原样迁移，不删除事实；日常接手不默认读取历史全文。`next-batch` 仍保持 review-first 和 expected-hash Apply，只写三份 kit 文档，不触碰 case state、不执行 heavy-tool、不写 authority/confirmed、不自动 commit/push 或声明远程 CI green。

验证结果：迁移前活动文档中实际存在的 271 个 Batch 537–808 section 已逐段按完整正文核对，全部原样进入历史归档且无遗漏/重复；原文没有独立 Batch 565 section，因此未伪造历史。活动文档现仅含 Batch 810 与 Batch 809，并由 repository invariant 锁定常态最多两个 active Batch。`next-batch` focused tests 覆盖 WhatIf 零写入、history-first rotation、同 Batch ID 内容冲突与 active/history ID collision 拒绝、无歧义长度前缀 planning hash、hash-bound Apply、0/1/2/3 exact write prefix 恢复、selection package 已关闭后的 committed response-loss replay、non-prefix drift fail-closed，并在 publication 后对三份文件做最终 exact-byte 复验；Apply 在 repo-scoped kit mutation lease 内重建计划，单文件写入使用同目录临时文件 `Sync` 后原子替换。最终 focused CLI 回归通过（41.185 秒），Linux/Darwin kit mutation lock 交叉编译通过，`releasecheck/defaultdocs/manifest` focused packages 通过，最终完整 `go test ./... -count=1` 通过（CLI 346.624 秒），`go vet ./...`、`status`、`packs`、`doctor` 与 `git diff --check` 通过；`git diff --check` 仅输出 Windows LF→CRLF 提示。完成态 `release-check -Format json` 返回 `ready=true` / `summary=release gate inventory ok`；普通 batch 不等待或声明 remote CI green。

### Batch 809：durable member-lane execution handoff, result intake, reviewer relay, replacement takeover, and evidence-bound completion

状态：已完成 durable member-lane execution handoff、strict result intake、reviewer relay、replacement generation fence、current-loop checkpoint/resume、evidence-bound completion、独立终审与 Windows 本机 release minimum；implementation commit/push待统一完成。本批选择 `mission-commander`，关闭主Agent只能在旧聊天中隐式管理member session handoff/result、replacement无法判断迟到结果且complete可遗漏latest manifest的日常断点；本批不是字段、文案或summary投影微调。

目标：把 `mission-commander` candidate 收敛成 Windows 本机可验证的闭环：durable member-lane execution handoff, result intake, reviewer relay, replacement takeover, and evidence-bound completion。实现应复用既有 typed handoff/envelope 和 deterministic runtime 边界，让 Mission Commander 或 replacement executor 能从 durable docs/status 消费结果，不依赖上一会话隐性上下文；focused work 必须证明该候选命令所描述的能力：select a Mission Commander operational closure slice with status/handoff/continue product-path verification。

边界：本批不新增 PowerShell runtime logic，不执行 heavy-tool，不写 authority/confirmed，不自动执行 reviewer/adapter/pack-memory/gate/sync/promote mutation，不自动提交或声明 remote CI green；`/rekit next-batch -Apply` 只在 expected hash 匹配时写 `docs/batch-history.md`、`CHANGELOG.md` 与 `docs/batch-plan.md` 三份 exact planning receipt，并可从已完成的 exact write prefix 恢复。

验证结果：新增唯一`internal/rekit/memberexecution` owner，以intent→handoff→commit exact prefix publication和accepted/returned/failed append-only observation管理single-lane attempt；dispatch/observation Apply共享canonical lane mutation lease，锁内重建exact plan并重验owner generation、open intervention、transition与manifest，持锁到publication及final inspect。returned intake用anchored bounded reads把manifest与全部declared outputs按outputs→manifest→observation固化到immutable canonical evidence namespace，exact-prefix恢复、non-prefix拒绝、public reconcile旧generation竞态和returned/failed竞争终态均有回归；completion自动绑定manifest、全部outputs及canonical reviewer input，并在intent后、final commit前、`InspectLaneCompletion`与terminal status重验path/SHA/bytes、packet/route/shard/session、dispatch/completion/current owner/adoption lineage与exact member evidence set。逐组件no-follow拒绝symlink/Windows reparse，平台语义path key拒绝Linux/Darwin大小写alias、cleaned duplicate、额外/遗漏output；partial publication继续fail-closed可恢复。`run-current-step`和`run-current-loop`以nested+outer hash承载dispatch/observation；resume apply command完整保留attempt/outcome/reason/observedAt/member plan SHA，accepted与returned均执行真实checkpoint-bound Apply；direct member step真实刷新status，已mutation但refresh失败时stdout保留truthful partial receipt。Fresh临时case走通public onboard→status→overview/start→intent-only recovery→accepted→returned→真实continue receipt/fresh typed status→existing reviewer planner/wave→accepted verification/decision writeback→complete；未review、packet/input/receipt lineage drift或namespace漂移均阻断，completion复用existing packet integrity与facts lineage，不复制reviewer runtime。Go不spawn/poll/stop session、不执行heavy-tool、不写authority/confirmed；PowerShell仅为两个既有runner透传flags，public surface保持30命令。最终focused `fs/memberexecution/currentloop/workstream/cli`、全仓`go test ./... -count=1`（CLI 188.392秒）、Linux/Darwin受影响包交叉编译、`go vet ./...`、façade smoke、doctor（canonical skill 32758/32768 bytes）、30-command inventory与`git diff --check`通过；产品与安全定向终复核均无confirmed Critical/Important。完成态`release-check -Format json`通过，统一`release-run -Format json`以7/7通过（411.705秒，其中完整Go tests 408.788秒），所有步骤attempts=1。普通batch不等待或声明remote CI green。

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
