# Batch implementation plan

## 读取指南

先读本节的 current milestone / current batch state / next candidates；旧批次只在需要考古、验证历史决策或 release/debug 溯源时按 Batch ID 搜索 `docs/batch-history.md`。不要默认从 Batch 0 顺序重读 350+ 个历史批次。产品方向以 `docs/mission-control-product-direction.md` 为准，持续执行方式见 `docs/autonomous-goal.md`。

## 实施摘要

Batch 359 后，Go-owned/no-fallback public command surface、durable lanes、显式 reconcile、typed autonomy preflight、Mission brief / executor action、bounded reviewer dispatch → strict intake → verification-before-decision writeback → post-validation 的本机闭环、pack-memory promote/reconsume package E2E，以及 authorized execution observation evidence + bounded adapter execution report strict intake（含 authorized stopCondition boundaryHits、status summary enforcement、workspace-relative 与 case-relative machine-readable handoff）已形成底座。当前阶段继续从 contract/inventory field increments 转向 replaceable session executor / reviewer orchestration、lane executor / tool-adapter live validation、pack-memory product UX、Windows 本机 product-path 稳定与真实 release verification。

## 执行清单

### Current milestone

**Mission Commander operational closure and truthful release readiness**：把 durable lane/reviewer/autonomy contract 串成实际可运行、可跨会话接手、可验证的产品闭环，并区分 inventory ready、本地 gate executed 与远程 CI green。当前用户短期只要求 Windows 本机稳定可用；远程 Linux/macOS/Windows CI 因 runner/billing blocker 继续记录为 known gap，不阻塞本机 Mission Control 闭环。

### Current batch state

### Batch 428：reviewer-intake post-validation text consumption closure

状态：已完成本地实现、focused reviewer-intake CLI validation、durable docs 与 full local validation；准备 commit/push 与远程 release-gate inspection。

目标：Batch 427 已让 reviewer-intake `-Format text` 输出 intake status、verification/decision/post-validation、Mission Commander action、action queue 与 next actions，但 post-validation text 仍只有 `valid=true`，terminal Mission Commander / replacement executor 仍需解析 nested JSON `postValidation.overview` / `postValidation.handoff` 或额外运行 overview/handoff/doctor 才能确认 writeback 后的 ledger totals、handoff lane、handoff queue/current 与实际可接续 action。本批把 reviewer-intake post-validation snapshot 直接投影到 text output。

边界：只增强 reviewer-intake CLI text post-validation consumption、focused CLI coverage 与 durable docs；不改变 JSON contract、reviewer result strict contract、verification-before-decision writeback 顺序、postValidation semantics、Mission Commander next-action ordering、case durable schema、sync/promote review-first、public façade 删除门禁或远程 CI blocker 状态；不自动 spawn reviewer，不执行 reviewer session 管理或 heavy-tool，不写 authority/confirmed，不新增 PowerShell runtime logic。

已完成内容：

- reviewer-intake `-Format text` 的 post-validation section 现在输出 overview verification/decision totals、doctor row count、handoff lane/project/executor snapshot。
- 同一 section 还输出 nested handoff `missionCommanderActionQueue` summary/current，以及 post-validation next-action lines/reasons/boundaries，避免 text operator 回查 nested JSON 或重复运行 handoff。
- CLI coverage 锁定 preview postValidation text、already-complete postValidation handoff queue/current/next actions，以及 existing reviewer-intake commander queue contract。

验证结果：已通过 focused `go test ./internal/rekit/cli -run 'TestRunPlanSubagentsReviewerIntakeWhatIfApplyE2E|TestRunPlanSubagentsReviewerIntakeEmitsPartialRecoveryJSON' -count=1`、`go test ./...`、`go vet ./...`、`go run ./cmd/rekit -- -Command release-check -Format json`、`go run ./cmd/rekit -- -Command status`、`go run ./cmd/rekit -- -Command packs`、`go run ./cmd/rekit -- -Command doctor` 与 `git diff --check`。`release-check` 汇总 ready=true、summary=release gate inventory ok；`doctor` 输出 `pack validation ok`；`git diff --check` 仅报告 Windows LF/CRLF conversion warning，无 whitespace error。commit/push 与远程 release-gate inspection 待执行。

上一批摘要：Batch 427 已完成 plan-subagents / reviewer-intake CLI text action-queue parity，详见 `docs/batch-history.md`。

### Next candidates

1. **Reviewer post-validation terminal UX residuals（如仍有缺口）**：仅在 reviewer-intake complete/already-complete/partial recovery terminal path 仍需要跨 JSON 或重复运行 overview/handoff/doctor 手工拼接时推进；不重复做字段微批次，不自动 spawn reviewer，不改变 review-first writeback。
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
