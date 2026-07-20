# Batch implementation plan

## 读取指南

先读本节的 current milestone / current batch state / next candidates；旧批次只在需要考古、验证历史决策或 release/debug 溯源时按 Batch ID 搜索 `docs/batch-history.md`。不要默认从 Batch 0 顺序重读 350+ 个历史批次。产品方向以 `docs/mission-control-product-direction.md` 为准，持续执行方式见 `docs/autonomous-goal.md`。

## 实施摘要

Batch 359 后，Go-owned/no-fallback public command surface、durable lanes、显式 reconcile、typed autonomy preflight、Mission brief / executor action、bounded reviewer dispatch → strict intake → verification-before-decision writeback → post-validation 的本机闭环、pack-memory promote/reconsume package E2E，以及 authorized execution observation evidence + bounded adapter execution report strict intake（含 authorized stopCondition boundaryHits、status summary enforcement、workspace-relative 与 case-relative machine-readable handoff）已形成底座。当前阶段继续从 contract/inventory field increments 转向 replaceable session executor / reviewer orchestration、lane executor / tool-adapter live validation、pack-memory product UX、Windows 本机 product-path 稳定与真实 release verification。

## 执行清单

### Current milestone

**Mission Commander operational closure and truthful release readiness**：把 durable lane/reviewer/autonomy contract 串成实际可运行、可跨会话接手、可验证的产品闭环，并区分 inventory ready、本地 gate executed 与远程 CI green。当前用户短期只要求 Windows 本机稳定可用；远程 Linux/macOS/Windows CI 因 runner/billing blocker 继续记录为 known gap，不阻塞本机 Mission Control 闭环。

### Current batch state

### Batch 424：Authorized execution adapter/report consumption follow-through closure

状态：已完成本地实现、durable docs、focused/affected package validation、full local validation、commit/push 与远程 release-gate inspection。

目标：Batch 361/366/373/380/392/411/412/421 已让 authorized-gate adapter report contract、read-only validation、record handoff 与 action queue 可见；但主 Agent / replacement executor 仍需在 contract、validation、record result、duplicate replay 与 evidence review queue 之间手工拼接 outcome-specific lifecycle。本批把 authorized execution follow-through 显式结构化，覆盖 write-and-validate、valid record、invalid repair、recorded review、boundary/main escalation review 与 duplicate replay review。

边界：只增强 `gate -ExecutionReportContract`、`gate -ValidateExecutionReport` 与 execution evidence `gate -Apply -GateEventId ...` 的 JSON/text projection、package/CLI coverage 与 durable docs；不改变 authorized-gate request/decision write model、adapter sidecar strict intake、execution observation evidence write model、Mission Commander next-action ordering、case durable schema、sync/promote review-first、public façade 删除门禁或远程 CI blocker 状态；validation 仍 read-only，record 仍只写 bounded observation evidence，runtime 不执行 adapter/heavy-tool，不写 authority/confirmed，不新增 PowerShell runtime logic。

已完成内容：

- `AdapterExecutionReportContract`、`AdapterExecutionReportValidation` 与 execution evidence `ApplyResult` 新增 `authorizedExecutionFollowThrough`，按当前 state 输出 outcomes、boundary 与共享 `missionCommanderActionQueue`。
- contract 阶段输出 `write-and-validate-report`、`valid-report-record` 与 `invalid-report-repair` 三类 outcome，明确先写 sidecar 并运行 read-only validation，valid=true 后才能 record，invalid/missing 只进入 repair/rerun validation。
- validation valid 阶段输出 `valid-report-record` outcome；missing/invalid sidecar 输出 `invalid-report-repair` outcome，并把 repair hints 投影为 `repairActions[]`。
- execution evidence record 阶段输出 `recorded-evidence-review`、`boundary-or-escalation-review` 或 `duplicate-record-review` outcome，覆盖 normal review、main escalation 与 duplicate no-append/no-replay handoff。
- CLI text 同步打印 adapter report contract/validation/execution evidence follow-through summary、outcome、action、repair action、verification command、boundary 与 queue summary。
- coverage 锁定 contract 三 outcome、valid validation record handoff、missing/invalid sidecar repair hints、normal/boundary/duplicate execution evidence follow-through、text/JSON parity、read-only validation、bounded observation evidence record、duplicate no-append、no-heavy/no-authority/confirmed/no PowerShell runtime logic 边界。

验证结果：已通过 focused `go test ./internal/rekit/gate ./internal/rekit/cli -run "TestAdapterReport|TestValidateAdapter|TestRecordExecution|TestRunGate" -count=1`、affected package `go test ./internal/rekit/gate ./internal/rekit/cli -count=1`、`go test ./...`、`go vet ./...`、`go run ./cmd/rekit -- -Command release-check -Format json`、`go run ./cmd/rekit -- -Command status`、`go run ./cmd/rekit -- -Command packs`、`go run ./cmd/rekit -- -Command doctor` 与 `git diff --check`。`release-check` 汇总 ready=true、summary=release gate inventory ok；`status`、`packs`、`doctor` 正常，`doctor` 输出 `pack validation ok`；`git diff --check` 仅报告 Windows LF/CRLF conversion warning，无 whitespace error。已提交并推送 `1a46761 Add authorized execution follow-through`；远程 release-gate run `29715547553` 为 completed failure，Linux/Windows/macOS jobs 均 failure 且 `steps: []`，仍是既有 GitHub Actions runner/billing blocker，不能声明远程 CI green。

上一批摘要：Batch 423 已完成 Pack-memory accepted/rejected decision follow-through closure，详见 `docs/batch-history.md`。

### Next candidates

1. **Execution evidence review downstream artifact follow-through**：检查 `authorizedExecutionFollowThrough` 是否需要继续投影到 overview/handoff/continue/resume/checkpoint 的 evidence review downstream artifacts，优先选择 replacement executor 仍需跨 envelope 手工拼接的 Windows 本机 product-path 断点。
2. **Reviewer orchestration dispatch/intake action consumption closure**：围绕 reviewer dispatch packet、strict intake、post-validation handoff 与 Mission Commander action queue 的实际主 Agent 消费路径补齐 coverage，不自动 spawn reviewer，不改变 review-first writeback。
3. **Pack-memory decision follow-through downstream UX（如仍有缺口）**：Batch 423 已完成 candidate decision outcome projection；后续仅在 accepted/rejected 人工流程仍需跨 envelope 手工拼接时推进，不重复做字段微批次。
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
