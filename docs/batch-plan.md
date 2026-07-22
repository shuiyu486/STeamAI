# Batch implementation plan

## 读取指南

先读本节的 current milestone / current batch state / next candidates；旧批次只在需要考古、验证历史决策或 release/debug 溯源时按 Batch ID 搜索 `docs/batch-history.md`。不要默认从 Batch 0 顺序重读 350+ 个历史批次。产品方向以 `docs/mission-control-product-direction.md` 为准，持续执行方式见 `docs/autonomous-goal.md`。

## 实施摘要

Batch 359 后，Go-owned/no-fallback public command surface、durable lanes、显式 reconcile、typed autonomy preflight、Mission brief / executor action、bounded reviewer dispatch → strict intake → verification-before-decision writeback → post-validation 的本机闭环、pack-memory promote/reconsume package E2E，以及 authorized execution observation evidence + bounded adapter execution report strict intake（含 authorized stopCondition boundaryHits、status summary enforcement、workspace-relative 与 case-relative machine-readable handoff）已形成底座。当前阶段继续从 contract/inventory field increments 转向 replaceable session executor / reviewer orchestration、lane executor / tool-adapter live validation、pack-memory product UX、Windows 本机 product-path 稳定与真实 release verification。

## 执行清单

### Current milestone

**Mission Commander operational closure and truthful release readiness**：把 durable lane/reviewer/autonomy contract 串成实际可运行、可跨会话接手、可验证的产品闭环，并区分 inventory ready、本地 gate executed 与远程 CI green。当前用户短期只要求 Windows 本机稳定可用；远程 Linux/macOS/Windows CI 因 runner/billing blocker 继续记录为 known gap，不阻塞本机 Mission Control 闭环。

### Current batch state

### Batch 518：authorized execution adapter report summary handoff closure

状态：已完成 runtime/CLI/test/docs implementation、focused/package validation 与完整本地 release minimum；commit/push 与远程 release-gate inspection 待本轮收尾后记录。当前远程 release-gate 仍按既有 GitHub Actions runner/billing blocker 处理：`release-check` / `ciReleaseGate.ready` 只表示 inventory/workflow ready，不能声明 remote CI green。

目标：Batch 421/424/432 已覆盖 adapter execution report contract / validation / Mission Commander action queue，Batch 458-462 已补齐 authorized execution follow-through text，Batch 504 已补齐 recorded sidecar path、actualBudget 与 adapter provenance；但 replacement executor 若只想确认当前 contract/validation state、report/default path、是否 valid、是否 ready to record、是否 blocked/repair/main escalation、allowed status/output/stop/candidate totals、repair hints、outcome/next-action counts、report status/adapter id/actualBudget/refs/boundary hits 与 no-heavy/no-authority boundary，仍要手工拼完整 contract、validation、follow-through 和 action queue。Batch 518 将这些 adapter report 接续状态压缩为 compact `reportSummary`，同时保留完整 arrays 与 strict validation 明细。

边界：只增强 `gate -ExecutionReportContract` 与 `gate -ValidateExecutionReport` 的只读 downstream summary projection、CLI visibility 与测试；不执行 heavy tool、不 replay adapter、不改变 sidecar schema、authorized gate decision、record evidence validation、sync/promote、case durable schema、authority/confirmed、PowerShell façade 或公共 façade removal 门禁。`gate -Apply -GateEventId ... -ExecutionReportPath ...` 仍只在 sidecar `valid=true` 后记录 bounded observation evidence。

已完成内容：

