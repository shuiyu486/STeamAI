# Batch implementation plan

## 读取指南

先读本节的 current milestone / current batch state / next candidates；旧批次只在需要考古、验证历史决策或 release/debug 溯源时按 Batch ID 搜索 `docs/batch-history.md`。不要默认从 Batch 0 顺序重读 350+ 个历史批次。产品方向以 `docs/mission-control-product-direction.md` 为准，持续执行方式见 `docs/autonomous-goal.md`。

## 实施摘要

Batch 359 后，Go-owned/no-fallback public command surface、durable lanes、显式 reconcile、typed autonomy preflight、Mission brief / executor action、bounded reviewer dispatch → strict intake → verification-before-decision writeback → post-validation 的本机闭环、pack-memory promote/reconsume package E2E，以及 authorized execution observation evidence + bounded adapter execution report strict intake（含 authorized stopCondition boundaryHits、status summary enforcement、workspace-relative 与 case-relative machine-readable handoff）已形成底座。当前阶段继续从 contract/inventory field increments 转向 replaceable session executor / reviewer orchestration、lane executor / tool-adapter live validation、pack-memory product UX、Windows 本机 product-path 稳定与真实 release verification。

## 执行清单

### Current milestone

**Mission Commander operational closure and truthful release readiness**：把 durable lane/reviewer/autonomy contract 串成实际可运行、可跨会话接手、可验证的产品闭环，并区分 inventory ready、本地 gate executed 与远程 CI green。当前用户短期只要求 Windows 本机稳定可用；远程 Linux/macOS/Windows CI 因 runner/billing blocker 继续记录为 known gap，不阻塞本机 Mission Control 闭环。

### Current batch state

### Batch 411：Adapter report contract / validation next-action closure

状态：已完成本地实现、durable docs、focused/affected package validation 与 full local validation；待提交/推送与远程 release-gate inspection。

目标：Batch 392 已让 `gate -ExecutionReportContract` 与 `gate -ValidateExecutionReport` 输出 adapter sidecar `missionCommanderAction`，但主 Agent / replacement executor 仍需从 commander action、nextSteps、repairHints、liveValidation record/validate args 手工拼接 “read-only validate → valid=true 后 record → handoff” 或 “repair → rerun validation” 顺序。本批把 adapter report contract / validation 的可执行顺序收口为 `missionCommanderNextActions[]`。

边界：只增强 `gate -ExecutionReportContract` / `gate -ValidateExecutionReport` JSON projection、package/CLI tests 与 durable docs；validation 仍 read-only，不写 observations/authority/confirmed；contract 阶段的 record command 只作为 blocked handoff，必须等 valid=true 并替换 `<executor-id>`；runtime 不执行 heavy-tool、不新增 PowerShell runtime logic、不改变 sync/promote review-first、case durable schema、公共 façade 删除门禁或远程 CI blocker 状态。

已完成内容：

- `AdapterExecutionReportContract` 与 `AdapterExecutionReportValidation` 新增 `missionCommanderNextActions[]`。
- contract 阶段输出 read-only validation primary action、blocked record follow-up 与 handoff follow-up；record follow-up 明确 `do not record evidence until validation returns valid=true` 与 `replace <executor-id>` boundary。
- validation valid=true 阶段输出 record observation evidence next action 与 handoff next action；invalid/missing path 阶段输出 repair hint next action 与 rerun read-only validation next action，不输出 record `-Apply`。
- package 与 CLI coverage 锁定 contract/validation JSON projection、nested workspace case-relative product path、repair hints、no observations on validation、no authority/confirmed、no heavy-tool 与 no premature record boundaries。

验证结果：已通过 focused `go test ./internal/rekit/gate ./internal/rekit/cli -run "TestAdapterReportContractDescribesAuthorizedGateBoundaries|TestValidateAdapterExecutionReportReadOnlyPreflight|TestValidateAdapterExecutionReportMissingPathExposesMissionCommanderRepair|TestValidateAdapterExecutionReportReturnsInvalidEnvelopeReadOnly|TestRunGateAdapterReportReadOnlyPreflightFromNestedOutputWorkspace" -count=1`、affected package `go test ./internal/rekit/gate ./internal/rekit/cli -count=1`、affected vet `go vet ./internal/rekit/gate ./internal/rekit/cli`、`go test ./...`、`go vet ./...`、`go run ./cmd/rekit -- -Command release-check -Format json`、`go run ./cmd/rekit -- -Command status`、`go run ./cmd/rekit -- -Command packs`、`go run ./cmd/rekit -- -Command doctor` 与 `git diff --check`。`release-check` 汇总 ready=true、summary=release gate inventory ok；`status`、`packs`、`doctor` 正常，`doctor` 输出 `pack validation ok`；`git diff --check` 仅报告 Windows LF/CRLF conversion warning，无 whitespace error。

上一批摘要：Batch 410 已完成 duplicate execution evidence idempotent `missionCommanderNextActions[]`，详见 `docs/batch-history.md`。

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
