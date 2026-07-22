# Batch implementation plan

## 读取指南

先读本节的 current milestone / current batch state / next candidates；旧批次只在需要考古、验证历史决策或 release/debug 溯源时按 Batch ID 搜索 `docs/batch-history.md`。不要默认从 Batch 0 顺序重读 350+ 个历史批次。产品方向以 `docs/mission-control-product-direction.md` 为准，持续执行方式见 `docs/autonomous-goal.md`。

## 实施摘要

Batch 359 后，Go-owned/no-fallback public command surface、durable lanes、显式 reconcile、typed autonomy preflight、Mission brief / executor action、bounded reviewer dispatch → strict intake → verification-before-decision writeback → post-validation 的本机闭环、pack-memory promote/reconsume package E2E，以及 authorized execution observation evidence + bounded adapter execution report strict intake（含 authorized stopCondition boundaryHits、status summary enforcement、workspace-relative 与 case-relative machine-readable handoff）已形成底座。当前阶段继续从 contract/inventory field increments 转向 replaceable session executor / reviewer orchestration、lane executor / tool-adapter live validation、pack-memory product UX、Windows 本机 product-path 稳定与真实 release verification。

## 执行清单

### Current milestone

**Mission Commander operational closure and truthful release readiness**：把 durable lane/reviewer/autonomy contract 串成实际可运行、可跨会话接手、可验证的产品闭环，并区分 inventory ready、本地 gate executed 与远程 CI green。当前用户短期只要求 Windows 本机稳定可用；远程 Linux/macOS/Windows CI 因 runner/billing blocker 继续记录为 known gap，不阻塞本机 Mission Control 闭环。

### Current batch state

### Batch 536：execution evidence adapter context downstream handoff closure

状态：已完成 runtime/CLI/test/docs implementation、focused/package validation 与完整本地 release minimum；implementation commit/push 与远程 release-gate inspection 待执行。`release-check`/`ciReleaseGate.ready` 仍不能替代实际远程 job conclusion。

目标：Batch 504 已把 recorded adapter observation evidence 的 `executionReportPath`、actual budget 与 `adapterId/status` 投影到 `executionEvidenceReview[]`；Batch 535 又把 authorized-gate live validation 的 selected adapter detail 投影到 downstream handoff。但已记录 observation evidence 本身虽然在 observations ledger 的 `execution.adapterContext` 中保留匹配到的 concrete tooling candidate，Mission-layer review 与 replacement executor handoff 仍只看见 `adapterId/status`，需要回退完整 `gate -ExecutionReportContract`、adapter sidecar 或 observations ledger 才能确认 adapter `entry`、`purpose`、`sideEffects`、`reportGuidance[]`、`evidenceGuidance[]` 与 `stopConditionHints[]`。Batch 536 将 recorded evidence 的 adapter context detail 提升到 execution evidence review downstream/durable handoff。

边界：只增强已记录 authorized execution observation evidence 的只读 terminal/downstream/durable projection 与 CLI product-path coverage；不执行 adapter/heavy-tool、不 replay、不 validate 或 record sidecar、不写 observations/authority/confirmed，不改变 pending-gate blocker、authorized-gate strict record validation、sync/promote review-first、case durable schema、PowerShell façade 或公共 façade removal 门禁。

已完成内容：

- `mission.ExecutionEvidenceReviewItem` 新增 compact `adapterContext`，从 observation evidence 的 `execution.adapterContext` 解码 concrete tooling candidate detail；`ExecutionEvidenceReviewSummary` 新增 `latestAdapterContext`，并在 `HasAdapter` 中计入 adapter context。
- Status / overview / handoff / continue 的 JSON/text projection 在 execution evidence report/budget/adapter id/status 旁直接输出 adapter context identity、entry、purpose、sideEffects、gateActions、toolingCatalogPath、report/evidence guidance、stopConditionHints 与 `recordOnlyAfterGate`。
- Durable artifacts 同步写入 adapter context：project/lane handoff Markdown、lane `RESUME.md`、typed checkpoint、continue run `status.json` 与 `digest.md`，让 replacement executor 从 durable handoff 接手时不必回退完整 contract 或 ledger。
- CLI product-path coverage 扩展 `generic-binary-re` authorized gate flow，验证 record evidence、status JSON/text、overview JSON/text、handoff apply JSON/Markdown、lane RESUME/checkpoint、continue apply JSON/text 与 run digest 都保留 `dynamic-debug-or-writeback-action` adapter context detail，同时保留 no authority/confirmed invariants。
- 文档同步更新 `README.md`、`docs/release-readiness.md`、`rekit/tests/README.md`、`.claude/skills/rekit/SKILL.md`、`CHANGELOG.md`，并将 Batch 535 归档到 `docs/batch-history.md`。

