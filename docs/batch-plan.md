# Batch implementation plan

## 读取指南

先读本节的 current milestone / current batch state / next candidates；旧批次只在需要考古、验证历史决策或 release/debug 溯源时按 Batch ID 搜索 `docs/batch-history.md`。不要默认从 Batch 0 顺序重读 350+ 个历史批次。产品方向以 `docs/mission-control-product-direction.md` 为准，持续执行方式见 `docs/autonomous-goal.md`。

## 实施摘要

Batch 359 后，Go-owned/no-fallback public command surface、durable lanes、显式 reconcile、typed autonomy preflight、Mission brief / executor action、bounded reviewer dispatch → strict intake → verification-before-decision writeback → post-validation 的本机闭环、pack-memory promote/reconsume package E2E，以及 authorized execution observation evidence + bounded adapter execution report strict intake（含 authorized stopCondition boundaryHits、status summary enforcement、workspace-relative 与 case-relative machine-readable handoff）已形成底座。当前阶段继续从 contract/inventory field increments 转向 replaceable session executor / reviewer orchestration、lane executor / tool-adapter live validation、pack-memory product UX、Windows 本机 product-path 稳定与真实 release verification。

## 执行清单

### Current milestone

**Mission Commander operational closure and truthful release readiness**：把 durable lane/reviewer/autonomy contract 串成实际可运行、可跨会话接手、可验证的产品闭环，并区分 inventory ready、本地 gate executed 与远程 CI green。当前用户短期只要求 Windows 本机稳定可用；远程 Linux/macOS/Windows CI 因 runner/billing blocker 继续记录为 known gap，不阻塞本机 Mission Control 闭环。

### Current batch state

### Batch 511：release handoff remote blocker truthfulness closure

状态：已完成 latest-batch remote release-gate empty-steps blocker parser、negative remote-green guard、focused releasecheck/CLI parser coverage、用户与 durable docs、本地 release minimum；commit/push 与远程 release-gate inspection 待执行。

目标：Batch 510 release inspection docs 记录远程 release-gate jobs 为 `steps=[]`，但 `releaseHandoff.latestBatch.handoff.remoteReleaseGate` 只稳定识别 `steps: []`，导致 status/release-check 可能把已 inspection 的 empty-steps blocker 误投影为 `inspected`，且中英混合的 “不能声明 remote CI green” 也可能触发 `remote CI green` 子串误判。Batch 511 将 remote blocker 解析收口为 machine-readable truthfulness handoff，避免新会话或 release maintainer 误把 runner/billing blocker 当成 remote green/inspected-only。

边界：只增强 `release-check` / kit-mode `status` 的只读 latest-batch handoff parser 与测试；不执行远程 CI、不改变 `.github/workflows/release-gate.yml`、不改变 release gate inventory 语义、本地 release minimum、case runtime、sync/promote、authority/confirmed、heavy-tool、PowerShell façade 或公共 façade removal 门禁。远程 jobs `steps=[]` 仍记录为 known blocker，不能声明远程 CI green。

已完成内容：

- 新增 `latestBatchRemoteHasEmptySteps`，统一识别 `steps: []`、`steps=[]` 与中文 `steps 为空` / `steps为空`。
- `latestBatchRemoteReleaseGate` 先判断 empty-steps blocker，再判断 green / inspected，避免 `steps=[]` 被误判为普通 inspected。
- `latestBatchEvidence` 复用同一 empty-steps matcher，确保 latest batch evidence 输出 `remote release-gate jobs steps=[] recorded`。
- `latestBatchRemoteGreen` 增加中文中英混合 negative guard，避免 “不能声明 remote CI green” 触发 green。
- `release_handoff_test.go` 新增 coverage，锁定 `steps=[]` + completed failure + negative green phrase 的 blocked handoff。

验证结果：已通过 `gofmt -w internal/rekit/releasecheck/release_handoff.go internal/rekit/releasecheck/release_handoff_test.go`、focused `go test ./internal/rekit/releasecheck -run "TestLatestBatchHandoffExtractsValidationEvidence|TestLatestBatchRemoteGateDoesNotTreatNegativeGreenAsGreen|TestLatestBatchRemoteGateRecognizesEqualsEmptyStepsAndChineseNegativeGreen" -count=1`、package `go test ./internal/rekit/releasecheck -count=1`、CLI handoff `go test ./internal/rekit/cli -run "TestRunStatusJsonKit|TestRunReleaseCheckJsonInventory" -count=1`、focused `go vet ./internal/rekit/releasecheck`，以及完整本地 release minimum：`go run ./cmd/rekit -- -Command release-check -Format json`（release-check ready=true）、`go run ./cmd/rekit -- -Command status -Format text`、`go run ./cmd/rekit -- -Command packs -Format text`、`go run ./cmd/rekit -- -Command doctor -Format text`、`go test ./...`、`go vet ./...`、`git diff --check`。远程 release-gate 待 commit/push 后 inspection。

上一批摘要：Batch 510 已完成 adapter context validation text handoff closure，并归档到 `docs/batch-history.md`。

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
