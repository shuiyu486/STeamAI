# Batch implementation plan

## 读取指南

先读本节的 current milestone / current batch state / next candidates；旧批次只在需要考古、验证历史决策或 release/debug 溯源时按 Batch ID 搜索 `docs/batch-history.md`。不要默认从 Batch 0 顺序重读 350+ 个历史批次。产品方向以 `docs/mission-control-product-direction.md` 为准，持续执行方式见 `docs/autonomous-goal.md`。

## 实施摘要

Batch 359 后，Go-owned/no-fallback public command surface、durable lanes、显式 reconcile、typed autonomy preflight、Mission brief / executor action、bounded reviewer dispatch → strict intake → verification-before-decision writeback → post-validation 的本机闭环、pack-memory promote/reconsume package E2E，以及 authorized execution observation evidence + bounded adapter execution report strict intake（含 authorized stopCondition boundaryHits、status summary enforcement、workspace-relative 与 case-relative machine-readable handoff）已形成底座。当前阶段继续从 contract/inventory field increments 转向 replaceable session executor / reviewer orchestration、lane executor / tool-adapter live validation、pack-memory product UX、Windows 本机 product-path 稳定与真实 release verification。

## 执行清单

### Current milestone

**Mission Commander operational closure and truthful release readiness**：把 durable lane/reviewer/autonomy contract 串成实际可运行、可跨会话接手、可验证的产品闭环，并区分 inventory ready、本地 gate executed 与远程 CI green。当前用户短期只要求 Windows 本机稳定可用；远程 Linux/macOS/Windows CI 因 runner/billing blocker 继续记录为 known gap，不阻塞本机 Mission Control 闭环。

### Current batch state

### Batch 508：reviewer intake repair guidance planning handoff closure

状态：已完成 reviewer intake repair guidance planning artifacts、terminal text handoff、dispatch prompt guardrails、用户与 durable docs、focused runtime/CLI coverage、完整本地 release minimum、commit/push 与远程 release-gate inspection；本地验证通过，远程 release-gate 仍需按 completed failure + jobs `steps: []` 的既有 GitHub Actions runner/billing blocker 记录，不能声明 remote CI green。

目标：Batch 507 已让 blocked / event-id-collision / post-validation failed reviewer intake 在失败后返回 `repairGuidance[]` 与 terminal repair action/evidence/boundary；但 planning packet、summary 与 `plan-subagents -Format text` 在 dispatch 或构造 ReviewerResult 前仍未提前告诉主 Agent / replacement executor 哪些 reviewer output 会导致 blocked intake，以及如何修复。Batch 508 将同一 repair taxonomy 前移到 planning handoff，减少 dispatch/reviewer/result authoring 阶段的 JSON 回查与 blocked 后返工。

边界：只增强 plan-subagents planning artifacts / terminal handoff 与 reviewer prompt guardrails；不放宽 reviewer result validation、evidenceRef validation、owner binding、route output contract、deterministic event IDs、verification-before-decision 写入顺序、postValidation JSON contract、sync/promote、case durable schema 或 PowerShell runtime logic；runtime 仍不 spawn/monitor reviewer、不执行 heavy-tool、不写 authority/confirmed、不修改 managed/project source files；reviewer intake 仍由主 Agent 显式 WhatIf/Apply 拥有写回。

已完成内容：

- `ReviewerIntakeCommands` 新增 `repairGuidance[]`，planning packet JSON 直接携带缺证据、冲突、blocked action、verdict mismatch、low-confidence、event-id collision/post-validation failure 等 repair taxonomy。
- out-of-case / dispatch-only planning commands 追加 target 未 attach repair guidance，明确不要把 out-of-case artifacts 当成 runnable reviewer intake commands，也不要写 verification/decision ledger events。
- `summary.md` shard handoff 输出 `repair-guidance`、`repair-evidence` 与 `repair-boundary`，并在 checklist/conflict handling/writeback blockers 中指向 `reviewerIntakeCommands.repairGuidance`。
- `plan-subagents -Format text` 输出 `plan-subagents reviewer intake repair guidance/evidence/boundary` 行，让 terminal replacement executor 不必打开 packet JSON 即可看到 blocked intake 修复动作与 no-apply / no-heavy / no-authority 边界。
- reviewer dispatch prompt 增加避免 blocked intake 的 authoring guardrails：inspectable evidenceRefs、conflicts empty unless unresolved、recommendedVerdict 与 decision mapping 对齐、`tool_scope` 保持 read-only、不可避免时返回 safer needs-more-evidence/defer 并交给主 Agent 消费 repair guidance。
- README、`/rekit` skill、Agent Team usage、CHANGELOG 与 batch docs 同步说明 planning-stage repair guidance handoff。

