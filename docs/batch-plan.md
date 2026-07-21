# Batch implementation plan

## 读取指南

先读本节的 current milestone / current batch state / next candidates；旧批次只在需要考古、验证历史决策或 release/debug 溯源时按 Batch ID 搜索 `docs/batch-history.md`。不要默认从 Batch 0 顺序重读 350+ 个历史批次。产品方向以 `docs/mission-control-product-direction.md` 为准，持续执行方式见 `docs/autonomous-goal.md`。

## 实施摘要

Batch 359 后，Go-owned/no-fallback public command surface、durable lanes、显式 reconcile、typed autonomy preflight、Mission brief / executor action、bounded reviewer dispatch → strict intake → verification-before-decision writeback → post-validation 的本机闭环、pack-memory promote/reconsume package E2E，以及 authorized execution observation evidence + bounded adapter execution report strict intake（含 authorized stopCondition boundaryHits、status summary enforcement、workspace-relative 与 case-relative machine-readable handoff）已形成底座。当前阶段继续从 contract/inventory field increments 转向 replaceable session executor / reviewer orchestration、lane executor / tool-adapter live validation、pack-memory product UX、Windows 本机 product-path 稳定与真实 release verification。

## 执行清单

### Current milestone

**Mission Commander operational closure and truthful release readiness**：把 durable lane/reviewer/autonomy contract 串成实际可运行、可跨会话接手、可验证的产品闭环，并区分 inventory ready、本地 gate executed 与远程 CI green。当前用户短期只要求 Windows 本机稳定可用；远程 Linux/macOS/Windows CI 因 runner/billing blocker 继续记录为 known gap，不阻塞本机 Mission Control 闭环。

### Current batch state

### Batch 494：execution evidence review no-pack handoff product-path smoke closure

状态：已完成 execution evidence review no-pack handoff product-path smoke implementation、durable docs、focused handoff/status coverage 与完整本地 release minimum；commit/push 与远程 release-gate inspection 待执行。

目标：Batch 493 已证明 adapter observation evidence record 后，case-local lane cwd 可无 `-Target` / 无 `-Pack` 通过 `status` 第一屏看到 execution evidence review 与 Mission Commander action queue；但 action queue 的 primary command 是 `/rekit handoff main`，还需要 product-path smoke 证明 replacement executor 直接运行该 handoff primary command（no-target/no-pack、WhatIf/text/JSON）时也能消费同一 evidence review、outputRefs、follow-through outcome 与 action queue，而不必回到 status 或打开 ledger 手工拼接。Batch 494 将该路径补成本机 Windows 可验证 smoke。

边界：只增强本机 product-path coverage 与只读 handoff text visibility，不新增 heavy execution；`handoff -WhatIf` 仍不写 `.rekit`，`handoff -Apply` 仍只写 case-local handoff docs，不写 authority/confirmed；record evidence 仍要求显式 `gate -Apply` 与 `-Actor`，只写 bounded observation evidence；contract/validation/status 语义、sync/promote、case durable schema 与 PowerShell runtime logic 均不改变。

已完成内容：

- 扩展 `writeHandoffText`：在 handoff text 第一屏输出 `executionEvidenceReview[]` 的 event/gate/status/action/review/handoff/commander、outputRefs/evidenceRefs、follow-through、outcome evidence 与 boundary，与 status/overview 的 evidence review text 形状保持一致。
- 扩展 `TestRunGateAdapterReportNoPackProductPathFromNestedOutputWorkspace`：在 no-pack adapter evidence record 与 no-pack status smoke 后，从同一 case-local `workspace/main` lane cwd 无 `-Target` / 无 `-Pack` 运行 `handoff main -WhatIf -Format json`，验证 `_template` pack 来自 case metadata、preview 只读、lane 为 `main`、execution evidence review/action queue/nextSteps 保持 `ready-for-evidence-review` handoff。
- 验证 `handoff main -WhatIf -Format text` 输出 same evidence review、output ref、follow-through outcome evidence 与 Mission Commander action queue current action，且不输出 JSON object；用 `.rekit` snapshot 锁定 no-pack handoff preview/text 均不写 case durable state。

验证结果：已通过 focused `gofmt -w internal/rekit/cli/cli.go internal/rekit/cli/cli_test.go`、`go test ./internal/rekit/cli -run "TestRunGateAdapterReportNoPackProductPathFromNestedOutputWorkspace" -count=1`、`go test ./internal/rekit/cli -run "TestRunGateAdapterReportNoPackProductPathFromNestedOutputWorkspace|TestRunHandoff.*|TestRunStatusCaseMissionIncludesExecutionEvidenceReview|TestRunCaseLocalProductPathUsesCaseMetadataRuntime" -count=1`，以及完整本地 release minimum：`go run ./cmd/rekit -- -Command release-check -Format json`、`go run ./cmd/rekit -- -Command status`、`go run ./cmd/rekit -- -Command packs`、`go run ./cmd/rekit -- -Command doctor`、`go test ./...`、`go vet ./...`、`git diff --check`；`release-check ready=true`，status handoff recorded，go test ./... recorded，go vet ./... recorded，git diff --check recorded（仅 Windows LF/CRLF conversion warnings，无 whitespace error）。commit/push 与远程 release-gate 尚未重新检查，不能声明远程 CI green。

上一批摘要：Batch 493 已完成 execution evidence review no-pack status product-path smoke closure，详见 `docs/batch-history.md`。

### Next candidates

1. **Lane/tool-adapter live validation residuals（如仍有缺口）**：Batch 421/424/432 已覆盖 adapter contract/validation/report action queue、follow-through 与 contract liveValidation text，Batch 458 已补齐 authorizedExecutionFollowThrough outcome when/evidence text，Batch 459 已补齐 project/lane handoff Markdown 的 execution evidence followThrough outcome when/evidence，Batch 460 已补齐 overview text 的 execution evidence followThrough outcome when/evidence，Batch 461 已补齐 lane RESUME 的 execution evidence followThrough outcome when/evidence，Batch 462 已补齐 continue run digest 的 execution evidence followThrough outcome when/evidence；后续仅在 Windows 本机 product-path 仍存在 validation/record/evidence review handoff 到 replacement executor 的真实断点时推进；不新增 adapter/heavy-tool execution。
2. **Reviewer orchestration E2E residuals（如仍有缺口）**：仅在 dispatch/intake/post-validation terminal path 仍要求解析 nested JSON 或打开 artifact 才能接续时推进，不自动 spawn reviewer、不执行 heavy-tool。
3. **Pack-memory downstream UX residuals（如仍有缺口）**：Batch 423/429/430/431 已覆盖 candidate decision outcome、execution plan、WhatIf preview 与 review checklist text，Batch 456/457 已补齐 decision detail、cleanup action detail 与 reconsume command/boundary detail；后续仅在 accepted/rejected 人工流程、cleanup/reconsume 或 evidence review downstream UX 仍需跨 envelope 手工拼接时推进，不重复做字段微批次。
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
