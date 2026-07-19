# Batch implementation plan

## 读取指南

先读本节的 current milestone / current batch state / next candidates；旧批次只在需要考古、验证历史决策或 release/debug 溯源时按 Batch ID 搜索 `docs/batch-history.md`。不要默认从 Batch 0 顺序重读 350+ 个历史批次。产品方向以 `docs/mission-control-product-direction.md` 为准，持续执行方式见 `docs/autonomous-goal.md`。

## 实施摘要

Batch 359 后，Go-owned/no-fallback public command surface、durable lanes、显式 reconcile、typed autonomy preflight、Mission brief / executor action、bounded reviewer dispatch → strict intake → verification-before-decision writeback → post-validation 的本机闭环、pack-memory promote/reconsume package E2E，以及 authorized execution observation evidence + bounded adapter execution report strict intake（含 authorized stopCondition boundaryHits、status summary enforcement、workspace-relative 与 case-relative machine-readable handoff）已形成底座。当前阶段继续从 contract/inventory field increments 转向 replaceable session executor / reviewer orchestration、lane executor / tool-adapter live validation、pack-memory product UX、Windows 本机 product-path 稳定与真实 release verification。

## 执行清单

### Current milestone

**Mission Commander operational closure and truthful release readiness**：把 durable lane/reviewer/autonomy contract 串成实际可运行、可跨会话接手、可验证的产品闭环，并区分 inventory ready、本地 gate executed 与远程 CI green。当前用户短期只要求 Windows 本机稳定可用；远程 Linux/macOS/Windows CI 因 runner/billing blocker 继续记录为 known gap，不阻塞本机 Mission Control 闭环。

### Current batch state

### Batch 388：context routing and progressive disclosure

状态：已完成；文档层级、release handoff read-first、default docs / release-check inventory 与相关测试已校准，本地验证已通过，提交/推送在本批收尾执行。

目标：降低新会话和上下文压缩后的默认上下文压力，把长期历史从 `docs/batch-plan.md` 拆出到 `docs/batch-history.md`，新增 `docs/context-routing.md` 作为按需路由入口，并让 `CLAUDE.md`、`release-check -Format json` 的 `releaseHandoff.readFirst[]` 与 public default docs readiness 都优先指向短 current state，而不是默认串读所有 durable docs。

边界：本批只改维护文档、release/default docs inventory 与测试；不新增 PowerShell runtime logic、不执行 heavy-tool/debug/inject/patch/dump/network/symex、不写 authority/confirmed、不改变 sync/promote review-first、case durable schema、公共入口删除门禁或远程 CI blocker 状态。

已完成内容：

- 新增 `docs/context-routing.md`，作为新会话、上下文压缩后接手和每轮自主推进的第一路由入口；默认只读 `CLAUDE.md` 边界、context router、`docs/batch-plan.md` current/next、`CHANGELOG.md` 顶部与真实状态。
- 将 `docs/batch-plan.md` 从 300+ 历史批次的长日志收缩为 current milestone / current batch state / next candidates / 验证标准 / 风险短入口；完整旧批次归档到 `docs/batch-history.md`，按 Batch ID 搜索使用。
- 压缩根 `CLAUDE.md` 为边界 + 路由 + 维护入口 + 验证命令，不再作为完整设计文档承载所有 durable state。
- `releaseHandoff.readFirst[]` 从多个长文档收缩为 `docs/context-routing.md`、`docs/batch-plan.md`、`docs/release-readiness.md` 与 `CHANGELOG.md`，并把 next actions 改为通过 signals 按需读取详细文档。
- `release-check` required documents 与 public default docs readiness inventory 纳入 `docs/context-routing.md` / `docs/batch-plan.md`，用测试锁定 read-first 数量、文档数量和 required phrases。
- 保留用户当前 Windows 本机稳定优先策略：远程三平台 CI 仍如实记录为 GitHub runner/billing blocker，不让它阻塞本地 product iteration。

验证结果：已通过 `go test ./internal/rekit/defaultdocs ./internal/rekit/releasecheck ./internal/rekit/cli`、`go test ./...`、`go vet ./...`、`go run ./cmd/rekit -- -Command release-check -Format json`、`go run ./cmd/rekit -- -Command status`、`go run ./cmd/rekit -- -Command packs`、`go run ./cmd/rekit -- -Command doctor` 与 `git diff --check`。`git diff --check` 仅报告 Windows LF/CRLF conversion warning，无 whitespace error。

上一批摘要：Batch 387 已完成 replaceable session executor takeover E2E；`start main -Apply -Executor <replacement>` 可接手已有 durable `main` lane 而不是误建 `feature-main`，相关 runtime、CLI product-path coverage、durable docs 与本地 release gate 已通过并推送。

### Next candidates

1. **Reviewer orchestration E2E（Batch 389）**：在现有 reviewer-intake strict writeback 之上，补 Mission Commander 侧多 reviewer dispatch/result/intake/writeback 的更完整 Windows 本机 product path；runtime 仍不自动 spawn 时，至少要让主 Agent handoff、packet、result contract 与 post-validation 可连续消费。
2. **Pack-memory product UX hardening**：在 Batch 358 package E2E 之外，补真实多 pack/跨 case review UX、candidate cleanup、promote/reconsume guidance 和人工 merge/reject/cleanup 路径。
3. **Mission Commander Windows product UX closure**：围绕当前用户单机 Windows 使用，把 overview/handoff/continue/start/gate 的自然语言 Mission Commander 操作路径继续收口，优先减少用户记命令和跨会话接手摩擦。
4. **Cross-platform product-path E2E（降优先级）**：在本地 CLI/case E2E 已覆盖 nested cwd / case shim 的基础上，仅保持可在 runner 可用时执行的三平台 matrix 候选和 known gap 记录；不要在 GitHub runner/billing blocker 未解除前让它阻塞 Windows 本机迭代。
5. **Remote release-gate unblock / product-path matrix（blocked / low priority）**：GitHub Actions billing/spending limit 解除后，再读取真实 Linux/Windows/macOS job conclusions，并把 CLI/case E2E、`status.caseShim` readiness 和完整 installed user entrypoint E2E 纳入可用 runner 验证。
6. **Retained public façade decision**：只有真实 release-gate-green、public references、case shim、smoke retirement 与恢复计划均满足后，才执行独立 removal batch；否则明确保留期限和 blocker。

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
