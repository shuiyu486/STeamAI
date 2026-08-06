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

本轮按 `docs/health-recovery-and-real-executor-plan.md` 执行纠偏。阶段 1–3 的实现与 focused 验证已完成：默认 `vmp-re` public emitted-route 健康恢复；deterministic runtime 外新增 `cmd/rekit-host`，自动 attempt/claim、启动真实 Claude Code structured-output session、记录 accepted launch、真实结果落盘、submission-last、strict intake、durable recovery 与失败 replacement；explicit fresh `vmp-re` live gate 已真实走通自然语言 onboarding → 第一代 member → 人工纠偏 → 第二代 member → 独立 Reviewer → strict accepted lineage → feature completion，receipt 为 `passed=true`、`manualPlaceholders=0`、`manualResultWrites=0`、`cleanup=removed`。同一 manifest 已审核后不会重复规划 Reviewer；accepted completion 会从 canonical `ReviewerResult` 重验真实 accept 语义，拒绝账本把真实 reject 结果伪称为 accepted lineage；session receipt 将 owner/attempt/host-run launch 三种身份拆分。独立定向复核确认该防篡改修复为 `FIXED`。最终 Windows 本机验证通过：完整 CLI 包 `1649.745s`；修复默认 pack literal invariant 后，全仓 `go test ./... -timeout 40m -count=1` 通过（CLI `1625.888s`），`go vet ./...` 与 `git diff --check` 通过；`release-check` 返回 `ready=true` / 7 steps，status、10-pack packs、doctor 通过。最终 fresh real-Claude gate receipt `fixed-15` 为 `passed=true`、两代 member 各一次真实 completion、一个独立 Reviewer completion、`manualPlaceholders=0`、`manualResultWrites=0`、feature lane `closed`、`cleanup=removed`；仅保留仓库外最终 receipt。测试代码不得伪造 LLM 结果。

### Batch 821：unified current-step external session campaign handoff and resume closure

状态：已完成。

目标：Mission Commander或replacement executor从fresh status只使用`run-current-step`即可消费member/reviewer external attempt、claim、actual launch truth、running handoff与submission result resume，不再退出统一入口手工拼接nested `run-current-loop` modes。输入不足返回typed handoff；输入齐全时outer hash绑定exact nested plan；running不伪造liveness；result-ready继续到strict intake后的下一typed campaign boundary。

边界：不新增PowerShell runtime logic；Go runtime不spawn/poll/stop外部session、不调用Agent、不执行heavy-tool、不写authority/confirmed。Claim不代表launch，accepted launch不代表liveness；replacement只fence generation，不声称停止旧process。Observation inbox和explicit observation保持高于dispatcher/current-step external overlay；nested `run-current-loop`保留为external host和恢复/诊断API。

验证结果：Windows临时case E2E覆盖first/replacement attempt、ticket-only recovery、queued claim、accepted/failed launch、running wait/reconnect/explicit replacement、invalid submission replacement、member/reviewer result turn、observation priority与status/quickstart/takeover统一wrapper；attempt/claim/launch/recovery/replacement均覆盖成功响应丢失后的exact committed replay。External route按mode拒绝wrong flags；outer hash绑定route、stable public current request与exact nested job/checkpoint/attempt/dispatch/claim/submission identity。Result turn在relay已提交而checkpoint claim被Human-in-the-Lane拒绝时返回stage-aware、可解码`nested-partial` receipt，不否认已落盘mutation。PowerShell façade仅透传attempt/claim/launch/replacement参数，smoke通过；canonical skill精确32768 bytes。受影响六包完整测试通过（CLI 235.194秒），修复后全仓`go test ./... -count=1`（CLI 234.698秒）、`go vet ./...`与`git diff --check`通过。独立correctness/security/architecture审查发现的outer replay、priority、wrong-mode input、partial receipt、public nested route、outer identity、ticket recovery与running replacement问题均已修复；最后定向复核无surviving finding。README、canonical skill、Agent Team usage、release readiness、CHANGELOG与release invariants已同步；根CLAUDE、pack manifest/config/example无需修改，因为产品边界、schema与配置面未变。完成态`release-check -Format json`返回`ready=true`；status、10-pack packs、doctor通过。Windows `release-run -Format json`以7/7通过（581.421秒，其中完整Go tests 578.200秒），全部步骤attempts=1并生成Git-local v2 validation receipt。Implementation commit/push随后记录。普通batch不等待或声明remote CI green。

### Batch 820：durable external session inbox dispatcher closure

状态：已完成。

目标：把Batch 819的动态harness package收敛为durable case-local external session dispatcher。Attempt Apply按ticket-first/receipt-last发布；fresh replacement Mission Commander能识别ticket-only中断并执行从immutable ticket重建的exact恢复命令。Owner reservation、exclusive dispatch claim和actual launch truth分别记录；attempt只进入`committed`，只有accepted launch receipt才能派生actual `running`。Submission绑定current attempt、claim、accepted launch receipt与actual harness/session；replacement generation隔离旧代迟到claim/launch/result。

边界：Go runtime只管理deterministic request/claim/receipt/state/currentness，不spawn/poll/stop外部Claude Code session、不调用Agent、不执行heavy-tool、不写authority/confirmed。PowerShell façade只校验和透传新flags，不扫描inbox、不计算hash、不决定dispatcher state或发布receipt；public command数量、pack schema、case-local thin shim和sync/promote review-first边界不变。

验证结果：dispatcher/externalsession/CLI focused回归覆盖ticket-first partial recovery、首代ticket-only status/quickstart/takeover exact request与SHA同源、exclusive concurrent claim、accepted/failed launch、actual identity lineage、dispatch-required不可降级、current/pending generation分离、replacement fence、committed replay fresh input/lineage复验、strict malformed/unknown/trailing/noncanonical JSON、tamper与non-Windows symlink拒绝；Linux/Darwin/WASM compile-only通过。受影响五包完整测试、修复后全仓`go test ./... -count=1`（CLI 230.034秒）、`go vet ./...`与façade smoke通过。独立correctness/security/architecture终审发现的ticket模板lineage、reviewer actual identity、pending/current generation、legacy降级、replay drift、takeover同源、attempt状态语义和publication durability问题均已修复；最后两轮定向复核无surviving finding。受影响文档已同步README、canonical skill、Agent Team usage、release readiness和CHANGELOG；PowerShell deprecation无需新增矩阵行，因为public ownership未变。完成态`release-check -Format json`返回`ready=true`；status、10-pack packs、doctor与diff check通过。统一`release-run -Format json`以7/7通过（551.058秒，其中完整Go tests 548.272秒），全部步骤attempts=1并生成Git-local v2 validation receipt。Implementation commit/push随后记录；普通batch不等待或声明remote CI green。

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
