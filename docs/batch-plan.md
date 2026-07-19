# Batch implementation plan

## 读取指南

先读本节的 current milestone / current batch state / next candidates；旧批次只在需要考古、验证历史决策或 release/debug 溯源时按 Batch ID 搜索 `docs/batch-history.md`。不要默认从 Batch 0 顺序重读 350+ 个历史批次。产品方向以 `docs/mission-control-product-direction.md` 为准，持续执行方式见 `docs/autonomous-goal.md`。

## 实施摘要

Batch 359 后，Go-owned/no-fallback public command surface、durable lanes、显式 reconcile、typed autonomy preflight、Mission brief / executor action、bounded reviewer dispatch → strict intake → verification-before-decision writeback → post-validation 的本机闭环、pack-memory promote/reconsume package E2E，以及 authorized execution observation evidence + bounded adapter execution report strict intake（含 authorized stopCondition boundaryHits、status summary enforcement、workspace-relative 与 case-relative machine-readable handoff）已形成底座。当前阶段继续从 contract/inventory field increments 转向 replaceable session executor / reviewer orchestration、lane executor / tool-adapter live validation、pack-memory product UX、Windows 本机 product-path 稳定与真实 release verification。

## 执行清单

### Current milestone

**Mission Commander operational closure and truthful release readiness**：把 durable lane/reviewer/autonomy contract 串成实际可运行、可跨会话接手、可验证的产品闭环，并区分 inventory ready、本地 gate executed 与远程 CI green。当前用户短期只要求 Windows 本机稳定可用；远程 Linux/macOS/Windows CI 因 runner/billing blocker 继续记录为 known gap，不阻塞本机 Mission Control 闭环。

### Current batch state

### Batch 402：Execution evidence review handoff text suppression closure

状态：已完成本地实现与验证；提交/推送与远程 release-gate inspection 待收尾记录。

目标：Batch 401 已让 overview 与 project/lane handoff JSON `nextSteps[]` 抑制 escalated evidence 场景下的 autonomous continue；本批补齐人类/替换 executor 最常读的 Markdown 层，避免 project handoff 逐 lane 文本和 lane handoff 新会话开场仍把 ready lane continue 显示为当前动作。

边界：只增强已记录 observation evidence 的只读 handoff 文本消费顺序；不 replay heavy-tool、不运行 adapter、不执行 continue、不新增 observation/request、不写 authority/confirmed；不新增 PowerShell runtime logic、不改变 sync/promote review-first、case durable schema、公共 façade 删除门禁或远程 CI blocker 状态。

已完成内容：

- project handoff 逐 lane 行在 execution evidence review queue 存在时先输出 `evidence next action`，复用 Batch 401 的 review / handoff / main-review stop guidance。
- 当 review queue 含 boundary-hit/escalated/escalation evidence 时，project handoff 不再把 `next action：/rekit continue <lane>` / `continue command` 展示为当前动作，而改为 `evidence review 后继续候选`。
- lane handoff 新会话开场在 ready lane + escalated evidence 时要求先 review execution evidence、通知 main Agent，并明确当前不要执行 `/rekit continue <lane>`。
- CLI coverage 锁定 project handoff Markdown 和 lane handoff Markdown 的 evidence-first text consumption，同时保留 nested evidence item 的 commander state/primary/follow-up/boundary 投影。

验证结果：已通过 focused `go test ./internal/rekit/workstream ./internal/rekit/cli -run "TestRunGoGateApplyAppendsAuthorizedGateRequestVisibility|TestRunHandoffApplyWritesProjectAndLane" -count=1`、`go test ./...`、`go vet ./...`、`go run ./cmd/rekit -- -Command release-check -Format json`、`go run ./cmd/rekit -- -Command status`、`go run ./cmd/rekit -- -Command packs`、`go run ./cmd/rekit -- -Command doctor` 与 `git diff --check`。`release-check` 汇总 ready=true、ciReady=true、warnings=0、errors=0、summary=release gate inventory ok；`git diff --check` 仅报告 Windows LF/CRLF conversion warning，无 whitespace error。提交/推送与远程 release-gate inspection 待记录。

上一批摘要：Batch 401 已完成 execution evidence review next steps consumption follow-through；overview 与 project/lane handoff JSON `nextSteps[]` 已提升 evidence review guidance，详见 `docs/batch-history.md`。

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
