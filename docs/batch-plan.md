# Batch implementation plan

## 读取指南

先读本节的 current milestone / current batch state / next candidates；旧批次只在需要考古、验证历史决策或 release/debug 溯源时按 Batch ID 搜索 `docs/batch-history.md`。不要默认从 Batch 0 顺序重读 350+ 个历史批次。产品方向以 `docs/mission-control-product-direction.md` 为准，持续执行方式见 `docs/autonomous-goal.md`。

## 实施摘要

Batch 359 后，Go-owned/no-fallback public command surface、durable lanes、显式 reconcile、typed autonomy preflight、Mission brief / executor action、bounded reviewer dispatch → strict intake → verification-before-decision writeback → post-validation 的本机闭环、pack-memory promote/reconsume package E2E，以及 authorized execution observation evidence + bounded adapter execution report strict intake（含 authorized stopCondition boundaryHits、status summary enforcement、workspace-relative 与 case-relative machine-readable handoff）已形成底座。当前阶段继续从 contract/inventory field increments 转向 replaceable session executor / reviewer orchestration、lane executor / tool-adapter live validation、pack-memory product UX、Windows 本机 product-path 稳定与真实 release verification。

## 执行清单

### Current milestone

**Mission Commander operational closure and truthful release readiness**：把 durable lane/reviewer/autonomy contract 串成实际可运行、可跨会话接手、可验证的产品闭环，并区分 inventory ready、本地 gate executed 与远程 CI green。当前用户短期只要求 Windows 本机稳定可用；远程 Linux/macOS/Windows CI 因 runner/billing blocker 继续记录为 known gap，不阻塞本机 Mission Control 闭环。

### Current batch state

### Batch 408：Gate execution evidence next-action immediate projection closure

状态：已完成本地实现、durable docs、full local validation、提交/推送与远程 release-gate inspection。

目标：Batch 407 已让 continue result/run artifacts 暴露 `executionEvidenceReview[]` 与 `missionCommanderNextActions[]`，但 `gate -Apply -GateEventId ...` 刚记录 authorized execution observation evidence 后，主 Agent 仍需再跑 overview/handoff/continue/resume/checkpoint 才能拿到同一 evidence review queue 与 ordered next-action list。本批把刚记录的 bounded observation evidence 立即投影到 gate execution evidence result JSON 与 CLI text。

边界：只增强 gate execution evidence record result projection、shared mission builder reuse、CLI tests 与 durable docs；不 replay heavy-tool、不写 authority/confirmed、不新增 PowerShell runtime logic、不改变 sync/promote review-first、case durable schema、公共 façade 删除门禁或远程 CI blocker 状态；除既有 `gate -Apply -GateEventId ...` observation evidence append/idempotent no-op 外不新增写入路径。

已完成内容：

- `mission` package 新增共享 `ExecutionEvidenceReviewItems(...)` / `ExecutionEvidenceReviewItemFromObservation(...)` builder，workstream 复用同一 parser，避免 gate 与 handoff/resume/checkpoint 维护并行 evidence review item logic。
- `gate -Apply -GateEventId ...` execution evidence record JSON 新增 `executionEvidenceReview[]` 与 `missionCommanderNextActions[]`，把刚记录的 observation evidence 直接投影为 evidence-first handoff/overview/continue ordering。
- execution evidence CLI text 新增 `mission commander next action` lines，展示 state/source/blocked/requiresReview/command、reasons 与 boundary。
- normal succeeded evidence record 保留 evidence review primary `/rekit handoff main`、`/rekit overview` 与 review 后 `continue -WhatIf`，并追加 lane ready commander action；boundary-hit/escalated evidence 继续抑制 autonomous continue。
- CLI coverage 锁定 normal execution evidence JSON/text projection、adapter escalation suppression、no-heavy/no-authority boundary 与 shared builder behavior。

验证结果：已通过 focused `go test ./internal/rekit/cli -run "TestRunGateExecutionEvidenceTextOutputsNextActions|TestRunGoGateApplyAppendsAuthorizedGateRequestVisibility" -count=1`、affected package `go test ./internal/rekit/gate ./internal/rekit/mission ./internal/rekit/workstream -count=1`、`go test ./...`、`go vet ./...`、`go run ./cmd/rekit -- -Command release-check -Format json`、`go run ./cmd/rekit -- -Command status`、`go run ./cmd/rekit -- -Command packs`、`go run ./cmd/rekit -- -Command doctor` 与 `git diff --check`。`release-check` 汇总 ready=true、summary=release gate inventory ok；`git diff --check` 仅报告 Windows LF/CRLF conversion warning，无 whitespace error。已提交并推送 `c800e1e Add gate evidence next actions`；远程 release-gate run `29700552809` 为 completed failure，Linux/Windows/macOS jobs 均 failure 且 `steps: []`，仍是既有 GitHub Actions runner/billing blocker，不能声明远程 CI green。

上一批摘要：Batch 407 已完成 `/rekit continue` JSON/run artifacts 的 `missionCommanderNextActions[]` 投影，详见 `docs/batch-history.md`。

### Next candidates

1. **Pack-memory cleanup / reconsume command execution UX**：围绕 accepted/rejected candidate 决策后的 cleanup target 与 fresh/attached case verification，继续收口为主 Agent 可执行的 bounded checklist。
2. **Mission Commander overview action consumption closure**：继续让 overview JSON/text 的 lane action index 和 commander action 可被替换 executor 直接消费，优先选择 Windows 本机 product-path coverage。
3. **Authorized execution duplicate/idempotent review projection**：围绕 duplicate execution evidence record 的 `evidence-already-recorded` 语义，补齐 next-action projection 与 no-replay/no-append 文本/JSON coverage（如仍有真实消费断点）。
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
