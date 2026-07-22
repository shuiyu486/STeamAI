# Batch implementation plan

## 读取指南

先读本节的 current milestone / current batch state / next candidates；旧批次只在需要考古、验证历史决策或 release/debug 溯源时按 Batch ID 搜索 `docs/batch-history.md`。不要默认从 Batch 0 顺序重读 350+ 个历史批次。产品方向以 `docs/mission-control-product-direction.md` 为准，持续执行方式见 `docs/autonomous-goal.md`。

## 实施摘要

Batch 359 后，Go-owned/no-fallback public command surface、durable lanes、显式 reconcile、typed autonomy preflight、Mission brief / executor action、bounded reviewer dispatch → strict intake → verification-before-decision writeback → post-validation 的本机闭环、pack-memory promote/reconsume package E2E，以及 authorized execution observation evidence + bounded adapter execution report strict intake（含 authorized stopCondition boundaryHits、status summary enforcement、workspace-relative 与 case-relative machine-readable handoff）已形成底座。当前阶段继续从 contract/inventory field increments 转向 replaceable session executor / reviewer orchestration、lane executor / tool-adapter live validation、pack-memory product UX、Windows 本机 product-path 稳定与真实 release verification。

## 执行清单

### Current milestone

**Mission Commander operational closure and truthful release readiness**：把 durable lane/reviewer/autonomy contract 串成实际可运行、可跨会话接手、可验证的产品闭环，并区分 inventory ready、本地 gate executed 与远程 CI green。当前用户短期只要求 Windows 本机稳定可用；远程 Linux/macOS/Windows CI 因 runner/billing blocker 继续记录为 known gap，不阻塞本机 Mission Control 闭环。

### Current batch state

### Batch 532：reviewer intake orchestration progress terminal closure

状态：已完成 runtime/CLI/test/docs implementation、focused/package validation、完整本地 release minimum、implementation commit/push 与远程 release-gate inspection；已提交并推送 implementation commit `0ae6a43 Add reviewer intake orchestration progress`。远程 release-gate run `29916791462` completed failure，Linux/Windows/macOS jobs 均 completed failure 且 `steps=[]`，仍是既有 GitHub Actions runner/billing blocker，不能声明 remote CI green。本批按 release inspection cadence 只记录 implementation commit 的远程 run；不再为后续 release inspection commit 自身触发的 CI 追加第三个记录提交，除非出现不同于既有 `steps=[]` blocker 的新信号。

目标：Batch 525 已把 multi-shard reviewer dispatch progress 投影到 status/handoff/continue 与 durable artifacts，Batch 530-531 又补齐 reviewer intake terminal 的 compact writeback provenance 与 blocked repair summary；但 replacement executor 若只看 `plan-subagents -ReviewerResultPath ...` reviewer intake terminal top-level `summary`，仍只能看到 dispatch index/total 与 `nextDispatches[]`，需要回退完整 `orchestrationSnapshot`、packet dispatches 或 downstream status/handoff progress 才能确认 current shard 是否仍 open、completed/open totals、next open shard、remaining shard IDs 与 no-spawn/no-heavy/no-authority orchestration boundary。Batch 532 将 compact multi-reviewer orchestration progress 直接提升到 reviewer intake terminal envelope 与 text。

边界：只增强 `plan-subagents -ReviewerResultPath ...` reviewer intake 的只读 terminal progress summary 与 CLI product-path coverage；不自动 spawn/monitor reviewer、不执行 reviewer、不创建 reviewer result、不写 authority/confirmed、不执行 heavy-tool，不改变 WhatIf-before-Apply、verification-before-decision writeback、blocked fail-closed、repair guidance、postValidation overview/handoff/doctor snapshots、downstream dispatch handoff clearing、sync/promote review-first、case durable schema、PowerShell façade 或公共 façade removal 门禁。

已完成内容：

- `ReviewerIntakeSummary` 新增 `orchestrationProgress`，汇总 dispatch index/total、completed/open、current shard/status、next open shard、remaining shard IDs 与 read-only orchestration boundary。
- CLI reviewer intake summary text 新增 `reviewer intake summary orchestration progress/boundary` lines，使 multi-reviewer WhatIf/Apply 第一屏直接说明当前 shard 与剩余分片进度。
- `TestRunPlanSubagentsReviewerOrchestrationE2E` 覆盖 first/second shard reviewer intake preview/apply JSON summary progress，以及 first shard WhatIf text 第一屏 progress/boundary；共享 reviewer intake CLI JSON test shape 补齐 `orchestrationProgress`。
- 文档同步更新 `README.md`、`docs/release-readiness.md`、`rekit/tests/README.md`、`CHANGELOG.md`，并将 Batch 531 归档到 `docs/batch-history.md`。

验证结果：已通过 `gofmt -w internal/rekit/subagents/intake.go internal/rekit/cli/cli.go internal/rekit/cli/reviewer_intake_test.go internal/rekit/cli/cli_test.go`、focused `go test ./internal/rekit/subagents ./internal/rekit/cli -run "TestRunPlanSubagentsReviewerOrchestrationE2E|TestRunPlanSubagentsReviewerIntakeWhatIfApplyE2E|TestRunPlanSubagentsReviewerIntakeBlockedRepairGuidanceCaseLocalProductPath|TestRunPlanSubagentsReviewerIntakeCaseLocalProductPathUsesMetadataRuntime" -count=1`、package `go test ./internal/rekit/cli -count=1`、package `go test ./internal/rekit/subagents -count=1`，以及完整本地 release minimum：`go run ./cmd/rekit -- -Command release-check -Format json`（release-check ready=true）、`go run ./cmd/rekit -- -Command status -Format text`、`go run ./cmd/rekit -- -Command packs -Format text`、`go run ./cmd/rekit -- -Command doctor -Format text`、`go test ./...`、`go vet ./...`、`git diff --check`（仅 LF/CRLF conversion warnings，无 whitespace error）。implementation commit `0ae6a43 Add reviewer intake orchestration progress` 已推送；远程 release-gate run `29916791462` 已检查，run completed failure，Linux/Windows/macOS jobs 均 completed failure 且 `steps=[]`。该远程失败符合既有 runner/billing blocker，不能声明 remote CI green。`release-check`/`ciReleaseGate.ready` 仍不能替代实际远程 job conclusion。

上一批摘要：Batch 531 已完成 reviewer intake blocked repair summary closure，并归档到 `docs/batch-history.md`。

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
