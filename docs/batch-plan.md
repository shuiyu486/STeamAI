# Batch implementation plan

## 读取指南

先读本节的 current milestone / current batch state / next candidates；旧批次只在需要考古、验证历史决策或 release/debug 溯源时按 Batch ID 搜索 `docs/batch-history.md`。不要默认从 Batch 0 顺序重读 350+ 个历史批次。产品方向以 `docs/mission-control-product-direction.md` 为准，持续执行方式见 `docs/autonomous-goal.md`。

## 实施摘要

Batch 359 后，Go-owned/no-fallback public command surface、durable lanes、显式 reconcile、typed autonomy preflight、Mission brief / executor action、bounded reviewer dispatch → strict intake → verification-before-decision writeback → post-validation 的本机闭环、pack-memory promote/reconsume package E2E，以及 authorized execution observation evidence + bounded adapter execution report strict intake（含 authorized stopCondition boundaryHits、status summary enforcement、workspace-relative 与 case-relative machine-readable handoff）已形成底座。当前阶段继续从 contract/inventory field increments 转向 replaceable session executor / reviewer orchestration、lane executor / tool-adapter live validation、pack-memory product UX、Windows 本机 product-path 稳定与真实 release verification。

## 执行清单

### Current milestone

**Mission Commander operational closure and truthful release readiness**：把 durable lane/reviewer/autonomy contract 串成实际可运行、可跨会话接手、可验证的产品闭环，并区分 inventory ready、本地 gate executed 与远程 CI green。当前用户短期只要求 Windows 本机稳定可用；远程 Linux/macOS/Windows CI 因 runner/billing blocker 继续记录为 known gap，不阻塞本机 Mission Control 闭环。

### Current batch state

### Batch 540：release handoff current-batch truthfulness repair

状态：已完成 implementation、focused/package validation、完整本地 release minimum、implementation commit/push 与远程 release-gate inspection；implementation commit `74327c3 Fix current batch release handoff` 已推送到 `main`。远程 release-gate run `29946738172` completed failure，Linux/macOS/Windows jobs 均 completed failure 且 `steps=[]`，仍是既有 GitHub Actions runner/billing blocker，不能声明 remote CI green。按 cadence 只记录 implementation commit 触发的远程 run；不为本 release inspection commit 自身触发的 CI 追加第三个记录提交，除非出现不同于既有 `steps=[]` blocker 的新信号。

目标：修复真实 product-path truthfulness 断点：Batch 539 已完成并发布后，`release-check` / kit `status.projectHandoff` 仍报告 Batch 537。根因是 latest-batch parser 遍历整个 active document 并选择最后一个历史 `### Batch` section；这会让 replacement executor/release maintainer看到错误的 goal、validation、run 和 cadence evidence。

已完成内容：

- `latestBatchSummary` 现在选择 `docs/batch-plan.md` 中第一个 `### Batch` section，即 `Current batch state` 的 active batch，不再被后续历史 section 覆盖。
- `releaseCheckReady` 解析同时识别 `release-check ready=true` 与当前文档使用的 ``release-check -Format json`（ready=true）` 表述。
- regression fixture 包含 current Batch 539 与 previous Batch 538，锁定 current title/ID/goal/validation、local/release-check readiness 与 remote `steps=[]` gate；真实 `status -Format text` 已确认 latestBatch=Batch 539、releaseCheckReady=true、run=29945764199。

边界：本批只修复 repo-local release handoff 文档解析与只读 status/release-check truthfulness；不执行远程 CI、不改变 release policy、case runtime、heavy-tool、authority/confirmed、PowerShell façade 或 public removal gate。

验证结果：已通过 `gofmt -w internal/rekit/releasecheck/release_handoff.go internal/rekit/releasecheck/release_handoff_test.go internal/rekit/cli/cli_test.go`、`go test ./internal/rekit/releasecheck ./internal/rekit/cli -count=1`，以及 `go run ./cmd/rekit -- -Command status -Format text` 真实 product-path 检查；完整本地 release minimum 已通过 `go run ./cmd/rekit -- -Command release-check -Format json`（ready=true / release gate inventory ok）、`go run ./cmd/rekit -- -Command status -Format text`、`go run ./cmd/rekit -- -Command packs -Format text`、`go run ./cmd/rekit -- -Command doctor -Format text`、`go test ./...`、`go vet ./...`、`git diff --check`（仅 LF→CRLF working-copy warning，无 whitespace error）。implementation commit `74327c3 Fix current batch release handoff` 已推送；远程 release-gate run `29946738172` 已检查，Linux/macOS/Windows jobs 均 completed failure 且 `steps=[]`，仍符合既有 runner/billing blocker，不能声明 remote CI green。

