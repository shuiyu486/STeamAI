# Batch implementation plan

## 读取指南

先读本节的 current milestone / current batch state / next candidates；旧批次只在需要考古、验证历史决策或 release/debug 溯源时按 Batch ID 搜索 `docs/batch-history.md`。不要默认从 Batch 0 顺序重读 350+ 个历史批次。产品方向以 `docs/mission-control-product-direction.md` 为准，持续执行方式见 `docs/autonomous-goal.md`。

## 实施摘要

Batch 359 后，Go-owned/no-fallback public command surface、durable lanes、显式 reconcile、typed autonomy preflight、Mission brief / executor action、bounded reviewer dispatch → strict intake → verification-before-decision writeback → post-validation 的本机闭环、pack-memory promote/reconsume package E2E，以及 authorized execution observation evidence + bounded adapter execution report strict intake（含 authorized stopCondition boundaryHits、status summary enforcement、workspace-relative 与 case-relative machine-readable handoff）已形成底座。当前阶段继续从 contract/inventory field increments 转向 replaceable session executor / reviewer orchestration、lane executor / tool-adapter live validation、pack-memory product UX、Windows 本机 product-path 稳定与真实 release verification。

## 执行清单

### Current milestone

**Mission Commander operational closure and truthful release readiness**：把 durable lane/reviewer/autonomy contract 串成实际可运行、可跨会话接手、可验证的产品闭环，并区分 inventory ready、本地 gate executed 与远程 CI green。当前用户短期只要求 Windows 本机稳定可用；远程 Linux/macOS/Windows CI 因 runner/billing blocker 继续记录为 known gap，不阻塞本机 Mission Control 闭环。

### Current batch state

### Batch 499：reviewer writeback no-pack downstream handoff closure

状态：已完成 reviewer writeback no-pack downstream handoff implementation、durable docs、focused product-path coverage 与完整本地 release minimum；commit/push 与远程 release-gate inspection 待本批提交后执行。

目标：Batch 489 已证明 case-local nested lane workspace 在无 `-Target` / 无 `-Pack` 的真实 product path 中可完成 `plan-subagents` planning、reviewer result placement、reviewer intake WhatIf/Apply 与 postValidation overview/handoff/doctor snapshots；但 replacement executor 在 reviewer writeback 后仍需要从 verification/decision ledger、packet、reviewer result 与 postValidation artifact 手工拼接 packetId、routeId、shardId、reviewerSession、owner binding 与 evidenceRefs。Batch 499 将 reviewer writeback identity 补成 status/overview/handoff/RESUME/checkpoint/continue/digest 的可见闭环。

边界：只增强 reviewer writeback identity visibility 与本机 product-path coverage；runtime 仍不 spawn/monitor reviewer、不执行 heavy-tool、不写 authority/confirmed；reviewer intake 仍由主 Agent 显式 `-WhatIf` 后 `-Apply`，只追加 case-local verification/decision facts；`handoff -Apply` 仍只写 case-local handoff/resume/checkpoint 文档，`continue -Apply` 仍只写 run artifacts、lane RESUME/checkpoint、board 与允许的 lane output event ingestion；sync/promote、case durable schema、PowerShell runtime logic 与 public façade decision 均不改变。

已完成内容：

- 新增 `ReviewerWritebackItem` 投影：从 reviewer verification/decision facts 归并 packetId、routeId、shardId、packetPath、reviewerResultPath、reviewerSession、ownerExecutor/generation/binding、target、confidence、batchId 与 evidenceRefs，并按 lane 或 project 范围限制输出。
- status JSON/text 与 overview text 对 reviewer-authored verification/decision facts 输出 reviewer detail / owner / evidence ref 行；caseMission 新增 `reviewerWritebacks[]`，让新会话第一屏无需打开 ledger 或 reviewer result 即可确认 writeback 来源和 owner binding。
- handoff WhatIf/apply text、written project/lane handoff Markdown、lane `RESUME.md`、typed checkpoint、continue WhatIf/apply text 与 run `digest.md` 同步输出 reviewer writeback identity，补齐 postValidation handoff 到 replacement executor 的 downstream visibility。
- 扩展 `TestRunPlanSubagentsReviewerIntakeCaseLocalProductPathUsesMetadataRuntime`：从 nested case-local lane cwd 无 `-Target` / 无 `-Pack` planning、reviewer result 写入、intake WhatIf/Apply，到 status/default status、overview、handoff WhatIf/apply、written handoff、lane RESUME/checkpoint、continue WhatIf/apply 与 run digest，验证 `_template` pack 来自 case metadata、文本路径不输出 JSON object、reviewer writeback identity 全链路可见。

验证结果：已通过 focused `gofmt -w internal/rekit/cli/cli.go internal/rekit/cli/cli_test.go internal/rekit/cli/reviewer_intake_test.go internal/rekit/workstream/continue.go internal/rekit/workstream/handoff.go internal/rekit/workstream/start.go internal/rekit/workstream/reviewer_writeback.go`、`go test ./internal/rekit/cli -run "TestRunPlanSubagentsReviewerIntakeCaseLocalProductPathUsesMetadataRuntime" -count=1`、`go test ./internal/rekit/cli -run "TestRunPlanSubagentsReviewerIntake.*|TestRunStatusCaseMissionIncludesExecutionEvidenceReview|TestRunHandoff.*|TestRunContinue.*" -count=1`、`go test ./internal/rekit/cli ./internal/rekit/workstream -count=1`，以及完整本地 release minimum：`go run ./cmd/rekit -- -Command release-check -Format json`、`go run ./cmd/rekit -- -Command status`、`go run ./cmd/rekit -- -Command packs`、`go run ./cmd/rekit -- -Command doctor`、`go test ./...`、`go vet ./...`、`git diff --check`；`release-check ready=true`，status handoff recorded，packs inventory recorded，doctor validation recorded，go test ./... recorded，go vet ./... recorded，git diff --check recorded（仅 Windows LF/CRLF conversion warnings，无 whitespace error）。

上一批摘要：Batch 498 已完成 boundary-hit execution evidence no-pack product-path suppression closure，详见 `docs/batch-history.md`。

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
