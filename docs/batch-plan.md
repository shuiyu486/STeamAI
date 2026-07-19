# Batch implementation plan

## 读取指南

先读本节的 current milestone / current batch state / next candidates；旧批次只在需要考古、验证历史决策或 release/debug 溯源时按 Batch ID 搜索 `docs/batch-history.md`。不要默认从 Batch 0 顺序重读 350+ 个历史批次。产品方向以 `docs/mission-control-product-direction.md` 为准，持续执行方式见 `docs/autonomous-goal.md`。

## 实施摘要

Batch 359 后，Go-owned/no-fallback public command surface、durable lanes、显式 reconcile、typed autonomy preflight、Mission brief / executor action、bounded reviewer dispatch → strict intake → verification-before-decision writeback → post-validation 的本机闭环、pack-memory promote/reconsume package E2E，以及 authorized execution observation evidence + bounded adapter execution report strict intake（含 authorized stopCondition boundaryHits、status summary enforcement、workspace-relative 与 case-relative machine-readable handoff）已形成底座。当前阶段继续从 contract/inventory field increments 转向 replaceable session executor / reviewer orchestration、lane executor / tool-adapter live validation、pack-memory product UX、Windows 本机 product-path 稳定与真实 release verification。

## 执行清单

### Current milestone

**Mission Commander operational closure and truthful release readiness**：把 durable lane/reviewer/autonomy contract 串成实际可运行、可跨会话接手、可验证的产品闭环，并区分 inventory ready、本地 gate executed 与远程 CI green。当前用户短期只要求 Windows 本机稳定可用；远程 Linux/macOS/Windows CI 因 runner/billing blocker 继续记录为 known gap，不阻塞本机 Mission Control 闭环。

### Current batch state

### Batch 390：pack-memory product UX hardening

状态：已完成；本地 Windows product path 已覆盖 `promote -CreateCandidates` 候选审查、merge/reject/cleanup/reconsume guidance，提交/推送在本批收尾执行。

目标：在 Batch 358 pack-memory reconsume package E2E 之上，补齐 Mission Commander 可直接消费的 candidate review UX：`promote -CreateCandidates` JSON 要把 managed-doc candidate、tooling candidate、blocked item、cleanup target、人工 merge/reject 路径与 fresh/attached case reconsume guidance 结构化输出，避免主 Agent 从 generic `NextSteps` 手工推断。

边界：runtime 仍不自动合并 candidate、不删除 candidate、不把 `promote -Apply` 当作 candidate-scoped accept path；tooling candidate 仍需人工 review 后合入 `tooling/catalog.yml` 或 `tooling/recipes/*`；不新增 PowerShell runtime logic、不执行 heavy-tool、不写 authority/confirmed、不改变 sync/promote review-first、case durable schema、公共 façade 删除门禁或远程 CI blocker 状态。

已完成内容：

- `promote.CandidateResult` 新增 `reviewPlan` envelope，投影 mode、candidate/tooling roots、index path、per-item `reviewItems[]`、`cleanupTargets[]`、runtime boundary、completion criteria 与 pack-memory `reconsume` guidance。
- managed-doc candidates 现在给出 `candidatePath`、`packTarget`、candidate-safe merge hint、reject/cleanup action，并明确 `promote -Apply` 不是已接受 candidate 的 scoped apply 路径。
- tooling candidates 现在给出合入 `tooling/catalog.yml` / `tooling/recipes/*` 的 guidance、case-specific residue 检查、fresh/attached case reconsume 验证要求，并继续说明 `sync` 不复制 tooling recipes 到 case-local managed docs。
- cleanup guidance 现在覆盖 candidate 文件和 `indexPath`：reject / superseded / merge elsewhere 后应删除候选并更新或删除 stale index entry，避免后续 review 被过期 index 误导。
- WhatIf 输出 `candidate-review-preview`，只展示如果实际生成候选后如何 review/cleanup，不创建 candidate/index/tooling 文件。
- package 与 CLI tests 覆盖 WhatIf no-write、candidate/index/tooling candidate、blocked deny item、managed-doc/tooling merge hints、index cleanup guidance 与 fresh-case reconsume guidance。

验证结果：已通过 `go test ./internal/rekit/promote -count=1`、`go test ./internal/rekit/cli -run TestRunPromoteCreateCandidates -count=1`、`go test ./...`、`go vet ./...`、`go run ./cmd/rekit -- -Command release-check -Format json`、`go run ./cmd/rekit -- -Command status`、`go run ./cmd/rekit -- -Command packs`、`go run ./cmd/rekit -- -Command doctor` 与 `git diff --check`。`git diff --check` 仅报告 Windows LF/CRLF conversion warning，无 whitespace error；远程 release-gate 仍是既有 runner/billing known gap，未作为本机 batch blocker。

上一批摘要：Batch 389 已完成 reviewer orchestration E2E；`plan-subagents` / reviewer-intake / lane handoff 已覆盖 Mission Commander 侧多 reviewer dispatch/result/intake/writeback 与 reviewer session provenance，详见 `docs/batch-history.md`。

### Next candidates

1. **Mission Commander Windows product UX closure**：围绕当前用户单机 Windows 使用，把 overview/handoff/continue/start/gate 的自然语言 Mission Commander 操作路径继续收口，优先减少用户记命令和跨会话接手摩擦。
2. **Lane/tool-adapter live validation UX closure**：继续把 authorized-gate contract、adapter sidecar validation、record handoff 与 lane executor 接手提示做成更少命令记忆、更强 post-validation 的 Windows 本机闭环。
3. **Cross-platform product-path E2E（降优先级）**：在本地 CLI/case E2E 已覆盖 nested cwd / case shim 的基础上，仅保持可在 runner 可用时执行的三平台 matrix 候选和 known gap 记录；不要在 GitHub runner/billing blocker 未解除前让它阻塞 Windows 本机迭代。
4. **Remote release-gate unblock / product-path matrix（blocked / low priority）**：GitHub Actions billing/spending limit 解除后，再读取真实 Linux/Windows/macOS job conclusions，并把 CLI/case E2E、`status.caseShim` readiness 和完整 installed user entrypoint E2E 纳入可用 runner 验证。
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