上一批摘要：Batch 539 已完成 multi-reviewer ready-result batch intake，implementation commit `e368a0d Add multi-reviewer batch intake` 与 release inspection commit `226a2c3 Record Batch 539 release gate inspection` 已推送；implementation run `29945764199` completed failure，Linux/macOS/Windows jobs 均 `steps=[]`，仍是既有 runner/billing blocker，不能声明 remote CI green。

### Batch 539：multi-reviewer ready-result batch intake vertical slice

状态：已完成 implementation、focused/package 验证、完整本地 release minimum、implementation commit/push 与远程 release-gate inspection；implementation commit `e368a0d Add multi-reviewer batch intake` 已推送到 `main`。远程 release-gate run `29945764199` completed failure，Linux/macOS/Windows jobs 均 completed failure 且 `steps=[]`，仍是既有 GitHub Actions runner/billing blocker，不能声明 remote CI green。按 cadence 只记录 implementation commit 触发的远程 run；不为本 release inspection commit 自身触发的 CI 追加第三个记录提交，除非出现不同于既有 `steps=[]` blocker 的新信号。

目标：解决 multi-reviewer packet 的多个 result path 已到位后，Mission Commander 仍需逐 shard 手工拼接并重复执行 `-ReviewerResultPath ... -WhatIf/-Apply` 的 operational 断点。新增单一 Go-native case-local batch intake 入口，按 durable packet order 发现 ready results 并复用既有 strict single-result intake，而不是新增并行 writeback 实现或继续投影 summary 字段。

已完成内容：

- `plan-subagents -ReadyReviewerResults` 要求显式 `-PacketPath`、`-Lane`、`-Actor` 和 `-WhatIf` 或 `-Apply`；拒绝 planning scope flags，因为 intake scope 完全由 durable packet 决定；从 packet `shardHandoffs[]` 读取 reviewer result paths，只将存在、非目录、非 symlink 且非空的结果视为 ready，missing/empty 计入 waiting。
- ready shard 按 packet 顺序逐个复用 `IntakeReviewerResult`，并额外绑定当前 handoff expected shard，保留 packet/route/shard/items、owner binding、evidence、route output strict validation，以及每 shard verification-before-decision、post-validation 和 `already-complete` 幂等语义。
- batch 在首个 blocked、partial writeback、event-id collision、post-validation failure 或 strict intake error 处停止；strict error 返回非成功命令状态，同时 CLI 仍输出 recovery envelope；此前完整 shard 保留已完成写回，后续 shard 不处理。存在 waiting shard 时不会建议继续 lane。JSON/text 输出 totals、ready/waiting/processed/completed/alreadyComplete、stop shard/reason、逐 shard status/progress、next steps 与 no-spawn/no-heavy/no-authority boundary。
- package coverage 验证两 shard WhatIf no-write、Apply 写入两 verification + 两 decision、幂等 replay、path-to-shard binding、partial ready waiting guidance，以及第二 shard blocker 时第一 shard完整落账、第二 shard和后续不写；CLI nested cwd / no `-Target` / no `-Pack` product-path coverage验证 JSON/text preview、Apply、planning scope guard 与 malformed result strict error envelope。

边界：本批不自动 spawn、轮询或监控 reviewer，不创建 reviewer result，不绕过 strict single-result validation，不并发写 ledger；不写 authority/confirmed、不执行 heavy-tool、不新增 PowerShell runtime logic。最终 merge decision 仍由主 Agent拥有。

验证结果：已通过 `gofmt -w internal/rekit/subagents/intake.go internal/rekit/subagents/intake_test.go internal/rekit/cli/cli.go internal/rekit/cli/reviewer_intake_test.go`、focused `go test ./internal/rekit/subagents ./internal/rekit/cli -run "TestIntakeReadyReviewerResults|TestRunPlanSubagentsReadyReviewerResults|TestRunPlanSubagentsReviewerIntake" -count=1`、package `go test ./internal/rekit/subagents ./internal/rekit/cli -count=1`；完整本地 release minimum 已通过 `go run ./cmd/rekit -- -Command release-check -Format json`（ready=true / release gate inventory ok）、`go run ./cmd/rekit -- -Command status -Format text`、`go run ./cmd/rekit -- -Command packs -Format text`、`go run ./cmd/rekit -- -Command doctor -Format text`、`go test ./...`、`go vet ./...`、`git diff --check`（仅 LF→CRLF working-copy warning，无 whitespace error）。implementation commit `e368a0d Add multi-reviewer batch intake` 已推送；远程 release-gate run `29945764199` 已检查，Linux/macOS/Windows jobs 均 completed failure 且 `steps=[]`，仍符合既有 runner/billing blocker，不能声明 remote CI green。