验证结果：已通过 `gofmt -w internal/rekit/subagents/plan.go internal/rekit/subagents/plan_test.go internal/rekit/cli/cli.go internal/rekit/cli/cli_test.go`、focused `go test ./internal/rekit/subagents ./internal/rekit/cli -run "TestWritePlanIncludesShardHandoffs|TestWritePlanBindsAttachedCaseLaneExecutor|TestRunPlanSubagentsWritesReviewArtifacts|TestRunPlanSubagentsItemsFileAndOutOfCaseGuard" -count=1`，以及完整本地 release minimum：`go run ./cmd/rekit -- -Command release-check -Format json`、`go run ./cmd/rekit -- -Command status`、`go run ./cmd/rekit -- -Command packs`、`go run ./cmd/rekit -- -Command doctor`、`go test ./...`、`go vet ./...`、`git diff --check`；`release-check ready=true` recorded，status handoff recorded，packs inventory recorded，doctor validation recorded，go test ./... recorded，go vet ./... recorded，git diff --check recorded（仅 Windows LF/CRLF conversion warnings，无 whitespace error）。远程 release-gate inspection 待 commit/push 后记录；若 jobs 仍为 `steps: []`，按既有 blocker 处理。

上一批摘要：Batch 507 已完成 reviewer intake blocked repair terminal handoff closure，并归档到 `docs/batch-history.md`。

### Next candidates

1. **Pack-memory downstream UX residuals（如仍有缺口）**：Batch 500 已把 candidate decision/cleanup/reconsume expected evidence 收口为 `reviewArtifacts[]`，Batch 501 已把 open candidate residue 投影到 release/status handoff，Batch 502/503 已补齐 case-local status/default path、candidate identity、index mapping 与 derived review artifact visibility；后续仅在 accepted/rejected 人工流程、cleanup/reconsume 或 evidence review downstream UX 仍需跨 envelope 手工拼接时推进，不重复做字段微批次。
2. **Lane/tool-adapter live validation residuals（如仍有缺口）**：Batch 421/424/432 已覆盖 adapter contract/validation/report action queue、follow-through 与 contract liveValidation text，Batch 458-462 已补齐 authorizedExecutionFollowThrough / evidence review follow-through text，Batch 504 已补齐 recorded adapter sidecar path、actualBudget 与 adapter provenance；后续仅在 Windows 本机 product-path 仍存在 validation/record/evidence review handoff 到 replacement executor 的真实断点时推进；不新增 adapter/heavy-tool execution。
3. **Reviewer orchestration E2E residuals（如仍有缺口）**：Batch 489/499/505 已覆盖 reviewer intake product path、reviewer writeback identity 与 reviewer result provenance downstream handoff；后续仅在 dispatch/intake/post-validation terminal path 仍要求解析 nested JSON、打开 reviewer result artifact 或手工拼 route output 才能接续时推进，不自动 spawn reviewer、不执行 heavy-tool。
4. **Cross-platform product-path E2E（降优先级）**：在本地 CLI/case E2E 已覆盖 nested cwd / case shim 的基础上，仅保持可在 runner 可用时执行的三平台 matrix 候选和 known gap 记录；不要在 GitHub runner/billing blocker 未解除前让它阻塞 Windows 本机迭代。
5. **Retained public façade decision**：只有真实 release-gate-green、public references、case shim、smoke retirement 与恢复计划均满足后，才执行独立 removal batch；否则明确保留期限和 blocker。

### Escalation / stopping conditions

产品方向变化、runtime/policy durable schema 迁移、confirmed/authority 策略变化、未授权外部副作用、公共入口删除门禁不完整或真实 release gate 无法验证时升级。完成单个 batch、inventory ready、push 成功或工作树干净都不是长期 goal 完成。

## 验证标准

每个 active batch 记录实际执行过的命令及结果；`release-check`/`ciReleaseGate.ready` 只算 inventory readiness，不能替代本地命令执行或远程 job conclusions。优先保持 coherent vertical slice，不用逐字段 metadata batch 维持连续推进。

## 风险与注意事项

- `docs/batch-plan.md` 是 active/next 的 durable source，不只是一份已完成批次日志。
- `docs/batch-history.md` 是历史归档；不要把它重新并回 `docs/batch-plan.md`，也不要在默认 handoff/read-first 中要求全文读取。
- `CHANGELOG.md` 记录必要的用户可见变化和边界；逐步 plumbing 留在 batch history。
- 只有当前用户 goal/session 明确授权时才 commit/push 指定分支。

## 历史批次归档

完整历史已拆到 `docs/batch-history.md`。除非要查旧 batch 细节、验证历史决策或做 release/debug 溯源，不要默认读取历史归档全文。
