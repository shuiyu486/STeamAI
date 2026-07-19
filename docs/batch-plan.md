# Batch implementation plan

## 读取指南

先读本节的 current milestone / current batch state / next candidates；旧批次只在需要考古、验证历史决策或 release/debug 溯源时按 Batch ID 搜索 `docs/batch-history.md`。不要默认从 Batch 0 顺序重读 350+ 个历史批次。产品方向以 `docs/mission-control-product-direction.md` 为准，持续执行方式见 `docs/autonomous-goal.md`。

## 实施摘要

Batch 359 后，Go-owned/no-fallback public command surface、durable lanes、显式 reconcile、typed autonomy preflight、Mission brief / executor action、bounded reviewer dispatch → strict intake → verification-before-decision writeback → post-validation 的本机闭环、pack-memory promote/reconsume package E2E，以及 authorized execution observation evidence + bounded adapter execution report strict intake（含 authorized stopCondition boundaryHits、status summary enforcement、workspace-relative 与 case-relative machine-readable handoff）已形成底座。当前阶段继续从 contract/inventory field increments 转向 replaceable session executor / reviewer orchestration、lane executor / tool-adapter live validation、pack-memory product UX、Windows 本机 product-path 稳定与真实 release verification。

## 执行清单

### Current milestone

**Mission Commander operational closure and truthful release readiness**：把 durable lane/reviewer/autonomy contract 串成实际可运行、可跨会话接手、可验证的产品闭环，并区分 inventory ready、本地 gate executed 与远程 CI green。当前用户短期只要求 Windows 本机稳定可用；远程 Linux/macOS/Windows CI 因 runner/billing blocker 继续记录为 known gap，不阻塞本机 Mission Control 闭环。

### Current batch state

### Batch 389：reviewer orchestration E2E

状态：已完成；本地 Windows product path 已覆盖 Mission Commander 侧多 reviewer dispatch/result/intake/writeback 与 lane handoff provenance，提交/推送在本批收尾执行。

目标：在既有 `plan-subagents` planning artifacts 与 reviewer-intake strict writeback 之上，补齐主 Agent 可连续消费的多 reviewer orchestration：packet/summary/result JSON 要展示 dispatch lifecycle、每个 shard 的 reviewer result path、preview/apply intake command、owner binding 与 post-validation 状态；intake 写回后 lane handoff 要能直接看到 reviewer session provenance。

边界：runtime 仍不自动 spawn 或管理 reviewer/session；reviewer 不写文件或 ledger；intake 不写 authority/confirmed、不执行 heavy-tool、不改变 sync/promote review-first、case durable schema、公共 façade 删除门禁或远程 CI blocker 状态；本批不新增 PowerShell runtime logic。

已完成内容：

- `plan-subagents` result 与 `packet.json` 新增 `reviewerOrchestration` envelope，投影 mode、target lane、owner binding、result root、多 shard dispatches、max parallel、lifecycle、runtime boundary 与 completion criteria。
- `summary.md` 新增短 `reviewer orchestration` section，列出 lifecycle step 与 reviewer-dispatch，仍只作为主 Agent 调度短命 read-only reviewer 的 handoff，不把 runtime 变成自动 spawner。
- reviewer-intake result 新增 `orchestrationSnapshot`，在 WhatIf / Apply / already-complete / blocked / partial-write 状态中返回 dispatch index、total、before/after shard status 与 remaining dispatches，便于 Mission Commander 顺序收集和重试。
- packet identity 继续覆盖新增 orchestration 字段；Batch 389 前已生成、缺少 `reviewerOrchestration` 的旧 packet 仍可按 legacy identity intake，但非空 orchestration tamper 不会被 legacy fallback 绕过。
- verification / decision ledger 与 lane handoff 显示 `reviewerSession` 和 owner binding provenance，让替换 executor 或新会话无需重扫 reviewer result 文件即可看到 reviewer verdict 与 main merge decision 来源。
- CLI E2E 覆盖 attached case 中 `feature-login` owner executor、两 shard / 两 reviewer result、一 accept 一 reject、WhatIf→Apply 顺序写回、ledger provenance 与 lane handoff reviewer session projection。
- 文档维护规则同步补充：修改/创建/维护 durable docs 时也保持按需路由和渐进披露，顶部短执行区，细节按章节/专文路由，不把历史和长日志重新塞回 active docs。

验证结果：已通过 `go test ./internal/rekit/subagents -count=1`、`go test ./internal/rekit/cli -run TestRunPlanSubagents -count=1`、`go run ./cmd/rekit -- -Command release-check -Format json`、`go run ./cmd/rekit -- -Command status`、`go run ./cmd/rekit -- -Command packs`、`go run ./cmd/rekit -- -Command doctor`、`go test ./...`、`go vet ./...` 与 `git diff --check`。`git diff --check` 仅报告 Windows LF/CRLF conversion warning，无 whitespace error；远程 release-gate 仍是既有 runner/billing known gap，未作为本机 batch blocker。

上一批摘要：Batch 388 已完成 context routing / progressive disclosure；`docs/context-routing.md`、压缩后的 `docs/batch-plan.md`、根 `CLAUDE.md` 与 `releaseHandoff.readFirst[]` 已把默认读取路径收缩到短 current state，旧批次归档到 `docs/batch-history.md` 按需搜索。

### Next candidates

1. **Pack-memory product UX hardening**：在 Batch 358 package E2E 之外，补真实多 pack/跨 case review UX、candidate cleanup、promote/reconsume guidance 和人工 merge/reject/cleanup 路径。
2. **Mission Commander Windows product UX closure**：围绕当前用户单机 Windows 使用，把 overview/handoff/continue/start/gate 的自然语言 Mission Commander 操作路径继续收口，优先减少用户记命令和跨会话接手摩擦。
3. **Lane/tool-adapter live validation UX closure**：继续把 authorized-gate contract、adapter sidecar validation、record handoff 与 lane executor 接手提示做成更少命令记忆、更强 post-validation 的 Windows 本机闭环。
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
