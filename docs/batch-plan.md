# Batch implementation plan

## 读取指南

先读本节的 current milestone / current batch state / next candidates；旧批次只在需要考古、验证历史决策或 release/debug 溯源时按 Batch ID 搜索 `docs/batch-history.md`。不要默认从 Batch 0 顺序重读 350+ 个历史批次。产品方向以 `docs/mission-control-product-direction.md` 为准，持续执行方式见 `docs/autonomous-goal.md`。

## 实施摘要

Batch 359 后，Go-owned/no-fallback public command surface、durable lanes、显式 reconcile、typed autonomy preflight、Mission brief / executor action、bounded reviewer dispatch → strict intake → verification-before-decision writeback → post-validation 的本机闭环、pack-memory promote/reconsume package E2E，以及 authorized execution observation evidence + bounded adapter execution report strict intake（含 authorized stopCondition boundaryHits、status summary enforcement、workspace-relative 与 case-relative machine-readable handoff）已形成底座。当前阶段继续从 contract/inventory field increments 转向 replaceable session executor / reviewer orchestration、lane executor / tool-adapter live validation、pack-memory product UX、Windows 本机 product-path 稳定与真实 release verification。

## 执行清单

### Current milestone

**Mission Commander operational closure and truthful release readiness**：把 durable lane/reviewer/autonomy contract 串成实际可运行、可跨会话接手、可验证的产品闭环，并区分 inventory ready、本地 gate executed 与远程 CI green。当前用户短期只要求 Windows 本机稳定可用；远程 Linux/macOS/Windows CI 因 runner/billing blocker 继续记录为 known gap，不阻塞本机 Mission Control 闭环。

### Current batch state

### Batch 393：Authorized execution evidence Mission Commander closure

状态：已完成本地实现与验证；提交/推送在本批收尾执行。

目标：把 authorized-gate → execution report contract / adapter validation → record observation evidence 的最终记录结果继续收口为结构化 `missionCommanderAction`，让主 Agent 不必从 generic executor action 或 next steps 手工判断 evidence review、duplicate idempotency 与 boundary/escalation 处理。

边界：`missionCommanderAction` 只投影 guidance；`gate -Apply -GateEventId ...` 仍只记录 bounded observation evidence，不执行 heavy-tool、不写 authority/confirmed；duplicate eventId 不追加 observation；boundary/escalation evidence 只要求 main review，不推荐继续该 action；不新增 PowerShell runtime logic、不改变 sync/promote review-first、case durable schema、公共 façade 删除门禁或远程 CI blocker 状态。

已完成内容：

- `ApplyResult` 新增 top-level `missionCommanderAction`，普通 gate decision apply 继续复用 post-apply lane executor action，execution evidence record 使用 evidence-specific action。
- 普通 authorized execution evidence 记录后投影 `ready-for-evidence-review`，primary command 为 `/rekit handoff <lane>`，boundary 明确 bounded observation evidence only、`/rekit` did not execute heavy tool、review refs before authority/confirmed、no authority/confirmed writes。
- duplicate execution evidence 投影 `evidence-already-recorded`，明确 duplicate record did not append observation evidence，且不产生任何 `-Apply` follow-up。
- adapter report 或 explicit fields 记录出 `boundary-hit` / `escalated` / escalation / boundaryHits 时投影 `needs-main-escalation`，不推荐 continue，要求停止该 action 的 autonomous work 并通知 main Agent。
- CLI text output 对 execution evidence branch 也输出 evidence commander action；package 与 CLI coverage 锁定 normal record、duplicate no-append、adapter escalation、handoff/next steps 与 no-heavy/no-authority/no-confirmed invariants。

验证结果：已通过 focused `go test ./internal/rekit/gate -run 'TestRecordExecutionWritesObservationForAuthorizedGate|TestRecordExecutionDuplicateDoesNotAppend|TestRecordExecutionAcceptsAdapterReportEscalation' -count=1`、`go test ./internal/rekit/cli -run TestRunGate -count=1`、`go test ./internal/rekit/gate ./internal/rekit/cli -count=1`、`go test ./...`、`go vet ./...`、`go run ./cmd/rekit -- -Command release-check -Format json`、`go run ./cmd/rekit -- -Command status`、`go run ./cmd/rekit -- -Command packs`、`go run ./cmd/rekit -- -Command doctor` 与 `git diff --check`。`git diff --check` 仅报告 Windows LF/CRLF conversion warning，无 whitespace error；远程 release-gate 仍按既有 GitHub runner/billing known gap 处理，不能把 inventory readiness 声明为远程 CI green。

上一批摘要：Batch 392 已完成 lane/tool-adapter live validation UX closure；adapter execution report contract/validation 现在投影 `missionCommanderAction`，详见 `docs/batch-history.md`。

### Next candidates

1. **Pack-memory reconsume / tooling candidate follow-through**：围绕真实 pack tooling candidate 被人工合入后的 fresh/attached case reconsume，把 review plan 与后续 case consumption 的 Mission Commander 提示继续做成可验证 product path。
2. **Mission Commander lane handoff consumption closure**：继续让 overview/handoff/continue/gate/start 输出的 action guidance 在替换 executor 或新会话中可直接消费，优先选择能用 Windows 本机 product path 验证的闭环。
3. **Authorized execution evidence handoff consumption follow-through**：围绕新 `missionCommanderAction` 在 lane handoff / resume / checkpoint 中的 consumption，补齐替换 executor 从 authorized-gate 到 evidence review 的可复制闭环。
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
