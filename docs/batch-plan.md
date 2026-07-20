# Batch implementation plan

## 读取指南

先读本节的 current milestone / current batch state / next candidates；旧批次只在需要考古、验证历史决策或 release/debug 溯源时按 Batch ID 搜索 `docs/batch-history.md`。不要默认从 Batch 0 顺序重读 350+ 个历史批次。产品方向以 `docs/mission-control-product-direction.md` 为准，持续执行方式见 `docs/autonomous-goal.md`。

## 实施摘要

Batch 359 后，Go-owned/no-fallback public command surface、durable lanes、显式 reconcile、typed autonomy preflight、Mission brief / executor action、bounded reviewer dispatch → strict intake → verification-before-decision writeback → post-validation 的本机闭环、pack-memory promote/reconsume package E2E，以及 authorized execution observation evidence + bounded adapter execution report strict intake（含 authorized stopCondition boundaryHits、status summary enforcement、workspace-relative 与 case-relative machine-readable handoff）已形成底座。当前阶段继续从 contract/inventory field increments 转向 replaceable session executor / reviewer orchestration、lane executor / tool-adapter live validation、pack-memory product UX、Windows 本机 product-path 稳定与真实 release verification。

## 执行清单

### Current milestone

**Mission Commander operational closure and truthful release readiness**：把 durable lane/reviewer/autonomy contract 串成实际可运行、可跨会话接手、可验证的产品闭环，并区分 inventory ready、本地 gate executed 与远程 CI green。当前用户短期只要求 Windows 本机稳定可用；远程 Linux/macOS/Windows CI 因 runner/billing blocker 继续记录为 known gap，不阻塞本机 Mission Control 闭环。

### Current batch state

### Batch 426：Reviewer orchestration dispatch/intake action consumption closure

状态：已完成本地实现、durable docs、focused/affected package validation、full local validation、commit/push 与远程 release-gate inspection。

目标：Batch 415/416/389 已让 reviewer-intake 与 plan-subagents reviewer orchestration 暴露 Mission Commander action / next actions；Batch 420/421/422/423/425 已把其它 Mission Commander 产品路径收口为 action queue。但 reviewer dispatch packet、strict intake 与 post-validation handoff 的主 Agent 消费路径仍要求 replacement executor 从 `missionCommanderNextActions[]` 手工计算 current action、blocked preview/apply buckets 与 postValidation handoff。本批把 reviewer orchestration / intake 的 action queue 直接投影到 planning result、packet、summary 与 intake result。

边界：只增强 `plan-subagents` review artifact projection、reviewer-intake JSON contract、summary text、package/CLI coverage 与 durable docs；不自动 spawn reviewer，不执行 reviewer session 管理，不改变 reviewer result strict contract、verification-before-decision writeback 顺序、postValidation semantics、Mission Commander next-action ordering、case durable schema、sync/promote review-first、public façade 删除门禁或远程 CI blocker 状态；runtime 不执行 heavy-tool，不写 authority/confirmed，不新增 PowerShell runtime logic。

已完成内容：

- `subagents.Result` 与 `subagents.ReviewerOrchestrationPlan` 新增 `missionCommanderActionQueue`，复用 shared `mission.MissionCommanderActionQueueFor(...)` 汇总 reviewer dispatch、intake preview 与 intake apply next actions。
- `plan-subagents` packet nested `reviewerOrchestration` 与 summary.md 同步输出 queue summary/counts/current，让主 Agent / replacement executor 从 packet 或 summary 即可看到当前应先 dispatch 哪个 read-only reviewer，以及哪些 intake preview/apply 仍 blocked/requiresReview。
- `subagents.ReviewerIntakeResult` 新增 `missionCommanderActionQueue`，在 `finalizeReviewerIntakeResult(...)` 统一计算，覆盖 previewed、blocked/event-id-collision、verification-recorded partial recovery、complete 与 already-complete postValidation next actions。
- package/CLI tests 锁定 top-level/nested packet queue mirror、summary text queue line、attached/out-of-case reviewer orchestration product path、reviewer intake preview/apply/duplicate/blocked/partial recovery queue contract，以及 no auto-spawn/no-heavy/no-authority/confirmed/no PowerShell runtime logic 边界。

验证结果：已通过 focused `go test ./internal/rekit/subagents ./internal/rekit/cli -run 'TestWritePlan|TestIntakeReviewer|TestRunPlanSubagents' -count=1`、`go test ./...`、`go vet ./...`、`go run ./cmd/rekit -- -Command release-check -Format json`、`go run ./cmd/rekit -- -Command status`、`go run ./cmd/rekit -- -Command packs`、`go run ./cmd/rekit -- -Command doctor` 与 `git diff --check`。`release-check` 汇总 ready=true、summary=release gate inventory ok 且 release notes 覆盖 Batch 426；`doctor` 输出 `pack validation ok`；`git diff --check` 仅报告 Windows LF/CRLF conversion warning，无 whitespace error。已提交并推送 `2767efa Add reviewer orchestration action queues`；远程 release-gate run `29717910198` 为 completed failure，Linux/Windows/macOS jobs 均 failure 且 `steps: []`，仍是既有 GitHub Actions runner/billing blocker，不能声明远程 CI green。

上一批摘要：Batch 425 已完成 Execution evidence review downstream artifact follow-through，详见 `docs/batch-history.md`。

### Next candidates

1. **Reviewer orchestration dispatch/intake action consumption closure**：围绕 reviewer dispatch packet、strict intake、post-validation handoff 与 Mission Commander action queue 的实际主 Agent 消费路径补齐 coverage，不自动 spawn reviewer，不改变 review-first writeback。
2. **Pack-memory decision follow-through downstream UX（如仍有缺口）**：Batch 423/425 已分别完成 candidate decision outcome projection 与 execution evidence downstream follow-through；后续仅在 accepted/rejected 人工流程或 evidence review downstream UX 仍需跨 envelope 手工拼接时推进，不重复做字段微批次。
3. **Lane/tool-adapter live validation operational follow-through**：仅在 Windows 本机 product-path 仍存在 adapter contract/validation/report handoff 到 replacement executor 的真实断点时推进；不新增 adapter/heavy-tool execution。
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
