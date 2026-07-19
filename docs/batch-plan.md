# Batch implementation plan

## 读取指南

先读本节的 current milestone / current batch state / next candidates；旧批次只在需要考古、验证历史决策或 release/debug 溯源时按 Batch ID 搜索 `docs/batch-history.md`。不要默认从 Batch 0 顺序重读 350+ 个历史批次。产品方向以 `docs/mission-control-product-direction.md` 为准，持续执行方式见 `docs/autonomous-goal.md`。

## 实施摘要

Batch 359 后，Go-owned/no-fallback public command surface、durable lanes、显式 reconcile、typed autonomy preflight、Mission brief / executor action、bounded reviewer dispatch → strict intake → verification-before-decision writeback → post-validation 的本机闭环、pack-memory promote/reconsume package E2E，以及 authorized execution observation evidence + bounded adapter execution report strict intake（含 authorized stopCondition boundaryHits、status summary enforcement、workspace-relative 与 case-relative machine-readable handoff）已形成底座。当前阶段继续从 contract/inventory field increments 转向 replaceable session executor / reviewer orchestration、lane executor / tool-adapter live validation、pack-memory product UX、Windows 本机 product-path 稳定与真实 release verification。

## 执行清单

### Current milestone

**Mission Commander operational closure and truthful release readiness**：把 durable lane/reviewer/autonomy contract 串成实际可运行、可跨会话接手、可验证的产品闭环，并区分 inventory ready、本地 gate executed 与远程 CI green。当前用户短期只要求 Windows 本机稳定可用；远程 Linux/macOS/Windows CI 因 runner/billing blocker 继续记录为 known gap，不阻塞本机 Mission Control 闭环。

### Current batch state

### Batch 417：Start / replaceable executor takeover Mission Commander next-action projection closure

状态：已完成本地实现、durable docs、focused/affected package validation 与 full local validation；commit/push 与远程 release-gate inspection 待执行。

目标：Batch 387 已跑通 replaceable session executor takeover，Batch 391 让 `start` nested `executorAction.missionCommanderAction` 暴露 start/apply/continue guidance，Batch 404/405/407/413 又把 overview、handoff、continue、resume/checkpoint 的 Mission Commander next actions 收口到共享 list；但 `start -WhatIf/-Apply -Format json` 顶层仍缺 `missionCommanderAction` / `missionCommanderNextActions[]`，CLI text 也不打印 next-action list，替换 executor 仍需从 nested `executorAction.missionCommanderAction` 与 `nextSteps[]` 手工拼接“preview 后 apply、apply 后 continue/handoff、blocked lane 先 reconcile/gate/decision”的顺序。本批把 start / takeover 的 immediate product-path guidance 收口到顶层 Mission Commander projection。

边界：只增强 `start` JSON/text projection、package/CLI tests 与 durable docs；不改变 lane durable schema、executor takeover write model、resume/checkpoint schema、board/facts ledger、sync/promote review-first、公共 façade 删除门禁或远程 CI blocker 状态；runtime 不自动 spawn/stop/monitor session，不执行 continue/handoff/reconcile/gate，不执行 heavy-tool，不写 authority/confirmed，不新增 PowerShell runtime logic。

已完成内容：

- `workstream.StartResult` 现在输出 top-level `missionCommanderAction` 与 `missionCommanderNextActions[]`，复用 lane Mission Commander builder，把 start preview/apply 的 current commander action 与 next-action list 直接交给主 Agent / replacement executor。
- 创建 lane、进入 existing lane 或 executor claim/takeover 的 `start -WhatIf` 会把 start apply primary command 标记为 review-owned；continue/handoff follow-up 在 apply 成功并刷新后的 executor action 仍 ready 前保持 blocked/requiresReview，并携带 review start preview 与 apply-before-follow-up reasons。
- `start -Apply` 直接投影 ready lane 的 continue/handoff 或 blocked lane 的 reconcile/gate/decision next actions；CLI text 同步打印 `mission commander next action` lines，避免文本/default consumption 回查 JSON 或手工拼接。
- package/CLI coverage 锁定 new lane preview、existing main replacement takeover preview、apply continue projection、blocked lane text projection、ready/blocked next-action source、no authority/confirmed/no-heavy-tool/no PowerShell runtime logic 边界。

验证结果：已通过 focused `go test ./internal/rekit/workstream -run "TestLaneExecutorAction|TestStartMissionCommanderNextActions" -count=1`、`go test ./internal/rekit/cli -run "TestRunStart|TestRunReplaceableSessionExecutorTakeoverFromHandoffProductPath" -count=1`、affected package `go test ./internal/rekit/workstream ./internal/rekit/cli -count=1`、`go test ./...`、`go vet ./...`、`go run ./cmd/rekit -- -Command release-check -Format json`、`go run ./cmd/rekit -- -Command status`、`go run ./cmd/rekit -- -Command packs`、`go run ./cmd/rekit -- -Command doctor` 与 `git diff --check`。`release-check` 汇总 ready=true、summary=release gate inventory ok；`status`、`packs`、`doctor` 正常，`doctor` 输出 `pack validation ok`；`git diff --check` 仅报告 Windows LF/CRLF conversion warning，无 whitespace error。commit/push 与远程 release-gate inspection 待执行。

上一批摘要：Batch 416 已完成 Plan-subagents planning Mission Commander dispatch guidance closure，详见 `docs/batch-history.md`。

### Next candidates

1. **Mission Commander overview action consumption closure**：继续让 overview JSON/text 的 lane action index、execution evidence review 与 commander next actions 更贴近替换 executor 直接消费，优先选择 Windows 本机 product-path coverage 中仍需手工拼接的真实断点。
2. **Pack-memory accepted/rejected decision follow-through（如仍有缺口）**：Batch 409 已完成 candidate review/cleanup/reconsume next-action projection；后续仅在发现 accepted/rejected 决策后的实际人工流程仍需手工拼接时推进，不重复做字段微批次。
3. **Authorized execution adapter/report consumption follow-through**：围绕 adapter report validation、record handoff 与 duplicate replay guidance 的实际 lane executor 消费路径补齐 coverage，只做 Windows 本机可验证产品断点。
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
