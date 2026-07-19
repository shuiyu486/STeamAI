# Batch implementation plan

## 读取指南

先读本节的 current milestone / current batch state / next candidates；旧批次只在需要考古、验证历史决策或 release/debug 溯源时按 Batch ID 搜索 `docs/batch-history.md`。不要默认从 Batch 0 顺序重读 350+ 个历史批次。产品方向以 `docs/mission-control-product-direction.md` 为准，持续执行方式见 `docs/autonomous-goal.md`。

## 实施摘要

Batch 359 后，Go-owned/no-fallback public command surface、durable lanes、显式 reconcile、typed autonomy preflight、Mission brief / executor action、bounded reviewer dispatch → strict intake → verification-before-decision writeback → post-validation 的本机闭环、pack-memory promote/reconsume package E2E，以及 authorized execution observation evidence + bounded adapter execution report strict intake（含 authorized stopCondition boundaryHits、status summary enforcement、workspace-relative 与 case-relative machine-readable handoff）已形成底座。当前阶段继续从 contract/inventory field increments 转向 replaceable session executor / reviewer orchestration、lane executor / tool-adapter live validation、pack-memory product UX、Windows 本机 product-path 稳定与真实 release verification。

## 执行清单

### Current milestone

**Mission Commander operational closure and truthful release readiness**：把 durable lane/reviewer/autonomy contract 串成实际可运行、可跨会话接手、可验证的产品闭环，并区分 inventory ready、本地 gate executed 与远程 CI green。当前用户短期只要求 Windows 本机稳定可用；远程 Linux/macOS/Windows CI 因 runner/billing blocker 继续记录为 known gap，不阻塞本机 Mission Control 闭环。

### Current batch state

### Batch 396：Authorized execution evidence handoff consumption follow-through

状态：已完成本地实现与验证、提交/推送与远程 release-gate inspection。

目标：把 `gate -Apply -GateEventId ...` 已记录的 bounded observation evidence 接续到 project/lane handoff、lane `RESUME.md` 与 typed checkpoint，让替换 executor 不必回扫 observations/request ledger 就能看到哪些 authorized execution evidence 需要 review output/evidence refs、哪些 escalation/boundary-hit 需要 main review。

边界：只投影已存在 observation/request facts 的 review queue；`handoff -Apply` 与 resume refresh 仍只写 case-local handoff/resume/checkpoint files，不新增 observation/request/authority/confirmed，不执行 heavy-tool、不 replay adapter、不运行 continue、不创建 board/facts/lane；不新增 PowerShell runtime logic、不改变 sync/promote review-first、case durable schema、公共 façade 删除门禁或远程 CI blocker 状态。

已完成内容：

- `HandoffResult` JSON 与 lane checkpoint 新增 `executionEvidenceReview[]`，从 preauthorized authorized-gate execution observation 中投影 eventId、gateEventId、status、action、target、outputRefs/evidenceRefs、boundaryHits/escalation、review command、handoff command 与 boundary。
- project handoff 逐 lane 文本新增 execution evidence review 行；lane handoff 新增 `## execution evidence review` section；lane `RESUME.md` 的 Heavy-action gate decisions 后新增 execution evidence review checklist。
- review queue 明确 observation evidence 已记录、不得 replay heavy tool、review refs before authority/confirmed、no authority/confirmed；boundary-hit/escalated/escalation evidence 额外标记 main review before autonomous continuation。
- CLI/workstream coverage 锁定 succeeded execution evidence 与 adapter escalated evidence 在 project/lane handoff JSON/text、RESUME.md 和 checkpoint 中的一致投影。

验证结果：已通过 focused `go test ./internal/rekit/workstream ./internal/rekit/cli -run 'TestLaneCheckpointJSONContract|TestRunGoGateApplyAppendsAuthorizedGateRequestVisibility|TestRunHandoffApplyWritesProjectAndLane' -count=1`、`go test ./...`、`go vet ./...`、`go run ./cmd/rekit -- -Command release-check -Format json`、`go run ./cmd/rekit -- -Command status`、`go run ./cmd/rekit -- -Command packs`、`go run ./cmd/rekit -- -Command doctor` 与 `git diff --check`。`git diff --check` 仅报告 Windows LF/CRLF conversion warning，无 whitespace error。已提交并推送 `3f31ee7 Add execution evidence review handoffs`；远程 release-gate run `29691207309` 为 completed failure，Linux/macOS/Windows jobs 均 failure 且 `steps: []`，仍是既有 GitHub Actions runner/billing blocker，不能声明远程 CI green。

上一批摘要：Batch 395 已完成 Mission Commander lane handoff consumption closure；project/lane handoff 与 lane `RESUME.md` 已投影 commander primary/follow-up/boundary，详见 `docs/batch-history.md`。

### Next candidates

1. **Authorized execution evidence handoff consumption follow-through**：围绕新 `missionCommanderAction` 在 lane handoff / resume / checkpoint 中的 consumption，补齐替换 executor 从 authorized-gate 到 evidence review 的可复制闭环。
2. **Pack-memory cleanup / reconsume command execution UX**：围绕 accepted/rejected candidate 决策后的 cleanup target 与 fresh/attached case verification，继续收口为主 Agent 可执行的 bounded checklist。
3. **Mission Commander overview action consumption closure**：继续让 overview JSON/text 的 lane action index 和 commander action 可被替换 executor 直接消费，优先选择 Windows 本机 product-path coverage。
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
