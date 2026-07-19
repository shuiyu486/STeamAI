# Batch implementation plan

## 读取指南

先读本节的 current milestone / current batch state / next candidates；旧批次只在需要考古、验证历史决策或 release/debug 溯源时按 Batch ID 搜索 `docs/batch-history.md`。不要默认从 Batch 0 顺序重读 350+ 个历史批次。产品方向以 `docs/mission-control-product-direction.md` 为准，持续执行方式见 `docs/autonomous-goal.md`。

## 实施摘要

Batch 359 后，Go-owned/no-fallback public command surface、durable lanes、显式 reconcile、typed autonomy preflight、Mission brief / executor action、bounded reviewer dispatch → strict intake → verification-before-decision writeback → post-validation 的本机闭环、pack-memory promote/reconsume package E2E，以及 authorized execution observation evidence + bounded adapter execution report strict intake（含 authorized stopCondition boundaryHits、status summary enforcement、workspace-relative 与 case-relative machine-readable handoff）已形成底座。当前阶段继续从 contract/inventory field increments 转向 replaceable session executor / reviewer orchestration、lane executor / tool-adapter live validation、pack-memory product UX、Windows 本机 product-path 稳定与真实 release verification。

## 执行清单

### Current milestone

**Mission Commander operational closure and truthful release readiness**：把 durable lane/reviewer/autonomy contract 串成实际可运行、可跨会话接手、可验证的产品闭环，并区分 inventory ready、本地 gate executed 与远程 CI green。当前用户短期只要求 Windows 本机稳定可用；远程 Linux/macOS/Windows CI 因 runner/billing blocker 继续记录为 known gap，不阻塞本机 Mission Control 闭环。

### Current batch state

### Batch 392：Lane/tool-adapter live validation UX closure

状态：已完成本地实现与验证；提交/推送在本批收尾执行。

目标：把 authorized-gate → adapter execution report contract → read-only validation → record observation evidence 的主 Agent 接手路径继续收口为结构化 `missionCommanderAction`，让 lane executor / tool adapter 不必从 `liveValidation`、repair hints 与 next steps 手工拼命令或推断边界。

边界：`missionCommanderAction` 只投影 guidance，不执行 heavy-tool、不写 observations/authority/confirmed；`gate -ValidateExecutionReport` 仍 read-only；`gate -Apply -GateEventId ... -ExecutionReportPath ...` 仍只在 valid sidecar 后记录 bounded observation evidence，不写 authority/confirmed；不新增 PowerShell runtime logic、不改变 sync/promote review-first、case durable schema、公共 façade 删除门禁或远程 CI blocker 状态。

已完成内容：

- `AdapterExecutionReportContract` 与 `AdapterExecutionReportValidation` 新增 `missionCommanderAction`，复用 Mission Commander state/prompt/primary/follow-up/boundary 形状。
- contract 阶段投影 `needs-adapter-report-validation`，primary command 是 concrete read-only validation，follow-up 包含 valid=true 后的 concrete record evidence command 与 lane handoff，boundary 明确 validation read-only、record 只写 bounded observation evidence、`/rekit` never executes heavy tool、no authority/confirmed。
- validation 阶段按结果投影：valid sidecar → `ready-to-record-evidence`；缺 `-ExecutionReportPath` → `needs-execution-report-path`；一般 repair hint → `repair-adapter-report`；需要 main escalation 的 repair hint → `needs-main-escalation`。
- `NextSteps` 与 `missionCommanderAction` 同步使用 case-relative report path，保留 `<executor-id>` placeholder replacement 与 valid=true-before-record 语义。
- package 与 CLI coverage 锁定 contract handoff、valid preflight record handoff、missing path repair、invalid sidecar repair、boundary escalation state、nested workspace JSON product path，以及 no-observations/no-authority/no-confirmed invariants。

验证结果：已通过 focused `go test ./internal/rekit/gate -count=1`、`go test ./internal/rekit/cli -run 'TestRunGate|TestRunNestedWorkspaceAdapterReportProductPath|TestRunGenericBinaryReAdapterLiveValidationProductPath' -count=1`、`go test ./...`、`go vet ./...`、`go run ./cmd/rekit -- -Command release-check -Format json`、`go run ./cmd/rekit -- -Command status`、`go run ./cmd/rekit -- -Command packs`、`go run ./cmd/rekit -- -Command doctor` 与 `git diff --check`。`git diff --check` 仅报告 Windows LF/CRLF conversion warning，无 whitespace error。远程 release-gate 仍按既有 GitHub runner/billing known gap 处理，不能把 inventory readiness 声明为远程 CI green。

上一批摘要：Batch 391 已完成 Mission Commander Windows product UX closure；lane executor action 现在投影 `missionCommanderAction` state/prompt/primary/follow-up/boundary，详见 `docs/batch-history.md`。

### Next candidates

1. **Authorized execution evidence Mission Commander closure**：把 authorized-gate → execution report contract → validate → record evidence 的下一步提示继续压缩为主 Agent 可直接执行/交接的 bounded checklist，同时保持 no-heavy-tool/no-authority boundary。
2. **Pack-memory reconsume / tooling candidate follow-through**：围绕真实 pack tooling candidate 被人工合入后的 fresh/attached case reconsume，把 review plan 与后续 case consumption 的 Mission Commander提示继续做成可验证 product path。
3. **Mission Commander lane handoff consumption closure**：继续让 overview/handoff/continue/gate/start 输出的 action guidance 在替换 executor 或新会话中可直接消费，优先选择能用 Windows 本机 product path 验证的闭环。
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