上一批摘要：Batch 538 已完成 pack-memory durable candidate review workspace，implementation commit `a00c605 Add durable candidate review workspace` 与 release inspection commit `3ce9362 Record Batch 538 release gate inspection` 已推送；implementation run `29943533595` completed failure，Linux/macOS/Windows jobs 均 `steps=[]`，仍是既有 GitHub Actions runner/billing blocker，不能声明 remote CI green。

### Batch 538：pack-memory durable candidate review workspace vertical slice

状态：已完成 implementation、focused/package 验证、完整本地 release minimum、implementation commit/push 与远程 release-gate inspection；implementation commit `a00c605 Add durable candidate review workspace` 已推送到 `main`。远程 release-gate run `29943533595` completed failure，Linux/macOS/Windows jobs 均 completed failure 且 `steps=[]`，仍是既有 GitHub Actions runner/billing blocker，不能声明 remote CI green。按 cadence 只记录 implementation commit 触发的远程 run；不为本 release inspection commit 自身触发的 CI 追加第三个记录提交，除非出现不同于既有 `steps=[]` blocker 的新信号。

目标：解决 `promote -CreateCandidates` 生成 candidate reviewPlan 后，replacement executor 若需要 inspectable managed-doc diff、sanitized tooling preview 与 durable packet，必须额外重新运行 generic `promote -Review`，且该 generic packet 又不包含 candidate-specific decision/cleanup/reconsume plan 的 operational 断点。把 candidate generation/WhatIf、generic review inputs 与完整 candidate review plan 收口为同一次 case-local Go-native product-path 调用和一个跨会话可接手 workspace，而不是再增加一个 summary 字段。

已完成内容：

- `promote -CreateCandidates` 现在可与 `-Review`、`-ReviewOutputDir`、`-PacketPath`、`-DiffPath` 组合；CLI 在 candidate result 生成后写 durable candidate review workspace，并在 JSON `reviewWorkspace` / text 输出中返回 root、packet、summary 与 combined diff paths。
- workspace `packet.json` 封装完整 `candidateResult.reviewPlan` 和已写入 artifact paths 的 generic promote `reviewInput`，让 replacement executor 在一个 packet 中同时获得 candidate decisions/follow-through/proof handoff、managed-doc bounded diff 与 tooling sanitized preview。
- workspace `summary.md` 保持短执行区，直接给出 candidate totals、pending review/proof stage、packet/diff paths、接手 checklist、验证标准与 no-merge/no-cleanup/no-heavy/no-authority boundary。
- focused CLI coverage 验证 explicit workspace/packet/diff paths、WhatIf candidate no-write、managed-doc bounded diff、tooling sanitized preview 去 case path、packet/summary durable handoff与 text output；既有 nested cwd / no `-Target` / no `-Pack` pack-memory product-path smoke 已升级为 actual candidate generation + durable review workspace。

边界：本批只写 case-local review workspace 和既有 candidate roots；不自动 merge/cleanup candidate，不更新 pack source，不运行 doctor/init/reconsume，不创建 expected decision/cleanup/reconsume proof，不写 authority/confirmed、不执行 heavy-tool，不新增 PowerShell runtime logic。`-WhatIf` 仍不创建 candidate/index，但允许显式 `-Review` 写 review artifacts，这与既有 generic sync/promote review artifact 语义一致。

