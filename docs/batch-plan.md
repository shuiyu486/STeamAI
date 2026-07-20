# Batch implementation plan

## 读取指南

先读本节的 current milestone / current batch state / next candidates；旧批次只在需要考古、验证历史决策或 release/debug 溯源时按 Batch ID 搜索 `docs/batch-history.md`。不要默认从 Batch 0 顺序重读 350+ 个历史批次。产品方向以 `docs/mission-control-product-direction.md` 为准，持续执行方式见 `docs/autonomous-goal.md`。

## 实施摘要

Batch 359 后，Go-owned/no-fallback public command surface、durable lanes、显式 reconcile、typed autonomy preflight、Mission brief / executor action、bounded reviewer dispatch → strict intake → verification-before-decision writeback → post-validation 的本机闭环、pack-memory promote/reconsume package E2E，以及 authorized execution observation evidence + bounded adapter execution report strict intake（含 authorized stopCondition boundaryHits、status summary enforcement、workspace-relative 与 case-relative machine-readable handoff）已形成底座。当前阶段继续从 contract/inventory field increments 转向 replaceable session executor / reviewer orchestration、lane executor / tool-adapter live validation、pack-memory product UX、Windows 本机 product-path 稳定与真实 release verification。

## 执行清单

### Current milestone

**Mission Commander operational closure and truthful release readiness**：把 durable lane/reviewer/autonomy contract 串成实际可运行、可跨会话接手、可验证的产品闭环，并区分 inventory ready、本地 gate executed 与远程 CI green。当前用户短期只要求 Windows 本机稳定可用；远程 Linux/macOS/Windows CI 因 runner/billing blocker 继续记录为 known gap，不阻塞本机 Mission Control 闭环。

### Current batch state

### Batch 457：promote candidates cleanup/reconsume text identity closure

状态：已完成本地实现、focused 与 full local validation、durable docs、commit/push 与远程 release-gate inspection；远程 release-gate run `29749177561` 为 completed failure，Windows/macOS/Linux jobs 均 failure 且 `steps: []`，仍是既有 GitHub Actions runner/billing blocker。

目标：Batch 456 已让 `promote -CreateCandidates -Format text` 直接输出 decision outcome when/evidence、follow-through boundary、top-level reconsume commands/boundaries 与 reconsume check when/evidence；但 cleanup action、reconsume command 与 reconsume boundary legacy lines 仍是无 owner 的短行，Mission Commander / replacement executor 在处理多个 cleanup target 或多个 reconsume check 时仍需要把命令/边界手工匹配回 `cleanupTargets[]` 或 `reconsume.verificationChecklist[]`。Batch 457 补齐这些 terminal text identity。

边界：只增强 `promote -CreateCandidates -Format text` pack-memory cleanup/reconsume terminal handoff、focused CLI coverage 与 durable docs；保留既有 legacy text lines，不改变 JSON contract、candidate generation、candidate path/index semantics、manual merge/cleanup/reconsume guidance-only、default JSON compatibility，不执行 merge/cleanup/init/doctor、不写 authority/confirmed、不执行 heavy-tool、不新增 PowerShell runtime logic、不改变远程 CI blocker 状态。

已完成内容：

- cleanup action text 保留 legacy action line，并新增 `cleanup action detail`，携带 path、candidatePath、indexPath 与 action，便于逐 candidate 执行 cleanup / indexPath update review。
- reconsume command text 保留 legacy command line，并新增 `reconsume command detail`，携带 verification check name 与 command，便于区分 pack doctor、fresh-case reconsume 与 attached-case reconsume。
- reconsume boundary text 保留 legacy boundary line，并新增 `reconsume boundary detail`，携带 verification check name 与 boundary，避免多个 check 的 boundary 被手工混配。
- CLI coverage 锁定 WhatIf 与 actual create-candidates text 中的 cleanup action detail、fresh-case reconsume command detail、fresh-case reconsume boundary detail，以及 legacy text compatibility、existing action queue/next-action/no authority/confirmed/no-heavy 边界。

验证结果：已通过 focused `go test ./internal/rekit/cli -run 'TestRunPromoteCreateCandidates(WhatIf|WritesCandidates)' -count=1`、`go test ./...`、`go vet ./...`、`go run ./cmd/rekit -- -Command release-check -Format json`、`go run ./cmd/rekit -- -Command release-check -Format text`、`go run ./cmd/rekit -- -Command status`、`go run ./cmd/rekit -- -Command packs`、`go run ./cmd/rekit -- -Command doctor` 与 `git diff --check`；`release-check ready=true`，`git diff --check` 仅有 Windows LF/CRLF conversion warnings。已提交并推送 `22d35af Add promote candidate cleanup text identities`；远程 release-gate run `29749177561` 为 completed failure，Windows/macOS/Linux jobs 均 failure 且 `steps: []`，仍是既有 GitHub Actions runner/billing blocker，不能声明远程 CI green。

上一批摘要：Batch 456 已完成 promote candidates decision text detail closure，详见 `docs/batch-history.md`。

### Next candidates

1. **Lane/tool-adapter live validation residuals（如仍有缺口）**：Batch 421/424/432 已覆盖 adapter contract/validation/report action queue、follow-through 与 contract liveValidation text；后续仅在 Windows 本机 product-path 仍存在 validation/record/evidence review handoff 到 replacement executor 的真实断点时推进；不新增 adapter/heavy-tool execution。
2. **Reviewer orchestration E2E residuals（如仍有缺口）**：仅在 dispatch/intake/post-validation terminal path 仍要求解析 nested JSON 或打开 artifact 才能接续时推进，不自动 spawn reviewer、不执行 heavy-tool。
3. **Pack-memory downstream UX residuals（如仍有缺口）**：Batch 423/429/430/431 已覆盖 candidate decision outcome、execution plan、WhatIf preview 与 review checklist text，Batch 456/457 已补齐 decision detail、cleanup action detail 与 reconsume command/boundary detail；后续仅在 accepted/rejected 人工流程、cleanup/reconsume 或 evidence review downstream UX 仍需跨 envelope 手工拼接时推进，不重复做字段微批次。
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
