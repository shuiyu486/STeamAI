# Batch implementation plan

## 读取指南

先读本节的 current milestone / current batch state / next candidates；旧批次只在需要考古、验证历史决策或 release/debug 溯源时按 Batch ID 搜索 `docs/batch-history.md`。不要默认从 Batch 0 顺序重读 350+ 个历史批次。产品方向以 `docs/mission-control-product-direction.md` 为准，持续执行方式见 `docs/autonomous-goal.md`。

## 实施摘要

Batch 359 后，Go-owned/no-fallback public command surface、durable lanes、显式 reconcile、typed autonomy preflight、Mission brief / executor action、bounded reviewer dispatch → strict intake → verification-before-decision writeback → post-validation 的本机闭环、pack-memory promote/reconsume package E2E，以及 authorized execution observation evidence + bounded adapter execution report strict intake（含 authorized stopCondition boundaryHits、status summary enforcement、workspace-relative 与 case-relative machine-readable handoff）已形成底座。当前阶段继续从 contract/inventory field increments 转向 replaceable session executor / reviewer orchestration、lane executor / tool-adapter live validation、pack-memory product UX、Windows 本机 product-path 稳定与真实 release verification。

## 执行清单

### Current milestone

**Mission Commander operational closure and truthful release readiness**：把 durable lane/reviewer/autonomy contract 串成实际可运行、可跨会话接手、可验证的产品闭环，并区分 inventory ready、本地 gate executed 与远程 CI green。当前用户短期只要求 Windows 本机稳定可用；远程 Linux/macOS/Windows CI 因 runner/billing blocker 继续记录为 known gap，不阻塞本机 Mission Control 闭环。

### Current batch state

### Batch 510：adapter context validation text handoff closure

状态：已完成 adapter tooling candidate / selected adapter provenance contract text 与 validation text handoff、focused CLI coverage、用户与 durable docs、本地 release minimum、commit/push 与远程 release-gate inspection；远程 release-gate run `29872298902` completed failure，Linux/Windows/macOS jobs `steps=[]`，按既有 GitHub Actions runner/billing blocker 记录，不能声明远程 CI green。

目标：Batch 509 已把 adapter validation repair hints 的 evidence/boundary 收口到 contract/validation JSON/text 与 Mission Commander next actions；但 valid sidecar validation text 仍只显示 normalized sidecar identity、budget、refs、summary 与 follow-through。虽然 JSON 已携带 `adapterContext.candidates[]` / `adapterContext.selected`，replacement executor 在 case-local / authorized workspace 用 no-target validation 确认 sidecar 时，仍要切回 JSON 才能核对 sidecar `adapterId` 对应哪个 concrete tooling candidate、是否就是 selected adapter、以及该 adapter 的 report/evidence/stop-condition guidance。Batch 510 将 adapter context provenance 投影到 contract text 与 validation text。

边界：只增强 read-only `gate -ExecutionReportContract` / `gate -ValidateExecutionReport` text handoff；不改变 adapter candidate discovery/ranking、selected adapter matching、adapter report validation、record-only-after-valid=true 顺序、execution evidence record schema、sync/promote、case durable schema 或 PowerShell runtime logic；`/rekit` 仍不执行 heavy-tool、不 replay adapter、不写 authority/confirmed；record evidence 仍要求显式 `gate -Apply` 且 validation `valid=true`。

已完成内容：

- 新增 shared adapter tooling candidate text writer，contract `liveValidation.adapterCandidates[]` 与 `liveValidation.selectedAdapter` 使用同一 text projection。
- `gate -ExecutionReportContract -Format text` 在候选 adapter 行之外新增 `gate adapter report selected adapter...` / guidance / evidence / stop-condition lines。
- `gate -ValidateExecutionReport -Format text` 输出 `gate adapter report validation adapter candidate...` 与 `gate adapter report validation selected adapter...`，让 validation terminal handoff 直接显示 adapterContext candidate/selected provenance。
- `TestRunGateProjectsPackToolingAdapterCandidateProductPath` 覆盖 generic-binary-re pack tooling adapter：contract text selected adapter projection、case-local authorized workspace no-target validation text adapter context projection、record path仍保留 selected adapter evidence provenance。
- README、`/rekit` skill、Agent Team usage、CHANGELOG 与 batch docs同步说明 adapter context validation text handoff。

验证结果：已通过 `gofmt -w internal/rekit/cli/cli.go internal/rekit/cli/cli_test.go`、focused `go test ./internal/rekit/cli -run "TestRunGateProjectsPackToolingAdapterCandidateProductPath|TestRunGenericBinaryReAdapterLiveValidationProductPath|TestRunGateAdapterReportTextOutputsNextActions" -count=1`，以及完整本地 release minimum：`go run ./cmd/rekit -- -Command release-check -Format json`（release-check ready=true）、`go run ./cmd/rekit -- -Command status -Format text`、`go run ./cmd/rekit -- -Command packs -Format text`、`go run ./cmd/rekit -- -Command doctor -Format text`、`go test ./...`、`go vet ./...`、`git diff --check`。提交 `d71d6a7` 已 push 到 `main`；远程 release-gate run `29872298902` completed failure，Linux/Windows/macOS jobs 均 `steps=[]`，按既有 GitHub Actions runner/billing blocker 记录，不能声明远程 CI green。

上一批摘要：Batch 509 已完成 adapter validation repair evidence/boundary handoff closure，并归档到 `docs/batch-history.md`。

### Next candidates

1. **Pack-memory downstream UX residuals（如仍有缺口）**：Batch 500 已把 candidate decision/cleanup/reconsume expected evidence 收口为 `reviewArtifacts[]`，Batch 501 已把 open candidate residue 投影到 release/status handoff，Batch 502/503 已补齐 case-local status/default path、candidate identity、index mapping 与 derived review artifact visibility；后续仅在 accepted/rejected 人工流程、cleanup/reconsume 或 evidence review downstream UX 仍需跨 envelope 手工拼接时推进，不重复做字段微批次。
2. **Lane/tool-adapter live validation residuals（如仍有缺口）**：Batch 421/424/432 已覆盖 adapter contract/validation/report action queue、follow-through 与 contract liveValidation text，Batch 458-462 已补齐 authorizedExecutionFollowThrough / evidence review follow-through text，Batch 504 已补齐 recorded adapter sidecar path、actualBudget 与 adapter provenance；后续仅在 Windows 本机 product-path 仍存在 validation/record/evidence review handoff 到 replacement executor 的真实断点时推进；不新增 adapter/heavy-tool execution。
3. **Reviewer orchestration E2E residuals（如仍有缺口）**：Batch 489/499/505 已覆盖 reviewer intake product path、reviewer writeback identity 与 reviewer result provenance downstream handoff；后续仅在 dispatch/intake/post-validation terminal path 仍要求解析 nested JSON、打开 reviewer result artifact 或手工拼 route output 才能接续时推进，不自动 spawn reviewer、不执行 heavy-tool。
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