验证结果：已通过 `gofmt -w internal/rekit/promote/promote.go internal/rekit/cli/cli.go internal/rekit/cli/cli_test.go`、focused `go test ./internal/rekit/promote ./internal/rekit/cli -run "TestCreateCandidates|TestRunPromoteCreateCandidatesWritesDurableReviewWorkspace|TestRunPromoteCreateCandidatesCaseLocalProductPathUsesMetadataRuntime" -count=1`、package `go test ./internal/rekit/promote ./internal/rekit/cli -count=1`；完整本地 release minimum 已通过 `go run ./cmd/rekit -- -Command release-check -Format json`（ready=true / release gate inventory ok）、`go run ./cmd/rekit -- -Command status`、`go run ./cmd/rekit -- -Command packs`、`go run ./cmd/rekit -- -Command doctor`、`go test ./...`、`go vet ./...`、`git diff --check`（仅 LF→CRLF working-copy warning，无 whitespace error）。implementation commit `a00c605 Add durable candidate review workspace` 已推送；远程 release-gate run `29943533595` 已检查，Linux/macOS/Windows jobs 均 completed failure 且 `steps=[]`，仍符合既有 runner/billing blocker，不能声明 remote CI green。

上一批摘要：Batch 537 已完成 installed entrypoint product-path vertical slice closure，implementation commit `8e3c2fc Add installed entrypoint product slice` 与 release inspection commit `3b5205b Record Batch 537 release gate inspection` 已推送；implementation run `29938234343` completed failure，Linux/macOS/Windows jobs 均 `steps=[]`，仍是既有 GitHub Actions runner/billing blocker，不能声明 remote CI green。

### Batch 537：installed entrypoint product-path vertical slice closure

状态：已完成 runtime/CLI/test/docs implementation、focused/package validation、完整本地 release minimum、implementation commit/push 与远程 release-gate inspection；已提交并推送 implementation commit `8e3c2fc Add installed entrypoint product slice`。远程 release-gate run `29938234343` completed failure，Linux/macOS/Windows jobs 均 completed failure 且 `steps=[]`，仍是既有 GitHub Actions runner/billing blocker，不能声明 remote CI green。本批按 release inspection cadence 只记录 implementation commit 的远程 run；不再为 release inspection commit 自身触发的 CI 追加第三个记录提交，除非出现不同于既有 `steps=[]` blocker 的新信号。用户本轮要求完成本批次后停止，因此 Batch 537 完成并记录 release inspection 后不自动开启下一批。

目标：把 Windows 本机 installed case-local `/rekit` 第一屏、replacement executor takeover、reviewer dispatch/intake/writeback durable handoff 与 pack-memory candidate/proof handoff 串成同一个真实 product-path vertical slice。验证标准不是新增某个 JSON 字段，而是新会话在 case-local nested cwd 中只运行 `/rekit` / 默认 `status`，即可确认 pack 来源、thin shim readiness、installed shim 是否匹配 template、Mission Commander queue/current action、reviewer writeback / reviewer dispatch intake 状态、pack-memory candidate review/proof 下一步，以及可接手的 durable artifacts。

边界：只增强 case-local installed entrypoint 的只读 first-screen / durable handoff 与 CLI product-path coverage；case-local shim 仍是 metadata-only thin shim，回到 kit 仓库 canonical runtime；不复制 runtime logic，不新增 PowerShell runtime logic，不安装或修改用户级 `~/.claude/skills`，不展示 retained façade 或低层 `go run` / PowerShell 命令作为产品入口；reviewer intake 仍由主 Agent 显式 WhatIf → Apply，保持 verification-before-decision；pack-memory 仍只生成 candidates / proof guidance，不 merge、不 cleanup、不 reconsume、不运行 doctor/init、不执行 heavy-tool、不写 authority/confirmed。

已完成内容：

- `status.caseShim.entrypoint` 新增 installed case-local first-screen handoff：`caseLocalFirstScreenCommand=/rekit`、explicit fallback status command、installed/canonical shim path、metadata paths、durable artifacts、first-screen checks 与 thin-shim boundary；default/status text 同屏输出同一信息，且 `status case shim` 行禁止泄漏 retained façade / low-level backend details。
- case-local shim template 与 readiness guard 新增 first-screen / durable-artifact 接手短语，要求新会话先用 `/rekit` 确认 `status case shim ready=true` 与 `installedShimMatchesTemplate=true`，再按 Mission Commander queue/current action 与 durable artifacts 接手；shim drift 仍必须 repair preview-first。
- `TestRunInstalledCaseShimProductPathStatusAndRefresh` 从单一 shim-refresh smoke 扩展为真实 installed product-path vertical slice：`init -Apply` 后进入 nested case cwd，无 `-Target` / 无 `-Pack` 运行 status/start/plan-subagents/reviewer intake/continue/promote/status/doctor。
- 同一测试覆盖 replacement executor lane takeover、multi-reviewer planning、reviewer result WhatIf/Apply strict intake、reviewer writeback summary、remaining reviewer dispatch intake summary、lane `RESUME.md`、typed checkpoint 与 continue run `digest.md`，确保 durable artifacts 不需要重开 packet/result JSON 即可接手。
- 同一测试覆盖 pack-memory candidate generation 与 downstream status first screen：managed-doc candidate、blocked deny-pattern item、sanitized tooling candidate、open candidate inventory、review summary、proof summary 与 next missing proof 在 case-local default status 中可见；测试清理 pack candidate/tooling candidate residue，避免污染 pack source。