- `gate.AdapterExecutionReportContract` 与 `gate.AdapterExecutionReportValidation` 新增 `reportSummary`，由 `adapterReportHandoffSummary` 汇总 state、gate/action/lane、report/default path、reportPresent、valid/recordReady/recordBlocked、requiresValidation/repair/mainEscalation、allowed status/output/stop/candidate counts、repair/record-blocked/escalation hints、outcome/next-action/review counts、current action、report status/adapter id/actualBudget、output/evidence/boundary ref counts、failure code/stage 与 no-heavy/no-authority boundary。
- contract path、valid validation path、missing/invalid validation path 都投影同一 compact summary；valid sidecar summary 显示 `ready-to-record-evidence`，invalid sidecar summary 显示 `repair-adapter-report` 与 failure taxonomy，contract summary 显示 `needs-adapter-report-validation` 与 blocked record handoff。
- CLI text 新增 `gate adapter report contract summary...` 与 `gate adapter report validation summary...` lines，并输出 summary action queue / boundary；完整 contract、validation repair hints、Mission Commander action queue、authorized execution follow-through 与 adapter context provenance 仍保留给深度复核。
- gate package tests 与 CLI JSON/text E2E 覆盖 contract summary、valid validation summary、invalid validation summary、read-only no-observation mutation boundary、current action counts、failure code/stage 与 no-heavy/no-authority summary boundary。
- README、`.claude/skills/rekit/SKILL.md`、`docs/agent-team-usage.md` 与 `CHANGELOG.md` 已更新，说明 `reportSummary` 是 replacement executor 的 compact handoff，不替代 strict validation 或 evidence record gate。

验证结果：已通过 `gofmt -w internal/rekit/gate/gate.go internal/rekit/gate/gate_test.go internal/rekit/cli/cli.go internal/rekit/cli/cli_test.go`、focused `go test ./internal/rekit/gate ./internal/rekit/cli -run "TestAdapterReportContractDescribesAuthorizedGateBoundaries|TestValidateAdapterExecutionReportReadOnlyPreflight|TestValidateAdapterExecutionReportMissingPathExposesMissionCommanderRepair|TestValidateAdapterExecutionReportReturnsInvalidEnvelopeReadOnly|TestRunGateAdapterReportTextOutputsNextActions|TestRunGate" -count=1`、package `go test ./internal/rekit/gate ./internal/rekit/cli -count=1`、focused `go vet ./internal/rekit/gate ./internal/rekit/cli`，以及完整本地 release minimum：`go run ./cmd/rekit -- -Command release-check -Format json`（release-check ready=true）、`go run ./cmd/rekit -- -Command status -Format text`、`go run ./cmd/rekit -- -Command packs -Format text`、`go run ./cmd/rekit -- -Command doctor -Format text`、`go test ./...`、`go vet ./...`、`git diff --check`（仅 LF/CRLF conversion warnings，无 whitespace error）。package test 曾因 CLI JSON summary 新断言把 escalated valid sidecar 误判为 main escalation、并把 next-action counts 写成 4 而失败；已按实际 summary state 修正为 `RequiresMainEscalation=false`、valid/invalid validation `NextActionCount=2` / `ReviewRequiredActionCount=2` 后重跑通过。

上一批摘要：Batch 517 已完成 pack-memory promote review summary downstream closure，并归档到 `docs/batch-history.md`。

### Next candidates

1. **Lane/tool-adapter live validation residuals（如仍有缺口）**：Batch 421/424/432 已覆盖 adapter contract/validation/report action queue、follow-through 与 contract liveValidation text，Batch 458-462 已补齐 authorizedExecutionFollowThrough / evidence review follow-through text，Batch 504 已补齐 recorded adapter sidecar path、actualBudget 与 adapter provenance，Batch 518 已补齐 contract/validation compact `reportSummary`；后续仅在 Windows 本机 product-path 仍存在 validation/record/evidence review handoff 到 replacement executor 的真实断点时推进；不新增 adapter/heavy-tool execution。
2. **Reviewer orchestration E2E residuals（如仍有缺口）**：Batch 489/499/505 已覆盖 reviewer intake product path、reviewer writeback identity 与 reviewer result provenance downstream handoff；后续仅在 dispatch/intake/post-validation terminal path 仍要求解析 nested JSON、打开 reviewer result artifact 或手工拼 route output 才能接续时推进，不自动 spawn reviewer、不执行 heavy-tool。
3. **Pack-memory downstream UX residuals（如仍有缺口）**：Batch 500 已把 candidate decision/cleanup/reconsume expected evidence 收口为 `reviewArtifacts[]`，Batch 501 已把 open candidate residue 投影到 release/status handoff，Batch 502/503 已补齐 case-local status/default path、candidate identity、index mapping 与 derived review artifact visibility，Batch 517 已补齐 compact review summary；后续仅在 accepted/rejected 人工流程、cleanup/reconsume 或 evidence review downstream UX 仍需跨 envelope 手工拼接时推进，不重复做字段微批次。
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
