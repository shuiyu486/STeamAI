# Batch implementation plan

## 读取指南

先读本节的 current milestone / current batch state / next candidates；旧批次只在需要考古、验证历史决策或 release/debug 溯源时按 Batch ID 搜索 `docs/batch-history.md`。不要默认从 Batch 0 顺序重读 350+ 个历史批次。产品方向以 `docs/mission-control-product-direction.md` 为准，持续执行方式见 `docs/autonomous-goal.md`。

## 实施摘要

Batch 359 后，Go-owned/no-fallback public command surface、durable lanes、显式 reconcile、typed autonomy preflight、Mission brief / executor action、bounded reviewer dispatch → strict intake → verification-before-decision writeback → post-validation 的本机闭环、pack-memory promote/reconsume package E2E，以及 authorized execution observation evidence + bounded adapter execution report strict intake（含 authorized stopCondition boundaryHits、status summary enforcement、workspace-relative 与 case-relative machine-readable handoff）已形成底座。当前阶段继续从 contract/inventory field increments 转向 replaceable session executor / reviewer orchestration、lane executor / tool-adapter live validation、pack-memory product UX、Windows 本机 product-path 稳定与真实 release verification。

## 执行清单

### Current milestone

**Mission Commander operational closure and truthful release readiness**：把 durable lane/reviewer/autonomy contract 串成实际可运行、可跨会话接手、可验证的产品闭环，并区分 inventory ready、本地 gate executed 与远程 CI green。当前用户短期只要求 Windows 本机稳定可用；远程 Linux/macOS/Windows CI 因 runner/billing blocker 继续记录为 known gap，不阻塞本机 Mission Control 闭环。

### Current batch state

### Batch 520：execution evidence review summary boundary product-path smoke closure

状态：已完成 runtime/test/docs implementation、focused/package validation 与完整本地 release minimum；implementation commit/push 与远程 release-gate inspection 待完成后记录。远程 release-gate inspection 待 implementation commit push 后执行；在检查真实 GitHub Actions jobs 前不能声明 remote CI green。

目标：Batch 519 已新增 compact `executionEvidenceReviewSummary`，但 replacement executor 的真实 product path 还需要证明该 summary 在 case-local/no-pack/nested output workspace 中不只存在于 package-level JSON：succeeded adapter provenance、boundary-hit stopCondition 与 adapter/main escalation 场景都应能从 status/overview/handoff/continue artifacts 第一屏或 compact summary 中看到 latest report、adapter id/status、boundary/escalation、Mission Commander action queue、follow-through 与 no-heavy/no-authority 边界。

边界：只增强已记录 authorized execution observation evidence summary 的 product-path visibility、Markdown/digest/RESUME text projection 与 CLI E2E smoke；不执行 heavy tool、不 replay adapter、不改变 evidence record validation、sidecar schema、authorized gate decision、sync/promote、case durable schema、authority/confirmed、PowerShell façade 或公共 façade removal 门禁。`continue -Apply` 仍不写 authority/confirmed，`gate -Apply -GateEventId ...` 仍只记录 bounded observation evidence。

已完成内容：

- Project/lane handoff Markdown、continue digest、lane `RESUME.md` 与 overview text 的 execution evidence review summary lines 现在输出 action queue、latest review/handoff、commander state、adapter report provenance、boundary hits/escalation 与 follow-through compact detail。
- Continue run `status.json` 在完整 `executionEvidenceReview[]` 旁写入同一 `executionEvidenceReviewSummary`，避免 replacement executor 从 written run status 接手时看到零值 summary。
- Case-local nested output workspace no-pack product-path smoke 覆盖 status JSON/text、overview text、handoff WhatIf/Apply JSON、continue WhatIf/Apply JSON/text、continue run `status.json`、run `digest.md`、lane `RESUME.md` 与 checkpoint summary visibility。
- Adapter escalation 与 boundary-hit product-path coverage 锁定 latest escalation、boundary hits、adapter id/status、main escalation counts、action queue counts、follow-through state 与 no-heavy/no-authority boundary。
- `CHANGELOG.md` 已记录 Batch 520 用户可见变化；Batch 519 active summary 已归档到 `docs/batch-history.md`，保持 `docs/batch-plan.md` 只承载当前批次。

验证结果：已通过 `gofmt -w internal/rekit/cli/cli_test.go internal/rekit/overview/overview.go internal/rekit/workstream/continue.go internal/rekit/workstream/handoff.go internal/rekit/workstream/start.go`、focused `go test ./internal/rekit/workstream ./internal/rekit/overview ./internal/rekit/cli -run "TestRunGateAdapterReportNoPackProductPathFromNestedOutputWorkspace|TestRunGoGateApplyAppendsAuthorizedGateRequestVisibility|TestRunGateAdapterReportBoundaryHitNoPackProductPathSuppressesContinue" -count=1`，以及完整本地 release minimum：`go run ./cmd/rekit -- -Command release-check -Format json`（release-check ready=true）、`go run ./cmd/rekit -- -Command status -Format text`、`go run ./cmd/rekit -- -Command packs -Format text`、`go run ./cmd/rekit -- -Command doctor -Format text`、`go test ./...`、`go vet ./...`、`git diff --check`（仅 LF/CRLF conversion warnings，无 whitespace error）。focused CLI test 曾发现 continue run `status.json` 未写入 `executionEvidenceReviewSummary` 而解码为零值；已在 `writeContinueRunArtifacts` 写入 summary 后重跑 focused smoke 通过。远程 release-gate inspection 待 implementation commit push 后执行。

上一批摘要：Batch 519 已完成 execution evidence review summary downstream closure，并归档到 `docs/batch-history.md`。

### Next candidates

1. **Lane/tool-adapter live validation residuals（如仍有缺口）**：Batch 421/424/432 已覆盖 adapter contract/validation/report action queue、follow-through 与 contract liveValidation text，Batch 458-462 已补齐 authorizedExecutionFollowThrough / evidence review follow-through text，Batch 504 已补齐 recorded adapter sidecar path、actualBudget 与 adapter provenance，Batch 518 已补齐 contract/validation compact `reportSummary`；后续仅在 Windows 本机 product-path 仍存在 validation/record/evidence review handoff 到 replacement executor 的真实断点时推进；不新增 adapter/heavy-tool execution。
2. **Reviewer orchestration E2E residuals（如仍有缺口）**：Batch 489/499/505 已覆盖 reviewer intake product path、reviewer writeback identity 与 reviewer result provenance downstream handoff；后续仅在 dispatch/intake/post-validation terminal path 仍要求解析 nested JSON、打开 reviewer result artifact 或手工拼 route output 才能接续时推进，不自动 spawn reviewer、不执行 heavy-tool。
3. **Pack-memory downstream UX residuals（如仍有缺口）**：Batch 500 已把 candidate decision/cleanup/reconsume expected evidence 收口为 `reviewArtifacts[]`，Batch 501 已把 open candidate residue 投影到 release/status handoff，Batch 502/503 已补齐 case-local status/default path、candidate identity、index mapping 与 derived review artifact visibility，Batch 517 已补齐 compact review summary；后续仅在 accepted/rejected 人工流程、cleanup/reconsume 或 evidence review downstream UX 仍需跨 envelope 手工拼接时推进，不重复做字段微批次。
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