验证结果：已通过 focused `go test ./internal/rekit/cli -run "TestRunGateProjectsPackToolingAdapterCandidateProductPath" -count=1`；已通过 `gofmt -w internal/rekit/cli/cli.go internal/rekit/cli/cli_test.go internal/rekit/mission/brief.go internal/rekit/mission/evidence_review.go internal/rekit/overview/overview.go internal/rekit/workstream/continue.go internal/rekit/workstream/handoff.go internal/rekit/workstream/start.go` 与 package `go test ./internal/rekit/mission ./internal/rekit/overview ./internal/rekit/workstream ./internal/rekit/cli -count=1`；完整本地 release minimum 已通过 `go run ./cmd/rekit -- -Command release-check -Format json`（release-check ready=true / release gate inventory ok）、`go run ./cmd/rekit -- -Command status -Format text`、`go run ./cmd/rekit -- -Command packs -Format text`、`go run ./cmd/rekit -- -Command doctor -Format text`、`go test ./...`、`go vet ./...`、`git diff --check`（仅 LF→CRLF working-copy warning，无 whitespace error）。

上一批摘要：Batch 535 已完成 authorized-gate selected adapter detail handoff closure，implementation commit `b1b4b9b Add selected adapter handoff detail` 与 release inspection commit `aff90ba Record Batch 535 release gate inspection` 已推送；implementation run `29925950785` completed failure，Linux/macOS/Windows jobs 均 completed failure 且 `steps=[]`，仍是既有 GitHub Actions runner/billing blocker，不能声明 remote CI green；Batch 535 已归档到 `docs/batch-history.md`。

### Next candidates

1. **Lane/tool-adapter live validation residuals（如仍有缺口）**：Batch 421/424/432 已覆盖 adapter contract/validation/report action queue、follow-through 与 contract liveValidation text，Batch 458-462 已补齐 authorizedExecutionFollowThrough / evidence review follow-through text，Batch 504 已补齐 recorded adapter sidecar path、actualBudget 与 adapter provenance，Batch 518 已补齐 contract/validation compact `reportSummary`；后续仅在 Windows 本机 product-path 仍存在 validation/record/evidence review handoff 到 replacement executor 的真实断点时推进；不新增 adapter/heavy-tool execution。
2. **Reviewer orchestration E2E residuals（如仍有缺口）**：Batch 489/499/505/514-516/523-525/530-532 已覆盖 reviewer intake product path、reviewer dispatch/intake downstream/durable/progress handoff、reviewer writeback identity、reviewer result provenance summary、reviewer intake terminal summary/postValidation summary 的 compact reviewer writeback provenance、blocked reviewer intake repair guidance compact summary，以及 reviewer intake terminal compact orchestration progress；后续仅在 multi-reviewer 接续仍要求解析 nested JSON、打开 reviewer result artifact 或手工拼 route output 才能接续时推进，不自动 spawn reviewer、不执行 heavy-tool。
3. **Pack-memory downstream UX residuals（如仍有缺口）**：Batch 500 已把 candidate decision/cleanup/reconsume expected evidence 收口为 `reviewArtifacts[]`，Batch 501 已把 open candidate residue 投影到 release/status handoff，Batch 502/503 已补齐 case-local status/default path、candidate identity、index mapping 与 derived review artifact visibility，Batch 517/522/526 已补齐 compact review summary、proof presence 与 proof stage/next-missing handoff；后续仅在 accepted/rejected 人工流程、cleanup/reconsume 或 evidence review downstream UX 仍需跨 envelope 手工拼接时推进，不重复做字段微批次。
4. **Cross-platform product-path E2E（降优先级）**：在本地 CLI/case E2E 已覆盖 nested cwd / case shim 的基础上，仅保持可在 runner 可用时执行的三平台 matrix 候选和 known gap 记录；不要在 GitHub runner/billing blocker 未解除前让它阻塞 Windows 本机迭代。
5. **Retained public façade decision**：只有真实 release-gate-green、public references、case shim、smoke retirement 与恢复计划均满足后，才执行独立 removal batch；否则明确保留期限和 blocker。

### Escalation / stopping conditions

产品方向变化、runtime/policy durable schema 迁移、confirmed/authority 策略变化、未授权外部副作用、公共入口删除门禁不完整或真实 release gate 无法验证时升级。完成单个 batch、inventory ready、push 成功或工作树干净都不是长期 goal 完成。

## 验证标准

每个 active batch 记录实际执行过的命令及结果；`release-check`/`ciReleaseGate.ready` 只算 inventory readiness，不能替代本地命令执行或远程 job conclusions。优先保持 coherent vertical slice，不用逐字段 metadata batch 维持连续推进。

Batch 推送节奏默认收敛为最多两次 push：先用 implementation commit 覆盖代码、测试、文档与本地验证，再用 release inspection commit 只记录 implementation commit 的远程 run。不要继续为 release inspection commit 自己触发的 CI 追加第三个记录提交；除非出现不同于既有 `steps=[]` runner/billing blocker 的新远程信号，否则保持该 blocker 为已记录 known gap。

## 风险与注意事项

- `docs/batch-plan.md` 是 active/next 的 durable source，不只是一份已完成批次日志。
- `docs/batch-history.md` 是历史归档；不要把它重新并回 `docs/batch-plan.md`，也不要在默认 handoff/read-first 中要求全文读取。
- `CHANGELOG.md` 记录必要的用户可见变化和边界；逐步 plumbing 留在 batch history。
- 只有当前用户 goal/session 明确授权时才 commit/push 指定分支。

## 历史批次归档

完整历史已拆到 `docs/batch-history.md`。除非要查旧 batch 细节、验证历史决策或做 release/debug 溯源，不要默认读取历史归档全文。
