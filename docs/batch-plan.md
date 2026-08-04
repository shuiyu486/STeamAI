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


### Batch 811：bounded Mission Commander multi-segment run loop with durable external session handoff

状态：已完成。本批将bounded Mission Commander current-loop从reviewer单边接力扩展为统一durable external member/reviewer session handoff；主Agent或replacement executor可从status、quickstart、handoff与strict checkpoint恢复剩余预算和下一项exact observation preview，不依赖旧聊天，也不手工拼装attempt/checkpoint flags。

目标：让Mission Commander在member或reviewer外部session边界保存strict remaining-budget checkpoint，并让当前或replacement executor从统一durable operator package提交exact observation后继续下一segment。

实现：`currentLoopOperator.externalMemberHandoff`绑定member attempt、lane、owner generation、handoff/manifest/output路径及accepted/returned/failed alternatives，并随segment continuation和checkpoint持久化。External member checkpoint必须提交匹配attempt的恰好一个observation，空resume不得消耗remaining budget；accepted后能力收敛为returned/failed，returned前必须写strict manifest和全部declared bounded outputs，intake-ready后撤销旧handoff。Status、replacement takeover、handoff JSON与durable Markdown消费同一operator package和checkpoint-bound模板。Strict schema拒绝attempt generation、canonical paths、state、capability set、embedded contract漂移及非member stop夹带member handoff。

边界：Go runtime不spawn/poll/stop member或reviewer session，不执行heavy-tool、不写authority/confirmed；不新增PowerShell runtime logic，不自动跨external session边界Apply，不放宽既有member manifest/output、reviewer result、checkpoint one-shot/currentness或lane owner generation guards。

验证结果：focused `currentloop/mission/workstream/cli`回归通过；受影响四包完整测试通过（CLI 197.849秒），覆盖dispatch→checkpoint、status/quickstart/replacement/handoff JSON/Markdown/default text投影、空resume与不同attempt resume拒绝、accepted 3→2能力收敛、strict returned intake、`member-intake-ready`无旧handoff successor保留剩余预算，以及generation/path/state/capability/embedded-contract schema tamper fail-closed。完整 `go test ./... -count=1` 通过（CLI 201.798秒），`go vet ./...`、`status`、10-pack `packs`、`doctor` 与 `git diff --check` 通过。独立correctness/security审查最初确认3项Important：checkpoint attempt替换、returned剩余预算checkpoint失败与default text遗漏；修复后逐项复核关闭，未发现新的高置信Critical/Important。完成态`release-check -Format json`返回`ready=true` / `summary=release gate inventory ok`；implementation commit/push待统一记录，普通batch不等待或声明remote CI green。

### Batch 810：active batch plan bounded rotation and durable history archival

状态：已完成 active batch plan bounded rotation、durable history archival、三文档 exact-prefix 恢复与独立终审；implementation commit/push 待当前统一完成。本批解决 `docs/batch-plan.md` 已重新积累 271 个旧批次、上下文压缩后接手会被历史细节淹没的问题；将迁移前实际存在的 Batch 537–808 section 原样迁入 `docs/batch-history.md`，并让后续 `next-batch` 在规划新批次时自动把更早的 active batch 归档，防止活动文档再次膨胀。

目标：`docs/batch-plan.md` 顶部只承载 current milestone、next candidates、当前批次和最近完成批次；旧批次可按 Batch ID 在 `docs/batch-history.md` 找回。`docs/autonomous-goal.md` 明确方向变化写回顶部实施区、聊天 goal 保持简短，并通过仓库测试锁定 active batch 数量上限。

边界：历史内容原样迁移，不删除事实；日常接手不默认读取历史全文。`next-batch` 仍保持 review-first 和 expected-hash Apply，只写三份 kit 文档，不触碰 case state、不执行 heavy-tool、不写 authority/confirmed、不自动 commit/push 或声明远程 CI green。

验证结果：迁移前活动文档中实际存在的 271 个 Batch 537–808 section 已逐段按完整正文核对，全部原样进入历史归档且无遗漏/重复；原文没有独立 Batch 565 section，因此未伪造历史。活动文档现仅含 Batch 810 与 Batch 809，并由 repository invariant 锁定常态最多两个 active Batch。`next-batch` focused tests 覆盖 WhatIf 零写入、history-first rotation、同 Batch ID 内容冲突与 active/history ID collision 拒绝、无歧义长度前缀 planning hash、hash-bound Apply、0/1/2/3 exact write prefix 恢复、selection package 已关闭后的 committed response-loss replay、non-prefix drift fail-closed，并在 publication 后对三份文件做最终 exact-byte 复验；Apply 在 repo-scoped kit mutation lease 内重建计划，单文件写入使用同目录临时文件 `Sync` 后原子替换。最终 focused CLI 回归通过（41.185 秒），Linux/Darwin kit mutation lock 交叉编译通过，`releasecheck/defaultdocs/manifest` focused packages 通过，最终完整 `go test ./... -count=1` 通过（CLI 346.624 秒），`go vet ./...`、`status`、`packs`、`doctor` 与 `git diff --check` 通过；`git diff --check` 仅输出 Windows LF→CRLF 提示。完成态 `release-check -Format json` 返回 `ready=true` / `summary=release gate inventory ok`；普通 batch 不等待或声明 remote CI green。

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