验证结果：已通过 `gofmt -w internal/rekit/cli/cli_test.go`、focused `go test ./internal/rekit/cli -run "TestRunInstalledCaseShimProductPathStatusAndRefresh" -count=1`、combined focused `go test ./internal/rekit/cli -run "TestRunInstalledCaseShimProductPathStatusAndRefresh|TestRunPlanSubagentsReviewerOrchestrationE2E" -count=1`、package `go test ./internal/rekit/caseshim ./internal/rekit/cli -count=1`；完整本地 release minimum 已通过 `go run ./cmd/rekit -- -Command release-check -Format json`（release-check ready=true / release gate inventory ok）、`go run ./cmd/rekit -- -Command status -Format text`、`go run ./cmd/rekit -- -Command packs -Format text`、`go run ./cmd/rekit -- -Command doctor -Format text`、`go test ./...`、`go vet ./...`、`git diff --check`（仅 LF→CRLF working-copy warning，无 whitespace error）。implementation commit `8e3c2fc Add installed entrypoint product slice` 已推送；远程 release-gate run `29938234343` 已检查，run completed failure，Linux/macOS/Windows jobs 均 completed failure 且 `steps=[]`。该远程失败符合既有 runner/billing blocker，不能声明 remote CI green；`release-check`/`ciReleaseGate.ready` 仍不能替代实际远程 job conclusion。

上一批摘要：Batch 536 已完成 execution evidence adapter context downstream handoff closure，implementation commit `6e2fb2a Add execution evidence adapter context handoff` 与 release inspection commit `e8f6a76 Record Batch 536 release gate inspection` 已推送；implementation run `29931195286` completed failure，Linux/macOS/Windows jobs 均 completed failure 且 `steps=[]`，仍是既有 GitHub Actions runner/billing blocker，不能声明 remote CI green；Batch 536 已归档到 `docs/batch-history.md`。

### Post-Batch 537：documentation routing / goal handoff cleanup

状态：已完成文档上下文治理收尾，commit `04ebab1 Improve documentation context routing handoff` 已推送到 `main`。该收尾不是新的 product batch；它响应用户对上下文污染、新会话交接、按需路由/渐进披露和 goal 语句过长的反馈，补齐 durable docs、reference、配置说明与示例入口的路由边界，并将 `docs/autonomous-goal.md` 中给新会话的接手语句和 goal 语句压缩为更短的中大型 vertical slice 导向。

补充纠偏：用户指出近期多个 batch 连续把某个字段、summary、handoff detail 或 text line 从 A 投影到 B，单批合理但连续会退化为内部 contract 可见性微调。已将该反模式固化到根 `CLAUDE.md`、`docs/context-routing.md` 与 `docs/autonomous-goal.md`：后续先找用户 / Mission Commander / replacement executor / reviewer / lane executor / pack-memory 的真实 operational 断点，再把字段或文本作为中大型 vertical slice 的支撑；不要为了继续推进而不断寻找下一个 `latestX` / `summaryX` / `contextX`。

交接判断：当前适合切到新会话继续。新会话先按 `docs/context-routing.md` 读取最小上下文，确认本文件 current state、`CHANGELOG.md` 顶部与真实 git/验证状态；正式 goal 使用 `docs/autonomous-goal.md` 顶部给出的短 goal 语句。下一轮若继续长期推进，应从下面 Next candidates 中选择中大型 product-path vertical slice，不再把本轮文档收尾延伸成连续文档微批次。

验证结果：已通过 `go run ./cmd/rekit -- -Command release-check -Format json`、`go run ./cmd/rekit -- -Command status -Format text`、`go run ./cmd/rekit -- -Command doctor -Format text`、`go run ./cmd/rekit -- -Command packs -Format text`、`go test ./...`、`go vet ./...`、`git diff --check`（仅 LF→CRLF working-copy warning，无 whitespace error）。

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
