# Batch implementation plan

## 读取指南

先读本节的 current milestone / current batch state / next candidates；旧批次只在需要考古、验证历史决策或 release/debug 溯源时按 Batch ID 搜索 `docs/batch-history.md`。不要默认从 Batch 0 顺序重读 350+ 个历史批次。产品方向以 `docs/mission-control-product-direction.md` 为准，持续执行方式见 `docs/autonomous-goal.md`。

## 实施摘要

Batch 359 后，Go-owned/no-fallback public command surface、durable lanes、显式 reconcile、typed autonomy preflight、Mission brief / executor action、bounded reviewer dispatch → strict intake → verification-before-decision writeback → post-validation 的本机闭环、pack-memory promote/reconsume package E2E，以及 authorized execution observation evidence + bounded adapter execution report strict intake（含 authorized stopCondition boundaryHits、status summary enforcement、workspace-relative 与 case-relative machine-readable handoff）已形成底座。当前阶段继续从 contract/inventory field increments 转向 replaceable session executor / reviewer orchestration、lane executor / tool-adapter live validation、pack-memory product UX、Windows 本机 product-path 稳定与真实 release verification。

## 执行清单

### Current milestone

**Mission Commander operational closure and truthful release readiness**：把 durable lane/reviewer/autonomy contract 串成实际可运行、可跨会话接手、可验证的产品闭环，并区分 inventory ready、本地 gate executed 与远程 CI green。当前用户短期只要求 Windows 本机稳定可用；远程 Linux/macOS/Windows CI 因 runner/billing blocker 继续记录为 known gap，不阻塞本机 Mission Control 闭环。

### Current batch state

### Batch 629：pack-memory first-screen proof command closure

状态：已完成 runtime/test/doc 工作树实现、focused pack-memory first-screen evidence / installed case-local product-path 回归、CLI package validation、完整本机 `release-run` release minimum；implementation commit/push 与 push-triggered remote release-gate inspection 待执行。

目标：补齐 Batch 614/608/576 后的 pack-memory operational closure 断点：first-screen 已能显示 pack-memory counts、proof progress、next missing proof type/candidate/target，lower `status pack-memory next missing proof` 与 Mission Commander current action 也已经携带 packet-derived `DraftCommand` / `DraftApplyTemplate`。但 replacement executor 在默认 `/rekit` 第一屏仍需要继续下翻完整 pack-memory handoff 才能复制 proof draft WhatIf 与 `ExpectedProofSha256` hash-bound Apply 模板。本批把 next-missing-proof 的 proof draft/apply/boundary 直接提升到 first-screen evidence shortlist。

已实现内容：

- `statusMissionCommanderFirstScreenPackMemoryEvidence` 在 `ProofSummary.NextMissingProof` 已提供 `DraftCommand` / `DraftApplyTemplate` 时，新增只读 evidence 行：`next missing proof draft WhatIf`、`next missing proof apply template` 与 proof Apply boundary。
- pack-memory first-screen evidence shortlist 上限从 6 调整为 9，确保新增 proof 操作证据不会挤掉原有 high-value inventory evidence。
- Focused unit test 和 installed case-local product-path test 现在同时断言 first-screen 能看到 concrete `/rekit promote -PacketPath ... -DraftReviewProof -WhatIf -Format json`、`-ExpectedProofSha256 <proofSha256-from-WhatIf> -Apply -Format json` 与 status/release read-only boundary。

边界：本批只增强 status/default `/rekit` first-screen 的只读 text projection；不生成 proof、不运行 `promote -DraftReviewProof`、不写 candidate decision/proof/cleanup/verification、不 merge/retire/reconsume pack-memory candidates、不写 facts/ledger/authority/confirmed、不执行 heavy tool、不新增 PowerShell runtime logic。

验证结果：focused `go test ./internal/rekit/cli -run "TestStatusMissionCommanderFirstScreenPackMemoryEvidenceKeepsHighValueHead|TestRunStatusInstalledEntrypointFirstScreen" -count=1` 已通过；focused status/product-path regression `go test ./internal/rekit/cli -run "TestRunStatusJsonKit|TestRunStatusKitShowsOpenPackMemoryCandidates|TestStatusMissionCommanderFirstScreenPackMemoryEvidenceKeepsHighValueHead|TestRunInstalledCaseShimProductPathStatusAndRefresh" -count=1` 已通过；CLI package validation `go test ./internal/rekit/cli -count=1` 已通过；完整本机 `go run ./cmd/rekit -- -Command release-run -Format text` 已通过，返回 `ready=true` / `summary=release run ok`，聚合执行 `release-check`、`status`、`packs`、`doctor`、`go test ./...`、`go vet ./...`、`git diff --check` 7 步，`passed=7 failed=0 skipped=0`；本次 `go test ./...` step 为 `attempts=1`，未触发 cleanup-lock retry；`git diff --check` 仅保留 Windows 工作树 LF→CRLF 提示。implementation commit/push 与 push-triggered remote release-gate inspection 待执行。

下一批硬约束：Batch 630 不能继续做字段、summary、text、first-screen 或 handoff 可见性小修小补；必须选择端到端能力闭环，至少覆盖一个可运行 runtime 行为或写入前后状态转换，并用 CLI/product-path/E2E 测试证明 Mission Commander / replacement executor 能完成真实下一步。

### Batch 628：reviewer invalid packet retirement handoff closure

状态：已完成 runtime/test/doc 工作树实现、focused reviewer invalid packet / retirement CLI 回归、相关 package validation、完整本机 `release-run` release minimum、implementation commit/push 与 push-triggered remote release-gate inspection；implementation commit `dffa50d` 已推送。Push run `30210543983` completed failure，macOS/Linux/Windows jobs `89815909876`/`89815909913`/`89815909917` 均 `steps=[]` 且无 logs，仍属既有 runner/billing blocker；不为 release inspection record 自身追加第三个 inspection。

目标：补齐 Batch 627 后的 reviewer invalid packet operational closure 断点：status/intake 已能在 `packet.integrity.json` 可读而 packet 本体 unreadable/malformed 时保留 lane provenance 和具体 decode evidence；仓库也已有 `plan-subagents -RetireInvalidReviewerPacket` hash-bound retirement runtime。但 replacement executor 从 `status` / `handoff` / `continue` 看到 `reviewer-packet-integrity-invalid` 后，仍只能看到 regenerate guidance，无法直接接续“如果该 invalid packet 已废弃，先跑 retirement WhatIf → 使用返回的 expected hashes Apply”闭环。本批把 invalid packet handoff 直接连接到既有 retirement runtime 的公开 WhatIf 入口。

已实现内容：

- `ReviewerDispatchIntakeHandoff` 新增只读 `packetRetirementPreviewCommand`，`ReviewerDispatchIntakeSummary` 同步投影 `nextActionPacketRetirementPreviewCommand`；invalid packet 的 `nextAction` 现在优先指向 `/rekit plan-subagents -RetireInvalidReviewerPacket ... -WhatIf -Format json`。
- `reviewer-packet-integrity-invalid` item-level `runbookSteps[]` 现在明确分两路：obsolete invalid packet 先运行 retirement preview、复核 exact packet/integrity hashes、只使用 preview 返回的 hash-bound Apply；仍需 reviewer work 时再 regenerate canonical packet + sidecar。
- CLI text 的 reviewer dispatch next action 行新增 `packetRetirementPreview=`，并为每个 invalid packet item 打印 `reviewer packet retirement` preview 行；status/continue/handoff terminal 输出不再要求 replacement executor 手工拼 retirement command。
- Focused workstream 和 CLI product-path tests 覆盖 invalid packet status JSON summary/item 字段、status text retirement preview、runbook 与既有 WhatIf→hash-bound Apply retirement E2E。

边界：本批只增强 reviewer dispatch/intake/status/handoff 的只读 retirement handoff；不执行 retirement，不删除、修补或覆盖 packet/sidecar，不预填 expected hashes 到 status handoff，不 dispatch/collect/intake reviewer，不写 facts/ledger/authority/confirmed，不执行 heavy tool，不新增 PowerShell runtime logic。

验证结果：focused `go test ./internal/rekit/workstream -run "TestReviewerDispatchIntakeFailsClosedOnPacketIntegrityDrift|TestReviewerDispatchIntakeSummary" -count=1` 已通过；focused CLI `go test ./internal/rekit/cli -run "TestRunPlanSubagentsReviewerPacketRetirementWhatIfApplyE2E|TestParsePlanSubagentsReviewerPacketRetirement" -count=1` 已通过；相关 package validation `go test ./internal/rekit/workstream ./internal/rekit/cli -count=1` 已通过；完整本机 `go run ./cmd/rekit -- -Command release-run -Format text` 已通过，返回 `ready=true` / `summary=release run ok`，聚合执行 `release-check`、`status`、`packs`、`doctor`、`go test ./...`、`go vet ./...`、`git diff --check` 7 步，`passed=7 failed=0 skipped=0`；本次 `go test ./...` step 为 `attempts=1`，未触发 cleanup-lock retry；`git diff --check` 仅保留 Windows 工作树 LF→CRLF 提示。implementation commit `dffa50d` 已推送。Push-triggered release-gate run `30210543983` completed failure，macOS/Linux/Windows jobs `89815909876`/`89815909913`/`89815909917` 均为既有 `steps=[]` / runner/billing blocker，不能声明 remote CI green。

### Batch 627：reviewer packet integrity decode evidence closure

状态：已完成 runtime/test/doc 工作树实现、focused reviewer packet integrity 回归、workstream package validation、完整本机 `release-run` release minimum、implementation commit/push 与 push-triggered remote release-gate inspection；implementation commit `f2b9b05` 已推送。Push run `30209966418` completed failure，Linux/macOS/Windows jobs `89814417370`/`89814417385`/`89814417400` 均 `steps=[]` 且无 logs，仍属既有 runner/billing blocker；不为 release inspection record 自身追加第三个 inspection。

目标：补齐 reviewer dispatch intake 的可诊断 fail-closed 断点：Batch 578/579 之后 canonical reviewer packet 已有 `packet.integrity.json` sidecar 与 repair/retirement handoff，但当 packet 本体 unreadable 或 malformed、sidecar 仍能提供可信 lane provenance 时，downstream handoff 只给泛化 decode failed，replacement executor 无法区分 JSON 损坏、读取失败或路径/锁问题。本批在不修补 packet 的前提下，把具体 read/decode error 放入 `reviewer-packet-integrity-invalid` evidence，同时继续保留 integrity sidecar 的 target lane provenance。

已实现内容：

- `ReviewerDispatchIntakeHandoffs` 在 packet read/decode 失败且 `packet.integrity.json` 仍可读时，不再只返回泛化 decode failure；它会保留 sidecar 的 `packetId` / `targetLane`，并把 wrapped `read reviewer packet` 或 `decode reviewer packet JSON` 具体错误写入 fail-closed handoff evidence。
- 缺少 readable integrity sidecar 的 unreadable/malformed packet 仍按 legacy compatibility 静默跳过；lane filter 继续使用 sidecar `targetLane`，不会把其它 lane 的 invalid packet 投影到当前 lane。
- 现有 `reviewer-packet-integrity-invalid` Mission Commander action、boundary、CLI evidence text 与 retirement/repair guidance 继续复用同一 typed handoff；本批不新增 public command 或 packet schema。

边界：本批只增强 reviewer dispatch intake 的只读诊断 evidence；不修复、覆盖、删除或 retirement packet，不修改 `packet.integrity.json`，不 dispatch/collect/intake reviewer，不写 facts/ledger/authority/confirmed，不执行 heavy tool，不新增 PowerShell runtime logic。

验证结果：focused `go test ./internal/rekit/workstream -run "TestReviewerDispatchIntakeFailsClosedOnPacketIntegrityDrift" -count=1` 已通过；package validation `go test ./internal/rekit/workstream -count=1` 已通过；完整本机 `go run ./cmd/rekit -- -Command release-run -Format text` 已通过，返回 `ready=true` / `summary=release run ok`，聚合执行 `release-check`、`status`、`packs`、`doctor`、`go test ./...`、`go vet ./...`、`git diff --check` 7 步，`passed=7 failed=0 skipped=0`；本次 `go test ./...` step 为 `attempts=1`，未触发 cleanup-lock retry；`git diff --check` 仅保留 Windows 工作树 LF→CRLF 提示。implementation commit `f2b9b05` 已推送。Push-triggered release-gate run `30209966418` completed failure，Linux/macOS/Windows jobs `89814417370`/`89814417385`/`89814417400` 均为既有 `steps=[]` / runner/billing blocker，不能声明 remote CI green。

### Batch 626：release-run transient retry downstream truthfulness closure

状态：已完成 runtime/test/doc 工作树实现、focused release handoff/CLI 回归、releasecheck/CLI package validation、完整本机 `release-run` release minimum、implementation commit/push 与 push-triggered remote release-gate inspection；implementation commit `1a1f068` 已推送。Push run `30209344354` completed failure，macOS/Linux/Windows jobs `89812835298`/`89812835308`/`89812835325` 均 `steps=[]` / `runner_id=0` 且无 logs，仍属既有 runner/billing blocker；不为 release inspection record 自身追加第三个 inspection。

目标：补齐 Batch 625 后的下游 release readiness 断点：`release-run` 已能对 Windows Go test cleanup-lock transient 做一次可审计 retry，但 `release-check` / `status` 的 latest-batch handoff 仍只把成功聚合为 localValidationReady=true。若某批真正依赖 retry 才通过，replacement executor 需要在 release handoff/status 第一屏看到该 retry 证据；同时，当前 Batch 625 只写了“未触发 retry / attempts=1”，不能被宽松 cleanup-lock 文案误判为 transient retry。本批把 retry 审计作为 downstream truthfulness evidence/warning 接入，并保持 fail-closed ready 判定。

已实现内容：

- `ReleaseHandoffLatestBatchHandoff` 新增只读 `validationWarnings[]`；`statusProjectHandoff` 同步投影为 `latestValidationWarnings[]`。
- latest-batch parser 只在文本明确包含 `transientRetryReason`、`release-run step retry` 或 `attempts=2` 时识别 retry；普通 cleanup-lock 叙述、Batch 625 的 `attempts=1` 与“未触发 retry”不会误报。
- 已记录 retry 的 completed release-run 会保留 `localValidationReady=true` / `releaseCheckReady=true`，但 evidence 新增 `release-run transient retry recorded`，validation warning 提醒 review retry reason 与 first-attempt output；retry 后仍 `ready=false` / `failed=1` 的 release-run 保持 local validation not-ready，并继续指向重跑 full local release minimum。
- `release-check -Format text` 与 `status -Format text` 新增 latest batch validation warning 行；JSON 可供 automation 消费同一 warning。

边界：本批只增强 release readiness/status 的只读 truthfulness projection；不改变 `release-run` 执行/retry 语义，不改变 gateProfile/recommendedMinimum，不把 transient retry 当 remote green，不写 repo/case state，不联网读取 GitHub Actions，不写 authority/confirmed，不执行 heavy tool，不新增 PowerShell runtime logic。

验证结果：focused `go test ./internal/rekit/releasecheck ./internal/rekit/cli -run "TestLatestBatchHandoff(RecordsReleaseRunTransientRetryEvidence|DoesNotTreatFailedReleaseRunRetryAsReady|ExtractsValidationEvidence)|TestReleaseHandoffInventoryFromRepo|TestRunStatusJsonKit|TestRunReleaseCheckJsonInventory" -count=1` 已通过；package validation `go test ./internal/rekit/releasecheck ./internal/rekit/cli -count=1` 已通过；`go run ./cmd/rekit -- -Command status -Format text` 已确认当前 Batch 625 `attempts=1` 不再误报 `release-run transient retry recorded`。完整本机 `go run ./cmd/rekit -- -Command release-run -Format text` 已通过，返回 `ready=true` / `summary=release run ok`，聚合执行 `release-check`、`status`、`packs`、`doctor`、`go test ./...`、`go vet ./...`、`git diff --check` 7 步，`passed=7 failed=0 skipped=0`；本次 `go test ./...` step 为 `attempts=1`，未触发 cleanup-lock retry；`git diff --check` 仅保留 Windows 工作树 LF→CRLF 提示。implementation commit `1a1f068` 已推送。Push-triggered release-gate run `30209344354` completed failure，macOS/Linux/Windows jobs `89812835298`/`89812835308`/`89812835325` 均为既有 `steps=[]` / `runner_id=0` blocker，不能声明 remote CI green。

### Batch 625：release-run Windows Go test cleanup retry closure

状态：已完成 runtime/test/doc 工作树实现、focused release-run CLI 回归、完整本机 `release-run` release minimum、implementation commit/push 与 push-triggered remote release-gate inspection；implementation commit `db580f9` 已推送。Push run `30208604009` completed failure，Linux/macOS/Windows jobs `89810907570`/`89810907614`/`89810907617` 均 `steps=[]` / `runner_id=0` 且无 logs，仍属既有 runner/billing blocker；不为 release inspection record 自身追加第三个 inspection。

目标：补齐 Windows 本机 release verification 的真实稳定性断点：Batch 624 的首次完整 `release-run` 已看到 `go test ./...` 所有 package 输出 `ok`，但 Go 在清理临时 `*.test.exe` 时被 Windows 文件占用锁阻塞，导致整次本机 release minimum 误报 failed。维护者必须人工判断并重跑，容易把 transient cleanup lock 和真实测试失败混在一起。本批让 `release-run` 对该特定 cleanup-lock 形态做一次可审计重试，同时保持真实测试失败 fail-closed。

已实现内容：

- `releaseRunStepResult` 新增只读 retry metadata：`attempts`、`transientRetryReason`、`firstAttemptExitCode`、`firstAttemptError` 与 `firstAttemptOutputTail`，text 输出同步打印 `release-run step retry` 和 first-attempt tail 行。
- `go test ./...` step 首次失败时，只有输出/错误同时匹配 Windows `go: unlinkat ... .test.exe ... used by another process` cleanup-lock 形态，且没有 `FAIL` / setup failed / build failed / panic 等真实测试失败信号，才会重试一次；其它命令、普通 go test failure、build/setup failure 和 retry 后失败仍按原 release-run failure 语义记录。
- Tests 覆盖 cleanup lock first attempt → retry success、真实 go test failure + cleanup lock 不重试、普通 go test failure 不重试，以及既有 release-run step order / failure aggregation / release inspection handoff。

边界：本批只增强 Go-native `release-run` 的 Windows 本机验证稳定性与审计输出；不新增 public command，不改变 gateProfile/recommendedMinimum，不跳过任何 release step，不吞掉真实测试失败，不写 repo/case state，不联网读取 GitHub Actions，不写 authority/confirmed，不执行 heavy tool，不新增 PowerShell runtime logic。

验证结果：focused `go test ./internal/rekit/cli -run "TestRunReleaseRun" -count=1` 已通过。完整本机 `go run ./cmd/rekit -- -Command release-run -Format text` 已通过，返回 `ready=true` / `summary=release run ok`，聚合执行 `release-check`、`status`、`packs`、`doctor`、`go test ./...`、`go vet ./...`、`git diff --check` 7 步，`passed=7 failed=0 skipped=0`；本次 `go test ./...` step 为 `attempts=1`，未触发 cleanup-lock retry；`git diff --check` 仅保留 Windows 工作树 LF→CRLF 提示。implementation commit `db580f9` 已推送。Push-triggered release-gate run `30208604009` completed failure，Linux/macOS/Windows jobs `89810907570`/`89810907614`/`89810907617` 均为既有 `steps=[]` / `runner_id=0` blocker，不能声明 remote CI green。

### Batch 624：adapter execution report lifecycle runbook handoff closure

状态：已完成 runtime/test/doc 工作树实现、focused adapter gate/CLI 回归、完整本机 `release-run` release minimum、implementation commit/push 与 push-triggered remote release-gate inspection；implementation commit `c631cdc` 已推送。Push run `30207891944` completed failure，Linux/Windows/macOS jobs `89809035554`/`89809035577`/`89809035578` 均 `steps=[]` / `runner_id=0` 且无 logs，仍属既有 runner/billing blocker；不为 release inspection record 自身追加第三个 inspection。

目标：补齐 adapter execution report lifecycle 的真实接手断点：Batch 568/569/580/594/602/618 已让 sidecar scaffold/draft、read-only validation、hash-bound record currentness 与 action identity 形成闭环，但 replacement executor 执行 contract、scaffold、draft、validation、status handoff 或 record 后，仍需要从 `nextSteps[]`、Mission Commander command、path/hash 和 boundary 中手工拼接下一条 bounded operation。本批让每个 adapter report lifecycle envelope 自身携带可复制 runbook，直接说明确认当前 state/path/hash、先 validation、只使用 validation/status 返回的 expected hash record Apply、record 后转 evidence review 的顺序。

已实现内容：

- `AdapterExecutionReportContract`、`AdapterExecutionReportScaffold`、`AdapterExecutionReportDraft`、`AdapterReportLiveValidation`、`AdapterExecutionReportValidation` 与 execution evidence `ApplyResult` 新增只读 `runbookSteps[]`，由当前 lifecycle stage/state、report path、report SHA-256、Mission Commander command、next steps 与 boundary guard 派生并去重。
- Contract/live validation/scaffold/draft/validation/recorded-evidence snapshot/record result 均会输出 runbook；valid record-ready 分支明确要求替换 `<executor-id>` 并使用 validation/status envelope 中的 `-ExpectedExecutionReportSha256`，invalid/missing/malformed 分支保留 repair handoff 与 do-not-record boundary，record/duplicate 分支提示先 review outputRefs/evidenceRefs 再判断 authority/confirmed。
- CLI text 现在打印 `gate adapter report ... runbook` 与 `gate execution evidence runbook` 行；`status` 与 workstream authorized-gate live validation handoff JSON/text 复制同一 `runbookSteps[]`，让 replacement executor 从 default/status/overview/handoff/start/reconcile/continue 接手时不必回查完整 contract 或 sidecar。
- Focused tests 覆盖 contract、scaffold、draft、validation valid/invalid/missing、recorded evidence snapshot、bounded evidence record result、CLI text 输出与 project/lane handoff JSON runbook 投影。

边界：本批只增强 adapter execution report lifecycle 的只读 operational handoff 与测试；不新增 public command，不执行 adapter/heavy tool，不自动 validate/record，不放宽 expected-hash Apply currentness，不写 authority/confirmed，不新增 durable schema，不新增 PowerShell runtime logic。

验证结果：focused `go test ./internal/rekit/gate ./internal/rekit/cli` 已通过。首次完整 `go run ./cmd/rekit -- -Command release-run -Format text` 在 `go test ./...` 全部包输出 `ok` 后因 Windows 临时测试二进制清理失败（`go: unlinkat ... cli.test.exe: The process cannot access the file because it is being used by another process`）返回 6/7；随后单独 `go test ./...` 已通过，重跑完整本机 `go run ./cmd/rekit -- -Command release-run -Format text` 已通过，返回 `ready=true` / `summary=release run ok`，聚合执行 `release-check`、`status`、`packs`、`doctor`、`go test ./...`、`go vet ./...`、`git diff --check` 7 步，`passed=7 failed=0 skipped=0`；`git diff --check` 仅保留 Windows 工作树 LF→CRLF 提示。implementation commit `c631cdc` 已推送。Push-triggered release-gate run `30207891944` completed failure，Linux/Windows/macOS jobs `89809035554`/`89809035577`/`89809035578` 均为既有 `steps=[]` / `runner_id=0` blocker，不能声明 remote CI green。

### Batch 623：reviewer result collection recovery handoff closure

状态：已完成 runtime/test/doc 工作树实现、focused reviewer collection/recovery 回归、recovery disposition fail-closed 回归修正、完整本机 `release-run` release minimum、implementation commit/push 与 push-triggered remote release-gate inspection；implementation commit `84fe8af` 已推送。Push run `30205009789` completed failure，macOS/Windows/Linux jobs `89801445817`/`89801445818`/`89801445833` 均 `steps=[]` 且无 logs，仍属既有 runner/billing blocker；不为 release inspection record 自身追加第三个 inspection。

目标：补齐 reviewer result collection 冲突恢复的真实产品断点：Batch 549/558/600/622 已让 reviewer source capture、staging、collection 和 runbook 都可显式接手，但当 packet-derived candidate 已准备好、canonical reviewer result path 被不同 bytes 或可恢复 obstruction 占据时，`-CollectReviewerResult -WhatIf` 仍只返回普通 error（例如 refusing overwrite / non-empty regular file），replacement executor 需要人工知道要切到 `-RecoverReviewerResult -WhatIf`、补 reason、复核 hashes、Apply 后再重跑 collection。本批让 collection preview 自身在可恢复冲突时返回结构化 recovery handoff，同时保持 Apply fail-closed。

已实现内容：

- `ReviewerResultCollectionResult` 新增只读 conflict snapshot 字段：`recoveryRequired`、`reviewerResultKind`、`reviewerResultBytes`、`reviewerResultMode` 与 `reviewerResultLinkTarget`，用于描述阻塞 canonical result 的 exact kind/hash/size/mode，而不读取或覆盖 candidate 之外的内容。
- `CollectReviewerResult -WhatIf` 在正常 collection preflight 遇到不同 canonical bytes、empty-file 或 symlink obstruction 时，改为返回 `status=recovery-required` / `recoveryRequired=true`，并把 current Mission Commander action 指向 bounded `/rekit plan-subagents -RecoverReviewerResult ... -WhatIf -Format json`；runbook 继续要求先跑 recovery WhatIf、复核 exact hashes、只用返回的 hash-bound Apply，再重新跑 collection WhatIf。
- `-CollectReviewerResult -Apply` 保持原有 no-overwrite fail-closed；collection preview 不 quarantine、不删除、不写 receipt、不执行 recovery、不写 facts/authority/confirmed。
- CLI text 的 collection artifact 行新增 canonical kind/bytes，case-local obstruction recovery E2E 先从 collection WhatIf 看到 `recovery-required` handoff，再执行既有 Recover WhatIf→hash-bound Apply→collection→ready intake 链路。

边界：本批只增强 reviewer result collection 的可恢复冲突 WhatIf handoff 与测试；不自动执行 recovery，不改变 `-RecoverReviewerResult` 的 expected-hash Apply、quarantine/receipt 语义，不放宽 canonical directory / unsupported obstruction fail-closed，不改变 reviewer intake、facts/ledger、authority/confirmed、heavy-tool 或 PowerShell runtime logic。

验证结果：focused `go test ./internal/rekit/subagents ./internal/rekit/cli -run "TestCollectReviewerResultRejectsBindingsCollisionAndSymlink|TestRunPlanSubagentsReviewerResultObstructionRecoveryCaseLocalE2E" -count=1` 已通过；disposition regression focused `go test ./internal/rekit/subagents ./internal/rekit/cli -run "TestCollectReviewerResultRejectsBindingsCollisionAndSymlink|TestRetireAmbiguousReviewerResultRecoveryRetainsCanonical|TestRunPlanSubagentsReviewerResultObstructionRecoveryCaseLocalE2E" -count=1` 已通过。Package validation `go test ./internal/rekit/subagents ./internal/rekit/cli -count=1` 已通过。完整本机 `go run ./cmd/rekit -- -Command release-run -Format text` 已通过，返回 `ready=true` / `summary=release run ok`，聚合执行 `release-check`、`status`、`packs`、`doctor`、`go test ./...`、`go vet ./...`、`git diff --check` 7 步，`passed=7 failed=0 skipped=0`；`git diff --check` 仅保留 Windows 工作树 LF→CRLF 提示。implementation commit `84fe8af` 已推送。Push-triggered release-gate run `30205009789` completed failure，macOS/Windows/Linux jobs `89801445817`/`89801445818`/`89801445833` 均为既有 `steps=[]` / `runner_id=0` blocker，不能声明 remote CI green。

### Batch 622：reviewer result writeback runbook closure

状态：已完成 runtime/test/doc 工作树实现、focused reviewer writeback subagents 与 CLI product-path 回归、本机 release handoff bootstrap 修正、完整本机 `release-run` release minimum、implementation commit/push 与 push-triggered remote release-gate inspection；implementation commit `f26c6db` 已推送。Push run `30204276440` completed failure，Linux/Windows/macOS jobs `89799509740`/`89799509765`/`89799509774` 均 `steps=[]` 且无 logs，仍属既有 runner/billing blocker；不为 release inspection record 自身追加第三个 inspection。

目标：补齐 reviewer result writeback 执行结果的接手断点：Batch 600/601/607 已把 capture-first reviewer 链路写进 packet、summary、status/continue/handoff runbook，但 replacement executor 真正执行 `-CaptureReviewerResultSource`、`-StageReviewerResult` 或 `-CollectReviewerResult` 后，返回 envelope 仍主要依赖 `nextSteps[]` 与 `missionCommanderAction`，需要重开 packet/handoff 才能确认下一条 bounded operation、hash-bound Apply discipline 与 no-heavy/no-authority 边界。本批让 source capture → staging → collection 的每个执行结果自身携带可复制 runbook。

已实现内容：

- `ReviewerResultSourceCaptureResult`、`ReviewerResultStagingResult` 与 `ReviewerResultCollectionResult` 新增只读 `runbookSteps[]`；finalize 阶段从当前 stage/status、artifact hash envelope、`missionCommanderAction.primaryCommand`、`nextSteps[]` 与 boundary 派生去重 runbook。
- Runbook 明确要求先确认当前 stage/status 与 artifact hashes，再运行当前 Mission Commander command；每次 Apply 后必须重新运行下一步 WhatIf，并只使用返回的 hash-bound Apply command；capture、staging、collection 与 packet-level ready reviewer intake 保持四个独立 bounded operations。
- `plan-subagents -Format text` 的 reviewer result source capture / staging / collection 结果新增 `reviewer result ... runbook` terminal lines；JSON/text 都能直接暴露下一步命令、WhatIf→Apply 纪律与 boundary guard。
- Focused subagents 与 CLI product-path 测试覆盖 source capture/staging/collection preview/apply 的 JSON `runbookSteps[]` 和 text runbook 行，锁定 replacement executor 不必回读 packet/summary 即可安全接续。

边界：本批只增强 reviewer result writeback execution envelope 的只读 operational runbook 与测试；不改变 packet schema、reviewer result validation、source/candidate/canonical result publication、ready reviewer results intake、Mission Commander queue ordering、case durable authority/confirmed 或 heavy-tool 执行语义，不新增 PowerShell runtime logic。

验证结果：focused `go test ./internal/rekit/subagents -run "TestStageReviewerResultPublishesCandidateForCollection|TestCollectReviewerResultWhatIfApplyAndReplay" -count=1` 已通过；focused product-path `go test ./internal/rekit/cli ./internal/rekit/subagents -run "TestRunPlanSubagentsReviewerIntakeWhatIfApplyE2E|TestStageReviewerResultPublishesCandidateForCollection|TestCollectReviewerResultWhatIfApplyAndReplay" -count=1` 已通过。Package validation `go test ./internal/rekit/cli ./internal/rekit/subagents -count=1` 已通过。完整本机 `go run ./cmd/rekit -- -Command release-run -Format text` 已通过，返回 `ready=true` / `summary=release run ok`，聚合执行 `release-check`、`status`、`packs`、`doctor`、`go test ./...`、`go vet ./...`、`git diff --check` 7 步，`passed=7 failed=0 skipped=0`；`git diff --check` 仅保留 Windows 工作树 LF→CRLF 提示。implementation commit `f26c6db` 已推送。Push-triggered release-gate run `30204276440` completed failure，Linux/Windows/macOS jobs `89799509740`/`89799509765`/`89799509774` 均为既有 `steps=[]` / `runner_id=0` blocker，不能声明 remote CI green。

### Batch 621：release-run release inspection handoff closure

状态：已完成 runtime/test/doc 工作树实现、focused `release-run` handoff 回归、CLI package 验证、完整本机 `release-run` release minimum，以及 implementation commit/push 和 push-triggered remote release-gate inspection；implementation commit `d84b427` 已推送。Push run `30203150398` completed failure，Linux/macOS/Windows jobs `89796512974`/`89796512982`/`89796513018` 均 `steps=[]` 且无 logs，仍属既有 runner/billing blocker；不为 release inspection record 自身追加第三个 inspection。

目标：补齐 release-run 后的真实接手断点：Batch 615 让本机 release minimum 可由 Go-native `release-run` 聚合执行，Batch 619 让 completed batch parser 识别 `release-run` 成功证据；但维护者在本机验证完成后仍要手工拼接 `git status`、`main`/`origin/main` 同步、latest-batch release inspection cadence、`steps=[]` blocker 与“不要追加第三个 inspection record”边界。replacement executor 容易把 `release-run ready=true` 误读为 remote green，或在 release inspection commit 自己触发的 CI 之后继续追第三次记录。本批让 `release-run` 在同一 JSON/text envelope 中输出只读 release inspection handoff。

已实现内容：

- `release-run` 结果新增 `releaseInspection` handoff，执行本机 gateProfile steps 后只读汇总 local git branch、HEAD、origin/main、working tree clean、HEAD/origin ancestry 同步、latest-batch `releaseInspectionCadence`、remote gate detail、next actions、warnings 与 boundary。
- Text 输出新增 `release-run release inspection`、`release-run release inspection git`、remote gate、next action、boundary/warning 行；JSON 输出保留同一机器可读结构，供新会话/维护脚本直接判断 clean/sync、third-inspection guard 与 remote non-green boundary。
- Handoff 明确不 fetch GitHub、不读取 live Actions、不改变 `release-run` pass/fail 语义；dirty/非 main/未同步只让 `releaseInspection.ready=false` 并给出下一步，不把 local release minimum 变成失败。
- 单元测试覆盖 clean synced handoff、remote non-green truthfulness、dirty/unsynced blocked handoff 与既有 release-run failure aggregation。

边界：本批只增强 Go-native `release-run` 的只读 release inspection handoff；不新增 public command，不联网读取 GitHub Actions live state，不改变 recommendedMinimum，不写 repo/case durable state，不写 authority/confirmed，不执行 heavy tool，不新增 PowerShell runtime logic。

验证结果：focused `go test ./internal/rekit/cli -run "TestRunReleaseRun" -count=1` 已通过；related `go test ./internal/rekit/cli -count=1` 已通过；focused release inventory 回归 `go test ./internal/rekit/cli -run "TestRunReleaseRun|TestRunReleaseCheckJsonInventory" -count=1` 已通过。完整本机 `go run ./cmd/rekit -- -Command release-run -Format text` 已通过，返回 `ready=true` / `summary=release run ok`，聚合执行 `release-check`、`status`、`packs`、`doctor`、`go test ./...`、`go vet ./...`、`git diff --check` 7 步，`passed=7 failed=0 skipped=0`；新增 `releaseInspection` handoff 在当前未提交工作树下按预期显示 `ready=false`、`summary=release inspection handoff blocked` 与 clean-working-tree next action，不影响 local release minimum。implementation commit `d84b427` 已推送。Push-triggered release-gate run `30203150398` completed failure，Linux/macOS/Windows jobs `89796512974`/`89796512982`/`89796513018` 均为既有 `steps=[]` / `runner_id=0` blocker，不能声明 remote CI green。

### Batch 620：Mission Commander Markdown action identity closure

状态：已完成 runtime/test/doc 工作树实现、focused overview/handoff Markdown product-path 验证、完整本机 `release-run` release minimum，以及 implementation commit/push 和 PR-triggered remote release-gate inspection；implementation commit `4333fbd` 已推送。PR run `30200780699` completed failure，macOS/Linux/Windows jobs `89790230652`/`89790230682`/`89790230693` 均 `steps=[]` 且无 logs，仍属既有 runner/billing blocker；不为 release inspection record 自身追加第三个 inspection。

目标：补齐 durable Markdown 接手面的真实断点：Batch 618 已让 CLI text/default Mission Commander action lines 打印 lane、label、gateEventId、actionId，但 overview 与 project/lane handoff Markdown 仍只在 action queue current / next action 行显示 state/source/command。replacement executor 从 `.rekit/handoffs`、lane `RESUME.md` 或 overview Markdown 接手 adapter/evidence/reviewer action 时，仍要回查 JSON 才能确认 gate/action identity。本批让 Markdown 接手面与 CLI text identity 一致，同时保持原有 state/source/command 前缀兼容。

已实现内容：

- 新增 `MissionCommanderNextActionMarkdownLine` formatter，保持 `state/source/blocked/requiresReview/command` 前缀不变，并在末尾追加 lane、label、gateEventId、actionId。
- overview Markdown 的 Mission Commander action queue current 与 next actions、project handoff per-lane queue current/next actions、lane handoff Markdown queue current/next actions 统一复用该 formatter。
- 单元测试锁定 Markdown formatter 与 overview queue current identity；CLI overview/handoff/status product-path focused 回归确认既有前缀断言未破坏。

边界：本批只增强 overview / project handoff / lane handoff Markdown 的只读接手投影；不改变 Mission Commander queue ordering、adapter validation/record 语义、case durable state、authority/confirmed、heavy tool 或 PowerShell runtime logic。

验证结果：focused `go test ./internal/rekit/workstream ./internal/rekit/overview -run "TestMissionCommanderNextActionMarkdownLineIncludesIdentity|TestOverviewMissionCommanderActionQueuePrintsIdentity|TestOverviewNextSteps" -count=1` 已通过；related product-path `go test ./internal/rekit/cli ./internal/rekit/overview ./internal/rekit/workstream -run "TestRunOverview|TestRunHandoff|TestRunStatus|TestMissionCommanderNextActionMarkdownLineIncludesIdentity|TestOverviewMissionCommanderActionQueuePrintsIdentity" -count=1` 已通过。完整本机 `go run ./cmd/rekit -- -Command release-run -Format text` 已通过，返回 `ready=true` / `summary=release run ok`，聚合执行 `release-check`、`status`、`packs`、`doctor`、`go test ./...`、`go vet ./...`、`git diff --check` 7 步，`passed=7 failed=0 skipped=0`，`git diff --check` 仅保留 Windows 工作树 LF→CRLF 提示。implementation commit `4333fbd` 已推送。PR-triggered release-gate run `30200780699` completed failure，macOS/Linux/Windows jobs `89790230652`/`89790230682`/`89790230693` 均为既有 `steps=[]` / `runner_id=0` blocker，不能声明 remote CI green。

### Batch 619：release-run project handoff readiness closure

状态：已完成 runtime/test/doc 工作树实现、focused release handoff/status product-path 验证、完整本机 `release-run` release minimum，以及 implementation commit/push 和 PR-triggered remote release-gate inspection；implementation commit `ca254c5` 已推送。PR run `30200365431` completed failure，Linux/Windows/macOS jobs `89789151577`/`89789151581`/`89789151587` 均 `steps=[]` 且无 logs，仍属既有 runner/billing blocker；不为 release inspection record 自身追加第三个 inspection。

目标：补齐 completed batch project handoff 的真实断点：Batch 615 引入 Go-native `release-run` 后，Batch 617/618 文档开始用 `release-run ready=true / summary=release run ok / passed=7 failed=0 skipped=0` 作为完整本机 release minimum 证据；但 latest-batch parser 仍只把 7 条原子命令文本当作 localValidationReady 依据。结果 Batch 618 已 complete 且 cadence 已记录“continue the next batch”时，default `status` first-screen 仍把 project current action 指向“run the full local release minimum and update docs/batch-plan.md”，误导 replacement executor 重复做已完成验证。本批让 `release-run` 成功摘要成为一等本机 release minimum evidence，并驱动 completed batch current action 指向下一批。

已实现内容：

- `latestBatchHasLocalValidation` 与 `latestBatchReleaseCheckReady` 识别成功的 `release-run` 证据（`ready=true` 加 `summary=release run ok`，或 `passed=7 failed=0 skipped=0`），同时保留原子 7 步命令旧判定和 pending 文案 fail-closed。
- `latestBatchEvidence` 将成功 `release-run` 聚合输出映射回 release-check/status/packs/doctor/go test/go vet/git diff evidence labels，并新增 `release-run local release minimum recorded`，避免 `localValidationReady=true` 但 evidence 缺失。
- `status` project handoff / first-screen 回归锁定：当 latest batch release inspection cadence 已 complete 时，project current action 不得再包含 `run the full local release minimum`，而应指向继续下一批并保留 steps=[] blocker boundary。

边界：本批只增强 release/status latest-batch parser 与 project current-action handoff；不执行远程 CI，不改变 release cadence，不把 `release-run` 加入 recommendedMinimum，不写 repo/case durable state、authority/confirmed，不执行 heavy tool，不新增 PowerShell runtime logic。

验证结果：focused `go test ./internal/rekit/releasecheck ./internal/rekit/cli -run "TestLatestBatchHandoffAcceptsReleaseRunLocalMinimum|TestLatestBatchHandoffExtractsValidationEvidence|TestLatestBatchReleaseInspectionCadenceWaitsForImplementationCommit|TestRunStatusJsonKit|TestRunStatus" -count=1` 已通过；`go run ./cmd/rekit -- -Command status -Format text` 已确认 Batch 618 `localValidationReady=true` / `releaseCheckReady=true`，first-screen current action 为“do not create a third inspection record ... continue the next Windows-verifiable batch”，不再重复要求 full local release minimum。完整本机 `go run ./cmd/rekit -- -Command release-run -Format text` 已通过，返回 `ready=true` / `summary=release run ok`，聚合执行 `release-check`、`status`、`packs`、`doctor`、`go test ./...`、`go vet ./...`、`git diff --check` 7 步，`passed=7 failed=0 skipped=0`，`git diff --check` 仅保留 Windows 工作树 LF→CRLF 提示。implementation commit `ca254c5` 已推送。PR-triggered release-gate run `30200365431` completed failure，Linux/Windows/macOS jobs `89789151577`/`89789151581`/`89789151587` 均为既有 `steps=[]` / `runner_id=0` blocker，不能声明 remote CI green。

### Batch 618：adapter execution validation identity handoff closure

状态：已完成 runtime/test/doc 工作树实现、focused adapter validation / CLI text product-path 验证、完整本机 `release-run` release minimum，以及 implementation commit/push 和 PR-triggered remote release-gate inspection；implementation commit `483947c` 已推送。PR run `30199667894` completed failure，macOS/Windows/Linux jobs `89787316201`/`89787316236`/`89787316256` 均 `steps=[]` 且无 logs，仍属既有 runner/billing blocker；不为 release inspection record 自身追加第三个 inspection。

目标：补齐 adapter execution validation 的接手断点：Batch 580/594/602 已把 adapter report currentness 收紧为 read-only validation → hash-bound record，但 Mission Commander terminal text/current action 仍主要暴露 command/source；replacement executor 要判断“这是哪个 authorized gate 的 validate、record、repair、scaffold/draft 阶段”仍需回查 nested JSON 或手工关联 gate event。本批让 adapter report lifecycle 的 current/next action 直接携带 gate identity 与稳定 action identity。

已实现内容：

- `gate -ExecutionReportContract`、`gate -ValidateExecutionReport`、adapter scaffold/draft live snapshot 与 recorded evidence snapshot 的 `missionCommanderNextActions[]` / `missionCommanderActionQueue.currentAction` 统一通过 adapter report action helper 构造，保留 lane、label、gateEventId 与 `<gateEventId>:<adapter-report-stage>` actionId。
- CLI Mission Commander text renderer 在 `mission commander next action`、`mission commander action queue current` 与 status action queue bucket 行直接打印 lane、label、gateEventId、actionId；terminal replacement executor 不必解析 nested JSON 才能绑定 adapter gate identity。
- Adapter report contract/validation/scaffold/draft/recorded evidence focused 回归锁定 current action identity，CLI text 回归锁定 valid record、invalid repair、contract validation 的 gateEventId/actionId 输出。

边界：本批只增强 adapter execution validation / record handoff 的只读 projection 与 bounded observation evidence record 后的 current action identity；不执行 adapter/heavy tool，不自动 validate/record，不写 authority/confirmed，不新增 durable schema，不新增 PowerShell runtime logic。

验证结果：focused `go test ./internal/rekit/gate ./internal/rekit/cli -run "TestAdapterReportContractDescribesAuthorizedGateBoundaries|TestValidateAdapterExecutionReport(ReadOnlyPreflight|MissingPathExposesMissionCommanderRepair|ReturnsInvalidEnvelopeReadOnly)|TestAdapterReportLiveSnapshot(TracksRecordedReportIdentity|PreservesRecordedBoundaryEscalation|MarksMalformedSidecarPresent)|TestRunGateAdapterReportTextOutputsNextActions|TestRunStatus" -count=1` 已通过；完整本机 `go run ./cmd/rekit -- -Command release-run -Format text` 已通过，返回 `ready=true` / `summary=release run ok`，聚合执行 `release-check`、`status`、`packs`、`doctor`、`go test ./...`、`go vet ./...`、`git diff --check` 7 步，`passed=7 failed=0 skipped=0`，`git diff --check` 仅保留 Windows 工作树 LF→CRLF 提示。implementation commit `483947c` 已推送。PR-triggered release-gate run `30199667894` completed failure，macOS/Windows/Linux jobs `89787316201`/`89787316236`/`89787316256` 均为既有 `steps=[]` / `runner_id=0` blocker，不能声明 remote CI green。

### Batch 617：reviewer intake/postValidation full next-action summary

状态：已完成 runtime/test/doc 工作树实现、focused reviewer intake product-path 验证、完整本机 `release-run` release minimum，以及 implementation commit/push 和 PR-triggered remote release-gate inspection；implementation commit `f261e68` 已推送。PR run `30198335234` completed failure，Linux/macOS/Windows jobs `89783801315`/`89783801328`/`89783801336` 均 `steps=[]` 且无 logs，仍属既有 runner/billing blocker；不为 release inspection record 自身追加第三个 inspection。

目标：补齐 reviewer writeback 后的接手断点：Batch 616 已把 project handoff 升级为结构化 queue，但 reviewer-intake compact summary 与 postValidation summary 仍只保留 state/source/command/blocked/requiresReview；replacement executor 在 preview、complete 或 already-complete 后仍要回查 nested handoff queue / `missionCommanderNextActions[]` 才能看到 lane/label、packet identity、reason 与 no-heavy/no-authority boundary。本批让 reviewer intake 总 summary 与 postValidation summary 都保留完整 current/next action handoff 语义，并在 text path 直接输出 reason/boundary。

已实现内容：

- `ReviewerIntakeNextActionSummary` 与 `ReviewerPostValidationNextActionSummary` 新增 lane、label、gateEventId、actionId、reasons、boundary，只读复制对应 `MissionCommanderNextActionItem` 字段；不新增 durable schema 或 case 写入。
- `reviewerIntakeSummary` 与 `reviewerPostValidationSummary` 的 current/next action projection 现在保留完整 action item 语义；preview 分支可直接看到 reviewer packet action 的 packet/lane/reason/boundary，already-complete 分支可直接看到 lane continue 的 reason/boundary。
- `plan-subagents -Format text` 的 reviewer intake summary 与 postValidation summary current/next action lines 继续保持原有 state/source/command 前缀，同时追加 lane/label/gateEventId/actionId，并输出对应 reason/boundary lines，避免 replacement executor 解析 nested queue 才能安全接续。
- CLI reviewer-intake E2E 回归锁定 preview summary 的 WhatIf/no-heavy boundary、postValidation reviewer packet reason/boundary，以及 already-complete postValidation 的 ready lane continue reason/boundary。

边界：本批只增强 reviewer intake/postValidation compact summary 的只读 JSON/text 投影；不改变 reviewer result validation、verification-before-decision writeback、batch intake ordering、status/handoff priority、case durable state、authority/confirmed 或 heavy-tool 执行语义，不新增 PowerShell runtime logic。

验证结果：focused `go test ./internal/rekit/cli -run TestRunPlanSubagentsReviewerIntakeWhatIfApplyE2E -count=1` 已通过；完整本机 release minimum 已通过：`go run ./cmd/rekit -- -Command release-run -Format text` 返回 `ready=true` / `summary=release run ok`，聚合执行 `release-check`、`status`、`packs`、`doctor`、`go test ./...`、`go vet ./...`、`git diff --check` 7 步，`passed=7 failed=0 skipped=0`，`git diff --check` 仅保留 Windows 工作树 LF→CRLF 提示。implementation commit `f261e68` 已推送。PR-triggered release-gate run `30198335234` completed failure，Linux/macOS/Windows jobs `89783801315`/`89783801328`/`89783801336` 均为既有 `steps=[]` / `runner_id=0` blocker，不能声明 remote CI green。

### Batch 616：project handoff structured current-action queue

状态：已完成 runtime/test/doc 工作树实现、focused CLI product-path 验证、完整本机 `release-run` release minimum，以及 implementation commit/push 和 PR-triggered remote release-gate inspection；implementation commit `78e2c3f` 已推送。PR run `30196672446` completed failure，macOS/Linux/Windows jobs `89779297674`/`89779297695`/`89779297736` 均 `steps=[]` 且无 logs，仍属既有 runner/billing blocker；不为 release inspection record 自身追加第三个 inspection。

目标：补齐新会话接手的真实断点：Batch 611/612/613 已把 latest-batch project current action 推到第一屏和 runbook，但 JSON/status project handoff 仍主要依赖 `latestNextAction` 字符串；replacement executor 或工具化接手要自行解析 free-text，难以复用 Mission Commander queue 的 current/unblocked/review/boundary 语义。本批将 project-level latest-batch handoff 提升为结构化 `missionCommanderNextActions[]` 与 `missionCommanderActionQueue`，并让 first-screen/text/JSON 共享同一来源。

已实现内容：

- `statusProjectHandoff` 新增只读 `missionCommanderNextActions[]` 与 `missionCommanderActionQueue`，复用既有 `MissionCommanderNextActionItem` / `MissionCommanderActionQueue` schema；不新增 durable state 或 case-local 写入。
- `buildStatusProjectHandoff` 在构造 release handoff 时生成 project current action queue；`writeStatusMissionCommanderFirstScreenText` 不再临时从 `latestNextAction` 重新派生 project current，而是消费同一个 queue current action。
- `status -Format text` 在 latest-batch handoff 段输出 `status project handoff current action queue` 与 current/unblocked/reviewRequired buckets，让新会话从 text 或 JSON 都能拿到同一条可执行接手 action、reason 与 boundary。
- CLI 回归锁定 JSON queue current action 与 `latestNextAction` 同源、queue counts、reasons/boundary，以及 text/default status 的 project queue 输出。

边界：本批只增强 status/project handoff 的只读结构化接手投影；不改变 release cadence，不执行远程 CI，不写 repo/case durable state、authority/confirmed，不新增 PowerShell runtime logic，不把 remote CI inventory ready 说成 remote green。

验证结果：focused `go test ./internal/rekit/cli -run "TestRunStatusJsonKit|TestRunStatus|TestRunStatusKitShowsOpenPackMemoryCandidates" -count=1` 已通过；`go run ./cmd/rekit -- -Command status -Format json` 与 `go run ./cmd/rekit -- -Command status -Format text` 已手动确认 project handoff queue 与 first-screen current action 同源。完整本机 release minimum 已通过：`go run ./cmd/rekit -- -Command release-run -Format text` 返回 `ready=true` / `summary=release run ok`，聚合执行 `release-check`、`status`、`packs`、`doctor`、`go test ./...`、`go vet ./...`、`git diff --check` 7 步，`passed=7 failed=0 skipped=0`，`git diff --check` 仅保留 Windows 工作树 LF→CRLF 提示。implementation commit `78e2c3f` 已推送。PR-triggered release-gate run `30196672446` completed failure，macOS/Linux/Windows jobs `89779297674`/`89779297695`/`89779297736` 均为既有 `steps=[]` / `runner_id=0` blocker，不能声明 remote CI green。

### Batch 615：Go-native local release runner closure

状态：已完成 runtime/test/doc 工作树实现、focused package / release inventory 验证、完整本机 `release-run` release minimum，以及 implementation commit/push 和 PR-triggered remote release-gate inspection；implementation commit `6e1eebf` 已推送。`release-run` 首次 fail-closed 暴露 latest-batch 文档状态与 PowerShell façade freeze invariant 残留，均已修复并重跑通过。PR run `30195410301` completed failure，Windows/macOS/Linux jobs `89775948859`/`89775948866`/`89775948868` 均 `steps=[]` 且无 logs，仍属既有 runner/billing blocker；不为 release inspection record 自身追加第三个 inspection。

目标：补齐维护者本机 release gate 真实断点：`release-check` 只枚举 `local-ci-minimum` gate profile，本机 release minimum 仍要手工按顺序运行 7 条命令，容易漏步、丢失每步 exit code / duration / output tail，也没有统一 JSON/text summary 供 replacement executor 接手。本批将 resolved gateProfile steps 变成 Go-native local runner，但不改变 `release-check` 只读 inventory 语义，也不把 runner 自身加入 recommendedMinimum。

已实现内容：

- 新增 public command `release-run`，登记为 read-only public profile；Go-native public surface、symbol catalog、handler coverage、profile summary、boundary rows 与 PowerShell public façade cross-check 基线从 20/5 更新为 21/6。
- `runReleaseRun` 读取 `releasecheck.Build(ctx.RepoRoot).GateProfile`，顺序执行 resolved steps，跳过 unresolved steps，聚合 passed/failed/skipped、每步 exit code、durationMs、outputTail 与 error；默认 text/table 输出，`-Format json` 输出机器可读 envelope。
- `release-run` 明确拒绝 `-Target`、mutation/review/list flags 与 review artifacts flags；结果 boundary 声明只执行 kit repo gateProfile steps、不写 repo/case state、不执行 heavy tool、不写 authority/confirmed。
- retained `rekit/rekit.ps1` 只同步 ValidateSet、Go-default delegation、no-fallback、安全参数透传和 no-target guard，不新增 PowerShell runtime logic；`docs/powershell-deprecation.md` 将 `release-run` 纳入 Go-owned/no-fallback command ownership 与 public façade retention inventory。
- README、agent-team usage 与 release readiness 说明 `release-run` 是维护者本机 release minimum 聚合 runner，不替代 `release-check` inventory 或真实远程 CI green；recommendedMinimum 仍保持原子 7 步，不包含 `release-run`，避免递归。

边界：本批只新增 Go-native local release runner 与只读 release/public surface 文档/测试；不修改 `rekit/tests/catalog.json` recommendedMinimum，不执行或批准 heavy action，不写 repo/case durable state、authority/confirmed 或 case-specific artifact，不改变 sync/promote/gate/continue 语义，不新增 PowerShell runtime logic，不把 remote CI inventory ready 说成 remote green。

验证结果：focused `go test ./internal/rekit/releasecheck ./internal/rekit/cli -run "TestReleaseHandoffInventoryFromRepo|TestReleaseCheckIncludesManifestHeavyToolGateActions|TestPowerShellDeprecationInventoryFromRepo|TestRunStatusJsonKit|TestRunReleaseCheckJsonInventory|TestRunReleaseCheckTextInventory|TestRunReleaseRun|TestReleaseRunOutputTail" -count=1` 已通过；`go test ./internal/rekit/manifest -run TestPowerShellFacadeFreezeInvariants -count=1` 已通过；完整本地 release minimum 已通过：`go run ./cmd/rekit -- -Command release-run -Format text` 返回 `ready=true` / `summary=release run ok`，并聚合执行 `release-check`、`status`、`packs`、`doctor`、`go test ./...`、`go vet ./...`、`git diff --check` 7 步，`passed=7 failed=0 skipped=0`，`git diff --check` 仅保留 Windows 工作树 LF→CRLF 提示。implementation commit `6e1eebf` 已推送。PR-triggered release-gate run `30195410301` completed failure，Windows/macOS/Linux jobs `89775948859`/`89775948866`/`89775948868` 均为既有 `steps=[]` / `runner_id=0` blocker，不能声明 remote CI green。

### Batch 614：pack-memory focus first-screen evidence shortlist

状态：已完成 runtime/test/doc 工作树实现、focused CLI product-path 验证、完整本地 release minimum，以及 implementation commit/push 和 PR-triggered remote release-gate inspection；implementation commit `49f86fb` 已推送并进入 PR #15。当 `pack-memory-current-action` 成为 Mission Commander first-screen focus 时，default `status` / `/rekit` 现在会在 action/runbook 前输出 `status Mission Commander focus pack-memory evidence` 短名单，把导致 pack-memory focus 的 open pack counts、proof progress、next missing proof、receipt counts 与关键 inventory evidence 压到第一屏。PR run `30191742065` completed failure，Linux/macOS/Windows jobs `89766054766`/`89766054768`/`89766054787` 均 `steps=[]` 且无 logs，仍属既有 runner/billing blocker；不为 release inspection record 自身追加第三个 inspection。

目标：补齐 Batch 595/608/613 后仍存在的 pack-memory 接手断点：first-screen 能显示 pack-memory current action、routing explanation 与 runbook，但 replacement executor 仍需向下翻完整 pack-memory inventory 才能确认是哪一个 pack/candidate/proof residue 导致 focus。open candidate 多、review artifact 多或 proof stages 多时，第一屏缺少“为什么是这个 pack、缺哪种 proof、候选根在哪里”的短证据，容易把 runbook 当作通用步骤而忽略当前 residue。

已实现内容：

- `writeStatusMissionCommanderFirstScreenText` 在 `pack-memory-current-action` focus 分支新增 `writeStatusMissionCommanderFirstScreenPackMemoryEvidenceText`，位置在 focus action 后、pack-memory runbook 前；focus 优先级与 action queue 不变。
- 新增 `statusMissionCommanderFirstScreenPackMemoryEvidence`，复用 `ReleaseHandoffPackMemoryCandidateStatus` 既有字段生成最多 6 条高价值 evidence：candidate/tooling/index counts、review/cleanup/verification flags、proof progress/current stage/missing count、next missing proof type/candidate/target、decision receipt verification counts，以及 pack inventory evidence。
- Evidence shortlist 使用 head 截断保留 counts/proof/next-missing-proof 等高价值头部信息，避免 `mission.LimitStrings` 的 tail 截断把 first-screen 的核心 evidence 挤掉。
- Pack-memory open-candidate fixture 扩展 text 断言，锁定 `focus=pack-memory-current-action` 时 first-screen evidence 会显示 `_template` 的 counts、proof progress、next missing proof 与 candidate root/file-count evidence；纯函数测试覆盖 head 截断行为。
- README 与 agent-team usage 文档补充 pack-memory focus evidence shortlist；完整 lower-section `status pack-memory ...` inventory 仍保留，first-screen 只做接手短名单。

边界：本批只增强 default status / Mission Commander first-screen 的只读 pack-memory evidence shortlist 与产品路径测试；不改变 pack-memory inventory、action queue、proof draft、candidate decision、cleanup/provision/verification/retirement事务语义，不 merge/cleanup/provision/verify/write proof，不新增 JSON/durable schema，不执行 heavy tool，不写 authority/confirmed，不新增 PowerShell runtime logic。既有 `steps=[]` / `runner_id=0` runner/billing blocker 继续作为 known gap 记录，不能声明 remote CI green。

验证结果：focused `go test ./internal/rekit/cli -run "TestStatusMissionCommanderFirstScreenPackMemoryEvidenceKeepsHighValueHead|TestRunStatusKitShowsOpenPackMemoryCandidates" -count=1` 已通过；完整本地 release minimum 已通过：`go run ./cmd/rekit -- -Command release-check -Format json` 返回 `ready=true` / `summary=release gate inventory ok`，`go run ./cmd/rekit -- -Command status`、`go run ./cmd/rekit -- -Command packs`、`go run ./cmd/rekit -- -Command doctor`、`go test ./...`、`go vet ./...` 均通过，`git diff --check` 仅保留 Windows 工作树 LF→CRLF 提示；implementation commit `49f86fb` 已推送并进入 PR #15。PR-triggered release-gate run `30191742065` completed failure，Linux/macOS/Windows jobs `89766054766`/`89766054768`/`89766054787` 均为既有 `steps=[]` / `runner_id=0` blocker，不能声明 remote CI green。

### Batch 613：Mission Commander first-screen focus routing explanation

状态：已完成 runtime/test/doc 工作树实现、focused CLI product-path 验证、完整本地 release minimum，以及 implementation commit/push 和 PR-triggered remote release-gate inspection；implementation commit `1ed2f22` 已推送并进入 PR #15。默认 `status` / `/rekit` first-screen 现在会在 `focus=...` summary 后追加 `status Mission Commander first screen routing` 行，说明当前 focus 为什么胜出，以及哪些其它 current-action queues 被延后。PR run `30191350331` completed failure，Linux/macOS/Windows jobs `89765045215`/`89765045222`/`89765045223` 均 `steps=[]` 且无 logs，仍属既有 runner/billing blocker；不为 release inspection record 自身追加第三个 inspection。

目标：补齐 Batch 597/611/612 后仍存在的第一屏解释断点：Mission Commander first-screen 已能显示 case/reviewer/pack-memory/project focus 和 project runbook，但 replacement executor 仍只能看到最终 `focus=...`，不知道它是因为 reviewer dispatch 覆盖 case action、case action 需要 review、pack-memory candidate 未闭合，还是只是 project fallback；当下方仍打印多个 `status Mission Commander current action` 时，容易误以为较低优先级队列被忽略。

已实现内容：

- `writeStatusMissionCommanderFirstScreenText` 在 focus summary 之后输出 `status Mission Commander first screen routing` 只读说明；focus 选择逻辑与优先级保持不变。
- 新增 `statusMissionCommanderFirstScreenFocusRoutingReasons`，复用已有 `caseCurrent`、`reviewerCurrent`、`packCurrent`、`projectCurrent` 与 pack-memory readiness 判定，解释 reviewer-dispatch override、case needs-attention、reviewer queue open、pack-memory closure required、project fallback 或 no-action。
- Routing explanation 会在存在 lower-priority current actions 时追加 `deferred focus queues`，让 replacement executor 知道其它队列仍在下方保留，但当前 focus 有更高接手优先级。
- CLI focused tests 覆盖 routing pure helper 的 project fallback 与 reviewer-dispatch override/deferred queues，并扩展默认 `status` / `status -Format text` 断言，锁定 project fallback routing 行。
- README 与 agent-team usage 文档补充 first-screen 会显示 focus routing explanation 与 project runbook，避免用户把 routing explanation 误认为 durable state 或新 schema。

边界：本批只增强 default status / Mission Commander first-screen 的只读 text routing explanation 与产品路径测试；不改变 focus priority，不新增 JSON/durable schema，不改变 release inspection cadence，不执行 heavy tool，不写 authority/confirmed，不新增 PowerShell runtime logic。既有 `steps=[]` / `runner_id=0` runner/billing blocker 继续作为 known gap 记录，不能声明 remote CI green。

验证结果：focused `go test ./internal/rekit/cli -run "TestStatusMissionCommanderFirstScreenFocusRoutingReasons|TestRunStatus" -count=1` 已通过；完整本地 release minimum 已通过：`go run ./cmd/rekit -- -Command release-check -Format json` 返回 `ready=true` / `summary=release gate inventory ok`，`go run ./cmd/rekit -- -Command status`、`go run ./cmd/rekit -- -Command packs`、`go run ./cmd/rekit -- -Command doctor`、`go test ./...`、`go vet ./...` 均通过，`git diff --check` 仅保留 Windows 工作树 LF→CRLF 提示；implementation commit `1ed2f22` 已推送并进入 PR #15。PR-triggered release-gate run `30191350331` completed failure，Linux/macOS/Windows jobs `89765045215`/`89765045222`/`89765045223` 均为既有 `steps=[]` / `runner_id=0` blocker，不能声明 remote CI green。

### Batch 612：Mission Commander project current action first-screen runbook

状态：已完成 runtime/test/doc 工作树实现、focused CLI product-path 验证、完整本地 release minimum，以及 implementation commit/push 和 PR-triggered remote release-gate inspection；implementation commit `cb2ec80` 已推送并进入 PR #15。默认 kit-mode `status` / `status -Format text` 现在不仅投影 latest-batch project current action，还会在 `focus=project-current-action` 时输出 project runbook steps，让新会话从第一屏直接看到读取顺序、latest-batch action、release inspection cadence、local validation 与 remote non-green boundary。PR run `30191008102` completed failure，Linux/Windows/macOS jobs `89764153978`/`89764153979`/`89764154001` 均 `steps=[]` 且无 logs，仍属既有 runner/billing blocker；不为 release inspection record 自身追加第三个 inspection。

目标：补齐 Batch 611 后暴露的第一屏接手断点：project current action 已能显示 current/reason/boundary，但不像 reviewer/pack-memory focus 那样把下一步操作整理成可复制 runbook。replacement executor 仍需要从下方 release handoff 里拼出“先读哪些文档、按 cadence 做什么、验证前跑什么、远程 blocker 怎么处理”，增加接手摩擦。

已实现内容：

- `writeStatusMissionCommanderFirstScreenText` 在 `project-current-action` focus 分支追加 `writeStatusMissionCommanderFirstScreenProjectRunbookText`；case/reviewer/pack-memory 的既有优先级不变，project runbook 只在 project focus 成为兜底焦点时输出。
- `writeStatusMissionCommanderFirstScreenProjectRunbookText` 复用 `statusProjectHandoff` 的 `ReadFirst`、`LatestNextAction` / current command、release inspection cadence nextAction、validation commands 与 latest remote gate detail，生成 `status Mission Commander focus project runbook` 行。
- Runbook steps 明确：先读 `docs/context-routing.md` 与 `docs/batch-plan.md` 当前 batch 段；跟随 first-screen project current action；按 latest batch release inspection cadence 执行且无新远程信号不追加第三个 inspection record；handoff/release claim 前重跑本地 validation commands；remote gate 未明确 green 时不得宣称 remote CI green。
- CLI 回归扩展默认 `status` 与 `status -Format text` 断言，锁定 project runbook 的 batch/state/step/text 输出，避免后续 first-screen 只剩 summary 或 boundary。
- README 与 agent-team usage 文档补充 kit-mode latest-batch project focus 会显示 project current action 与 runbook，再回到 compact first-screen strip / queue current action 接手。

边界：本批只增强 default status / Mission Commander first-screen 的只读 project runbook projection 与产品路径测试；不执行 heavy tool，不改变 release inspection cadence，不修改 workflow，不写 authority/confirmed，不新增 durable schema，不新增 PowerShell runtime logic。既有 `steps=[]` / `runner_id=0` runner/billing blocker 继续作为 known gap 记录，不能声明 remote CI green，也不因此阻塞 Windows 本机产品路径。

验证结果：focused `go test ./internal/rekit/cli -run TestRunStatus -count=1` 已通过；完整本地 release minimum 已通过：`go run ./cmd/rekit -- -Command release-check -Format json` 返回 `ready=true` / `summary=release gate inventory ok`，`go run ./cmd/rekit -- -Command status`、`go run ./cmd/rekit -- -Command packs`、`go run ./cmd/rekit -- -Command doctor`、`go test ./...`、`go vet ./...` 均通过，`git diff --check` 仅保留 Windows 工作树 LF→CRLF 提示；implementation commit `cb2ec80` 已推送并进入 PR #15。PR-triggered release-gate run `30191008102` completed failure，Linux/Windows/macOS jobs `89764153978`/`89764153979`/`89764154001` 均为既有 `steps=[]` / `runner_id=0` blocker，不能声明 remote CI green。

### Batch 610：release handoff commit-ref scope guard

状态：已完成 runtime/test/doc 工作树实现、focused releasecheck parser 验证、完整本地 release minimum，以及 implementation commit/push 和 PR-triggered remote release-gate inspection；implementation commit `2ac506a` 已推送并进入 PR #15。PR run `30187565571` completed failure，Windows/Linux/macOS jobs `89754882530`/`89754882569`/`89754882571` 均 `steps=[]`、`runner_id=0` 且无 logs，仍属既有 runner/billing blocker。本批继续 Windows 本机 product-path 优先；远程 CI 若仍返回既有 blocker，不因此追加第三个 inspection record。

目标：补齐 Batch 605/609 后暴露的 release handoff parser 真实断点：`release-check` / `status` 会把 latest-batch implementation commit refs 投影到 first-screen 和 JSON handoff；Batch 609 inspection 文案里曾把 implementation commit、PR run 与 job IDs 放在同一句 evidence clause，旧 `latestBatchCommitRefs` 只要看到该 clause 含 `implementation commit` 就扫描所有 backtick hex token，导致 remote run/job 数字误出现在 `commitRefs[]` / `status latest batch commit`。replacement executor 可能把 GitHub run/job ID 当作 implementation commit，release inspection 证据链被污染。

已实现内容：

- `latestBatchCommitRefs` 现在先把 commit evidence clause 缩到 implementation commit scope，再扫描 backtick tokens；遇到 `PR #`、`remote` / `远程`、`release-gate run`、`workflow run`、`pr run`、`jobs` / `job` 等 remote-inspection marker 后停止收集 commit refs。
- 新增 `latestBatchCommitRefScope` / `latestBatchCommitMarkerIndex` helper，仍保留全数字合法 short SHA（例如 `7896077`）支持，但不再把同一句后半段的 PR run/job refs 归入 implementation commit refs。
- Releasecheck 回归新增同一句混排 fixture：同一个 evidence clause 同时包含 `Implementation commit`、commit ref `7896077`、PR run `30186884673`、job refs `897...` 与 `steps=[]`，断言 `commitRefs[]` 只包含 `7896077`，同时 remote release gate 仍解析为 completed failure with `steps=[]` blocker。

边界：本批只增强 release/status read-only latest-batch handoff parser 与测试覆盖；不执行远程 CI，不改变 release inspection cadence，不修改 workflow，不创建或删除 PR/run，不写 authority/confirmed，不新增 durable schema，不新增 PowerShell runtime logic。文档可继续把 implementation commit 与 remote run/job refs 写在同一句，parser 必须自行区分 scope。

验证结果：focused `go test ./internal/rekit/releasecheck -run "TestLatestBatchHandoffExtractsValidationEvidence|TestLatestBatchCommitRefsIgnoreRemoteRefsInSameEvidenceClause|TestLatestBatchRemoteGateDoesNotTreatNegativeGreenAsGreen|TestLatestBatchReleaseInspectionCadenceWaitsForImplementationCommit" -count=1` 已通过；完整本地 release minimum 已通过：`go run ./cmd/rekit -- -Command release-check -Format json` 返回 `ready=true` / `summary=release gate inventory ok`，`go run ./cmd/rekit -- -Command status`、`go run ./cmd/rekit -- -Command packs`、`go run ./cmd/rekit -- -Command doctor`、`go test ./...`、`go vet ./...` 均通过，`git diff --check` 仅保留 Windows 工作树 LF→CRLF 提示；implementation commit `2ac506a` 已推送并进入 PR #15。PR-triggered release-gate run `30187565571` completed failure，Windows/Linux/macOS jobs `89754882530`/`89754882569`/`89754882571` 均为既有 `steps=[]` / `runner_id=0` blocker，不能声明 remote CI green。

### Batch 611：Mission Commander project current action first-screen

状态：已完成 runtime/test/doc 工作树实现、focused releasecheck selector/parser 验证、focused CLI product-path 验证、完整本地 release minimum，以及 implementation commit/push 和 PR-triggered remote release-gate inspection；implementation commit `0fc49f0` 已推送并进入 PR #15，默认 kit-mode `status` / `status -Format text` 现已在 legacy text 路径投影 latest-batch project current action，并在 Mission Commander first-screen 里把 release handoff 的 next action 置于 case/reviewer/pack-memory 之后兜底。PR run `30190378620` completed failure，macOS/Windows/Linux jobs `89762460487`/`89762460488`/`89762460507` 均 `steps=[]` 且无 logs，仍属既有 runner/billing blocker；不为 release inspection record 自身追加第三个 inspection。

目标：补齐默认 `/rekit` 与 `status` 的第一屏接手断点：case/reviewer/pack-memory current action 已有 first-screen strip，但 kit-mode 默认入口在没有 case mission 时只显示 project handoff summary / remote gate / cadence，仍容易让新会话先看到 `focus=none` 或先翻 handoff 才知道下一步。需要把 latest-batch next action 直接变成 project-level Mission Commander current action，再让 case/reviewer/pack-memory 继续按既有优先级覆盖它。

已实现内容：

- `writeStatusMissionCommanderFirstScreenText` 新增 project-level current action 投影；当 case/reviewer/pack-memory 没有更高优先级 focus 时，会显示 `focus=project-current-action`，并输出 `scope=project` 的 current action 行、reason 与 boundary。
- `statusProjectHandoffCurrentAction` 复用 `LatestNextAction` / release inspection cadence next action 组装只读 `MissionCommanderNextActionItem`；`LatestRemoteReleaseGateDetail` 与 release inspection cadence boundary 只作为 reason/boundary，不把既有 `steps=[]` runner/billing blocker 升级成 hard blocked current action。
- kit-mode legacy `status` / 默认 `status` 现都会先投影 project current action，再继续输出现有 project handoff 细节；`status -Format text` 继续保留 case/reviewer/pack-memory first-screen 行为。
- CLI 回归扩展默认 `status` 与 `status -Format text` 的第一屏断言，锁定 `focus=project-current-action`、`scope=focus-project`、`source=releaseHandoffLatestBatch`、`requiresReview=` 与 latest-batch boundary 文案。
- README 与 agent-team usage 文档补充默认入口会先显示 latest-batch project current action，再回到 compact first-screen strip / queue current action 接手。

边界：本批只增强默认 status/Mission Commander first-screen 的只读 project current action projection 与测试覆盖；不执行远程 CI，不改变 release inspection cadence，不修改 workflow，不写 authority/confirmed，不新增 durable schema，不新增 PowerShell runtime logic。既有 `steps=[]` runner/billing blocker 继续作为 known gap 记录，不因此阻塞 Windows 本机产品路径。

验证结果：focused `go test ./internal/rekit/releasecheck -run TestLatestBatchSummarySelectsHighestBatchSection -count=1` 与 `go test ./internal/rekit/cli -run TestRunStatus -count=1` 已通过；完整本地 release minimum 已通过：`go run ./cmd/rekit -- -Command release-check -Format json` 返回 `ready=true` / `summary=release gate inventory ok`，`go run ./cmd/rekit -- -Command status`、`go run ./cmd/rekit -- -Command packs`、`go run ./cmd/rekit -- -Command doctor`、`go test ./...`、`go vet ./...` 均通过，`git diff --check` 仅保留 Windows 工作树 LF→CRLF 提示；implementation commit `0fc49f0` 已推送并进入 PR #15。PR-triggered release-gate run `30190378620` completed failure，macOS/Windows/Linux jobs `89762460487`/`89762460488`/`89762460507` 均为既有 `steps=[]` / `runner_id=0` blocker，不能声明 remote CI green。

### Batch 609：execution evidence review acknowledgement closure

状态：已完成 runtime/test/doc 工作树实现、focused mission/workstream/overview/CLI product-path 验证、完整本地 release minimum，以及 implementation commit/push 和 PR-triggered remote release-gate inspection；implementation commit `7896077` 已推送并进入 PR #15。PR run `30186884673` completed failure，Linux/Windows/macOS jobs `89753087844`/`89753087828`/`89753087808` 均 `steps=[]`、`runner_id=0` 且无 logs，仍属既有 runner/billing blocker。本批继续 Windows 本机 product-path 优先；远程 CI 若仍返回既有 blocker，不因此追加第三个 inspection record。

目标：补齐 Batch 580/603/594/602 后仍存在的 adapter downstream 断点：authorized adapter sidecar 经 valid=true validation/hash-bound record 后，observation evidence 已进入 ledger，status/overview/handoff/continue 会把 `executionEvidenceReview[]` 提升为 Mission Commander current action；但 review 完成后缺少 append-only acknowledgement 出口，exact recorded sidecar 还可能以 `evidence-already-recorded` adapter live snapshot 重新顶回 current action。replacement executor 只能反复看到 `/rekit handoff main` review queue，无法用既有 note ledger 明确关闭“已 review”状态。

已实现内容：

- `mission.ExecutionEvidenceReviewItemsWithLedgerFacts` 新增 ledger-aware projection，消费 observations + verifications + decisions；明确终结性的 related verification/decision note 会过滤已 review 的 observation evidence，旧 `ExecutionEvidenceReviewItems` 保持给 gate immediate / raw observation parsing 使用。
- Acknowledgement 规则 fail-closed：verification 必须 `status=resolved|accepted|rejected|confirmed|superseded` 且 `verdict=accepted|rejected`；decision 必须同样具备 closed status 且 `decision=accept|reject|supersede`。`open`、`deferred`、`needs_more_evidence`、`inconclusive`、仅 `target` 指向 evidence 或缺 terminal status 的 note 不关闭 review。
- acknowledgement IDs 会把同一 observation 的 `eventId` 与 `execution.gateEventId` 互相展开；主 Agent 可用 `-Related <observationEventId>` 或 `-Related <gateEventId>` 表达 review closure，并同时抑制 exact `evidence-already-recorded` adapter live snapshot 的 duplicate-record next action。
- status/overview/project handoff/lane handoff/continue 均改用 ledger-aware evidence review projection 与 acknowledged adapter-action merge；recorded handoff summary 仍保留只读 state/provenance，但不再作为 review-owned current action 回流。
- Nested no-pack/case-local adapter product-path 回归在 hash-bound adapter record 后追加 `note -Kind verification -WhatIf` → `-ExpectedNoteEventSha256` Apply，验证 acknowledgement 只追加 verifications ledger，不写 authority/confirmed，并使 status/handoff/continue 的 evidence review / duplicate-record current action 清零。

边界：本批只消费既有 append-only note ledger 来关闭已 review 的 observation evidence queue；不新增 durable schema，不 replay adapter/heavy tool，不重新 validate/record sidecar，不自动写 authority/confirmed/decision outcome，不改变 `gate -Apply` observation evidence 写入语义，不新增 PowerShell runtime logic。需要更多证据或 inconclusive 的 review 必须保持 queue open。

验证结果：focused `go test ./internal/rekit/mission -run TestExecutionEvidenceReviewItemsWithLedgerFactsHonorsRelatedReviewNotes` 已通过；focused `go test ./internal/rekit/cli -run TestRunGateAdapterReportNoPackProductPathFromNestedOutputWorkspace` 已通过；受影响 package `go test ./internal/rekit/mission ./internal/rekit/workstream ./internal/rekit/overview` 已通过；adapter product-path focused CLI `go test ./internal/rekit/cli -run "TestRunGateAdapterReport(NoPackProductPathFromNestedOutputWorkspace|BoundaryHitNoPackProductPathSuppressesContinue|TextOutputsNextActions|ReadOnlyPreflightFromCallerCwdBridge)"` 已通过；完整本地 release minimum 已通过：`go run ./cmd/rekit -- -Command release-check -Format json` 返回 `ready=true` / `summary=release gate inventory ok`，`go run ./cmd/rekit -- -Command status`、`go run ./cmd/rekit -- -Command packs`、`go run ./cmd/rekit -- -Command doctor`、`go test ./...`、`go vet ./...` 均通过，`git diff --check` 仅保留 Windows 工作树 LF→CRLF 提示；implementation commit `7896077` 已推送并进入 PR #15。PR-triggered release-gate run `30186884673` completed failure，Linux/Windows/macOS jobs `89753087844`/`89753087828`/`89753087808` 均为既有 `steps=[]` / `runner_id=0` blocker，不能声明 remote CI green。

### Batch 608：pack-memory first-screen runbook closure

状态：已完成 runtime/test/doc 工作树实现、focused installed entrypoint product-path test、broader CLI regression、完整本地 release minimum、implementation commit/push 与 implementation remote release-gate inspection；implementation commit `849e101` 已推送并进入 PR #14。PR run `30184059490` completed failure，Linux/Windows/macOS jobs `89745514664`/`89745514633`/`89745514666` 均 `steps=[]`、`runner_id=0` 且无 logs，仍属既有 runner/billing blocker，不能声明 remote CI green。本 release inspection record 仅记录该 implementation PR run；不要为 release inspection commit 自身 CI 追加第三个记录提交，除非出现不同于既有 `steps=[]` 的新远程信号。

目标：补齐 Batch 597/607 后仍存在的 pack-memory first-screen 接手断点：`status` / 默认 `/rekit` 第一屏已能聚焦 `pack-memory-current-action`，但只显示 current command/reason/boundary，没有把当前 pack 的 proof draft 与后续 verification/provisioning/retirement 接手顺序前置。replacement executor 仍需要向下翻完整 handoff 才能确认应先运行 proof draft WhatIf、再用返回的 `-ExpectedProofSha256` 做 Apply，随后才是 candidate verification / provisioning / retirement follow-up。

已实现内容：

- `writeStatusMissionCommanderFirstScreenText` 的 `pack-memory-current-action` 分支现在复用 `PackMemoryCandidates` 当前项，在 focus action 后立即输出 `status Mission Commander focus pack-memory runbook` 行，包含 pack、state、step 序号与只读 runbook text。
- 新增 `writeStatusMissionCommanderFirstScreenPackMemoryRunbookText` 小 helper，只做只读 text projection，不重新推导 pack-memory state、不复制 releasecheck runbook 逻辑；空 current pack 时保持静默。
- Installed case-local `/rekit` 产品路径回归扩展默认 `status -Format text` 断言，锁定 pack-memory first-screen 中的 proof draft WhatIf/ExpectedProofSha256 Apply 路径与后续 verification/provisioning/retirement 跟进提示；原有 lower-section pack-memory summary 与 current action 断言继续保留。

边界：本批只增强默认 status/Mission Commander first-screen 的只读 pack-memory runbook projection 与测试覆盖；不 merge、cleanup、provision、verify 或 retire pack-memory candidates，不写 proof/ledger/authority/confirmed，不新增 durable schema，不新增 PowerShell runtime logic。pack-memory 仍必须由主 Agent 按 WhatIf→hash-gated Apply 显式推进。

验证结果：focused `go test ./internal/rekit/cli -run "TestRunStatus|TestRunContinue|TestRunPlanSubagentsReviewerIntakeWhatIfApplyE2E|TestRunInstalled" -count=1` 已通过；exact installed entrypoint test `go test ./internal/rekit/cli -run "TestRunInstalledCaseShimProductPathStatusAndRefresh" -count=1` 已通过；完整本地 release minimum 已通过：`go run ./cmd/rekit -- -Command release-check -Format json` 返回 `ready=true`，`go run ./cmd/rekit -- -Command status`、`go run ./cmd/rekit -- -Command packs`、`go run ./cmd/rekit -- -Command doctor`、`go test ./...`、`go vet ./...` 均通过；`git diff --check` 仅保留 Windows 工作树 LF→CRLF 提示。implementation commit `849e101` 已推送；PR #14 run `30184059490` completed failure，Linux/Windows/macOS jobs `89745514664`/`89745514633`/`89745514666` 均 `steps=[]`、`runner_id=0` 且无 logs，仍为既有 runner/billing blocker；本地可验证 release gate 通过但不能声明 remote CI green。

### Batch 607：reviewer current-action first-screen runbook closure

状态：已完成 runtime/test/doc 工作树实现、focused installed entrypoint reviewer product-path test、focused CLI reviewer/status/continue tests、完整本地 release minimum、implementation commit/push 与 implementation remote release-gate inspection；implementation commit `92e1090` 已推送并进入 PR #13。PR run `30182807927` completed failure，Linux/Windows/macOS jobs `89742195513`/`89742195522`/`89742195524` 均 `steps=[]`、`runner_id=0` 且无 logs，仍属既有 runner/billing blocker，不能声明 remote CI green。本 release inspection record 仅记录该 implementation PR run；不要为 release inspection commit 自身 CI 追加第三个记录提交，除非出现不同于既有 `steps=[]` 的新远程信号。

目标：补齐 Batch 590/599/601 后仍存在的 reviewer first-screen 接手断点：reviewer dispatch/intake 的完整 `nextActionRunbookSteps` 已在 summary、status lower section、continue/handoff 与 durable runbook 中存在，默认 `/rekit` / `status` 第一屏也能聚焦 reviewer current action；但 first-screen focus strip 只显示 current command/reason/boundary，没有把当前 shard 的 capture-first runbook 前置。replacement executor 仍需要向下翻完整 handoff 才能确认应先调度 read-only reviewer、保存 symlink-free JSON input、运行 source capture WhatIf/hash-gated Apply，再 staging/collection/intake，第一屏接手仍不够闭环。

已实现内容：

- `writeStatusMissionCommanderFirstScreenText` 的 `reviewer-current-action` 分支现在复用 `ReviewerDispatchIntakeSummary.NextActionRunbookSteps`，在 focus action 之后立即输出 `status Mission Commander focus reviewer runbook` 行，包含 current shard、state、step 序号与 runbook text。
- 新增 `writeStatusMissionCommanderFirstScreenReviewerRunbookText` 小 helper，只做只读 text projection，不重新推导 reviewer state、不复制 workstream runbook 逻辑；空 `NextActionShardID` 时保持静默。
- Installed case-local `/rekit` 产品路径回归扩展默认 `status -Format text` 断言，锁定 reviewer focus strip 中的 `work from this first-screen handoff`、source capture preview 与 staging preview 步骤；原有 lower-section `status case mission reviewer dispatch next action runbook` 与 `continue` reviewer runbook 断言继续保留。

边界：本批只增强默认 status/Mission Commander first-screen 的只读 reviewer runbook projection 与测试覆盖；不 spawn、stop、poll 或 monitor reviewer，不创建 reviewer result，不执行 capture/staging/collection/intake，不写 facts/ledger/authority/confirmed，不新增 durable schema，不新增 PowerShell runtime logic。reviewer capture/staging/collection/intake 仍必须由主 Agent 按 WhatIf→hash-gated Apply 显式执行。

验证结果：focused `go test ./internal/rekit/cli -run "TestRunStatus|TestRunContinue|TestRunPlanSubagentsReviewerIntakeWhatIfApplyE2E|TestRunInstalled" -count=1` 已通过；exact installed entrypoint test `go test ./internal/rekit/cli -run "TestRunInstalledCaseShimProductPathStatusAndRefresh" -count=1` 已通过；完整本地 release minimum 已通过：`go run ./cmd/rekit -- -Command release-check -Format json` 返回 `ready=true`，`go run ./cmd/rekit -- -Command status`、`go run ./cmd/rekit -- -Command packs`、`go run ./cmd/rekit -- -Command doctor`、`go test ./...`、`go vet ./...` 均通过；`git diff --check` 仅保留 Windows 工作树 LF→CRLF 提示。implementation commit `92e1090` 已推送；PR #13 run `30182807927` completed failure，Linux/Windows/macOS jobs `89742195513`/`89742195522`/`89742195524` 均 `steps=[]`、`runner_id=0` 且无 logs，仍为既有 runner/billing blocker；本地可验证 release gate 通过但不能声明 remote CI green。

### Batch 606：pending-gate concrete review-first handoff closure

状态：已完成 runtime/test/doc 工作树实现、focused mission / CLI / gate pending-gate product-path tests、受影响 package tests、完整本地 release minimum、implementation commit/push 与 implementation remote release-gate inspection；implementation commit `a062add` 已推送并进入 PR #12。PR run `30181368510` completed failure，macOS/Windows/Linux jobs `89738355125`/`89738355149`/`89738355157` 均 `steps=[]`、`runner_id=0` 且 logs 404，仍属既有 runner/billing blocker，不能声明 remote CI green。本 release inspection record 仅记录该 implementation PR run；不要为 release inspection commit 自身 CI 追加第三个记录提交，除非出现不同于既有 `steps=[]` 的新远程信号。

目标：补齐 Batch 586/587/597 后仍存在的 pending-gate first-screen 接手断点：blocked continue/start/reconcile/status 已能投影 `pendingGateHandoffs[]` 的 concrete `gate -Action ... -WhatIf/-Apply` handoff，但 Mission Commander lane action 仍把 pending-gate primary 降级为 `/rekit handoff <lane>` 或 `<action>` 占位符。replacement executor 在默认 `/rekit` / `status` 第一屏看到 pending-gate blocker 时，仍需要回到 handoff/ledger 才能知道应先 review 哪个 concrete gate preview，容易把 review-first gate decision 流程拆散。

已实现内容：

- `LaneExecutorAction` 现在把当前 lane 的 pending-gate request 明细传入 `LaneMissionCommanderActionForLane`；当只有单条 pending-gate 且 `gate.action` 明确时，Mission Commander primary action 直接变为 `/rekit gate <action> -Lane <lane> -WhatIf`，bounded `-Apply`、`continue -WhatIf` 与 `handoff` 只作为 follow-up。
- `MissionCommanderNextActions` 将 `needs-gate-decision` 的 `-WhatIf` 视为可执行 review current action：queue first-screen 会把 concrete gate preview 排到 current action，而不是把 blocked Apply、blocked continue 或 handoff 当成下一步。
- 多条 pending-gate 或 action 不完整时继续 fail-closed：primary 保持 handoff，follow-up 只列出可识别 action 的 concrete `gate ... -WhatIf` preview，并在无法一一识别时保留 `<action>` 占位 preview 作为人工选择边界。
- CLI/gate/status 回归同步覆盖 pending-gate continue/reconcile/gate Apply/text/status 产品路径，断言 JSON/text current action 使用 concrete `/rekit gate debug -Lane ... -WhatIf`，bounded Apply 保持 follow-up，且单 concrete gate 不再泄漏 `/rekit gate <action>` 占位符。

边界：本批只增强 pending-gate 的只读 Mission Commander action/queue/current-action 投影与测试覆盖；`gate -WhatIf` 仍只读，`gate -Apply` 仍只记录 pending-gate / authorized-gate request decision 或 bounded execution observation evidence，不执行 heavy-tool、不批准 heavy action、不写 authority/confirmed；多 gate/缺 action 场景继续 handoff review-first，不猜测 action；不新增 durable schema，不新增 PowerShell runtime logic。

验证结果：focused `go test ./internal/rekit/mission -count=1` 已通过；focused CLI pending-gate 产品路径 `go test ./internal/rekit/cli -run "TestRunContinueBlocksPendingGateBeforeWrites|TestRunReconcileApplyProjectsGateDecisionHandoffsAfterInterventionResolution|TestRunGateApplyAppendsPendingGateRequest|TestRunGateTextOutputsExecutorActions|TestRunStatusCaseMissionPromotesPendingGateWhatIfCurrentAction" -count=1` 已通过；受影响 package `go test ./internal/rekit/mission ./internal/rekit/cli ./internal/rekit/gate ./internal/rekit/releasecheck -count=1` 已通过；完整本地 release minimum 已通过：`go run ./cmd/rekit -- -Command release-check -Format json` 返回 `ready=true`，`go run ./cmd/rekit -- -Command status`、`go run ./cmd/rekit -- -Command packs`、`go run ./cmd/rekit -- -Command doctor`、`go test ./...`、`go vet ./...` 与 `git diff --check` 均通过（仅保留 Windows 工作树 LF→CRLF 提示）。implementation commit `a062add` 已推送；PR #12 run `30181368510` completed failure，macOS/Windows/Linux jobs `89738355125`/`89738355149`/`89738355157` 均 `steps=[]`、`runner_id=0` 且 logs 404，仍为既有 runner/billing blocker；本地可验证 release gate 通过但不能声明 remote CI green。

### Batch 605：release handoff multi-commit ref completeness closure

状态：已完成 runtime/test/doc 工作树实现、focused release handoff commit-ref parser tests、真实 `status` / `release-check -Format json` commitRefs 复核、完整本地 release minimum、implementation commit/push 与 implementation remote release-gate inspection；implementation commit `0428930` 已推送并进入 PR #11。PR run `30179936468` completed failure，Windows/macOS/Linux jobs `89734693035`/`89734693042`/`89734693051` 均 `steps=[]`、`runner_id=0` 且无 logs，仍属既有 runner/billing blocker，不能声明 remote CI green。本 release inspection record 仅记录该 implementation PR run；不要为 release inspection commit 自身 CI 追加第三个记录提交，除非出现不同于既有 `steps=[]` 的新远程信号。

目标：补齐 Batch 604 暴露出的 multi-implementation-commit handoff 断点：`status` / `release-check` 会把 latest-batch implementation commit refs 投影到 first-screen 和 JSON handoff，但旧 `looksLikeCommitRef` 为避免 remote run/job 数字误判，要求 token 必须包含 a-f 字母。合法 Git 短 SHA 可以全是数字，例如 Batch 604 的 `9887297`，导致 replacement executor 从 first-screen 只能看到 `eb3c238` / `44de375`，漏掉中间 implementation fix commit，release inspection 与 commit evidence chain 不完整。

已实现内容：

- `latestBatchCommitRefs` 改为只遍历 scoped `latestBatchEvidenceClauses` 中的 implementation commit/push evidence clause，再提取该 clause 内 backtick hex token；`looksLikeCommitRef` 允许 7-40 位 hex token（包括全数字短 SHA）。
- 新增 `latestBatchCommitEvidenceClause` / `backtickTokens` helper，继续排除 `do not` / `不要` / `不为` policy/boundary clause，并避免从 remote run/job evidence clause 提取 run ID 或 job ID。
- Releasecheck 回归 fixture 覆盖 `implementation commits `abc123d` / `9887297`` 与 remote run `123456789` 同句场景，断言全数字 short SHA 被保留、remote run ID 不进入 `commitRefs`。
- 真实 product-path 复核：`status` 现在对 Batch 604 输出 `status latest batch commit：eb3c238`、`9887297`、`44de375`；`release-check -Format json` 的 `releaseHandoff.latestBatch.handoff.commitRefs` 同步返回三项。

边界：本批只增强 release/status read-only latest-batch implementation commit evidence parsing 与 first-screen/JSON handoff completeness；不执行远程 CI，不改变 release inspection cadence，不修改 workflow，不创建或删除 PR/run，不写 authority/confirmed，不新增 PowerShell runtime logic。

验证结果：focused `go test ./internal/rekit/releasecheck -run "TestLatestBatchHandoffExtractsValidationEvidence|TestLatestBatchReleaseReadinessPrefersExplicitRemoteEvidenceOverStalePendingText|TestLatestBatchRemoteGateRecognizesEqualsEmptyStepsAndChineseNegativeGreen" -count=1` 已通过；package `go test ./internal/rekit/releasecheck -count=1` 已通过；真实 `status` 与 `release-check -Format json` commitRefs 复核已通过；完整本地 release minimum 已通过：`go run ./cmd/rekit -- -Command release-check -Format json` 返回 `ready=true`，`go run ./cmd/rekit -- -Command status`、`go run ./cmd/rekit -- -Command packs`、`go run ./cmd/rekit -- -Command doctor`、`go test ./...`、`go vet ./...` 与 `git diff --check` 均通过（仅保留 Windows 工作树 LF→CRLF 提示）。implementation commit/push 与远程 release-gate inspection 待最终执行。

### Batch 604：release readiness evidence-scope parser closure

状态：已完成 runtime/test/doc 工作树实现、focused release readiness parser tests、受影响 package tests、完整本地 release minimum、implementation commit/push 与 final implementation remote release-gate inspection；implementation commits `eb3c238` / `9887297` / `44de375` 已推送并进入 PR #10。最终 PR run `30179376831` completed failure，Windows/Linux/macOS jobs `89733303757`/`89733303769`/`89733303806` 均 `steps=[]`、`runner_id=0` 且无 logs，仍属既有 runner/billing blocker，不能声明 remote CI green。本 release inspection record 仅记录最终 implementation PR run；不要为 release inspection commit 自身 CI 追加第三个记录提交，除非出现不同于既有 `steps=[]` 的新远程信号。

目标：补齐 Batch 603 暴露出的 release readiness parser 真实断点：latest-batch parser 会从 `docs/batch-plan.md` 的状态/验证结果短文本推导 Mission Commander release handoff，但此前仍以全文关键词识别 `commit/push`、`release inspection`、`release-gate run`、`steps=[]`。当当前批次为了说明 push cadence 或 known blocker 而写入 policy/boundary 文案时，parser 可能把尚未提交/尚未检查远程 run 的批次误判为已 inspection 或 blocked remote run。release/status first-screen 应只从明确 evidence/status 句推导 readiness，不让 policy guidance 伪造远程状态。

已实现内容：

- `latestBatchRemoteReleaseGate` 改为先用 `latestBatchEvidenceClauses` 将状态/验证结果按句切分，再由 `latestBatchRemoteEvidenceText` 只保留明确 remote run/job/completed/explicit green evidence；pending remote inspection 仍优先返回 `not-recorded`。
- `latestBatchRemoteReleaseGateDetail`、remote job/run refs、`steps=[]` evidence 与 cadence 的 `remote release-gate steps=[] blocker recorded` 统一消费 scoped remote evidence text，不再从 policy/boundary 句抓取 jobs、run refs 或 empty steps。
- `latestBatchImplementationCommitReady` 改为 evidence 优先：只要存在“已推送/已提交并推送/implementation commit(s) `<sha>`/recorded”类 evidence clause 即标记 implementation ready；没有 pushed evidence 时，pending wording 才保持 implementation-pending。`latestBatchInspectionCommitReady` 只在 scoped remote gate evidence 非 `not-recorded` 时标记 inspection ready。
- Releasecheck 回归覆盖：local validation 已完成但未提交/未检查远程 run 时保持 `implementation-pending`；implementation commit 已推送但远程 run 尚未检查时保持 `inspection-pending`；policy-only `steps=[]`/`no third inspection` 不产生 remote evidence；已有 Batch 603 completed `steps=[]` inspection record 仍解析为 complete blocked state。

边界：本批只增强 release/status read-only latest-batch handoff truthfulness；不执行远程 CI，不修改 workflow，不改变 release gate inventory，不创建或删除 PR/run，不写 authority/confirmed，不新增 PowerShell runtime logic。

验证结果：focused `go test ./internal/rekit/releasecheck -run "TestLatestBatch|TestReleaseHandoffInventoryFromRepo" -count=1` 已通过；受影响 package `go test ./internal/rekit/releasecheck ./internal/rekit/cli -count=1` 已通过；完整本地 release minimum 已通过：`go run ./cmd/rekit -- -Command release-check -Format json` 返回 `ready=true`，`go run ./cmd/rekit -- -Command status`、`go run ./cmd/rekit -- -Command packs`、`go run ./cmd/rekit -- -Command doctor`、`go test ./...`、`go vet ./...` 与 `git diff --check` 均通过（仅保留 Windows 工作树 LF→CRLF 提示）。read-only status 同时验证：当前 Batch 604 在最终 parser fix 尚未提交推送/尚未检查远程 workflow run 时保持 `implementation-pending` / `remoteReleaseGate=not-recorded`；explicit remote run evidence 优先于 stale pending wording 的 completed blocker 场景由 focused tests 覆盖。

### Batch 603：execution evidence review runbook closure

状态：已完成 runtime/test/doc 工作树实现、focused execution evidence review tests、受影响 package tests、完整本地 release minimum、implementation commit/push 与 implementation remote release-gate inspection；implementation commit `67c8199` 已推送并进入 PR #9。PR run `30178318236` completed failure，macOS/Linux/Windows jobs `89730638005`/`89730638010`/`89730638018` 均 `steps=[]`、`runner_id=0` 且无 logs，仍属既有 runner/billing blocker，不能声明 remote CI green。本 release inspection record 仅记录该 implementation PR run；不要为 release inspection commit 自身 CI 追加第三个记录提交，除非出现不同于既有 `steps=[]` 的新远程信号。

目标：补齐 Batch 580/536 后仍存在的 execution evidence review 接手断点：bounded observation evidence 已记录后，status/overview/handoff 能显示 review queue、summary、follow-through 与 boundary，但 replacement executor 仍需要从多处字段拼接“review evidence → no-heavy/no-authority → handoff/continue”的具体步骤。Mission Commander first-screen、handoff Markdown 与 resume/checkpoint 应直接投影有序 runbook，让主 Agent 或替换 executor 不打开 observations ledger、完整 sidecar 或 follow-through JSON 也能安全接手。

已实现内容：

- `mission.ExecutionEvidenceReviewItem` 新增只读 `reviewRunbookSteps[]`，由 `ExecutionEvidenceReviewRunbookSteps` 从 `gateEventId` / `reviewCommand`、recorded execution report path + SHA-256、`outputRefs`、`evidenceRefs`、boundary/escalation 状态、no-heavy/no-authority boundary 与 Mission Commander handoff/follow-up commands 派生；需要 main review 的 evidence 会过滤 autonomous `/rekit continue` follow-up。
- Evidence item 构建、workstream lane-specific continue command rebind、gate duplicate/immediate execution evidence review override 均在 Mission Commander action/follow-through 更新后重算 `reviewRunbookSteps[]`，避免 action 覆盖后 runbook stale。
- `status` case mission text、overview text、generic execution evidence review text、overview JSON、project/lane handoff Markdown、lane `RESUME.md` 与 checkpoint/digest text 同步输出完整 runbook steps；runbook 输出不再使用 tail `LimitStrings` 截断，保证 step 1 始终保留 review evidence/currentness 入口。
- CLI/product-path 回归扩展 execution evidence review JSON/text 断言，覆盖 succeeded observation evidence、duplicate/already-recorded evidence、boundary/escalated evidence、project/lane handoff Markdown 与 Mission Commander next action suppression。
- `release-check` / `status` latest-batch parser 同步修复 policy-only `steps=[]` wording 误判：当当前批次已完成本地验证但尚未创建代码提交、尚未检查对应远程 workflow run 时，remote gate 保持 `not-recorded`，release inspection cadence 保持 `implementation-pending`，不从 boundary 文案伪造 inspection evidence。

边界：本批只增强已记录 observation evidence 的只读 downstream/durable runbook projection；不 replay adapter/heavy tool，不自动 validate/record sidecar，不改变 `gate -Apply` observation evidence 写入语义，不写 authority/confirmed，不新增 durable schema，不新增 PowerShell runtime logic。

验证结果：focused `go test ./internal/rekit/cli -run "TestRunStatusCaseMissionIncludesExecutionEvidenceReview|TestRunOverviewJsonIncludesMissionCommanderActionQueue|TestRunGoGateApplyAppendsAuthorizedGateRequestVisibility" -count=1` 已通过；focused release parser `go test ./internal/rekit/releasecheck -run "TestLatestBatchReleaseInspectionCadenceWaitsForImplementationCommit|TestLatestBatchRemoteGateDoesNotTreatNegativeGreenAsGreen|TestLatestBatchRemoteGateIgnoresPolicyOnlyEmptyStepsBeforeInspection|TestLatestBatchRemoteGateRecognizesEqualsEmptyStepsAndChineseNegativeGreen" -count=1` 已通过；受影响 package `go test ./internal/rekit/mission ./internal/rekit/workstream ./internal/rekit/gate ./internal/rekit/overview ./internal/rekit/cli ./internal/rekit/releasecheck -count=1` 已通过；完整本地 release minimum 已通过：`go run ./cmd/rekit -- -Command release-check -Format json` 返回 `ready=true`，`go run ./cmd/rekit -- -Command status`、`go run ./cmd/rekit -- -Command packs`、`go run ./cmd/rekit -- -Command doctor`、`go test ./...`、`go vet ./...` 与 `git diff --check` 均通过（仅保留 Windows 工作树 LF→CRLF 提示）。

### Batch 602：adapter currentness handoff closure

状态：已完成 runtime/test/doc 工作树实现、focused adapter currentness tests、受影响 package tests、完整本地 release minimum、implementation commit/push 与 implementation remote release-gate inspection；implementation commit `bd34204` 已推送并进入 PR #8。PR run `30177220326` completed failure，Windows/macOS/Linux jobs `89727842288`/`89727842311`/`89727842312` 均 `steps=[]` 且无 logs，仍属既有 runner/billing blocker，不能声明 remote CI green。本 release inspection record 仅记录该 implementation PR run；不要为 inspection commit 自身 CI 追加第三个记录提交，除非出现不同于既有 `steps=[]` 的新远程信号。

目标：补齐 Batch 594/580 后仍暴露的 authorized adapter report 接手断点：底层 record Apply 已支持 `-ExpectedExecutionReportSha256`，但 `gate -ExecutionReportContract`、scaffold/draft preview/apply、status/workstream handoff 与 text/JSON 消费者仍可能在 pre-validation 阶段复制裸 record command/args。Mission Commander 与 replacement executor 应先运行 read-only validation；只有 valid validation/status 绑定当前 sidecar hash 后，才暴露包含 `-ExpectedExecutionReportSha256` 的 record current action。

已实现内容：

- `AdapterReportLiveValidation`、contract summary、authorized execution follow-through 与 contract Mission Commander action/next actions 改为 validation-only pre-validation handoff：`RecordArgs` / `CaseRelativeRecordArgs` / record command 字段在 contract 阶段保持 empty/omitempty，text 输出改为“valid=true 后使用 validation/status returned hash-bound record command”。
- Scaffold/draft preview/apply 不再填充 `RecordCommand`，Mission Commander action queue 删除 `adapterReportScaffold.record` / `adapterReportDraft.record` blocked action，只保留 apply/validate/handoff；next steps 明确先 run read-only validation，再用 validation/status 返回的 hash-bound record command。
- Status、authorized gate adapter handoff Markdown 与 CLI text merge valid live snapshot 时，只在 `validation.Valid && recordReady && !recordBlocked` 时从 `validation.MissionCommanderAction.PrimaryCommand` 投影 record command，并要求该 command 带 `-ExpectedExecutionReportSha256`；否则显示 guidance note 而不是可运行裸 record。
- CLI/gate/workstream tests 同步覆盖 contract/scaffold/draft pre-validation 不暴露 runnable record Apply、overview/status/handoff helper 不再要求 `CaseRelativeRecordCommand`、nested case-local/no-pack product path 与 `generic-binary-re` adapter candidate record 路径先读取 validation 的 `recordExpectedReportSha256` 再 record。

边界：本批只收紧 authorized adapter report handoff currentness 与 Mission Commander 可复制命令；不删除底层 legacy record Apply 兼容执行入口，不自动 validate/record，不执行 adapter/heavy tool，不写 authority/confirmed，不新增 durable schema，不新增 PowerShell runtime logic。

验证结果：focused `go test ./internal/rekit/gate -run "TestAdapterReportContractDescribesAuthorizedGateBoundaries|TestScaffoldAdapterExecutionReportPreviewApplyAndReplay|TestDraftAdapterExecutionReportPreviewApplyReplayAndScaffoldReplace|TestValidateAdapterExecutionReport" -count=1` 已通过；focused CLI adapter tests 已通过；受影响 package `go test ./internal/rekit/gate ./internal/rekit/cli ./internal/rekit/workstream -count=1` 已通过；完整本地 release minimum 已通过：`go run ./cmd/rekit -- -Command release-check -Format json` 返回 `ready=true`，`go run ./cmd/rekit -- -Command status`、`go run ./cmd/rekit -- -Command packs`、`go run ./cmd/rekit -- -Command doctor`、`go test ./...`、`go vet ./...` 与 `git diff --check` 均通过（仅保留 Windows 工作树 LF→CRLF 提示）。

### Batch 601：capture-first reviewer handoff closure

状态：已完成 runtime/test/doc 工作树实现、focused reviewer product-path tests、受影响 package tests、完整本地 release minimum、implementation commit/push 与 implementation remote release-gate inspection；implementation commit `19d6db6` 已推送。implementation run `30175252468` completed failure，macOS/Windows/Linux jobs `89722869546`/`89722869553`/`89722869556` 均 `steps=[]`，仍属既有 runner/billing blocker，不能声明 remote CI green。本 release inspection record 仅记录该 implementation run；不要为 inspection commit 自身 CI 追加第三个记录提交，除非出现不同于既有 `steps=[]` 的新远程信号。

目标：补齐 Batch 600 后的下游接手断点：runtime 已提供 `plan-subagents -CaptureReviewerResultSource`，但 fresh plan terminal、reviewer orchestration summary、status/handoff/continue runbook 与 Mission Commander current action 仍可能把 replacement executor 引回旧的“保存 reviewer JSON/source 后直接 staging”路径。新会话第一屏应明确先保存 symlink-free case-local input，运行 capture WhatIf，复核 `inputSha256` 后 hash-gated Apply 发布 packet-derived source，再进入 staging、collection 与 intake。

已实现内容：

- Fresh `ReviewerResultStagingCommands` 兼容新增 `sourceCaptureInput`、`sourceCaptureCommand` 与 `sourceCaptureApply`，plan terminal 现在先打印 `plan-subagents reviewer source capture command`，再打印 staging command；`reviewerOrchestration.scope`、lifecycle、shard next action、dispatch command 与 reviewer prompt 都改为 capture-first，不再要求主 Agent 手写 `reviewerStagingCommands.sourcePath`。
- Downstream `ReviewerDispatchIntakeHandoff` / compact summary 兼容新增 source capture preview/apply command 字段；status/handoff/continue、durable runbook 与 Mission Commander reviewer queue 的 waiting shard next action 现在显示“保存 reviewer JSON input → source capture preview → expected-input-hash Apply → staging preview”的最小接续链路。
- Workstream 对 legacy packet 保留 fallback 派生 capture preview/apply 命令；fresh canonical packet 继续从 packet-derived source/candidate/canonical bindings 重建 staging/collection command，invalid/forged source/candidate 仍 fail-closed，不自动覆盖或绕过 collection。
- CLI text 输出同步在 reviewer dispatch next action、per-item reviewer result source 与 plan shard handoff 中显示 source capture preview/apply，installed case-local `/rekit` product-path 覆盖 partial reviewer intake 后继续阻塞普通 `continue`，并把 remaining shard 的 first-screen runbook 指向 capture-first。
- `.claude/skills/rekit/SKILL.md` 同步更新 reviewer path 说明，避免技能入口继续描述 Batch 562 的手写 bounded source 路径。

边界：本批只增强 transient handoff/runbook/terminal guidance 与兼容新增 optional packet/summary 字段；不新增 required durable schema，不执行 reviewer spawn/stop/poll/monitor，不创建 reviewer result/candidate/canonical artifact，不执行 capture/staging/collection/intake，不写 facts/ledger/authority/confirmed，不新增 PowerShell runtime logic。legacy direct/noncanonical packet intake 兼容路径保留。

验证结果：focused `go test ./internal/rekit/workstream ./internal/rekit/cli -run "TestReviewerDispatchIntake|TestRunPlanSubagentsReviewerOrchestrationE2E|TestRunInstalledCaseShimProductPathStatusAndRefresh|TestRunPlanSubagentsReviewerIntakeWhatIfApplyE2E" -count=1` 已通过；受影响 package `go test ./internal/rekit/subagents ./internal/rekit/workstream ./internal/rekit/cli -count=1` 已通过；完整本地 release minimum 已通过：`go run ./cmd/rekit -- -Command release-check -Format json` 返回 `ready=true`，`go run ./cmd/rekit -- -Command status`、`go run ./cmd/rekit -- -Command packs`、`go run ./cmd/rekit -- -Command doctor`、`go test ./...`、`go vet ./...` 与 `git diff --check` 均通过（仅保留 Windows 工作树 LF→CRLF 提示）。

### Batch 600：reviewer result source capture product-path closure

状态：已完成 runtime/test/doc 工作树实现、focused product-path tests、完整本地 release minimum、implementation commit/push 与 implementation remote release-gate inspection；implementation commit `e8e085a` 已推送。implementation run `30173906579` completed failure，macOS/Linux/Windows jobs `89719477720`/`89719477726`/`89719477756` 均 `steps=[]`，仍属既有 runner/billing blocker，不能声明 remote CI green。本 release inspection record 仅记录该 implementation run；不要为 inspection commit 自身 CI 追加第三个记录提交，除非出现不同于既有 `steps=[]` 的新远程信号。

目标：补齐 Batch 562/558 后仍存在的 reviewer result source 写入断点：fresh canonical packet 已提供 packet-derived `reviewerStagingCommands.sourcePath`，`-StageReviewerResult` 也要求 source path 精确匹配该 packet-derived path，但主 Agent 仍需手工把 read-only reviewer 返回的唯一 JSON 写入 `results/sources/<shard>.json`。这一步在 deterministic `/rekit` WhatIf→Apply 链外，容易写错 shard/source、不能 hash-gated apply，也不能在产品路径中证明 preview 不写入与 Apply exact replay。

已实现内容：

- `plan-subagents` 新增独立 `-CaptureReviewerResultSource` 模式，接收 `-PacketPath`、`-ShardId`、`-ReviewerResultInputPath`、`-Lane` 与 `-Actor`；WhatIf stable 读取 symlink-free case-local non-empty regular reviewer JSON input，复用 strict reviewer result validator 绑定 packet/route/shard/items/evidence，并返回 input SHA-256、packet-derived source path 与 hash-bound Apply command。
- Capture Apply 必须携带 WhatIf 返回的 `-ExpectedReviewerResultInputSha256`，在共享 packet/shard mutation lock 内重读 packet/input、重验 packet integrity/currentness，并以 no-overwrite exact publication 写入 packet-derived `reviewerStagingCommands.sourcePath`；input drift、different existing source、source/case namespace symlink 或 packet/source binding drift 均 fail-closed，exact replay 幂等。
- Capture 只发布 bounded source artifact；不执行 staging、collection 或 reviewer intake，不 spawn/stop/poll/monitor reviewer，不写 facts/ledger/authority/confirmed。Apply 后 Mission Commander action 指向独立 staging preview，让 source capture→staging→collection→intake 保持四个显式 WhatIf/Apply 边界。
- CLI parse/dispatch/text 输出新增 capture flags 与 `plan-subagents reviewer result source capture` terminal lines；旧 `-ReviewerResultSourcePath` / `-ExpectedSourceSha256` 仍只属于 staging，新的 `-ReviewerResultInputPath` / `-ExpectedReviewerResultInputSha256` 只属于 capture，避免 input hash 与 source hash 语义混用。
- CLI reviewer product-path helper 不再手工写 packet-derived source：测试先写 case-local input，再运行 capture WhatIf→expected-input-hash Apply，确认 preview zero-write、Apply exact bytes 到 source，然后继续既有 staging WhatIf→expected-source-hash Apply、collection WhatIf→Apply 与 reviewer intake。

边界：本批只新增 case-local bounded reviewer result source artifact capture；不创建 reviewer result candidate/canonical result，不 intake reviewer writeback，不执行 reviewer/adapter/heavy tool，不写 facts/ledger/authority/confirmed，不新增 durable schema，不新增 PowerShell runtime logic。legacy staging 兼容路径保留，但 fresh packet product path 应优先使用 capture 再 staging。

验证结果：focused `go test ./internal/rekit/subagents ./internal/rekit/cli -count=1` 已通过；完整本地 release minimum 已通过：`go run ./cmd/rekit -- -Command release-check -Format json` 返回 `ready=true`，`go run ./cmd/rekit -- -Command status`、`go run ./cmd/rekit -- -Command packs`、`go run ./cmd/rekit -- -Command doctor`、`go test ./...`、`go vet ./...` 与 `git diff --check` 均通过（仅保留 Windows 工作树 LF→CRLF 提示）。

### Batch 599：reviewer dispatch current-action queue first-screen closure

状态：已完成 runtime/test/doc 工作树实现、focused product-path test、完整本地 release minimum、implementation commit/push 与 implementation remote release-gate inspection；implementation commit `cc30646` 已推送。implementation run `30172310025` completed failure，Linux/Windows/macOS jobs `89715369696`/`89715369698`/`89715369707` 均 `steps=[]`，仍属既有 runner/billing blocker，不能声明 remote CI green。本 release inspection record 仅记录该 implementation run；不要为 inspection commit 自身 CI 追加第三个记录提交，除非出现不同于既有 `steps=[]` 的新远程信号。

目标：补齐 Batch 597/598 后仍暴露的 reviewer orchestration first-screen 断点：`overview` / `status` 的全局 `MissionCommanderActionQueue` 已能把 reviewer dispatch/intake action 排为 case current action，且 `reviewerDispatchIntakeSummary.nextActionRunbookSteps[]` 已给出当前 shard runbook，但 default `/rekit` 第一屏没有单独的 reviewer work queue/current action 概览。replacement executor 在多 packet / shard handoff 中仍要从 full handoff rows 推断哪个 reviewer packet/shard 是当前工作。

已实现内容：

- `statusCaseMission` 新增只读 `reviewerDispatchIntakeActionQueue` JSON 字段，从既有 `ReviewerDispatchIntakeHandoffs` 复用 `workstream.MissionCommanderNextActionsWithReviewerDispatches(nil, ...)` 与 `mission.MissionCommanderActionQueueFor` 派生 reviewer-only current action；不新增 reviewer packet schema，也不复制 packet/shard priority 规则。
- `writeStatusMissionCommanderFirstScreenText` 在 first-screen strip 中新增 `reviewerCurrent`、`reviewerQueueTotal`、`reviewerQueueBlocked`、`reviewerQueueRequiresReview`，并在全局 case current action 本身来自 `reviewerDispatchIntakeHandoffs` 时把 focus 显示为 `reviewer-current-action`，同时输出 focus-reviewer 与 reviewer current action details。
- `status -Format text` 新增 `status case mission reviewer dispatch queue` 与 per-bucket action/reason/boundary 行，和既有 `reviewer dispatch intake summary` / `next action runbook` 并列，帮助新会话直接定位当前 packet/shard 的 bounded reviewer handoff。
- Installed case-local `/rekit` product-path E2E 扩展 JSON/text 断言：partial reviewer intake 后 reviewer-only queue current action 与全局 case current action一致，first-screen 聚焦 reviewer current action，并保留 runbook/staging guidance。

边界：本批只增强 status/default `/rekit` 的只读 reviewer dispatch/intake current-action projection；不 spawn/stop/poll/monitor reviewer，不创建 reviewer result，不执行 staging/collection/intake，不写 facts/ledger/authority/confirmed，不新增 durable schema，不新增 PowerShell runtime logic。

验证结果：focused `go test ./internal/rekit/cli -run "TestRunInstalledCaseShimProductPathStatusAndRefresh" -count=1` 已通过；完整本地 release minimum 已通过：`go run ./cmd/rekit -- -Command release-check -Format json` 返回 `ready=true`，`go run ./cmd/rekit -- -Command status`、`go run ./cmd/rekit -- -Command packs`、`go run ./cmd/rekit -- -Command doctor`、`go test ./...`、`go vet ./...` 与 `git diff --check` 均通过（仅保留 Windows 工作树 LF→CRLF 提示）。

### Batch 598：empty-case onboarding current action closure

状态：已完成 runtime/test/doc 工作树实现、focused tests、完整本地 release minimum、implementation commit/push 与 implementation remote release-gate inspection；implementation commit `6a3f130` 已推送。implementation run `30171293954` completed failure，macOS/Linux/Windows jobs `89712780321`/`89712780326`/`89712780336` 均 `steps=[]`，仍属既有 runner/billing blocker，不能声明 remote CI green。本 release inspection record 仅记录该 implementation run；不要为 inspection commit 自身 CI 追加第三个记录提交，除非出现不同于既有 `steps=[]` 的新远程信号。

目标：修复 Batch 597 后仍暴露的新 case / 空 mission 接手断点：case-local 默认 `/rekit` first-screen 在 `.rekit/board.json` 缺失时可能显示 `focus=none`，只在后续 generic `missionBriefNextActions[]` 中提示运行 overview。新会话或 replacement executor 需要第一屏直接看到最小可执行 onboarding current action，先初始化 bounded case-local Mission Commander board，再回到 `/rekit` 选择 `continue main` 或 `start <name>`。

已实现内容：

- `buildStatusCaseMission` 的 missing-board 分支新增只读 `caseMissionOnboarding` action，并用现有 `mission.MissionCommanderActionQueueFor` 派生 `MissionCommanderActionQueue` 与 `MissionCommanderNextActions`；default `/rekit` / `status -Format text` 因此会在 first-screen strip 聚焦 `case-current-action`，显示 `/rekit overview -Target ... -Format text`、reason 与 boundary。
- `MissionBriefNextActions` 改为先输出 `follow Mission Commander current action: /rekit overview -Target ... -Format text`，再提示 board 初始化后重新运行 `/rekit` 并选择 `/rekit continue main` 或 `/rekit start <name> -WhatIf -Format text`，避免空 mission 让用户在 generic next steps 中找入口。
- Missing-board JSON/text 回归测试扩展为断言 onboarding current action、queue counts、first-screen focus、reason/boundary 与 status zero-write；installed case-local `/rekit` product-path E2E 也覆盖初始新 case first-screen 不再是 `focus=none`。
- README、canonical skill 与 case-shim template 同步说明默认 `/rekit` first-screen 会先显示 current/onboarding action strip。

边界：本批只增强 status/default `/rekit` 的只读 onboarding handoff projection；不让 `status` 初始化 board，不自动 start/continue/apply，不执行 reviewer/adapter/heavy tool，不写 facts/ledger/authority/confirmed，不新增 durable schema，不新增 PowerShell runtime logic。

验证结果：focused `go test ./internal/rekit/cli -run "TestRunStatusCaseMissionDoesNotInitializeMissingBoard|TestRunInstalledCaseShimProductPathStatusAndRefresh" -count=1` 已通过；完整本地 release minimum 已通过：`go run ./cmd/rekit -- -Command release-check -Format json` 返回 `ready=true`，`go run ./cmd/rekit -- -Command status`、`go run ./cmd/rekit -- -Command packs`、`go run ./cmd/rekit -- -Command doctor`、`go test ./...`、`go vet ./...` 与 `git diff --check` 均通过（仅保留 Windows 工作树 LF→CRLF 提示）。

### Batch 597：Mission Commander first-screen current action strip

状态：已完成 runtime/test/doc 工作树实现、focused tests、完整本地 release minimum、implementation commit/push 与 implementation remote release-gate inspection；implementation commit `bb90da2` 已推送。implementation run `30169713088` completed failure，macOS/Windows/Linux jobs `89708686973`/`89708686980`/`89708687021` 均 `steps=[]`，仍属既有 runner/billing blocker，不能声明 remote CI green。本 release inspection record 仅记录该 implementation run；不要为 inspection commit 自身 CI 追加第三个记录提交，除非出现不同于既有 `steps=[]` 的新远程信号。

目标：补齐 Batch 596 后仍存在的新会话接手 first-screen 断点：case-local 无参数 `/rekit` 默认仍走 table/legacy status 输出，虽然内部已有 queue-aware case current action 与 pack-memory action queue，但用户/替换 executor 必须在 case shim、case mission、project handoff、pack-memory 等大量行中手工寻找真正下一步。默认第一屏应在最靠前位置给出一个 compact Mission Commander strip：同时显示 case current action、pack-memory current action、焦点来源与 counts；当 reviewer/adapter/evidence 等 case current action 需要复核时优先聚焦 case，当 case ready 但 pack-memory 仍有 open residue 时聚焦 pack-memory bounded proof/review/cleanup/reconsume action。

已实现内容：

- `runStatusLegacyText` 与 `runStatusText` 在 case shim 输出后立即调用统一 `writeStatusMissionCommanderFirstScreenText`，把 `status Mission Commander first screen` 放到无参数 `/rekit` / `status -Format text` 第一屏靠前位置。
- first-screen strip 复用既有 `status.caseMission.missionCommanderActionQueue` 与 `status.projectHandoff.packMemoryCandidates.missionCommanderActionQueue`，输出 case current action、pack-memory current action、queue counts、focus、blocked/requiresReview/source/state/command/reasons/boundary；不复制 queue selection 或 pack-memory action 派生逻辑。
- 焦点选择保持产品语义：case current action 若来自 reviewer/adapter/evidence/blocker 或 requiresReview/blocked 则优先；否则 open pack-memory residue 的 current action 优先于普通 ready lane continue；若都不存在则显示 none。
- Installed case-local `/rekit` product-path E2E 覆盖 initial none focus、partial reviewer intake 时 case focus，以及 reviewer 完成但 pack-memory candidate proof open 时 pack-memory focus；同时断言 default output 不泄漏 JSON、不使用 `<packet.json>` placeholder。

边界：本批只增强 case-local status/default `/rekit` 的只读 first-screen projection；不新增 durable schema，不自动 spawn/stop/poll/monitor reviewer，不执行 adapter/heavy tool，不生成或写入 pack-memory proof，不写 ledger/facts/authority/confirmed，不新增 PowerShell runtime logic。

验证结果：focused `go test ./internal/rekit/cli -run TestRunInstalledCaseShimProductPathStatusAndRefresh -count=1` 已通过；完整本地 release minimum 已通过：`go run ./cmd/rekit -- -Command release-check -Format json` 返回 `ready=true`，`go run ./cmd/rekit -- -Command status`、`go run ./cmd/rekit -- -Command packs`、`go run ./cmd/rekit -- -Command doctor`、`go test ./...`、`go vet ./...` 与 `git diff --check` 均通过（仅保留 Windows 工作树 LF→CRLF 提示）。实现前一次 `release-check` / `go test ./...` 曾因 Batch 597 文档仍标记进行中而 fail-closed；状态与完整验证记录更新后重跑通过。

### Batch 596：Mission Commander action queue current-action closure

状态：已完成 runtime/test/doc 工作树实现、focused tests、完整本地 release minimum、implementation commit/push 与 implementation remote release-gate inspection；implementation commit `93812c6` 已推送。implementation run `30161847547` completed failure，Linux/Windows/macOS jobs `89688329614`/`89688329665`/`89688329800` 均 `steps=[]`，仍属既有 runner/billing blocker，不能声明 remote CI green。本 release inspection record 仅记录该 implementation run；不要为 inspection commit 自身 CI 追加第三个记录提交，除非出现不同于既有 `steps=[]` 的新远程信号。本轮完成后按用户要求收尾停止，不继续开启 Batch 597。

目标：修复 Batch 590、547、595 后仍暴露的 first-screen operational 断点：`overview` / `status` 已有统一 `MissionCommanderActionQueue` 并能合并 reviewer、adapter、evidence 与 lane action，但 current-action selection 会先取 unblocked follow-up，导致 active reviewer dispatch/intake blocker 存在时，case-local `/rekit` 第一屏仍可能推荐普通 `/rekit handoff <lane>` 或保留 `/rekit continue <lane>`，replacement executor 需要回查完整 handoff 才能知道应先调度 read-only reviewer 或处理 review-owned current action。

已实现内容：

- `MissionCommanderActionQueueFor` 的 current-action 选择改为基于完整队列优先级：先选择 non-follow-up 且未 blocked 的 primary action，再选择 non-follow-up blocker/review action，随后才回退到 blocked/requiresReview 或普通 follow-up；这样 reviewer/evidence/adapter blocker 不会被 ready lane 的普通 handoff follow-up 遮蔽。
- `overview.nextSteps[]` 现在把 `follow Mission Commander current action: <command>` 放到第一屏 next step，并在 queue current action 需要 review、blocked 或来自 reviewer/adapter/evidence 等非普通 ready lane source 时过滤 generic `/rekit continue ...`。
- `status.caseMission.missionBriefNextActions[]` 改为复用 `overview` inventory 的 queue-aware `NextSteps`，使 JSON/text 与 unified queue current action 保持一致。
- Installed case-local `/rekit` product-path E2E 覆盖 partial reviewer intake 后 `status -Format json/text`：remaining reviewer shard 成为 current action，brief next action 跟随 `dispatch read-only reviewer`，且不再推荐 `/rekit continue login`。

边界：本批只改变只读 Mission Commander action queue selection 与 overview/status first-screen handoff；不新增 schema 字段，不自动 spawn/stop/poll/monitor reviewer，不执行 adapter/heavy tool，不写 ledger/facts/authority/confirmed，不新增 PowerShell runtime logic。

验证结果：focused `go test ./internal/rekit/mission -count=1`、`go test ./internal/rekit/overview -count=1`、`go test ./internal/rekit/cli -run TestRunInstalledCaseShimProductPathStatusAndRefresh -count=1` 已通过；完整本地 release minimum 已通过：`go run ./cmd/rekit -- -Command release-check -Format json` 返回 `ready=true`，`go run ./cmd/rekit -- -Command status`、`go run ./cmd/rekit -- -Command packs`、`go run ./cmd/rekit -- -Command doctor`、`go test ./...`、`go vet ./...` 与 `git diff --check` 均通过（仅保留 Windows 工作树 LF→CRLF 提示）。

### Batch 595：pack-memory candidate Mission Commander current action queue closure

状态：已完成 release/status pack-memory current action queue runtime、case-local default `/rekit` product-path binding、入口文档更新、完整本地 release minimum、implementation commit/push 与 implementation remote release-gate inspection；implementation commit `f76df49` 已推送。implementation run `30160413914` completed failure，Linux/Windows/macOS jobs `89684716377`/`89684716436`/`89684716444` 均 `steps=[]`，仍属既有 runner/billing blocker，不能声明 remote CI green。本 release inspection record 仅记录该 implementation run；不要为 inspection commit 自身 CI 追加第三个记录提交，除非出现不同于既有 `steps=[]` 的新远程信号。本批承接 Batch 570-576、591-593 的 pack-memory review/proof/cleanup/reconsume/retirement closure：release/status 已能列出 per-pack open residue、proof summary 与 next missing proof，但 multi-pack 或 case-local replacement executor 仍可能只看到泛化 `nextAction`，需要手工遍历 `packs[]` 才知道当前应处理哪个 pack、运行哪个 bounded proof/verification/retirement command；无参数 `/rekit` default/table text 还绕过 case-local packet-derived review workspace binding，导致 first-screen current action 仍可能使用 `<packet.json>` 占位。

目标：让 `release-check` / `status` 的 `packMemoryCandidates` 直接提供 Mission Commander 可消费的 `missionCommanderNextActions[]` 与 `missionCommanderActionQueue`，按 pack 排序选择当前 open pack 的最小 bounded action；case-local status/default `/rekit` 在绑定 `promote -CreateCandidates -Review` packet-derived workspace 后必须重算 queue，使 missing proof current action 直接包含 concrete `-PacketPath ...` command，不再要求主 Agent 手工拼 packet/proof/evidence path。

已实现内容：

- `ReleaseHandoffPackMemoryCandidateList` 新增只读 `missionCommanderNextActions[]` 与 `missionCommanderActionQueue`，从既有 per-pack `ProofSummary.NextMissingProof`、decision verification/provision/retirement receipt state、`DecisionDraftHandoff` 与 `Action` 派生，不改变 promote/proof/cleanup 事务语义。
- `release-check` / `status` text 输出 pack-memory action queue、current action、next action reason 与 boundary；所有 action 均保持 requiresReview，并明确 queue 只是 handoff，不执行 merge/cleanup/provision/verify/write proof。
- Case-local default `/rekit` table/text 路径现在与 `status -Format json/text` 一样复用 packet-derived candidate decision draft handoff binding，并在 binding 后重算 pack-memory action queue；installed case product-path 回归覆盖 `promote -CreateCandidates -Review` 后 JSON/text first-screen current action 使用 concrete packet path，且不回退到 `<packet.json>`。

边界：本批只增强 release/status/default `/rekit` 的只读 pack-memory Mission Commander handoff；不新增 pack-memory mutation，不 merge/cleanup candidates，不 provision/verify/retire workspace，不写 proof/facts/authority/confirmed，不执行 heavy tool，不新增 PowerShell runtime logic。

验证结果：focused `go test ./internal/rekit/releasecheck ./internal/rekit/cli -run "TestReleaseHandoffPackMemoryCandidatesDetectsOpenResidue|TestRunInstalledCaseShimProductPathStatusAndRefresh" -count=1` 已通过；扩展 focused `go test ./internal/rekit/releasecheck ./internal/rekit/cli -run "TestReleaseHandoffPackMemoryCandidatesDetectsOpenResidue|TestReleaseHandoffPackMemoryCandidateDecisionVerificationReceipt|TestReleaseHandoffPackMemoryCandidateVerificationRetirementLifecycle|TestRunInstalledCaseShimProductPathStatusAndRefresh|TestRunPromoteCandidateDecisionCaseLocalPreviewAndApply|TestRunPromoteCandidateReviewWorkspaceProductPath" -count=1` 已通过；完整本地 release minimum 已通过：`go run ./cmd/rekit -- -Command release-check -Format json` 返回 `ready=true`，`go run ./cmd/rekit -- -Command status`、`go run ./cmd/rekit -- -Command packs`、`go run ./cmd/rekit -- -Command doctor`、`go test ./...`、`go vet ./...` 与 `git diff --check` 均通过（仅保留 Windows 工作树 LF→CRLF 提示）。

### Batch 594：authorized adapter report hash-bound record product-path closure

状态：已完成 runtime next-step boundary 收紧、nested case-local/no-pack CLI product-path coverage、入口文档更新、完整本地 release minimum、implementation commit/push 与 implementation remote release-gate inspection；implementation commit `12f4a6e` 已推送。implementation run `30159484087` completed failure，macOS/Windows/Linux jobs `89682350095`/`89682350118`/`89682350148` 均 `steps=[]`，仍属既有 runner/billing blocker，不能声明 remote CI green。本 release inspection record 仅记录该 implementation run；不要为 inspection commit 自身 CI 追加第三个记录提交，除非出现不同于既有 `steps=[]` 的新远程信号。本批承接 Batch 580 的 validate→record currentness 与 Batch 568/569 scaffold/draft sidecar lifecycle：runtime 已能在 `gate -Apply -ExecutionReportPath ... -ExpectedExecutionReportSha256 ...` 写 observations 前重读 sidecar 并校验 hash，但真实 replacement executor product path 仍可能从 scaffold/draft `nextSteps[]` 或裸 record template 接续，而不是从 validation/status 的 hash-bound command 接续。

目标：把 authorized adapter execution report 的可运行接手链路收敛为 validation/status-derived hash-bound record：scaffold/draft 只指向先运行 read-only validation；`status` / validation 在 valid sidecar 后必须暴露 `reportSha256` / `recordExpectedReportSha256` 和包含 `-ExpectedExecutionReportSha256` 的 record current action；若 sidecar 在 validation 与 record 之间漂移，record 必须 fail-closed 且 `.rekit` zero-write。

已实现内容：

- `ScaffoldAdapterExecutionReport` / `DraftAdapterExecutionReport` 的 preview/apply/replay `nextSteps[]` 不再把裸 record template 作为最终可运行 handoff，而是明确要求先运行 read-only validation，并使用 validation/status 返回的 hash-bound record command 在 `valid=true` 后记录 bounded observation evidence。
- Nested output workspace CLI product-path 覆盖从 `status -Format json` 读取 valid sidecar live snapshot，断言 `reportSha256` / `recordExpectedReportSha256` 已进入 handoff 和 Mission Commander current action，且 record command 包含 `-ExpectedExecutionReportSha256`。
- 同一 E2E 在 record 前篡改 sidecar，验证旧 expected hash record 返回 `adapter execution report sha256 changed after validation` 且 `.rekit` snapshot 不变；恢复 exact bytes 后改用 hash-bound record 成功写 observation evidence，继续保持 no-heavy/no-authority/no-confirmed。

边界：本批只收紧 authorized adapter report handoff currentness 与 product-path 回归；不改变 legacy 裸 record template 的兼容字段，不执行 adapter/heavy tool，不自动 validate/record，不写 authority/confirmed，不新增 PowerShell runtime logic。

验证结果：focused `go test ./internal/rekit/cli -run "TestRunGateAdapterReport(NoPackProductPathFromNestedOutputWorkspace|TextOutputAndValidationHandoff|SelectedAdapterStatusAndRecordClosure)" -count=1` 已通过；focused `go test ./internal/rekit/gate -run "Test(RecordExecutionWritesObservationForAuthorizedGate|RecordExecutionDuplicateDoesNotAppend|AdapterReportContractDescribesAuthorizedGateBoundaries|AdapterReportContractProjectsPackToolingCandidateOperationalClosure|ScaffoldAdapterExecutionReportPreviewApplyAndReplay|DraftAdapterExecutionReportPreviewApplyReplayAndScaffoldReplace|ValidateAdapterExecutionReport)" -count=1` 已通过；完整本地 release minimum 已通过：`go run ./cmd/rekit -- -Command release-check -Format json` 返回 `ready=true`，`go run ./cmd/rekit -- -Command status`、`go run ./cmd/rekit -- -Command packs`、`go run ./cmd/rekit -- -Command doctor`、`go test ./...`、`go vet ./...` 与 `git diff --check` 均通过（仅保留 Windows 工作树 LF→CRLF 提示）。

### Batch 593：pack-memory retirement inventory product-path closure

状态：已完成 release/status strict discovery runtime、releasecheck/CLI product-path coverage、入口文档更新、完整本地 release minimum、implementation commit/push 与 implementation remote release-gate inspection；implementation commit `5182327` 已推送。implementation run `30158605320` completed failure，macOS/Windows/Linux jobs `89680094182`/`89680094198`/`89680094207` 均 `steps=[]`，仍属既有 runner/billing blocker，不能声明 remote CI green。本 release inspection record 仅记录该 implementation run；不要为 inspection commit 自身 CI 追加第三个记录提交，除非出现不同于既有 `steps=[]` 的新远程信号。本批承接 Batch 592 的 verification runbook：runbook 已要求 retirement Apply 后 rerun status/release-check，但真实 product-path 暴露出 `-DraftReviewProof -ProofPath ...` 写出的非 canonical receipt cleanup proof 可能未被 release/status receipt-derived discovery 识别，导致 retirement receipt 已 `retired`、pendingVerification=0 后仍保留 cleanup residue。

目标：收紧 pack-memory accepted candidate final verification retirement 的下游 inventory closure：`release-check` / `status` 必须只在每个 receipt action 的 strict cleanup proof、accepted candidate final verification proof、canonical workspace retirement receipt 都到位且 workspace absent 时清零；同时要识别公共 CLI `-DraftReviewProof -ProofPath` 合法写出的自定义 `*.candidate-cleanup-proof.{md,json,txt}`，不能只依赖 canonical stem 文件名。

已实现内容：

- `packMemoryCandidateDecisionCleanupArtifacts` 保留 canonical stem cleanup proof 快路径，并新增只读 fallback discovery：扫描 `review-artifacts` 顶层 `*.candidate-cleanup-proof.{md,json,txt}`，先 strict decode 再按 schema/kind/pack、packet/decision hashes、receipt path/hash、candidatePath、packTarget 与 decision 判断是否属于当前 receipt action。
- 自定义 cleanup proof 被计入 present 前仍复用 `validatePackMemoryCandidateCleanupProof`，重新校验 receipt/transaction/committed marker、backup hashes、candidate absent、index entry absent、accepted packTarget hash、relative stored paths 与 evidence hash；畸形、drift、symlink、non-regular 或错误绑定 proof 继续 fail-closed。
- CLI product-path 覆盖 managed accept + tooling reject 的所有 action cleanup proofs、final verification proof、retirement WhatIf→expected-hash Apply 后，`status -Format json/text` 与 `release-check -Format json` 都保持 read-only，并把 `packMemoryCandidates` 清为 ready/total=0；retirement receipt 后 workspace 重现仍保持 warning fail-closed。

边界：本批只增强 release/status read-only cleanup proof discovery 与 final closure 判断；不把 retirement receipt 替代 per-action cleanup proof，不删除/重建 workspace，不执行 provisioning/verification/retirement/heavy tool，不写 facts/authority/confirmed，不新增 PowerShell runtime logic。

验证结果：focused `go test ./internal/rekit/releasecheck ./internal/rekit/cli -run "TestReleaseHandoffPackMemoryCandidateDecisionVerificationReceipt|TestReleaseHandoffPackMemoryCandidateVerificationRetirementLifecycle|TestReleaseHandoffPackMemoryCandidatesDetectsOpenResidue|TestRunPromoteCandidateDecisionCaseLocalPreviewAndApply" -count=1` 已通过；完整本地 release minimum 已通过：`go run ./cmd/rekit -- -Command release-check -Format json` 返回 `ready=true`，`go run ./cmd/rekit -- -Command status`、`go run ./cmd/rekit -- -Command packs`、`go run ./cmd/rekit -- -Command doctor`、`go test ./...`、`go vet ./...` 与 `git diff --check` 均通过（仅保留 Windows 工作树 LF→CRLF 提示）。

### Batch 592：pack-memory final verification retirement handoff closure

状态：已完成 runtime 派生字段、CLI text 输出、入口文档更新、focused tests、完整本地 release minimum、implementation commit/push 与 implementation remote release-gate inspection；implementation commit `f1af394` 已推送。implementation run `30157616192` completed failure，Linux/Windows/macOS jobs `89677919166`/`89677919192`/`89677919199` 均 `steps=[]`，仍属既有 runner/billing blocker，不能声明 remote CI green。本 release inspection record 仅记录该 implementation run；不要为 inspection commit 自身 CI 追加第三个记录提交，除非出现不同于既有 `steps=[]` 的新远程信号。

目标：补齐 pack-memory accepted candidate final verification 后的 operational closure：`VerifyCandidateDecision` 已能写 repo-local verification proof，并在 canonical provisioning workspace 可绑定时返回 `retirementPreviewCommand`，但 Mission Commander / replacement executor 仍要从 proof path、provision intent/receipt、retirement preview command 与 `NextSteps` 手工拼接 proof→retirement→status/release-check 的下一步顺序。`CandidateDecisionVerificationResult` 应直接提供返回 envelope 级 `verificationRunbookSteps[]`，覆盖 verification WhatIf、Apply/replay、canonical retirement handoff 与无 retirement command 的 fallback。

已实现内容：

- `CandidateDecisionVerificationResult` 新增返回级派生 `verificationRunbookSteps[]`；WhatIf runbook 提示先复核 pack/fresh/attached doctor 与 reconsume validation，再用 identical Apply 写 repo-local proof。
- Apply/replay runbook 提示保留 `verificationProofPath` 作为 accepted-candidate reconsume evidence；当 canonical provisioning artifacts 可绑定时，直接提示复核 provision intent/receipt、运行 `retirementPreviewCommand` WhatIf、再运行 returned expected-hash retirement Apply，并以 status/release-check 确认 closure。
- 非 canonical fresh/attached roots 或缺 provisioning artifacts 时，runbook 明确无 retirement preview command，保留 proof 并回到 status/release-check 接续。
- `verificationRunbookSteps[]` 只在返回 envelope/text 中提供 operational handoff，不写入 durable verification proof，避免 handoff guidance 成为 proof authority。
- CLI text `writePromoteCandidateVerificationText` 新增 `promote candidate verification runbook：step=<n> text=<...>` 行，使 terminal first-screen 可直接从 verification result 接续 retirement。

边界：本批只增强 final verification 结果的只读 downstream runbook projection；不改变 verification proof、provisioning、retirement intent/receipt 或 deletion semantics；不创建 verification cases、不执行 retirement、不 merge/cleanup candidates、不运行 heavy tool；不写 authority/confirmed；不新增 PowerShell runtime logic。

验证结果：focused `go test ./internal/rekit/promote ./internal/rekit/cli -run "TestVerifyCandidateDecisionPreviewsAppliesAndReplays|TestRetireCandidateVerificationWorkspacePreviewsAppliesAndFailsClosedOnRecreation|TestWritePromoteCandidateVerificationTextIncludesRunbookSteps" -count=1` 已通过；完整本地 release minimum 已通过：`go run ./cmd/rekit -- -Command release-check -Format json` 返回 `ready=true`，`go run ./cmd/rekit -- -Command status`、`go run ./cmd/rekit -- -Command packs`、`go run ./cmd/rekit -- -Command doctor`、`go test ./...`、`go vet ./...`、`git diff --check` 与 `powershell.exe -NoProfile -ExecutionPolicy Bypass -File rekit/tests/facade-smoke.ps1 -Pack _template` 均通过（仅保留 Windows 工作树 LF→CRLF 提示）。

### Batch 591：pack-memory candidate decision downstream runbook closure

状态：已完成 runtime 派生字段、CLI text 输出、入口文档更新、focused tests、完整本地 release minimum、implementation commit/push 与 implementation remote release-gate inspection；implementation commit `031b381` 已推送。implementation run `30157225900` completed failure，macOS/Linux/Windows jobs `89676973276`/`89676973285`/`89676973296` 均 `steps=[]`，仍属既有 runner/billing blocker，不能声明 remote CI green。本 release inspection record 仅记录该 implementation run；不要为 inspection commit 自身 CI 追加第三个记录提交，除非出现不同于既有 `steps=[]` 的新远程信号。

目标：补齐 pack-memory downstream UX residual：candidate decision WhatIf/Apply 已能 strict 绑定 packet/candidate/target/evidence hashes、写 receipt、返回 verification provisioning/verification commands 或 recovery envelope，但 Mission Commander / replacement executor 仍要跨 `actions[]`、`receipt`、`recoveryActions[]`、`nextSteps[]` 与后续 proof/reconsume命令手工拼接操作顺序。`CandidateDecisionResult` 应直接提供结果级 `decisionRunbookSteps[]`，覆盖 preview、accepted apply、reject/superseded-only apply 与 rollback/recovery envelope。

已实现内容：

- `CandidateDecisionResult` 新增派生 `decisionRunbookSteps[]`；WhatIf preview、Apply success、rollback/recovery 与 interrupted transaction recovery 都复用同一 helper，只读取已有 actions、receipt、recoveryActions、nextSteps、failedAction 与 counts。
- preview runbook 提示先检查 planned actions / evidence refs；accepted preview 明确 Apply 后仍需要 verification provisioning + pack/fresh/attached reconsume proof，reject/superseded-only preview 明确 Apply 只做 cleanup/index closure。
- Apply success runbook 按 accepted vs no accepted 分流：accepted 输出 receipt retention、receipt `verificationProvisionCommand` WhatIf、expected-hash provisioning Apply、`verificationCommand` 与 verification proof/status/doctor closure；reject/superseded-only 输出 cleanup/index confirmation 且说明不需要 fresh/attached reconsume proof。
- rollback/recovery envelope runbook 在 cleanup failure、pre-commit failure、unfinished transaction 或 interrupted transaction recovery 后停下 downstream closure，提示先查看 `backupRoot` / `recoveryActions`，恢复后重跑 WhatIf 再 retry Apply。
- CLI text `writePromoteCandidateDecisionText` 新增 `promote candidate decision runbook：step=<n> text=<...>` 行，使 terminal first-screen 不必解析 nested receipt/nextSteps 才能接续。

边界：本批只增强 candidate decision 结果的只读 downstream runbook projection；不改变 candidate decision transaction、receipt、verification provisioning/final verification/retirement 语义；不生成 proof、不 merge tooling candidate、不执行 doctor/init/reconsume/heavy tool；不写 authority/confirmed；不新增 PowerShell runtime logic。

验证结果：focused `go test ./internal/rekit/promote ./internal/rekit/cli -run "TestApplyCandidateDecisionsPreviewsAndAppliesReviewedManagedCandidate|TestApplyCandidateDecisionsClosesToolingOnlyReject|TestApplyCandidateDecisionsRejectAndRollbackCleanupFailure|TestWritePromoteCandidateDecisionTextIncludesRunbookSteps" -count=1` 已通过；完整本地 release minimum 已通过：`go run ./cmd/rekit -- -Command release-check -Format json` 返回 `ready=true`，`go run ./cmd/rekit -- -Command status`、`go run ./cmd/rekit -- -Command packs`、`go run ./cmd/rekit -- -Command doctor`、`go test ./...`、`go vet ./...`、`git diff --check` 与 `powershell.exe -NoProfile -ExecutionPolicy Bypass -File rekit/tests/facade-smoke.ps1 -Pack _template` 均通过（仅保留 Windows 工作树 LF→CRLF 提示）。

### Batch 590：reviewer dispatch/intake first-screen runbook closure

状态：已完成 runtime handoff 派生字段、CLI/product-path text 输出、入口文档更新、focused tests、完整本地 release minimum、implementation commit/push 与 implementation remote release-gate inspection；implementation commit `dab7802` 已推送。implementation run `30156522308` completed failure，macOS/Windows/Linux jobs `89675249671`/`89675249673`/`89675249708` 均 `steps=[]`，仍属既有 runner/billing blocker，不能声明 remote CI green。本 release inspection record 仅记录该 implementation run；不要为 inspection commit 自身 CI 追加第三个记录提交，除非出现不同于既有 `steps=[]` 的新远程信号。

目标：补齐 reviewer orchestration E2E residual：当 replacement executor 从 status/handoff/continue、lane `RESUME.md`、checkpoint 或 digest 接手 open reviewer packet 时，不需要打开完整 `packet.json`、`summary.md` 或 nested dispatch JSON，也能知道当前 shard 的下一步最小操作序列。open `reviewerDispatchIntakeHandoffs[]` 应提供 per-shard `runbookSteps[]`，compact `reviewerDispatchIntakeSummary` 应提供当前 `nextActionRunbookSteps[]`，覆盖 waiting、staging-ready、collection-ready、intake-ready、owner adoption、prompt repair 与 result recovery 等状态。

已实现内容：

- `ReviewerDispatchIntakeHandoff` 新增派生 `runbookSteps[]`，`ReviewerDispatchIntakeSummary` 新增 `nextActionRunbookSteps[]`；二者只从既有 packet/handoff paths、commands、state 与 boundary 生成，不改变 packet schema 或 reviewer intake writeback 语义。
- reviewer runbook 按状态输出 dispatch/save JSON、staging WhatIf→expected-source-hash Apply、collection WhatIf→Apply、batch/single intake WhatIf→Apply，以及 owner adoption、prompt artifact repair、result recovery / disposition / invalid source/candidate 的 fail-closed repair sequence。
- status/continue text、project/lane handoff Markdown、lane `RESUME.md`、typed checkpoint 与 continue digest 通过既有 handoff envelope 投影 summary next-action runbook 与 per-shard runbook，使 first-screen 和 durable artifact 均可接续。

边界：本批只增强 reviewer dispatch/intake handoff 的只读 projection；不自动 spawn、stop、poll 或 monitor reviewer；不执行 heavy tool；不写 authority/confirmed；不改变 `plan-subagents` packet schema、reviewer result contract、collection/recovery/intake writeback 语义或 PowerShell runtime logic。

验证结果：focused `go test ./internal/rekit/workstream ./internal/rekit/cli -run "TestReviewerDispatchIntakeRunbookStepsCoverReviewerLifecycle|TestReviewerDispatchIntakeSummaryProjectsWaitingNextAction|TestRunInstalledCaseShimProductPathStatusAndRefresh" -count=1` 已通过；完整本地 release minimum 已通过：`go run ./cmd/rekit -- -Command release-check -Format json` 返回 `ready=true`，`go run ./cmd/rekit -- -Command status`、`go run ./cmd/rekit -- -Command packs`、`go run ./cmd/rekit -- -Command doctor`、`go test ./...`、`go vet ./...`、`git diff --check` 与 `powershell.exe -NoProfile -ExecutionPolicy Bypass -File rekit/tests/facade-smoke.ps1 -Pack _template` 均通过（仅保留 Windows 工作树 LF→CRLF 提示）。

### Batch 589：note WhatIf hash-bound record currentness closure

状态：已完成 runtime、CLI product-path handoff、retained façade 参数透传、入口文档更新、focused tests、完整本地 release minimum、implementation commit/push 与 implementation remote release-gate inspection；implementation commit `d9dbe05` 已推送。implementation run `30155766789` completed failure，macOS/Linux/Windows jobs `89673348320`/`89673348342`/`89673348344` 均 `steps=[]`，仍属既有 runner/billing blocker，不能声明 remote CI green。本 release inspection record 仅记录该 implementation run；不要为 inspection commit 自身 CI 追加第三个记录提交，除非出现不同于既有 `steps=[]` 的新远程信号。本批承接 Batch 588 的 open-candidate decision closure：handoff 已推荐先写 `note -Kind decision -Related <candidateEventId>`，但主 Agent 若先预览再手动 record，旧路径只复制裸 note command，`createdAt`、`eventId` 或 decision 参数 drift 会导致写入的事实不等于已复核的 preview。

目标：让 append-only note 写入形成 deterministic WhatIf→record currentness 闭环；`note -WhatIf` 返回 `eventSha256` 与 hash-bound `recordCommand`，record 时带 `-ExpectedNoteEventSha256` 并在写 ledger 前重建 event 校验，drift 则 fail-closed zero-write。status/continue/start/reconcile 的 open-decision handoff 只提示运行 WhatIf 返回的 recordCommand，避免裸 record 绕过 preview hash。

已实现内容：

- `note.Append` 支持 `CreatedAt` 与 `ExpectedEventSHA256`，在 append 前构造 event、生成 SHA-256、输出 `eventSha256` / `expectedEventSha256` / 可重放 `recordCommand`；expected hash 长度或内容不匹配时在 `mission.AppendFact` 前返回错误且不写 ledger。
- `recordCommand` 仅对公共 CLI 可完整重放的 note event 输出；含 reviewer-intake 内部字段等不可 CLI-replayable event 只输出 hash，避免生成无法重建 hash 的误导命令。
- CLI parse/text 输出支持 `-CreatedAt` 与 `-ExpectedNoteEventSha256`；status 以及 blocked continue/start/reconcile 的 open-decision handoff 统一为“先 `note -WhatIf`，再运行返回的 hash-bound `recordCommand`”。retained `rekit.ps1` 只补新 flags 透传到 Go backend，不新增业务 runtime。

边界：本批只收紧 note append preview→record currentness，不改 ledger schema，不写 authority/confirmed，不执行 heavy tool，不改变 `continue -Apply` / `gate -Apply` 既有边界。旧不带 expected hash 的 note record 仍兼容；hash-bound record 是 handoff 推荐路径。

验证结果：focused `go test ./internal/rekit/note ./internal/rekit/workstream ./internal/rekit/cli -run "TestAppendWhatIfOmitsRecordCommandForInternalFields|TestRunNoteHashBoundRecordRejectsDrift|TestRunNoteAppendWhatIfDoesNotWrite|TestRunNoteAppendWhatIfTextHandoffDoesNotWrite|TestRunNoteAppendTextHandoffWritesFactEvent|TestRunNoteAppendRejectsInvalidInputs|TestRunStatusJsonAndTextCaseMissionHandoffs|TestRunStatusFromInstalledCaseLocalShim|TestRunStartProjectsExecutorActionForExistingLaneBlockers|TestRunContinueBlocksOpenDecisionBeforeWrites|TestRunReconcileApplyProjectsGateDecisionHandoffsAfterInterventionResolution|TestRunContinueShowsAuthorizedGateAdapterHandoffAndEvidenceReview" -count=1` 已通过；完整本地 release minimum 已通过：`go run ./cmd/rekit -- -Command release-check -Format json` 返回 `ready=true`，`go run ./cmd/rekit -- -Command status`、`go run ./cmd/rekit -- -Command packs`、`go run ./cmd/rekit -- -Command doctor`、`go test ./...`、`go vet ./...`、`git diff --check` 与 `powershell.exe -NoProfile -ExecutionPolicy Bypass -File rekit/tests/facade-smoke.ps1 -Pack _template` 均通过（仅保留 Windows 工作树 LF→CRLF 提示）。remote inspection 已记录：implementation run `30155766789` completed failure，macOS/Linux/Windows jobs `89673348320`/`89673348342`/`89673348344` 均 `steps=[]`，未提供代码执行日志，按既有 runner/billing blocker 记录，不能声明 remote CI green。

### Batch 588：related decision note candidate blocker closure

状态：已完成 runtime、CLI product-path coverage、入口文档更新、focused tests、完整本地 release minimum、implementation commit/push 与 implementation remote release-gate inspection；implementation commit `8d47b04` 已推送。implementation run `30154082878` completed failure，Windows/macOS/Linux jobs `89669186876`/`89669186902`/`89669186911` 均 `steps=[]`，仍属既有 runner/billing blocker，不能声明 remote CI green。本 release inspection record 仅记录该 implementation run；不要为 inspection commit 自身 CI 追加第三个记录提交，除非出现不同于既有 `steps=[]` 的新远程信号。本批修复 Batch 586–587 后暴露的更深闭环断点：blocked continue/start/reconcile 已能给出 open candidate handoff，并推荐 `note -Kind decision -Related <candidateEventId>`，但 Mission 判定若只看 candidate 自身 status，会导致主 Agent 按 handoff 记录 terminal decision note 后，candidate blocker 仍不消失。

目标：让既有 append-only decision note 的 `related` refs 成为 open candidate lifecycle closure 的 deterministic 输入；当 decision note 明确 `accept` / `reject` / `supersede` 且 `related` 指向 candidate `eventId` 时，Mission brief、status/overview、continue/start/reconcile handoff 与 lane executor action 都不再把该 candidate 视为 open blocker。

已实现内容：

- Mission 层新增 `EffectiveOpenCandidates(facts)`，先读取 `OpenCandidates`，再用 terminal closing decision note 的 `related` event IDs 过滤已关闭 candidate；`defer`、`pending-user`、`status=open`、仅 `target` 指向 candidate 或缺 `eventId` 的 candidate 仍保持 open。
- `OpenDecisionItems`、`OpenDecisionLanes`、Mission brief count/blocked lanes、blocked continue open-decision handoff、status `openDecisionHandoffs[]` 与 overview open candidates 均改用 effective open candidate 语义，避免同一 ledger 在不同入口出现 blocker 漂移。
- CLI product-path coverage 覆盖 `note -Kind decision -Related cand... -Decision accept` 后，post-note Mission Commander action 变为 ready、status 不再投影 open decision handoff，随后 owner-bound `continue <lane> -Apply -Executor ... -ExpectedExecutorGeneration ...` 可继续并写 run status。

边界：本批只消费既有 append-only decision refs，不新增 schema 字段，不修改 candidate 原事件，不写 authority/confirmed，不执行 heavy tool，不新增 PowerShell runtime logic。`target` 不作为 lifecycle closure；只有 `related` 指向 concrete candidate `eventId` 且 decision 为 terminal accept/reject/supersede 时才关闭 candidate blocker。

验证结果：focused `go test ./internal/rekit/mission ./internal/rekit/workstream ./internal/rekit/cli ./internal/rekit/overview -run "TestEffectiveOpenCandidatesHonorsRelatedDecisionNotes|TestRunNoteDecisionRelatedCandidateClosesOpenCandidateBlocker|TestRunContinueBlocksOpenDecisionBeforeWrites|TestRunStartProjectsExecutorActionForExistingLaneBlockers|TestRunReconcileApplyProjectsGateDecisionHandoffsAfterInterventionResolution|TestLaneExecutorActionSnapshotsKeepNextActionsLaneLocal" -count=1` 已通过；完整本地 release minimum 已通过：`go run ./cmd/rekit -- -Command release-check -Format json` 返回 `ready=true`，`go run ./cmd/rekit -- -Command status`、`go run ./cmd/rekit -- -Command packs`、`go run ./cmd/rekit -- -Command doctor`、`go test ./...`、`go vet ./...` 与 `git diff --check` 均通过（仅保留 Windows 工作树 LF→CRLF 提示）。remote inspection 已记录：implementation run `30154082878` completed failure，Windows/macOS/Linux jobs `89669186876`/`89669186902`/`89669186911` 均 `steps=[]`，未提供代码执行日志，按既有 runner/billing blocker 记录，不能声明 remote CI green。

### Batch 587：start/reconcile pending gate / open decision handoff closure

状态：已完成 runtime、CLI product-path coverage、入口文档更新、完整本地 release minimum、implementation commit/push 与 implementation remote release-gate inspection；implementation commit `1e72510` 已推送。implementation run `30153277971` completed failure，macOS/Linux/Windows jobs `89667127082`/`89667127098`/`89667127104` 均 `steps=[]`，仍属既有 runner/billing blocker，不能声明 remote CI green。本 release inspection record 仅记录该 implementation run；不要为 inspection commit 自身 CI 追加第三个记录提交，除非出现不同于既有 `steps=[]` 的新远程信号。本批延续 Batch 585–586 的 replacement executor 接手闭环：新会话常先运行 `start <lane> -Apply -Executor ...` 登记/接管 durable lane，或在 `reconcile <lane> -Apply` 清除 open intervention 后回到 lane；过去若该 lane 仍有 pending gate/open decision blocker，结果只给 executor blocker counts / generic next action，缺少 concrete gate/note handoff，替换执行体仍需切回 status/handoff 才能拿到下一步命令。

目标：让 `start -WhatIf/-Apply` 与 `reconcile -WhatIf/-Apply` 复用 blocked continue 的 workstream-level gate/decision handoff builder，在 JSON/text 第一屏直接输出 `pendingGateHandoffs[]` / `openDecisionHandoffs[]`、case-local gate/note WhatIf/Apply/record command、decision/continue boundary 与 evidence；owner takeover 与 reconcile 既有写入语义不变。

已实现内容：

- `StartResult` 与 `ReconcileResult` 新增 `pendingGateHandoffs[]` / `openDecisionHandoffs[]`，并由共享 `gateDecisionHandoffsForLane` / `gateDecisionHandoffs` 从 lane facts 生成，避免 start/reconcile 复制 command/boundary 构造逻辑。
- `start -WhatIf/-Apply` 在创建/进入/接管已有 lane 后直接投影当前 lane-local pending gate / open decision handoff；replacement executor 用 `-Executor` takeover 后可在同一结果里看到新的 owner-bound continue command 和剩余 gate/decision handoff。
- `reconcile -WhatIf/-Apply` 在 intervention resolution 结果里同样投影剩余 pending gate / open decision handoff；若 intervention 已清除但 lane 仍 blocked，Mission Commander action queue 会停在 gate/decision handoff，而不是建议继续 lane。
- CLI text writer 复用 continue handoff 输出并支持 `start` / `reconcile` / `continue` prefix，terminal 第一屏能区分来源并显示 `start pending gate handoff`、`start open decision handoff`、`reconcile pending gate handoff` 与 `reconcile open decision handoff`。
- CLI product-path coverage 覆盖已有 blocked lane 的 start preview/apply JSON/text handoff，以及 reconcile apply 清除 intervention 后剩余 pending gate/open candidate 时的 JSON/text handoff与不推荐 continue 行为。

边界：本批只增强 start/reconcile 结果的只读 handoff projection；`start -Apply` 仍只写 case-local lane/board/resume/checkpoint/executor metadata，`reconcile -Apply` 仍只写 intervention resolution、lane events、lane.json、board、RESUME/checkpoint；gate Apply 只重放/记录 request decision，decision note record 只追加 case-local decision ledger state；不写 authority/confirmed、不执行 heavy tool、不新增 PowerShell runtime logic。

验证结果：focused `go test ./internal/rekit/workstream ./internal/rekit/cli -run "TestRunStartProjectsExecutorActionForExistingLaneBlockers|TestRunReconcileApplyProjectsGateDecisionHandoffsAfterInterventionResolution|TestRunReconcileApplyReplaysExistingResolutionToRefreshDurableState|TestRunContinueBlocks(PendingGate|OpenDecision)BeforeWrites" -count=1` 已通过；完整本地 release minimum 已通过：`go run ./cmd/rekit -- -Command release-check -Format json` 返回 `ready=true`，`go run ./cmd/rekit -- -Command status`、`go run ./cmd/rekit -- -Command packs`、`go run ./cmd/rekit -- -Command doctor`、`go test ./...`、`go vet ./...` 与 `git diff --check` 均通过（仅保留 Windows 工作树 LF→CRLF 提示）。remote inspection 已记录：implementation run `30153277971` completed failure，macOS/Linux/Windows jobs `89667127082`/`89667127098`/`89667127104` 均 `steps=[]`，未提供代码执行日志，按既有 runner/billing blocker 记录，不能声明 remote CI green。

### Batch 586：blocked continue pending gate / open decision handoff closure

状态：已完成 runtime、CLI product-path coverage、入口文档更新、完整本地 release minimum、implementation commit/push 与 implementation remote release-gate inspection；implementation commit `7e36786` 已推送。implementation run `30152274807` completed failure，Windows/macOS/Linux jobs `89664563778`/`89664563793`/`89664563853` 均 `steps=[]`，仍属既有 runner/billing blocker，不能声明 remote CI green。本 release inspection record 仅记录该 implementation run；不要为 inspection commit 自身 CI 追加第三个记录提交，除非出现不同于既有 `steps=[]` 的新远程信号。本批延续 Batch 581–585 的 Mission Commander blocker 接手闭环：replacement executor 最常直接运行 `continue <lane>`，但 pending gate 或 open candidate/decision 仍可能让 `continue -Apply` 创建 run、写 facts、刷新 lane/board/RESUME/checkpoint，迫使主 Agent从 status/overview/handoff 另行回查 gate 或 decision handoff。

目标：让 blocked `continue -WhatIf/-Apply` 在遇到 lane-local pending-gate request 或 open candidate/decision 时保持 zero-write，并直接返回结构化 `pendingGateHandoffs[]` / `openDecisionHandoffs[]`，CLI text 同步输出 concrete handoff、decision/continue boundary 与 evidence；pending gate handoff 暴露 case-local `gate -WhatIf` 与 bounded request-decision `gate -Apply`，open decision handoff 暴露 source list、decision note WhatIf 与 record command。

已实现内容：

- `ContinueResult` 新增 `pendingGateRequired`、`openDecisionRequired`、`pendingGateHandoffs[]` 与 `openDecisionHandoffs[]`；open intervention 与 reviewer dispatch/intake 仍先短路，pending gate / open decision 在没有更高优先级 blocker 时成为 `continue` 的 blocked contract。
- `ContinuePreview` 与 `ContinueApply` 在写 run、facts、lane、board、RESUME 或 checkpoint 前读取 lane facts，发现 pending-gate request 或 open candidate/decision 即返回 blocked preview identity，`Applied=false`、`Writes=[]`，并保留 Mission brief、executor action、execution evidence review、authorized gate adapter handoff 与 Mission Commander action queue。
- Pending gate handoff 生成 `/rekit handoff <lane>`、case-local `/rekit gate -Action ... -Lane ... -WhatIf` 与 `/rekit gate -Action ... -Lane ... -Apply -Actor ...`，并明确 Apply 只重放/记录 gate request decision，不执行或批准 heavy action。
- Open decision handoff 生成 `/rekit handoff <lane>`、source list command、decision note WhatIf 与 record command，并明确 record 只追加 case-local decision ledger state，不写 authority/confirmed、不执行 heavy tool。
- `writeContinueText` 在 blocked 分支直接打印 `continue pending gate handoff`、`continue open decision handoff`、decision/continue boundary 与 evidence，使 terminal replacement executor 不解析 JSON、不切回 status/handoff 也能执行下一步只读 review。
- CLI product-path coverage 覆盖 pending-gate 与 open-decision blocked `continue -Apply` JSON/text zero-write、concrete handoff、Mission Commander action queue current action，以及既有 authorized gate visibility 测试改为断言 open-decision fail-closed 不再写 run artifacts。

边界：本批只封住 `continue` 在 pending gate / open decision blocker 下的写入缺口并增强 handoff；blocked `continue -WhatIf/-Apply` 不创建 run、不写 facts/lane/board/RESUME/checkpoint；`gate -Apply` 仍只写 request ledger decision，`note -Kind decision` 仍只写 case-local decision ledger，不写 authority/confirmed、不执行 heavy tool、不新增 PowerShell runtime logic。

验证结果：focused `go test ./internal/rekit/workstream ./internal/rekit/cli -count=1` 已通过；完整本地 release minimum 已通过：`go run ./cmd/rekit -- -Command release-check -Format json` 返回 `ready=true`，`go run ./cmd/rekit -- -Command status`、`go run ./cmd/rekit -- -Command packs`、`go run ./cmd/rekit -- -Command doctor`、`go test ./...`、`go vet ./...` 与 `git diff --check` 均通过（仅保留 Windows 工作树 LF→CRLF 提示）。remote inspection 已记录：implementation run `30152274807` completed failure，Windows/macOS/Linux jobs `89664563778`/`89664563793`/`89664563853` 均 `steps=[]`，未提供代码执行日志，按既有 runner/billing blocker 记录，不能声明 remote CI green。

### Batch 585：blocked continue reconcile handoff closure

状态：已完成 runtime、CLI product-path coverage、入口文档更新、完整本地 release minimum、implementation commit/push 与 implementation remote release-gate inspection；implementation commit `e3e04a2` 已推送。implementation run `30150639182` completed failure，Windows/Linux/macOS jobs `89660325546`/`89660325548`/`89660325553` 均 `steps=[]`，仍属既有 runner/billing blocker，不能声明 remote CI green。本 release inspection record 仅记录该 implementation run；不要为 inspection commit 自身 CI 追加第三个记录提交，除非出现不同于既有 `steps=[]` 的新远程信号。本批延续 Batch 581–584 的 intervention handoff/reconcile 接手闭环：replacement executor 最常直接运行 `continue <lane>`，但当该 lane 因 effective open intervention fail-closed 时，blocked continue 结果过去只给 generic next step，仍需要切回 status/overview/handoff 才能拿到 concrete reconcile WhatIf→Apply handoff。

目标：让 blocked `continue -WhatIf/-Apply` 在遇到 effective open intervention 时保持 zero-write，同时直接返回结构化 `reconcileHandoffs[]`，并在 CLI text 输出 `continue reconcile handoff`、decision/continue boundary 与 evidence；若 lane 已有 current executor，handoff 中的 reconcile WhatIf/Apply 命令要保留 `-Executor <current>`，避免替换 executor 从 continue 入口恢复时丢失当前 lane executor identity。

已实现内容：

- `ContinueResult` 新增 `reconcileHandoffs[]`，blocked open-intervention path 会从 effective open interventions 生成 concrete reconcile WhatIf/Apply、review command、boundary 与 evidence；缺 eventId 时仍使用 `<eventId>` placeholder 作为 fail-closed fallback。
- `writeContinueText` 在 blocker 段直接打印 `continue reconcile handoff`、decision boundary、continue boundary 与 evidence，使 terminal replacement executor 不解析 JSON、不切回 status/handoff 也能执行下一步 read-only reconcile preview。
- `InterventionSummary` 保留 `scope`、`approvedBy` 与 `batchId`，用于 blocked continue handoff evidence；owner-bound lane 的 continue handoff 会把 current executor 注入 reconcile WhatIf/Apply command。
- CLI product-path coverage 覆盖 blocked `continue -Apply` JSON/text zero-write、concrete reconcile handoff、Mission Commander action queue current action，以及 owner-bound `continue -WhatIf` 保留 current executor identity。

边界：本批只增强 blocked `continue` 的 intervention handoff 输出；blocked `continue -WhatIf/-Apply` 仍 zero-write，不创建 run、不写 facts/lane/board/RESUME/checkpoint；`reconcile -WhatIf` 仍 read-only，`reconcile -Apply` 仍是显式 bounded write；不写 authority/confirmed、不执行 heavy tool、不新增 PowerShell runtime logic。

验证结果：focused `go test ./internal/rekit/workstream ./internal/rekit/cli -count=1` 已通过；完整本地 release minimum 已通过：`go run ./cmd/rekit -- -Command release-check -Format json` 返回 `ready=true`，`go run ./cmd/rekit -- -Command status`、`go run ./cmd/rekit -- -Command packs`、`go run ./cmd/rekit -- -Command doctor`、`go test ./...`、`go vet ./...` 与 `git diff --check` 均通过（仅保留 Windows 工作树 LF→CRLF 提示）。remote inspection 已记录：implementation run `30150639182` completed failure，Windows/Linux/macOS jobs `89660325546`/`89660325548`/`89660325553` 均 `steps=[]`，未提供代码执行日志，按既有 runner/billing blocker 记录，不能声明 remote CI green。

### Batch 584：reconcile replay fail-closed preflight zero-write closure

状态：已完成 runtime、CLI product-path coverage、docs、完整本地 release minimum、implementation commit/push 与 implementation remote release-gate inspection；implementation commit `b521ef2` 已推送。implementation run `30149515188` completed failure，Linux/Windows/macOS jobs `89657341646`/`89657341670`/`89657341682` 均 `steps=[]`，仍属既有 runner/billing blocker，不能声明 remote CI green。本 release inspection record 仅记录该 implementation run；不要为 inspection commit 自身 CI 追加第三个记录提交，除非出现不同于既有 `steps=[]` 的新远程信号。本批收紧 Batch 583 的 deterministic replay/recovery 边界：existing resolution fact 若缺少 `eventId` / `time`，或 `executor` / `actor` / `reason` 与当前 bounded command 不一致，必须在任何 lane event、lane.json、board、RESUME 或 checkpoint 写入前 fail-closed。

目标：把 existing resolution replay 的完整性与 identity 校验前移到生成 deterministic lane event 之前，确保 invalid replay 是 zero-write；合法 replay 仍可复用 existing resolution identity/time，补齐 lane-local events 和 durable state。

已实现内容：

- `ReconcileApply` 在进入 replay lane event/refresh 前先校验 existing resolution 的 `eventId`、`time`、`executor`、`actor` 与 `reason`；缺失或不一致直接返回错误，不生成 lane event。
- 保留 Batch 583 合法 replay 语义：唯一 matching resolution fact 可继续 `already-appended` fact、补齐 deterministic lane events、刷新 lane.json/board/RESUME/checkpoint。
- CLI product-path coverage 新增 invalid replay zero-write 断言：existing resolution executor drift 时，同一 `reconcile -Apply` 返回 fail-closed 错误且 `.rekit` snapshot 完全不变。

边界：本批只收紧 bounded `reconcile -Apply` replay preflight；`reconcile -WhatIf` 仍 read-only，preview 不接受 resolved replay；invalid replay 不写任何 lane-local 或 durable artifact；不写 authority/confirmed、不执行 heavy tool、不新增 PowerShell runtime logic。

验证结果：focused `go test ./internal/rekit/workstream ./internal/rekit/cli -count=1` 已通过；完整本地 release minimum 已通过：`go run ./cmd/rekit -- -Command release-check -Format json` 返回 `ready=true`，`go run ./cmd/rekit -- -Command status`、`go run ./cmd/rekit -- -Command packs`、`go run ./cmd/rekit -- -Command doctor`、`go test ./...`、`go vet ./...` 与 `git diff --check` 均通过（仅保留 Windows 工作树 LF→CRLF 提示）。remote inspection 已记录：implementation run `30149515188` completed failure，Linux/Windows/macOS jobs `89657341646`/`89657341670`/`89657341682` 均 `steps=[]`，未提供代码执行日志，按既有 runner/billing blocker 记录，不能声明 remote CI green。

### Batch 583：reconcile partial replay durable recovery closure

状态：已完成 runtime、CLI product-path coverage、docs、完整本地 release minimum、implementation commit/push 与 implementation remote release-gate inspection；implementation commit `d51bcf0` 已推送。implementation run `30149260711` completed failure，Linux/Windows/macOS jobs `89656696957`/`89656696912`/`89656696940` 均 `steps=[]`，仍属既有 runner/billing blocker，不能声明 remote CI green。本 release inspection record 仅记录该 implementation run；不要为 inspection commit 自身 CI 追加第三个记录提交，除非出现不同于既有 `steps=[]` 的新远程信号。本批延续 Batch 581–582 的 intervention handoff/reconcile 接手闭环：Mission Commander 已能引导 selected `reconcile -WhatIf` → bounded `-Apply`，但 `reconcile -Apply` 涉及 resolution fact、lane event、lane.json、board、RESUME 与 checkpoint 多文件写入；若在 resolution fact 已写入后中断，下一次重试会因为 source intervention 已被 `resolvesEventId` 过滤出 effective-open 集合而无法补齐 durable lane state。

目标：让同一个 bounded `reconcile -Apply -InterventionId <source>` 在检测到已有且唯一的 matching `action=reconcile status=resolved resolvesEventId=<source>` resolution fact 时，进入 deterministic replay/recovery：不重复追加 intervention resolution fact，而是复用 existing `resolutionEventId` / `time`，补齐或验证 deterministic lane reconcile/takeover events，并刷新 lane.json、board、RESUME 和 checkpoint，使 replacement executor 可从 partial apply 后恢复到 ready-to-continue handoff。

已实现内容：

- `ReconcileApply` 的 apply context 现在允许 resolved replay selection：普通 `ReconcilePreview` 仍只接受 effective-open intervention；apply 重试可在 source intervention 已被 matching resolution fact 关闭后，严格绑定同 lane/source/resolution 继续恢复。
- resolution fact 已存在时返回 `fact-jsonl already-appended`，并校验 existing resolution 的 `executor` / `actor` / `reason` 与当前 bounded command 一致；缺失 `eventId` / `time`、多个 matching resolution 或不一致字段均 fail-closed。
- lane-local `intervention-reconciled` / `executor-takeover` events 现在按 existing resolution time 生成 deterministic event IDs；已存在时验证关键字段后标记 `already-appended`，不存在时补 append，随后刷新 lane state、durable `RESUME.md` 与 checkpoint。
- CLI product-path coverage 模拟 resolution fact 已写但 durable lane state 未刷新，验证 replay 不重复 intervention fact、补齐 lane events/lane.json/RESUME，并恢复 ready Mission Commander continue command。

边界：本批只增强 bounded `reconcile -Apply` 的 partial-write recovery；`reconcile -WhatIf` 仍 read-only，preview 不接受 resolved replay；replay 只在已有唯一 matching resolution fact 且 bounded command identity 一致时执行；不写 authority/confirmed、不执行 heavy tool、不新增 PowerShell runtime logic。

验证结果：focused `go test ./internal/rekit/workstream ./internal/rekit/cli -count=1` 已通过；完整本地 release minimum 已通过：`go run ./cmd/rekit -- -Command release-check -Format json` 返回 `ready=true`，`go run ./cmd/rekit -- -Command status`、`go run ./cmd/rekit -- -Command packs`、`go run ./cmd/rekit -- -Command doctor`、`go test ./...`、`go vet ./...` 与 `git diff --check` 均通过（仅保留 Windows 工作树 LF→CRLF 提示）。remote inspection 已记录：implementation run `30149260711` completed failure，Linux/Windows/macOS jobs `89656696957`/`89656696912`/`89656696940` 均 `steps=[]`，未提供代码执行日志，按既有 runner/billing blocker 记录，不能声明 remote CI green。

### Batch 582：multi-intervention concrete reconcile preview option handoff closure

状态：已完成 runtime、CLI product-path coverage、docs、完整本地 release minimum、implementation commit/push 与 implementation remote release-gate inspection；implementation commit `0b78531` 已推送。implementation run `30148331819` completed failure，Linux/Windows/macOS jobs `89654278355`/`89654278337`/`89654278333` 均 `steps=[]`，仍属既有 runner/billing blocker，不能声明 remote CI green。本 release inspection record 仅记录该 implementation run；不要为 inspection commit 自身 CI 追加第三个记录提交，除非出现不同于既有 `steps=[]` 的新远程信号。本批延续 Batch 581 的 Mission Commander intervention handoff closure：单个 open intervention 已可 concrete WhatIf→Apply，但多个 open interventions 全都有 concrete `eventId` 时，Mission Commander 仍只能给 `/rekit handoff <lane>` 加 `<eventId>` placeholder，replacement executor 需要回查 ledger 手工构造每个 preview command。

目标：多个 open interventions 时不自动选择任一 intervention，primary 继续保持 `/rekit handoff <lane>`；但 follow-up 应按每个 concrete `eventId` 列出 `/rekit reconcile <lane> -InterventionId <eventId> -WhatIf` preview option，并追加 blocked `continue -WhatIf`。当 open intervention 缺少 `eventId` 或存在重复/不完整 selection 信息时，继续保留 `<eventId>` placeholder 作为 fail-closed fallback。

已实现内容：

- Mission `LaneMissionCommanderActionForLane` 的 multi-intervention 分支现在通过 `multiInterventionReconcilePreviewCommands` 生成去重后的 concrete reconcile preview commands；all-concrete 多 intervention 不再显示 `<eventId>` placeholder。
- mixed/unidentified 多 intervention 会同时投影已知 concrete preview option 与 `<eventId>` placeholder，避免 silent selection 或猜测缺失 event。
- CLI overview product-path coverage 验证多个 concrete main interventions 时，first-screen summary 保持 handoff primary，同时列出每个 concrete `-WhatIf` follow-up，并确保 overview 是 read-only。

边界：本批只改变 Mission Commander 可消费 handoff 与 follow-up option 列表；多 intervention primary 仍不自动选择、不 Apply；`reconcile -WhatIf` 仍 read-only，`reconcile -Apply` 不在本批多 intervention primary 中自动暴露；不写 authority/confirmed、不执行 heavy tool、不新增 PowerShell runtime logic。

验证结果：focused `go test ./internal/rekit/mission ./internal/rekit/cli -count=1` 已通过；完整本地 release minimum 已通过：`go run ./cmd/rekit -- -Command release-check -Format json` 返回 `ready=true`，`go run ./cmd/rekit -- -Command status`、`go run ./cmd/rekit -- -Command packs`、`go run ./cmd/rekit -- -Command doctor`、`go test ./...`、`go vet ./...` 与 `git diff --check` 均通过（仅保留 Windows 工作树 LF→CRLF 提示）。remote inspection 已记录：implementation run `30148331819` completed failure，Linux/Windows/macOS jobs `89654278355`/`89654278337`/`89654278333` 均 `steps=[]`，未提供代码执行日志，按既有 runner/billing blocker 记录，不能声明 remote CI green。

### Batch 581：blocked lane concrete reconcile WhatIf→Apply handoff closure

状态：已完成 runtime、CLI product-path coverage、docs、完整本地 release minimum、implementation commit/push 与 implementation remote release-gate inspection；implementation commit `60e49ff` 已推送。implementation run `30147267697` completed failure，Linux/Windows/macOS jobs `89651422740`/`89651422756`/`89651422759` 均 `steps=[]`，仍属既有 runner/billing blocker，不能声明 remote CI green。本 release inspection record 仅记录该 implementation run；不要为 inspection commit 自身 CI 追加第三个记录提交，除非出现不同于既有 `steps=[]` 的新远程信号。本批延续 Mission Commander operational closure：blocked lane 的 first-screen action 过去在 open intervention 场景给出 `/rekit reconcile <lane> -InterventionId <eventId> -Apply` 占位符或直 Apply，replacement executor 必须先手工回查 ledger 选择 eventId，且第一动作缺少 read-only reconcile preview。

目标：让 `status` / `overview` / `start` / `handoff` / `continue` 的 Mission Commander action queue 在单个 open intervention 且有 concrete `eventId` 时，primary action 直接给出 `/rekit reconcile <lane> -InterventionId <eventId> -WhatIf`；bounded `/rekit reconcile ... -Apply` 仅作为 blocked/review-required follow-up，随后才是 blocked `continue -WhatIf` 与 `handoff`。当存在多个 open intervention 或没有 eventId 时，仍 fail-closed 到 handoff 和 placeholder WhatIf，不猜测 event。

已实现内容：

- Mission `LaneExecutorAction` 现在把 effective open intervention items 传入 Mission Commander action 生成逻辑；单一 concrete intervention 会生成 concrete `-WhatIf` primary、bounded `-Apply` follow-up 与 review boundary。
- `MissionCommanderNextActions` 对 concrete reconcile `-WhatIf` primary 标记为 unblocked 但 requires review，使 action queue 的 current action 可以先执行只读 preview；`-Apply` / `continue -WhatIf` / `handoff` 仍保持 blocked follow-up。
- CLI product-path tests 覆盖 overview concrete `int-main-1`、start/handoff/continue concrete `evt-human-stop`、durable handoff / lane RESUME projection，以及 placeholder fail-closed 行为不被误升级。

边界：本批只改变 Mission Commander 可消费 handoff 与 action queue 排序；`reconcile -WhatIf` 仍 read-only，`reconcile -Apply` 只写 bounded intervention resolution / lane state / durable handoff，不写 authority/confirmed、不执行 heavy tool；多 intervention 或缺 eventId 时不猜测；不新增 PowerShell runtime logic。

验证结果：focused `go test ./internal/rekit/mission ./internal/rekit/workstream ./internal/rekit/cli -count=1` 已通过；完整本地 release minimum 已通过：`go run ./cmd/rekit -- -Command release-check -Format json` 返回 `ready=true`，`go run ./cmd/rekit -- -Command status`、`go run ./cmd/rekit -- -Command packs`、`go run ./cmd/rekit -- -Command doctor`、`go test ./...`、`go vet ./...` 与 `git diff --check` 均通过（仅保留 Windows 工作树 LF→CRLF 提示）。remote inspection 已记录：implementation run `30147267697` completed failure，Linux/Windows/macOS jobs `89651422740`/`89651422756`/`89651422759` 均 `steps=[]`，未提供代码执行日志，按既有 runner/billing blocker 记录，不能声明 remote CI green。

### Batch 580：authorized adapter report validate→record hash handoff closure

状态：已完成 runtime、CLI、Mission/workstream downstream projection、tests、docs、完整本地 release minimum、implementation commit/push 与 implementation remote release-gate inspection；implementation commit `6fbb9f7` 已推送。implementation run `30145529056` completed failure，Linux/Windows/macOS jobs `89646557292`/`89646557332`/`89646557343` 均 `steps=[]`，仍属既有 runner/billing blocker，不能声明 remote CI green。本 release inspection record 仅记录该 implementation run；不要为 inspection commit 自身 CI 追加第三个记录提交，除非出现不同于既有 `steps=[]` 的新远程信号。本批延续 Batch 546–547、568–569：adapter execution report sidecar 已能 scaffold/draft、strict validate 并在 valid 后显式 record bounded observation evidence，但 validate→record 之间 sidecar 仍可能 drift；replacement executor 看到 valid validation envelope 或 downstream record command 后，如果 sidecar 被改写，旧命令缺少 currentness proof，可能把 stale adapter report 记录为 execution observation evidence。

目标：让 read-only `gate -ValidateExecutionReport` 计算 adapter report sidecar SHA-256，并只在 `valid=true` 时投影带 `-ExpectedExecutionReportSha256 <hash>` 的 record command；`gate -Apply -ExecutionReportPath ...` 在记录 observation evidence 前，如果收到 expected hash，必须重新读取 sidecar 并复核当前 SHA-256，drift 时 fail-closed 且 zero-write。status / overview / handoff / continue / durable Markdown / lane RESUME / checkpoint / run digest 也要投影 report hash 与 hash-gated record command。

已实现内容：

- `gate` strict adapter report reader 现在在完成 non-symlink regular file、size limit 与 strict JSON decode 前后保留 raw sidecar bytes SHA-256；validation JSON/text、compact `reportSummary` 与 Mission Commander action/next steps 在 valid sidecar 上同步输出 `reportSha256` 和 `recordExpectedReportSha256`。
- validation 生成的 record command 现在带 `-ExpectedExecutionReportSha256`；contract/scaffold/draft 阶段仍保留无 expected hash 的 generic record guidance，避免把未验证 sidecar 误标为 current。
- `gate -Apply` 记录 adapter execution evidence 时若提供 expected hash，会先校验 hex SHA-256 格式，再重读 sidecar、比较当前 hash；hash mismatch 返回 `adapter execution report sha256 changed after validation`，不写 observations ledger。
- observation evidence、Mission `executionEvidenceReview[]` / summary、status/overview/handoff/continue text、durable handoff Markdown、lane `RESUME.md`、checkpoint 与 continue run digest 均投影 recorded `executionReportSha256` / latest report hash，replacement executor 不必回查 sidecar 才能确认 evidence 绑定的 report bytes。
- CLI/gate/product-path coverage 覆盖 read-only validation hash handoff、valid record command hash injection、invalid report只投影 raw hash不投影 expected record hash、record drift fail-closed zero-write、nested no-pack product path及 downstream/durable evidence review hash projection。

边界：本批只增强 adapter execution report validate→record currentness handoff 与已记录 observation evidence 的只读 projection；`/rekit` 不执行 adapter/heavy tool、不 replay、不自动 record、不写 authority/confirmed；validation 仍 read-only；record 仍只在主 Agent 显式 `gate -Apply` 且 sidecar `valid=true` 后写 bounded observation evidence；不新增 PowerShell runtime logic，不把真实 case artifact / trace / dump / payload / flag / 客户信息写入模板仓库。

验证结果：focused `go test ./internal/rekit/gate ./internal/rekit/mission ./internal/rekit/workstream ./internal/rekit/cli -count=1` 已通过；完整本地 release minimum 已通过：`go run ./cmd/rekit -- -Command release-check -Format json` 返回 `ready=true`，`go run ./cmd/rekit -- -Command status`、`go run ./cmd/rekit -- -Command packs`、`go run ./cmd/rekit -- -Command doctor`、`go test ./...`、`go vet ./...` 与 `git diff --check` 均通过（仅保留 Windows 工作树 LF→CRLF 提示）。remote inspection 已记录：implementation run `30145529056` completed failure，Linux/Windows/macOS jobs `89646557292`/`89646557332`/`89646557343` 均 `steps=[]`，未提供代码执行日志，按既有 runner/billing blocker 记录，不能声明 remote CI green。

### Batch 579：reviewer prompt artifact deterministic repair closure

状态：已完成 runtime、CLI、workstream intake projection、tests、docs、完整本地 release minimum、implementation commit/push 与 implementation remote release-gate inspection；implementation commit `bbbf219` 已推送。implementation run `30141869464` completed failure，Linux/macOS/Windows jobs `89636549004`/`89636549030`/`89636549032` 均 `steps=[]`，仍属既有 runner/billing blocker，不能声明 remote CI green。本 release inspection record 仅记录该 implementation run；不要为 inspection commit 自身 CI 追加第三个记录提交，除非出现不同于既有 `steps=[]` 的新远程信号。本批延续 Batch 577–578：`plan-subagents` 已生成 hash-bound `prompts/<shard>.prompt.md` prompt artifact，downstream `status` / `handoff` / `continue` 也会在 artifact missing / invalid / symlink / unverified / drift 时 fail-closed，但 replacement executor 仍缺少一个 Go-native、hash-gated、packet-derived 的恢复命令，只能手工 restore/regenerate，容易误覆盖 drifted prompt 或重新调度 stale reviewer 输入。

目标：让 blocked prompt artifact handoff 提供 deterministic repair path：主 Agent 先运行 `plan-subagents -RepairReviewerPromptArtifact -PacketPath ... -ShardId ... -WhatIf` 复核 canonical packet 派生的 prompt bytes/path/hash，再用返回的 `-ExpectedPromptSha256 ... -Apply` 只恢复 missing `prompts/<shard>.prompt.md` 或 exact replay；已有 drifted / empty / symlink / directory / non-regular / different artifact 必须 fail-closed 且不覆盖。

已实现内容：

- 新增 `subagents.RepairReviewerPromptArtifact` runtime 与 `ReviewerPromptArtifactRepairResult`，严格绑定 attached case、canonical review packet namespace、packet integrity、manifest route、lane、shard、`ShardHandoff` / `ReviewerOrchestration.dispatches[]` prompt path/hash/prompt 一致性。
- `plan-subagents -RepairReviewerPromptArtifact` CLI 支持 WhatIf / expected-hash Apply 与 JSON/text 输出；WhatIf 不写入，Apply 在共享 reviewer lock 内重读 packet/integrity、重算 prompt SHA-256，并通过 temp + no-replace link 只发布 missing canonical prompt artifact。
- downstream reviewer dispatch intake handoff 新增 `dispatchPromptRepairCommand` 与 summary `nextActionDispatchPromptRepairCommand`；prompt artifact blocked state 的 next action 现在指向 repair WhatIf command，同时继续清空 runnable reviewer dispatch command，直到 currentness 恢复为 `promptCurrent=true`。
- Subagents unit coverage 与 CLI E2E 覆盖 missing prompt repair preview/apply、exact replay、drifted prompt no-overwrite、status JSON/text repair command projection，以及 Batch 578 currentness fail-closed 不回退。

边界：本批只做 packet-derived prompt artifact repair preview/hash-gated restore；不修补 packet、不覆盖 drifted artifact、不 spawn/stop/poll/monitor reviewer、不创建 reviewer result、不执行 staging/collection/intake Apply、不写 verification/decision/authority/confirmed、不执行 heavy tool、不新增 PowerShell runtime logic。

验证结果：focused `go test ./internal/rekit/subagents ./internal/rekit/workstream ./internal/rekit/cli -count=1` 已通过；完整本地 release minimum 已通过：`go run ./cmd/rekit -- -Command release-check -Format json` 返回 `ready=true`，`go run ./cmd/rekit -- -Command status`、`go run ./cmd/rekit -- -Command packs`、`go run ./cmd/rekit -- -Command doctor`、`go test ./...`、`go vet ./...` 与 `git diff --check` 均通过。remote inspection 已记录：implementation run `30141869464` completed failure，Linux/macOS/Windows jobs `89636549004`/`89636549030`/`89636549032` 均 `steps=[]`，未提供代码执行日志，按既有 runner/billing blocker 记录，不能声明 remote CI green。

### Batch 578：reviewer prompt artifact currentness handoff closure

状态：已完成 runtime、CLI、workstream intake projection、tests、docs、完整本地 release minimum、implementation commit/push 与 implementation remote release-gate inspection；implementation commit `3d60078` 已推送。implementation run `30140168734` completed failure，Linux/Windows/macOS jobs `89631766382`/`89631766398`/`89631766439` 均 `steps=[]`，仍属既有 runner/billing blocker，不能声明 remote CI green。本 release inspection record 仅记录该 implementation run；不要为 inspection commit 自身 CI 追加第三个记录提交，除非出现不同于既有 `steps=[]` 的新远程信号。本批延续 Batch 577：`plan-subagents` 已生成 hash-bound `prompts/<shard>.prompt.md` artifact 并投影 path/hash，但 downstream `status` / `handoff` / `continue` 仍可能在 prompt artifact 被删除、清空、换成 symlink、移动到 packet prompts 目录外或 SHA-256 drift 后继续显示 dispatch command，replacement executor 容易把 stale/missing prompt 当成可调度 reviewer 输入。

目标：让 reviewer dispatch intake handoff 在推荐调度 read-only reviewer 前只读验证 prompt artifact currentness：必须位于同一 packet review root 的 `prompts/` 目录、non-symlink、non-empty regular file，且实际 SHA-256 匹配 packet 中的 `promptSha256`。缺失、invalid、symlink、unverified 或 drift 时 fail-closed 为 blocked prompt artifact state，并在 JSON/text/durable handoff 中投影 state、current、actual hash、failure 与 next action。

已实现内容：

- `ReviewerDispatchIntakeHandoff` 新增 `dispatchPromptState`、`dispatchPromptCurrent`、`dispatchPromptActualSha256` 与 `dispatchPromptFailure`；summary 同步新增 latest / next-action prompt currentness 字段与 `promptArtifactBlocked` 计数。
- workstream intake handoff 现在从 dispatch 或 `agentToolRequest` 解析 prompt path/hash，校验 prompt parent 必须绑定同一 review packet 的 `prompts/` 目录，并通过 stable read 计算 SHA-256；missing / invalid / symlink / unverified / drift 均不会继续暴露 runnable dispatch command。
- Mission Commander action priority、next action、evidence、boundary、status/handoff/continue text 与 durable Markdown handoff 均投影 prompt artifact currentness；blocked prompt artifact 要求先 restore/regenerate prompt artifact 并验证 `promptSha256`，再调度 reviewer。
- Workstream unit coverage 与 CLI E2E 覆盖 current prompt artifact、missing prompt artifact fail-closed、hash drift fail-closed、status JSON/text prompt blocker projection，以及 existing reviewer staging/collection/intake flow 不回退。

边界：本批只对既有 reviewer prompt artifact 做只读 currentness validation 与 downstream handoff 投影；不重写 prompt artifact、不修补 packet、不 spawn/stop/poll/monitor reviewer、不创建 reviewer result、不执行 staging/collection/intake Apply、不写 verification/decision/authority/confirmed、不执行 heavy tool、不新增 PowerShell runtime logic。

验证结果：focused `go test ./internal/rekit/subagents ./internal/rekit/workstream ./internal/rekit/cli -count=1` 已通过；完整本地 release minimum 已通过：`go run ./cmd/rekit -- -Command release-check -Format json` 返回 `ready=true`，`go run ./cmd/rekit -- -Command status`、`go run ./cmd/rekit -- -Command packs`、`go run ./cmd/rekit -- -Command doctor`、`go test ./...`、`go vet ./...` 与 `git diff --check` 均通过。remote inspection 已记录：implementation run `30140168734` completed failure，Linux/Windows/macOS jobs `89631766382`/`89631766398`/`89631766439` 均 `steps=[]`，未提供代码执行日志，按既有 runner/billing blocker 记录，不能声明 remote CI green。

### Batch 577：reviewer dispatch prompt artifact closure

状态：已完成 runtime、CLI、workstream intake projection、tests、docs、完整本地 release minimum、implementation commit/push 与 implementation remote release-gate inspection；implementation commit `916e1e9` 已推送。implementation run `30138173646` completed failure，Windows/Linux/macOS jobs `89626133314`/`89626133327`/`89626133336` 均 `steps=[]`，仍属既有 runner/billing blocker，不能声明 remote CI green。本 release inspection record 仅记录该 implementation run；不要为 inspection commit 自身 CI 追加第三个记录提交，除非出现不同于既有 `steps=[]` 的新远程信号。本批延续 Batch 514、523–525、541、549、562 与 567：reviewer orchestration 已能生成 multi-shard dispatch/intake/collection handoff，但 replacement executor 仍需要从 nested JSON 中手工复制 `dispatchPrompt` 或 `agentToolRequest.prompt` 长文本，容易漏掉 owner binding、route output contract、result/staging path、no-heavy/no-authority boundary 与只读 reviewer 约束。

目标：让 `plan-subagents` 为每个 reviewer shard 生成 durable prompt artifact，并把 `promptPath` / `promptSha256` 投影到 planning packet、reviewer orchestration summary、shard handoff、status/handoff/continue reviewer dispatch intake 与 CLI text first-screen；replacement executor 只需读取 hash-bound prompt artifact 调度 read-only reviewer，而不再从 nested JSON 手工复制长 prompt。

已实现内容：

- `plan-subagents` 在 review root 下新增 `prompts/<shard>.prompt.md` artifact root；每个 shard 的 dispatch prompt 会以确定性 newline-normalized bytes 写入 prompt artifact，并计算 SHA-256。
- `ShardHandoff`、`ReviewerDispatch`、`ReviewerAgentToolRequest` 与 `ReviewerOrchestrationSummary.dispatches[]` 现在同步携带 `dispatchPromptPath` / `dispatchPromptSha256` 或 `promptPath` / `promptSha256`；dispatch command 优先显示 `prompt artifact "<path>" (sha256=<hash>)`。
- reviewer lifecycle、Mission Commander next actions、summary.md、terminal text、case `status` / `handoff` / `continue` reviewer dispatch intake handoff 均投影 prompt artifact path/hash，并提示先验证 hash 再把 artifact 内容交给 read-only reviewer。
- CLI/package coverage 覆盖 prompt artifact 写入、path/hash 传播、artifact content/hash 校验、first dispatch summary text、shard handoff text 与 downstream reviewer dispatch intake prompt artifact visibility。

边界：本批只生成和投影 reviewer prompt artifact；不 spawn、stop、poll 或 monitor reviewer，不创建 reviewer result，不执行 staging/collection/intake Apply，不写 verification/decision/authority/confirmed，不执行 heavy tool，不新增 PowerShell runtime logic。旧 raw `dispatchPrompt` 字段仍保留用于兼容既有 JSON 消费者，但 product-path handoff 优先使用 hash-bound prompt artifact。

验证结果：focused `go test ./internal/rekit/subagents ./internal/rekit/workstream ./internal/rekit/cli -count=1` 已通过；完整本地 release minimum 已通过：`go run ./cmd/rekit -- -Command release-check -Format json` 返回 `ready=true`，`go run ./cmd/rekit -- -Command status`、`go run ./cmd/rekit -- -Command packs`、`go run ./cmd/rekit -- -Command doctor`、`go test ./...`、`go vet ./...` 与 `git diff --check` 均通过。remote inspection 已记录：implementation run `30138173646` completed failure，Windows/Linux/macOS jobs `89626133314`/`89626133327`/`89626133336` 均 `steps=[]`，未提供代码执行日志，按既有 runner/billing blocker 记录，不能声明 remote CI green。

### Batch 576：case-local pack-memory next-missing proof binding closure

状态：已完成 runtime、CLI、release/status handoff、tests/docs、本地 release minimum、implementation commit/push 与 implementation remote release-gate inspection；implementation commit `f237cf3` 已推送。implementation run `30136214782` completed failure，Windows/Linux/macOS jobs `89620488573`/`89620488596`/`89620488627` 均 `steps=[]`，仍属既有 runner/billing blocker，不能声明 remote CI green。本 release inspection record 仅记录该 implementation run；不要为 inspection commit 自身 CI 追加第三个记录提交，除非出现不同于既有 `steps=[]` 的新远程信号。本批延续 Batch 565 与 Batch 570–575：case-mode `status` 已能从 attached case 的 `.rekit/reviews/**/packet.json` 绑定 `DecisionDraftHandoff`，但 nested `packMemoryCandidates.reviewSummary.proofSummary.nextMissingProof` 仍保留 `<packet.json>` / `<review-evidence-ref>` placeholder，replacement executor 在第一屏看到 decision draft handoff 后，还要手工回找 packet 与 evidence refs 才能接续 proof draft。

目标：让 case-local `status` 在发现严格绑定当前 repo/case/pack 的 durable candidate review packet 时，把同一 packet-derived review workspace 注入 next missing proof summary，使 Mission Commander / replacement executor 可直接从 first-screen status 拿到 concrete packet path、candidate decision path、evidence refs 与 runnable proof draft command；kit-only `release-check` 仍保持只读 fallback，不推断 case-local packet。

已实现内容：

- `ReleaseHandoffPackMemoryCandidateReviewNextMissingProof` 新增只读 handoff 字段 `packetPath`、`candidateDecisionPath` 与 `evidenceRefs`，用于承载 case-mode status 的 packet-derived proof draft context。
- case-mode `status` 在绑定 durable `DecisionDraftHandoff` 后，同步把 matching packet path、candidate decision path 与 candidate decision note evidence refs 注入 nested next missing proof；draft command / apply template 会替换 `<packet.json>`、`<candidate-decisions.json>` 与 `<review-evidence-ref>` placeholder。
- CLI text first-screen 的 `status pack-memory next missing proof` 行现在显示 packet、candidateDecision、draft、draftApply，并逐行输出 proof evidence ref 与 boundary，避免 replacement executor 扫描完整 review packet 才能执行 proof draft。
- CLI product-path coverage 覆盖 nested JSON proof binding、placeholder removal、packet-derived evidence refs、text command/evidence/boundary 输出；releasecheck/promote focused coverage 保持 kit-only handoff 与 proof draft attestation 不回退。

边界：本批只在 case-local status 读取已存在、严格绑定的 durable review packet 后增强 proof handoff；不生成 proof、不运行 proof draft、不写 decision/proof file、不 merge/cleanup candidates、不运行 doctor/init/reconsume、不执行 heavy tool、不写 authority/confirmed、不新增 PowerShell runtime logic。`release-check` / kit-only status 不推断 case-local packet，也不能据此声明 remote CI green。

验证结果：focused `go test ./internal/rekit/cli ./internal/rekit/releasecheck ./internal/rekit/promote -count=1` 已通过；完整本地 release minimum 已通过：`go run ./cmd/rekit -- -Command release-check -Format json` 返回 `ready=true`，`go run ./cmd/rekit -- -Command status`、`go run ./cmd/rekit -- -Command packs`、`go run ./cmd/rekit -- -Command doctor`、`go test ./...`、`go vet ./...` 与 `git diff --check` 均通过（仅保留 Windows 工作树 LF→CRLF 提示）。remote inspection 已记录：implementation run `30136214782` completed failure，Windows/Linux/macOS jobs `89620488573`/`89620488596`/`89620488627` 均 `steps=[]`，未提供代码执行日志，按既有 runner/billing blocker 记录，不能声明 remote CI green。

### Batch 575：pack-memory lifecycle proof deterministic draft closure

状态：已完成 runtime、CLI、release/status handoff、tests、本地 release minimum、implementation commit/push 与 implementation remote release-gate inspection；implementation commit `0524688` 已推送。implementation run `30134331373` completed failure，macOS/Windows/Linux jobs `89615073082`/`89615073103`/`89615073118` 均 `steps=[]`，仍属既有 runner/billing blocker，不能声明 remote CI green。本 release inspection record 仅记录该 implementation run；不要为 inspection commit 自身 CI 追加第三个记录提交，除非出现不同于既有 `steps=[]` 的新远程信号。本批延续 Batch 570–574：candidate decision note、receipt-derived cleanup proof 与 lifecycle proof strict attestation 已闭环，但 `pack-doctor-output`、`fresh-case-reconsume-proof` 与 `attached-case-reconsume-proof` 仍需要 replacement executor 手工撰写 strict JSON proof note，容易遗漏 stage/checks/evidence hash 或写入 case 绝对路径。

目标：让 Mission Commander / replacement executor 在 review candidate 后，用 Go-native `promote -DraftReviewProof` 为 lifecycle/reconsume proof 生成 deterministic、packet/candidate/evidence-bound strict JSON proof note；先 WhatIf 复核 `proofSha256` 与 exact proof bytes，再用 expected-hash Apply 只写 repo-local review artifact，随后由 release/status strict attestation 接续 proof summary。

已实现内容：

- 新增 `promote.DraftCandidateLifecycleProof` runtime，支持 `pack-doctor-output`、`fresh-case-reconsume-proof` 与 `attached-case-reconsume-proof`；严格绑定 candidate review packet、attached repo/case/pack、canonical candidate/tooling roots、explicit candidatePath、proof path、reason/actor 与 repo-local hashed evidence refs。
- Lifecycle proof draft 写出 strict JSON `pack-memory-candidate-lifecycle-proof`，包含 proof type、repo-relative candidatePath、packTarget、reviewItem stage、checks、boundary 与 evidence SHA-256；Apply 要求 WhatIf 返回的 `proofSha256`，只写 repo-local proof file 或 exact replay，different existing proof fail-closed。
- CLI `promote -DraftReviewProof` 会按 proof type 分流到 lifecycle draft runtime，JSON/text 输出 preview/apply command、checks、evidence 与 no-heavy/no-authority boundary；lifecycle draft 显式拒绝 `-CandidateDecisionPath` 与 `-ProofDecision`，并拒绝 case-local evidence refs。
- release/status next missing lifecycle proof handoff 投影 packet-required WhatIf/Apply templates；open review artifact 的 managed/tooling packTarget 与 generated lifecycle proof note 保持一致，使 Batch 574 strict attestation 可直接识别生成的 proof。
- package/CLI/releasecheck coverage 锁定 preview no-write、hash-gated Apply、exact replay、absolute path zero-persistence、unsafe input refusal、case-local product path与 downstream status/release proof handoff。

边界：本批只生成和写入 lifecycle/reconsume proof note，并增强只读 release/status handoff；不运行 doctor/init/reconsume、不 merge/cleanup candidates、不执行 verification provisioning/final verification/retirement、不执行 heavy tool、不写 authority/confirmed、不新增 PowerShell runtime logic。

验证结果：focused `go test ./internal/rekit/promote ./internal/rekit/cli ./internal/rekit/releasecheck -count=1` 已通过；完整本地 release minimum 已通过：`go run ./cmd/rekit -- -Command release-check -Format json` 返回 `ready=true`，`go run ./cmd/rekit -- -Command status`、`go run ./cmd/rekit -- -Command packs`、`go run ./cmd/rekit -- -Command doctor`、`go test ./...`、`go vet ./...` 与 `git diff --check` 均通过（仅保留 Windows 工作树 LF→CRLF 提示）。remote inspection 已记录：implementation run `30134331373` completed failure，macOS/Windows/Linux jobs `89615073082`/`89615073103`/`89615073118` 均 `steps=[]`，未提供代码执行日志，按既有 runner/billing blocker 记录，不能声明 remote CI green。

### Batch 574：pack-memory lifecycle proof strict release attestation

状态：已完成 release/status lifecycle proof strict attestation runtime、tests、本地 release minimum、implementation commit/push 与 implementation remote release-gate inspection；implementation commit `d8bf7c6` 已推送。implementation run `30130691085` completed failure，Linux/Windows/macOS jobs `89604301700`/`89604301705`/`89604301761` 均 `steps=[]`，仍属既有 runner/billing blocker，不能声明 remote CI green。本 release inspection record 仅记录该 implementation run；不要为 inspection commit 自身 CI 追加第三个记录提交，除非出现不同于既有 `steps=[]` 的新远程信号。本批延续 Batch 570–573：candidate decision note 与 receipt-derived cleanup proof 已 strict attested，但 open review artifacts 中的 `candidate-cleanup-proof`、`pack-doctor-output`、`fresh-case-reconsume-proof` 与 `attached-case-reconsume-proof` 仍可能因 loose Markdown/command transcript placeholder 被当作 proof present，使 proof summary 提前推进 cleanup/reconsume stage。

目标：让 release/status 对 pack-memory lifecycle proof 只接受 strict JSON `pack-memory-candidate-lifecycle-proof`，并按 proof type 绑定 pack、candidatePath、packTarget、reviewItem、proof stage、checks、boundary 与 repo-local hashed evidence refs；loose placeholder、malformed JSON、absolute/escaping paths、missing checks、candidate cleanup state drift 或 evidence mismatch 都 fail-closed 为 release handoff warning。

已实现内容：

- `releasecheck` 在扫描 open review artifacts 时，不再只对 `candidate-decision-note` 做 strict validation；`candidate-cleanup-proof`、`pack-doctor-output`、`fresh-case-reconsume-proof` 与 `attached-case-reconsume-proof` 也必须 strict decode `pack-memory-candidate-lifecycle-proof` JSON。
- Lifecycle proof validator 绑定 `schemaVersion=1`、kind、pack、proofType、candidatePath、packTarget、reviewItem proofType/stage、reason、actor、boundary、no absolute/no escaping stored paths，以及 non-empty repo-local evidence refs；evidence 必须是 non-empty regular file 且 SHA-256 匹配。
- Per-proof checks fail-closed：cleanup proof 需要 `candidate-absent`，managed-doc cleanup 还需要 `index-entry-absent` 并重查当前 index；doctor proof 需要 `pack-doctor`；fresh/attached reconsume proof 分别需要 `fresh-case-reconsume` / `attached-case-reconsume` 加 `pack-doctor`。
- Promote/release review artifact format guidance 从 loose Markdown/command transcript 更新为 strict JSON proof note guidance；releasecheck coverage 显式断言 loose cleanup/doctor/fresh/attached lifecycle proof 被 warning 拒绝，valid lifecycle proof 才能计入 proof summary present。

边界：本批只增强 release/status 对已有 lifecycle proof 的只读 strict attestation 与 artifact guidance；不生成 lifecycle proof、不 merge/cleanup candidates、不运行 doctor/init/reconsume、不执行 verification provisioning/final verification/retirement、不执行 heavy tool、不写 authority/confirmed、不新增 PowerShell runtime logic。

验证结果：focused `go test ./internal/rekit/releasecheck ./internal/rekit/cli ./internal/rekit/promote -count=1` 已通过；完整本地 release minimum 已通过：`go run ./cmd/rekit -- -Command release-check -Format json`、`go run ./cmd/rekit -- -Command status`、`go run ./cmd/rekit -- -Command packs`、`go run ./cmd/rekit -- -Command doctor`、`go test ./...`、`go vet ./...` 与 `git diff --check` 均通过（仅保留 Windows 工作树 LF→CRLF 提示）。远程 release-gate implementation run `30130691085` 已检查，结论为 completed failure；Linux/Windows/macOS jobs `89604301700`/`89604301705`/`89604301761` 均 `steps=[]`，未提供代码执行日志，按既有 runner/billing blocker 记录，不能声明 remote CI green。

### Batch 573：pack-memory candidate decision proof strict release attestation

状态：已完成 release/status strict attestation runtime、tests、本地 release minimum、implementation commit/push 与 implementation remote release-gate inspection；implementation commit `1da3ec5` 已推送。implementation run `30127801426` completed failure，macOS/Linux/Windows jobs `89595308951`/`89595309015`/`89595309019` 均 `steps=[]`，仍属既有 runner/billing blocker，不能声明 remote CI green。本 release inspection record 仅记录该 implementation run；不要为 inspection commit 自身 CI 追加第三个记录提交，除非出现不同于既有 `steps=[]` 的新远程信号。本批延续 Batch 570–572：`promote -DraftReviewProof -ProofType candidate-decision-note` 已能生成 deterministic proof note，cleanup proof 也已 strict attested，但 open-residue `candidate-decision-note` 在 release/status 中仍可能因 loose `# decision` placeholder 被当作 proof present。

目标：让 release/status 对 open pack-memory `candidate-decision-note` 只接受 strict JSON `pack-memory-candidate-review-proof`，并绑定 schema/kind/proof type、pack、candidate identity/hash、packTarget、decision/reviewItem、boundary 与 relative evidence refs；loose Markdown、malformed JSON、hash drift、absolute/escaping paths、unsupported decision 或 invalid evidence 都 fail-closed 为 release handoff warning，而不是关闭 review stage。

已实现内容：

- `releasecheck` 的 open candidate review proof scanner 现在返回 error 并使用 `os.Lstat` 拒绝 symlink、目录、空文件与 oversize proof；`candidate-decision-note` proof 必须 strict decode 为 `pack-memory-candidate-review-proof` JSON，`proofType=candidate-decision-note`、`schemaVersion=1`、pack、candidatePath、candidateHash、packTarget、decision、reviewItem、reason、actor 与 boundary 均需匹配。
- Decision proof validation 会重算当前 candidate SHA-256；managed-doc `accept` 还会验证 packTarget 仍在 pack root 且 hash 匹配，`reject` / `superseded` 不允许伪造 packTarget hash；tooling candidate 继续不能被 auto-accept。
- Proof note 持久路径必须保持 relative、non-escaping、non-absolute；evidence refs 必须非空、去重并携带 SHA-256，repo-local evidence 存在时必须是 non-empty regular file 且 hash 匹配，case-local evidence 在 kit-only release/status 下只保留 relative/hash attestation，不要求把 case artifact 复制进 kit 仓库。
- CLI/status product-path fixture 不再用 `# decision` 占位关闭 proof stage，而是写 strict JSON proof；releasecheck coverage 显式断言 loose `# decision` 被 warning 拒绝后再验证 valid proof 可计入 `ProofPresent=true`。

边界：本批只增强 release/status 对已有 candidate decision proof 的只读 strict attestation；不生成 proof、不推断 case-local packet、不 merge/cleanup candidates、不运行 doctor/init/reconsume、不执行 verification provisioning/final verification/retirement、不执行 heavy tool、不写 authority/confirmed、不新增 PowerShell runtime logic。

验证结果：focused `go test ./internal/rekit/releasecheck ./internal/rekit/cli ./internal/rekit/promote -count=1` 已通过；完整本地 release minimum 已通过：`go run ./cmd/rekit -- -Command release-check -Format json`、`go run ./cmd/rekit -- -Command status`、`go run ./cmd/rekit -- -Command packs`、`go run ./cmd/rekit -- -Command doctor`、`go test ./...`、`go vet ./...` 与 `git diff --check` 均通过（仅保留 Windows 工作树 LF→CRLF 提示）。remote inspection 已记录：implementation run `30127801426` completed failure，macOS/Linux/Windows jobs `89595308951`/`89595309015`/`89595309019` 均 `steps=[]`，仍属既有 runner/billing blocker，当前不能声明 remote CI green。

### Batch 572：pack-memory cleanup proof strict release attestation

状态：已完成 release/status strict attestation runtime、tests、本地 release minimum、implementation commit/push 与 implementation remote release-gate inspection；implementation commit `6b2dc9e` 已推送。implementation run `30125818500` completed failure，Linux/macOS/Windows jobs `89588941832`/`89588941883`/`89588941910` 均 `steps=[]`，仍属既有 runner/billing blocker，不能声明 remote CI green。本 release inspection record 仅记录该 implementation run；不要为 inspection commit 自身 CI 追加第三个记录提交，除非出现不同于既有 `steps=[]` 的新远程信号。本批延续 Batch 571：candidate cleanup proof 已能由 committed receipt deterministic draft，但 release/status 仍把 `review-artifacts/*.candidate-cleanup-proof.{md,json,txt}` 的任意非目录文件当作 proof present，导致 `# cleanup proof` 这类 loose placeholder 可关闭 release handoff。

目标：让 release/status 对 receipt-derived `candidate-cleanup-proof` 只接受 strict JSON proof note，并重新绑定 durable receipt、candidate decision、transaction journal、committed marker、backup hashes、candidate absent、index entry absent、accepted packTarget hash 与 evidence refs；伪 proof、hash drift、绝对路径持久化或当前 cleanup state drift 均 fail-closed 为 release handoff warning。

已实现内容：

- `releasecheck` 在扫描 receipt-derived cleanup proof 时不再只看文件存在；会 strict decode `pack-memory-candidate-review-proof` JSON，并校验 `schemaVersion`、`proofType=candidate-cleanup-proof`、pack、packet hash、decision hash、candidate/packTarget/reviewItem identity、decision/reason/actor、boundary 与 no absolute / no escaping stored paths。
- Cleanup proof validator 绑定 committed receipt path/hash、transaction journal path/hash、committed marker path/hash、candidate backup path/hash、target backup path/hash、candidate absent 当前状态、managed-doc index entry absent 当前状态、index presence，以及 accepted packTarget current hash 必须等于 reviewed candidate backup hash。
- Evidence refs 必须非空、repo/case-local、relative stored、非 symlink regular file 且 SHA-256 匹配；malformed/loose Markdown proof、non-regular proof、receipt/backup/journal/hash/path/current-state drift 都会使 pack-memory candidate inventory 进入 warning，而不是误报 release ready。
- Release/status handoff 保持只读：只从 durable receipt/action 保留 hidden full-path authority 用于本机校验，对 JSON 输出仍只暴露 repo-relative public identity；proof 缺失继续投影 Batch 571 的 `promote -DraftReviewProof -ProofType candidate-cleanup-proof -CandidateDecisionPath ...` WhatIf/Apply template。

边界：本批只增强 release/status 对已存在 cleanup proof 的只读 strict attestation；不生成 proof、不 merge pack sources、不 cleanup candidate/index、不运行 doctor/init/reconsume、不执行 verification provisioning/final verification/retirement、不执行 heavy tool、不写 authority/confirmed、不新增 PowerShell runtime logic。

验证结果：focused `go test ./internal/rekit/releasecheck ./internal/rekit/cli ./internal/rekit/promote -count=1` 已通过；完整本地 release minimum 已通过：`go run ./cmd/rekit -- -Command release-check -Format json`、`go run ./cmd/rekit -- -Command status`、`go run ./cmd/rekit -- -Command packs`、`go run ./cmd/rekit -- -Command doctor`、`go test ./...`、`go vet ./...` 与 `git diff --check` 均通过（仅保留 Windows 工作树 LF→CRLF 提示）。remote inspection 已记录：implementation run `30125818500` completed failure，Linux/macOS/Windows jobs `89588941832`/`89588941883`/`89588941910` 均 `steps=[]`，仍属既有 runner/billing blocker，当前不能声明 remote CI green。

### Batch 571：pack-memory candidate cleanup proof draft closure

状态：已完成 Go runtime、CLI route/text、release/status receipt-derived handoff、tests、本地 release minimum、implementation commit/push 与 implementation remote release-gate inspection；implementation commit `8a69ced` 已推送。implementation run `30121928443` completed failure，macOS/Linux/Windows jobs `89576438286`/`89576438289`/`89576438299` 均 `steps=[]`，仍属既有 runner/billing blocker，不能声明 remote CI green。本 release inspection record 仅记录该 implementation run；不要为 inspection commit 自身 CI 追加第三个记录提交，除非出现不同于既有 `steps=[]` 的新远程信号。本批延续 Batch 570：candidate decision note proof 已能 deterministic draft，但 candidate decision Apply 已清理 candidate/index 之后，`candidate-cleanup-proof` 仍只能靠人工命令 transcript 或 loose Markdown，replacement executor 在 candidate 文件已消失后还必须手工从 receipt/backup/journal 拼证明。

目标：让 Mission Commander / replacement executor 在 committed candidate decision receipt 之后，用 Go-native `promote -DraftReviewProof -ProofType candidate-cleanup-proof -CandidateDecisionPath ...` 生成可复核、hash-gated、repo-local 的 cleanup proof draft；release/status 即使 candidatePath 已删除，也能从 durable receipt 投影下一份 missing cleanup proof、所需 packet/decision 与 WhatIf/Apply template。

已实现内容：

- `promote.DraftCandidateReviewProof` 支持 `candidate-cleanup-proof`：严格绑定 candidate review packet、candidate decision file、decision receipt、transaction journal、committed marker、candidate backup、candidatePath absent、managed-doc index entry absent 与 accepted packTarget current hash。
- Cleanup proof note 持久内容只保存 repo-relative receipt/transaction/committed/backup/candidate/packTarget identity、packet/decision/receipt/journal/backup hashes、decision/reason/actor、evidence SHA-256 与 boundary；不保存 repoRoot/caseRoot 绝对路径。Apply 仍要求 WhatIf 返回的 `proofSha256`，只写 repo-local `review-artifacts` proof file 或 exact replay；different existing proof fail-closed。
- CLI `promote -DraftReviewProof` 只在 `-ProofType candidate-cleanup-proof` 下接受 `-CandidateDecisionPath`，text 输出 cleanup receipt/transaction/committed/backup、candidate absent 与 index absent checks；case-local product path 覆盖 decision Apply 后 cleanup proof WhatIf/Apply/text 接续。
- release/status 对 committed candidate decision receipts 生成 receipt-derived `candidate-cleanup-proof` review artifact、proof summary stage、`RequiresPacket` / `RequiresCandidateDecision` 与 draft/apply templates；cleanup proof 缺失会阻塞 release handoff，proof 到位后继续既有 accepted verification/provision/retirement flow。

边界：本批只生成和写入 cleanup proof note，并增强只读 release/status handoff；不 merge pack sources、不 cleanup candidate/index（cleanup 已由 explicit candidate decision Apply 完成并被 receipt 证明）、不运行 doctor/init/reconsume、不执行 verification provisioning/final verification/retirement、不执行 heavy tool、不写 authority/confirmed、不新增 PowerShell runtime logic。

验证结果：focused `go test ./internal/rekit/promote ./internal/rekit/cli ./internal/rekit/releasecheck -count=1` 已通过；完整本地 release minimum 已通过：`go run ./cmd/rekit -- -Command release-check -Format json` 返回 `ready=true`，`go run ./cmd/rekit -- -Command status`、`go run ./cmd/rekit -- -Command packs`、`go run ./cmd/rekit -- -Command doctor`、`go test ./...`、`go vet ./...` 与 `git diff --check` 均通过（仅保留 Windows 工作树 LF→CRLF 提示）。remote inspection 已记录：implementation run `30121928443` completed failure，macOS/Linux/Windows jobs `89576438286`/`89576438289`/`89576438299` 均 `steps=[]`，仍属既有 runner/billing blocker，当前不能声明 remote CI green。

### Batch 570：pack-memory candidate review proof draft closure

状态：已完成 Go runtime、CLI routing/text handoff、release/status next-missing proof template projection、tests、本地 release minimum、implementation commit/push 与 implementation remote release-gate inspection；implementation commit `163efef` 已推送。implementation run `30115319001` completed failure，Windows/macOS/Linux jobs `89554436431`/`89554436471`/`89554436637` 均 `steps=[]`，仍属既有 runner/billing blocker，不能声明 remote CI green。本 release inspection record 仅记录该 implementation run；不要为 inspection commit 自身 CI 追加第三个记录提交，除非出现不同于既有 `steps=[]` 的新远程信号。本批延续 Batch 500–534、538、563–566 的 pack-memory candidate review/decision/proof closure：之前 release/status 能指出下一份 missing proof，但 replacement executor 仍需手工撰写 proof note 内容与绑定 packet/candidate/evidence/hash，容易把 proof 写成不可审计的空 skeleton 或混入 case 绝对路径。

目标：让 Mission Commander / replacement executor 在 review candidate 后，用 Go-native `promote -DraftReviewProof` 生成可复核、hash-gated、repo-local 的 `candidate-decision-note` proof draft：先 WhatIf 复核 proof note 内容与 `proofSha256`，再 Apply 写入 `packs/<pack>/promote-candidates/review-artifacts/*.{md,json,txt}`，随后由只读 release/status proof summary 识别 presence 并推进后续 cleanup/reconsume proof 阶段。

已实现内容：

- 新增 `promote.DraftCandidateReviewProof` runtime 与 `promote -DraftReviewProof` CLI path：从 durable candidate review packet、explicit candidate path、proof decision、reason、actor 与 evidence refs 生成 deterministic `pack-memory-candidate-review-proof` note，返回 preview/apply commands、`proofSha256`、proof/candidate bindings、next steps 与 no-heavy/no-authority boundary。
- Apply 要求 `-ExpectedProofSha256` 匹配 WhatIf；只写 repo-local pack review-artifacts proof file 或 exact replay。different existing proof、unsupported proof type、unsupported/per-candidate decision、tooling auto-accept、缺 evidence、缺 candidate、越界 proof path、symlink/non-regular candidate/packTarget/evidence 均 fail-closed。
- Proof note 持久内容只保存 repo-relative candidate/packTarget identity、packet hash、candidate hash、accepted managed-doc packTarget hash、evidence SHA-256、decision/reason/actor 与 boundary；不会把 repoRoot/caseRoot 绝对路径写入 pack evidence。CLI result 仍返回绝对 path 方便本机执行，draft commands 支持 release/status 给出的 repo-relative `packs/...` proof/candidate args。
- release/status next missing proof 保持只读 inventory，不推断 case-local packet、不写 proof，但在 `candidate-decision-note` 上投影需要 `<packet.json>` 的 draft/apply template、`RequiresPacket`、`RequiresExplicitReview` 与 review-required boundary；CLI text first-screen 同步输出 template。
- package/CLI/releasecheck focused coverage 锁定 preview no-write、hash-gated Apply、exact replay、repo-relative template args、status/release proof handoff、absolute path zero-persistence 与 unsafe input refusal。

边界：本批只生成和写入 review 后的 repo-local candidate decision proof note；不 merge pack sources、不 cleanup candidate/index、不运行 doctor/init/reconsume、不执行 heavy tool、不写 authority/confirmed、不创建或推断 case-local packet、不新增 PowerShell runtime logic。

验证结果：focused `go test ./internal/rekit/promote ./internal/rekit/cli ./internal/rekit/releasecheck -count=1` 通过；完整本地 release minimum 已通过：`go run ./cmd/rekit -- -Command release-check -Format json` 返回 `ready=true`，`go run ./cmd/rekit -- -Command status`、`go run ./cmd/rekit -- -Command packs`、`go run ./cmd/rekit -- -Command doctor`、`go test ./...`、`go vet ./...` 与 `git diff --check` 均通过（仅保留 Windows 工作树 LF→CRLF 提示）。remote inspection 已记录：implementation run `30115319001` completed failure，Windows/macOS/Linux jobs `89554436431`/`89554436471`/`89554436637` 均 `steps=[]`，仍属既有 runner/billing blocker，当前不能声明 remote CI green。

### Batch 569：authorized-gate adapter report deterministic draft sidecar closure

状态：已完成 Go runtime、CLI text/status/workstream handoff、tests、本地 release minimum、implementation commit/push 与 implementation remote release-gate inspection；implementation commit `23841d7` 已推送。implementation run `30111240646` completed failure，Windows/Linux/macOS jobs `89541005392`/`89541005458`/`89541005459` 均 `steps=[]`，仍属既有 runner/billing blocker，不能声明 remote CI green。本 release inspection record 仅记录该 implementation run；不要为 inspection commit 自身 CI 追加第三个记录提交，除非出现不同于既有 `steps=[]` 的新远程信号。本批延续 Batch 568 的 scaffold lifecycle：scaffold 只给 replacement executor 一个 exact placeholder，而 draft path 进一步关闭 executor 必须手工编辑 `adapter-report.json` 关键字段的 product-path 断点。

目标：让 replacement executor 在 authorized-gate adapter sidecar lifecycle 中，从 contract/status/handoff 第一屏拿到 deterministic `gate -DraftExecutionReport` preview/apply handoff：先复核 executor-reported bounded fields 与 exact hash，再 hash-gated 写入缺失 sidecar或替换 exact scaffold，然后 read-only validate，最后只在 `valid=true` 后 record bounded observation evidence。

已实现内容：

- 新增 Go-native `gate -DraftExecutionReport` runtime/CLI path：preview 根据 authorized gate event、selected/explicit adapterId、execution status、actual budget、outputRefs/evidenceRefs、boundaryHits、escalation 与 summary 生成 deterministic `adapter-report.json` draft、`reportSha256`、hash-gated apply command、validate/record commands、Mission Commander action queue 与 no-heavy/no-record boundary。
- Draft Apply 要求 `-ExpectedExecutionReportSha256` 匹配 preview；只创建缺失 sidecar、幂等复用 exact draft，或替换 exact scaffold template。different existing sidecar、wrong hash、缺 adapterId（无 pack tooling candidate 时）、越界 refs、invalid status/boundary/summary 均 fail-closed。
- `gate -ExecutionReportContract` live validation handoff、case `status` JSON/text 与 workstream/durable Markdown handoff 现在投影 workspace-relative / case-relative draft preview/apply commands 与 `<reportSha256-from-draft-preview>` placeholder；exact scaffold live snapshot 返回 `adapter-report-scaffold-awaiting-draft`，把 current action 指向 draft，而不是把 placeholder 当成 invalid manually-repair sidecar 或 record-ready report。
- CLI/gate coverage 锁住 draft parse、preview no-write、hash-gated Apply、exact replay、wrong-hash refusal、different-existing refusal、exact scaffold replacement、validate-after-draft、status JSON/text draft handoff，以及 observations/authority/confirmed zero-write boundary。

边界：本批只生成和写入 bounded adapter execution report sidecar draft；不执行 adapter/heavy tool，不自动 validate 或 record observation evidence，不写 authority/confirmed，不创建输出 artifact，不改变 `valid=true` 后显式 record boundary，不新增 PowerShell runtime logic。

验证结果：focused `go test ./internal/rekit/gate ./internal/rekit/cli ./internal/rekit/workstream -count=1` 通过；完整本地 release minimum 已通过：`go run ./cmd/rekit -- -Command release-check -Format json` 返回 `ready=true`，`go run ./cmd/rekit -- -Command status`、`go run ./cmd/rekit -- -Command packs`、`go run ./cmd/rekit -- -Command doctor`、`go test ./...`、`go vet ./...` 与 `git diff --check` 均通过（仅保留 Windows 工作树 LF→CRLF 提示）。remote inspection 已记录：implementation run `30111240646` completed failure，Windows/Linux/macOS jobs `89541005392`/`89541005458`/`89541005459` 均 `steps=[]`，仍属既有 runner/billing blocker，当前不能声明 remote CI green。

### Batch 568：authorized-gate adapter report scaffold lifecycle closure

状态：已完成 Go runtime、CLI text/status/workstream handoff、tests、本地 release minimum、implementation commit/push 与 implementation remote release-gate inspection；implementation commit `fcf3947` 已推送。implementation run `30092290998` completed failure，Windows/Linux/macOS jobs `89478076140`/`89478076208`/`89478076210` 均 `steps=[]`，仍属既有 runner/billing blocker，不能声明 remote CI green。本 release inspection record 仅记录该 implementation run；不要为 inspection commit 自身 CI 追加第三个记录提交，除非出现不同于既有 `steps=[]` 的新远程信号。本批延续 Batch 527–528、535、547 的 authorized-gate adapter handoff closure，但不再只是字段投影：新增 Go-native deterministic scaffold preview/apply lifecycle，关闭 replacement executor 必须手工拼 `adapter-report.json` sidecar 的 product-path 断点。

目标：让 replacement executor 在 authorized-gate adapter sidecar lifecycle 中，从 contract/status/handoff/continue 第一屏直接获得可复核、hash-gated 的 `adapter-report.json` scaffold preview/apply、validate 与 record 顺序，而不是手工复制 sidecar template、猜 workspace-relative/case-relative path 或在 validate/record 前绕过 review-first boundary。

已实现内容：

- `gate -ExecutionReportContract` 的 live validation handoff 现在生成 deterministic `AdapterReportSidecarTemplate` bytes 与 `sidecarTemplateSha256`，并投影 workspace-relative / case-relative scaffold preview/apply args 和 commands；status、handoff、continue 与 durable Markdown 复用同一 scaffold/hash/validate/record handoff。
- 新增 `gate -ScaffoldExecutionReport` Go-native runtime/CLI path：默认只读 preview 返回 exact sidecar template、`reportSha256`、authorized report path、apply/validate/record commands、Mission Commander action queue 与 no-heavy/no-record boundary；`-Apply` 必须携带 preview 返回的 `-ExpectedExecutionReportSha256`。
- Scaffold Apply 只写缺失的 bounded `adapter-report.json` template；若目标已存在且 bytes 完全相同则返回 `already-scaffolded`，若目标存在不同 bytes 或 expected hash drift 则 fail-closed。workspace-relative path 从 authorized output workspace 接续，case-relative path 可从任意 case-local cwd 接续。
- CLI product path 覆盖 scaffold JSON preview no-write、text Apply、wrong-hash refusal、different existing sidecar refusal、status JSON/text scaffold handoff，以及 observations/authority/confirmed zero-write boundary；package tests 覆盖 preview/apply/replay、cwd-relative authorized path 与 contract scaffold args/hash projection。

边界：本批只写缺失 sidecar scaffold；不执行 adapter/heavy tool，不 validate sidecar，不 record observation evidence，不写 authority/confirmed，不自动创建输出 artifact，不改变 `gate -Apply -GateEventId ...` 的 valid=true record boundary，不新增 PowerShell runtime logic。

验证结果：focused `go test ./internal/rekit/gate ./internal/rekit/cli ./internal/rekit/workstream` 通过；完整本地 release minimum 已通过：`go run ./cmd/rekit -- -Command release-check -Format json` 返回 `ready=true`，`go run ./cmd/rekit -- -Command status`、`go run ./cmd/rekit -- -Command packs`、`go run ./cmd/rekit -- -Command doctor`、`go test ./...`、`go vet ./...` 与 `git diff --check` 均通过（仅保留 Windows 工作树 LF→CRLF 提示）。remote inspection 尚未执行，当前不能声明 remote CI green。

### Batch 567：reviewer dispatch next-action handoff closure

状态：已完成 Go runtime、CLI text/status product-path、tests、本地 release minimum、implementation commit/push 与 implementation remote release-gate inspection；implementation commit `b7f1abc` 已推送。implementation run `30086308317` completed failure，Linux/Windows/macOS jobs `89459180192`/`89459180264`/`89459180311` 均 `runner_id=0` 且 `steps=[]`，仍属既有 runner/billing blocker，不能声明 remote CI green。本 release inspection record 仅记录该 implementation run；不要为 inspection commit 自身 CI 追加第三个记录提交，除非出现不同于既有 `steps=[]` 的新远程信号。

目标：关闭 multi-shard reviewer dispatch/intake 接手断点：downstream `status` / `handoff` / `continue` summary 既要保留 latest packet/shard progress，也要明确 replacement executor 当前真正该处理的 shard。旧 summary 容易在 latest shard 仍 waiting、较早 shard 已 source-ready / collection-ready / intake-ready 时只展示 latest 或 batch preview，迫使接手者手工扫描完整 item 列表、source/candidate paths 与 staging/collection commands。

已实现内容：

- `ReviewerDispatchIntakeSummary` 新增 `nextActionShardId`、`nextActionState`、next-action reviewer result source/candidate path 与 state、staging command、collection preview/apply、single preview/apply、batch preview/apply，并继续保留 latest packet/shard fields；summary next action 选择复用 Mission Commander reviewer dispatch priority，ready staging/collection/intake 不再被 later waiting shard 覆盖。
- `status` / `handoff` / `continue` text first-screen 新增 `reviewer dispatch next action` line，同屏显示 next-action shard/state、source/candidate state/path、staging/collection/batch/single preview/apply 与 nextAction，避免 replacement executor 从 compact summary + per-item details 中手工拼当前命令。
- CLI nested product path 覆盖 waiting→source-ready→staging/collection→ready batch intake：all-waiting 时 next action 指向第一 open shard 的 dispatch/staging source；source ready 时提升 `ready-for-reviewer-result-staging-preview` 与 `-StageReviewerResult`；ready intake 时仍投影 packet-level `-ReadyReviewerResults` batch preview。
- Workstream tests 锁定 ready packet batch preview 不被 later waiting shard 覆盖，并锁定 all-waiting summary 中 next-action shard 可与 latest shard 分离。

边界：本批只增强 reviewer dispatch/intake 的只读 summary/text handoff 与本机 product-path coverage；不 spawn、stop、poll 或 monitor reviewer，不创建 reviewer result，不执行 staging/collection/intake，不写 facts/authority/confirmed，不执行 heavy tool，不新增 PowerShell runtime logic。

验证结果：focused `go test ./internal/rekit/workstream ./internal/rekit/cli -run "TestReviewerDispatchIntakeSummaryPrefersReadyPacketBatchCommand|TestReviewerDispatchIntakeSummaryProjectsWaitingNextAction|TestRunPlanSubagentsReviewerOrchestrationE2E" -count=1` 通过；完整本地 release minimum 已通过：`go run ./cmd/rekit -- -Command release-check -Format json` 返回 `ready=true`，`go run ./cmd/rekit -- -Command status`、`go run ./cmd/rekit -- -Command packs`、`go run ./cmd/rekit -- -Command doctor`、`go test ./...`、`go vet ./...` 与 `git diff --check` 均通过（仅保留 Windows 工作树 LF→CRLF 提示）。remote inspection：implementation run `30086308317` 的 Linux job `89459180192`、Windows job `89459180264`、macOS job `89459180311` 均 completed failure、`runner_id=0` 且 `steps=[]`，未获得 runner 执行；该信号与既有 billing/spending-limit blocker 相同，当前不能声明 remote CI green。

上一批摘要：Batch 566 已完成 pack-memory verification provisioning handoff closure；implementation commit `1ece655` 与 release inspection commit `5f8c216` 已推送；implementation run `30083919952` completed failure，Linux/Windows/macOS jobs `89451553739`/`89451553777`/`89451553842` 均 `runner_id=0` 且 `steps=[]`，仍属既有 runner/billing blocker，不能声明 remote CI green。

### Batch 566：pack-memory verification provisioning handoff closure

状态：已完成 Go runtime、CLI text/status product-path、release/status inventory、tests、本地 release minimum、implementation commit/push 与 implementation remote release-gate inspection；implementation commit `1ece655` 已推送。implementation run `30083919952` completed failure，Linux/Windows/macOS jobs `89451553739`/`89451553777`/`89451553842` 均 `runner_id=0` 且 `steps=[]`，仍属既有 runner/billing blocker，不能声明 remote CI green。本 release inspection record 仅记录该 implementation run；不要为 inspection commit 自身 CI 追加第三个记录提交，除非出现不同于既有 `steps=[]` 的新远程信号。

目标：关闭 accepted pack-memory candidate decision 之后、final verification 之前的 provisioning 接手断点。Batch 559/560 已提供 `promote -ProvisionCandidateVerificationCases` 与 verification/retirement runtime，但 replacement executor 在 decision Apply 或 downstream status 接手时仍容易只看到泛化 verification command，必须手工判断是否已创建 canonical fresh/attached cases、是否已有 `provision.intent.json` / `provision.receipt.json`、是否应 resume expected-hash Apply 或可进入 final verification。

已实现内容：

- accepted candidate decision `-Apply` 的 result next steps 与 text receipt line 现在直接输出 receipt-level `verificationProvisionCommand`、`verificationCommand` 与“先 provisioning WhatIf→expected-hash Apply，再 final verification”的顺序，避免把 final verification command 当作可直接运行的下一步。
- `release-check` / kit-mode `status` / case-mode `status` 在 pending accepted decision receipt 上扫描 source-case-local canonical verification workspace 的 `provision.intent.json` 与 `provision.receipt.json`，投影 `provisionStatus`（required / in-progress / complete）、intent/receipt paths、`provisionSha256`、expected-hash `provisionApplyCommand` 与 next action。
- provision artifacts 校验绑定 repo/case/pack、packet/decision/receipt hashes、canonical workspace、fresh/attached roots、verification preview command、case write plans 与 exact provision hash；receipt 缺 intent、intent/receipt drift、hash mismatch、symlink/non-regular/unbounded artifact 或 workspace binding drift 均 fail-closed 为 release/status warning。
- CLI nested source-case product path 覆盖 decision Apply → pending provisioning status required（只读/no mutation）→ provisioning Apply → status complete（输出 apply/resume command 与 `verificationCommand` next action）→ final verification → retirement handoff，证明 replacement executor 可从 status 第一屏接续而不重新解析 provision artifacts。

边界：本批只增强 accepted candidate verification provisioning 的只读 downstream handoff 与 decision terminal handoff；不创建 verification cases、不执行 final verification、不 retire workspace、不 merge/cleanup candidate、不写 facts/authority/confirmed、不执行 heavy tool、不新增 PowerShell runtime logic。provisioning本身仍只由显式 `promote -ProvisionCandidateVerificationCases -WhatIf/-Apply` 执行。

验证结果：focused `go test ./internal/rekit/promote ./internal/rekit/releasecheck ./internal/rekit/cli -count=1` 通过；完整本地 release minimum 已通过：`go run ./cmd/rekit -- -Command release-check -Format json` 返回 `ready=true`，`go run ./cmd/rekit -- -Command status`、`go run ./cmd/rekit -- -Command packs`、`go run ./cmd/rekit -- -Command doctor`、`go test ./...`、`go vet ./...` 与 `git diff --check` 均通过（仅保留 Windows 工作树 LF→CRLF 提示）。remote inspection：implementation run `30083919952` 的 Linux job `89451553739`、Windows job `89451553777`、macOS job `89451553842` 均 completed failure、`runner_id=0` 且 `steps=[]`，未获得 runner 执行；该信号与既有 billing/spending-limit blocker 相同，当前不能声明 remote CI green。

上一批摘要：Batch 565 已完成 case-local candidate review packet handoff closure；implementation commit `730741a` 与 release inspection commit `33fc23e` 已推送；implementation run `30079861360` completed failure，macOS/Windows/Linux jobs `89438735982`/`89438736026`/`89438736057` 均 `steps=[]`，仍属既有 runner/billing blocker，不能声明 remote CI green。

### Batch 564：pack-memory candidate decision draft handoff closure

状态：已完成 Go runtime、CLI、package/release/status product-path implementation、本地验证、implementation commit/push 与 implementation remote release-gate inspection；implementation commit `d1814c1`已推送。implementation run `30076216089` completed failure，Windows/macOS/Linux jobs `89427406535`/`89427406669`/`89427406747` 均`steps=[]`，仍属既有runner/billing blocker，不能声明remote CI green。本release inspection record仅记录该implementation run；不要为inspection commit自身CI追加第三个记录提交，除非出现不同于既有`steps=[]`的新远程信号。

目标：关闭 Batch 563 之后的 operational handoff 断点：虽然 `promote -DraftCandidateDecision` 已能自动生成 packet-bound decision file，但 `promote -CreateCandidates -Review` workspace、terminal、Mission Commander action queue，以及 downstream `status` / `release-check` open pack-memory candidate handoff 仍可能只提示人工 review/cleanup，replacement executor 需要手工回忆 draft command、packet path、decision path、evidence refs 与 expected-hash Apply 语义。

已实现内容：

- `promote -CreateCandidates -Review` 现在在 durable `candidateResult.reviewPlan.decisionDraftHandoff` 中写入 packet path、case-local decision path、review evidence refs、supported decisions、preview commands、`<decisionSha256-from-WhatIf>` Apply templates、next action 与 no-merge/no-cleanup/no-heavy/no-authority boundary；workspace `summary.md` 与 CLI text 同步输出 draft preview/apply handoff。
- Mission Commander queue 保持 review-first：current action 仍是 `reviewPlan.decisionChecklist`，draft preview 作为 review 后下一步进入 `reviewPlan.decisionDraftHandoff`，避免 draft action 抢占 review 或绕过人工复核。actual candidates 生成可运行 draft command；WhatIf candidates 因 candidate bytes 未 materialize，只给出先 rerun without `-WhatIf` 的 materialize guidance。
- `release-check` 与 kit/case `status` 的 open pack-memory candidate handoff 新增 `decisionDraftHandoff`，从 repo-local proof refs、supported decisions 与 pack candidate residue生成只读 guidance，提示从 attached source case 重新生成 review workspace 或先补充 repo-local evidence ref，再使用 packet handoff 的 draft preview/apply path；release/status 不推断 case-local packet、不写 decision file。
- CLI JSON/text product paths 覆盖 review workspace handoff、status/release-check downstream handoff、nested case cwd/no `Target`/no `Pack` 状态投影；package tests 证明 workspace packet handoff 可直接调用 `DraftCandidateDecisions` 生成可用 preview，且 WhatIf 不写 decision file。

边界：本批只把 Batch 563 的 Go-native draft path接入 durable review/status/release handoff；不 merge/cleanup candidates，不写 pack source，不运行 doctor/init/reconsume，不创建 proof，不写 facts/authority/confirmed，不执行 heavy tool，不新增 PowerShell runtime logic。release/status handoff 只读且不声称 remote CI green。

验证结果：focused `go test ./internal/rekit/promote ./internal/rekit/cli ./internal/rekit/releasecheck -count=1`通过；完整`go test ./...`通过；本地release minimum已通过：`go run ./cmd/rekit -- -Command release-check -Format json`返回`ready=true`，`go run ./cmd/rekit -- -Command status`、`go run ./cmd/rekit -- -Command packs`、`go run ./cmd/rekit -- -Command doctor`、`go vet ./...`与`git diff --check`均通过（仅保留Windows工作树LF→CRLF提示）。remote inspection：implementation run `30076216089` 的 Windows job `89427406535`、macOS job `89427406669`、Linux job `89427406747` 均 completed failure 且 `steps=[]`，未获得runner执行；该信号与既有billing/spending-limit blocker相同，当前不能声明 remote CI green。

上一批摘要：Batch 563已完成 pack-memory candidate decision draft closure；implementation commit `188ddc0`与release inspection commit `9934a22`已推送；implementation run `30071689533` completed failure，Windows/Linux/macOS jobs `89413717542`/`89413717566`/`89413717568`均`steps=[]`，仍属既有runner/billing blocker。

### Batch 563：pack-memory candidate decision draft closure

状态：已完成 Go runtime、CLI、package/CLI product-path implementation、本地验证、implementation commit/push 与 implementation remote release-gate inspection；implementation commit `188ddc0`已推送。implementation run `30071689533` completed failure，Windows/Linux/macOS jobs `89413717542`/`89413717566`/`89413717568` 均`steps=[]`，仍属既有runner/billing blocker，不能声明remote CI green。本release inspection record仅记录该implementation run；不要为inspection commit自身CI追加第三个记录提交，除非出现不同于既有`steps=[]`的新远程信号。

目标：关闭 pack-memory candidate decision product path 中 Mission Commander / replacement executor 必须手工拼完整 `CandidateDecisionFile` 的断点。旧路径要求人工计算 packet SHA-256、candidate SHA-256、accepted pack target SHA-256、evidence SHA-256，并精确覆盖所有 pending review items；该手工 JSON 既易错，也会让后续 verification/provisioning/retirement 闭环在入口处依赖不可审计的手写中间件。

已实现内容：

- 新增 `promote -DraftCandidateDecision` Go-native draft/record path：`-WhatIf` 从 durable candidate review packet 与 evidence refs 生成完整 `pack-memory-candidate-decisions` JSON preview，自动补齐 exact packet hash、candidate hash、accepted managed-doc pack target hash 与 evidence SHA-256，并返回 deterministic `decisionSha256` 与 hash-gated Apply command；`-Apply` 要求 `-ExpectedDecisionSha256` 匹配 preview 后只写 case-local decision JSON。
- draft 严格绑定 attached repo/case/pack、canonical `promote-candidates` / `tooling/candidates` roots、candidate index、manifest managed target、packet pending review authority 与 create-candidate writes；duplicate/invalid review authority、candidate drift、managed index/target drift、unsafe evidence 或 out-of-case decision file 均 fail-closed。
- decision modes 支持 `accept`、`reject`、`superseded` 与 `accept-managed-reject-tooling`；tooling candidate auto-accept 继续 fail-closed，混合 managed accept + tooling reject 不再需要主 Agent 手工分流和手写 hash。existing `promote -CandidateDecisionPath ... -WhatIf/-Apply` 继续消费 drafted decision file，后续 verification provisioning/final verification/retirement path 不变。
- CLI parse/routing/text 输出新增 draft fields、`decisionSha256`、preview/apply command 与 no-merge/no-cleanup/no-heavy boundary；provision/verify/retire routes 显式拒绝 draft/hash flags，避免 `-DraftCandidateDecision` 或 `-ExpectedDecisionSha256` 被前置 promote 子路由静默吞掉。CLI nested case product path 覆盖 draft preview/apply/text 后继续进入既有 decision preview/apply、verification provisioning、final verification与retirement闭环。

边界：draft 只创建或复用 exact case-local decision JSON；不 merge/cleanup candidate，不写 pack source，不运行 doctor/init/reconsume，不执行 verification/provisioning/retirement，不写 facts/authority/confirmed，不执行 heavy tool，不新增 PowerShell runtime logic。

验证结果：focused `go test ./internal/rekit/promote ./internal/rekit/cli -count=1`通过；完整`go test ./...`通过；本地release minimum已通过：`go run ./cmd/rekit -- -Command release-check -Format json`返回`ready=true`，`go run ./cmd/rekit -- -Command status`、`go run ./cmd/rekit -- -Command packs`、`go run ./cmd/rekit -- -Command doctor`、`go vet ./...`与`git diff --check`均通过（仅保留Windows工作树LF→CRLF提示）。独立只读审查发现并已修复promote前置 verification/provision/retire 子路由可能吞掉draft/hash flags的问题；复核后focused/full/local release minimum均通过。

上一批摘要：Batch 562已完成 reviewer orchestration packet-derived staging source path closure；implementation commit `91416dc`与release inspection commit `8397714`已推送；implementation run `30068351329`三平台jobs均completed failure且`steps=[]`，仍属既有runner/billing blocker。

### Batch 562：reviewer orchestration packet-derived staging source path closure

状态：已完成runtime、CLI、durable handoff与nested product path implementation；focused reviewer orchestration tests、完整`go test ./...`与本地release minimum已通过。implementation commit `91416dc`已推送；implementation run `30068351329` completed failure，Linux/Windows/macOS jobs `89403665163`/`89403665173`/`89403665190` 均`steps=[]`，仍属既有runner/billing blocker，不能声明remote CI green。本release inspection record仅记录该implementation run；不要为inspection commit自身CI追加第三个记录提交，除非出现不同于既有`steps=[]`的新远程信号。

目标：关闭Batch 558/549后remaining reviewer orchestration E2E断点：planning/status/handoff仍让主Agent把read-only reviewer JSON保存到任意`<case-local-reviewer-json>`，replacement executor必须手工选择或回忆source落点，且downstream无法从packet-derived state判断“source已ready，可运行staging preview”。新路径把source落点收口到canonical review namespace并与staging Apply严格绑定。

已实现内容：

- fresh canonical reviewer packet为每个shard生成`.rekit/reviews/<review>/results/sources/<shard>.json`，并在`reviewerStagingCommands.sourcePath/sourcePathArgument/previewCommand`、shard handoff、dispatch prompt、terminal text与summary中直接投影；main agent动作改为保存唯一ReviewerResult JSON到`reviewerStagingCommands.sourcePath`，再运行staging WhatIf与expected-source-hash Apply。
- `StageReviewerResult`在新packet携带source path时要求`-ReviewerResultSourcePath`与packet-derived `reviewerStagingCommands.sourcePath` exact匹配；forged或任意case-local source不再可运行。legacy packet缺少source field时仍保留既有兼容staging语义。
- durable reviewer dispatch intake从packet/resultRoot重建canonical source/candidate/result bindings，投影`reviewerResultSourcePath`与`reviewerResultSourceState`；source ready时提升`ready-for-reviewer-result-staging-preview`，invalid source fail-closed，candidate ready与canonical collected仍接续collection preview与reviewer intake preview。Mission Commander next actions、status/handoff/continue text/Markdown同步输出source state与staging command。
- CLI nested cwd/no `Target`/no `Pack` product path覆盖read-only reviewer JSON保存到packet-derived source、staging WhatIf/expected-hash Apply、collection WhatIf/Apply与packet-level ReadyReviewerResults；workstream tests覆盖source-ready promotion、forged source binding suppression与canonical command rebuild。
- collection-bound canonical reviewer intake不再可被直接写`reviewerResultPath`绕过：fresh canonical packet的direct single/batch intake在canonical result path上要求packet-derived candidate存在且candidate/canonical bytes完全一致；candidate missing、invalid或不匹配会投影`reviewer-result-collection-required`/candidate-invalid并要求先走staging→collection。dispatch-only/noncanonical reviewer prompt也不再引用不可用的`reviewerStagingCommands.sourcePath`，保持legacy direct intake或attach/regenerate guidance。

边界：runtime不spawn、stop、poll或monitor reviewer；不执行heavy-tool；staging/collection不写facts/authority/confirmed；`continue -Apply`和reviewer intake仍保持既有显式WhatIf→Apply边界；不新增PowerShell runtime logic。

验证结果：focused `go test ./internal/rekit/subagents ./internal/rekit/workstream ./internal/rekit/cli -count=1`通过；完整`go test ./...`通过；本地release minimum已通过：`go run ./cmd/rekit -- -Command release-check -Format json`返回`ready=true`，`go run ./cmd/rekit -- -Command status`、`go run ./cmd/rekit -- -Command packs`、`go run ./cmd/rekit -- -Command doctor`、`go vet ./...`与`git diff --check`均通过。implementation commit `91416dc Close reviewer staging source paths`已推送；remote inspection：run `30068351329`的Linux job `89403665163`、Windows job `89403665173`、macOS job `89403665190`均completed failure且`steps=[]`，未获得runner执行；该信号与既有billing/spending-limit blocker相同，当前不能声明remote CI green。

上一批摘要：Batch 561已完成continue executor-generation stale-writer guard closure；implementation commit `46c1c66 Guard continue against stale executors`与release inspection commit `a5c7343 Record Batch 561 release gate inspection`已推送；对应remote run `30035030511`三平台jobs均completed failure且`steps=[]`，仍为既有runner/billing blocker。

### Batch 561：continue executor-generation stale-writer guard closure

状态：已完成runtime、CLI、durable handoff、tests与nested product path implementation；独立审查识别的shared mutation serialization与namespace rebind问题已修复，focused/repeat tests、跨平台test-binary编译与完整本地release minimum已通过。implementation commit `46c1c66 Guard continue against stale executors`已推送；对应remote run `30035030511` completed failure，Linux/Windows/macOS jobs均`runner_id=0`且`steps=[]`，仍属既有runner/billing blocker，不能声明remote CI green。

目标：关闭replacement executor通过`start` takeover或`reconcile`刷新durable lane owner后，旧executor仍可调用`continue`并写入run、facts、lane resume/checkpoint与board的真实断点。新路径要求`continue`同时提供当前`-Executor`与`-ExpectedExecutorGeneration`，并把两者作为lane-local optimistic concurrency precondition。

已实现内容：

- `continue -WhatIf`与`continue -Apply`在任何写入前strict读取所选lane的`currentExecutor`与`executorGeneration`；缺少调用方binding、executor不匹配或generation stale均fail-closed。legacy从未分配owner（empty executor/generation 0）的lane保持兼容。
- stale调用保持zero-write：不创建`.rekit/runs/**`，不追加`.rekit/facts/**`，不刷新lane `RESUME.md`/checkpoint，也不修改`.rekit/board.json`；Apply与`start` takeover、`reconcile` takeover、写durable owner snapshot的`handoff -Apply`共用kernel-backed case/lane mutation lease并在锁内重读。
- 所有workstream writer按真实全局写集取得project-exclusive lease，existing lane另取exclusive lane lease；stable external namespace不受shell cache环境变量影响，并以resolved case identity派生key。既有case-root `.re-template.yml`（若存在）提供`.rekit`换绑之外的stable lease primitive；case-local namespace与canonical instance/lane identity在mutation前重验。进程退出由kernel释放；unlock/close/identity错误显式返回。
- runtime-generated status/overview/handoff/start/reconcile/continue、Mission Commander actions与durable `RESUME.md`/checkpoint/run digest从current lane authority重建`-Executor ... -ExpectedExecutorGeneration ...` continue command，旧owner字符串不能在takeover后存活。
- nested case cwd、无`Target`、无`Pack`产品路径覆盖session-A generation 1→session-B generation 2 takeover、stale A preview/apply完整case snapshot zero-write、current B text preview/JSON apply成功。

验证结果：focused workstream owner guard/locking/concurrency tests重复10次通过，workstream全套重复10次通过；CLI nested replaceable-session product path重复10次及完整CLI tests通过。Linux/macOS/Windows/wasm workstream test binary交叉编译通过；独立P0/P1终审无存活finding。完整本地release minimum已通过：`go run ./cmd/rekit -- -Command release-check -Format json`返回`ready=true`，`go run ./cmd/rekit -- -Command status`、`go run ./cmd/rekit -- -Command packs`、`go run ./cmd/rekit -- -Command doctor`、`go test ./...`、`go vet ./...`与`git diff --check`均通过。remote inspection：run `30035030511`的Linux job `89300743081`、Windows job `89300743034`、macOS job `89300743005`均completed failure且`steps=[]`，未获得runner执行；该信号与既有billing/spending-limit blocker相同。executor takeover仍只由显式`start`/`reconcile`拥有；guard不自动spawn、停止、轮询或管理session，不执行heavy action，不写authority/confirmed。

上一批摘要：Batch 560已完成pack-memory candidate verification workspace retirement closure；implementation commit `303414e Retire candidate verification workspaces`已推送，对应remote run `30021860514`三平台jobs均completed failure、`runner_id=0`且`steps=[]`，仍属既有runner/billing blocker。

### Batch 560：pack-memory candidate verification workspace retirement closure

状态：已完成runtime/CLI/release handoff/tests/docs implementation、nested source-case product path、多轮独立审查修复、focused重复验收、完整本地release minimum与implementation commit `303414e Retire candidate verification workspaces`/push。对应remote run `30021860514` completed failure；Linux/Windows/macOS jobs均completed failure、`runner_id=0`且`steps=[]`，仍属既有runner/billing blocker，不能声明remote CI green。

目标：关闭accepted candidate完成provisioning与final verification后，source-case-local canonical verification workspace只能长期残留或由维护者手工递归删除的operational断点。由Go-native runtime提供strict review-first WhatIf→expected-hash Apply retirement，在保留repo-local final evidence的同时只移除exact provisioned trees。

当前实现内容：

- final verification result返回`-RetireCandidateVerificationWorkspace -WhatIf` handoff；WhatIf绑定packet/decision/decision receipt、final proof、provision intent/receipt、canonical workspace及fresh/attached完整exact tree plans，并返回`ExpectedRetirementSHA256` Apply命令，不写intent/receipt、不删除workspace。
- Apply在shared candidate lock内重读全部authority/evidence/tree bindings，先写repo-local durable retirement intent，再按统一batch preflight确定性删除exact leaves/directories/roots与provision artifacts，最后写repo-local retirement receipt。任一missing/extra/different/symlink/non-regular object或proof/provision drift均在删除前fail-closed。
- intent后的中断允许从exact remaining subset crash resume；completed receipt exact replay幂等。receipt current后workspace/root重现会fail-closed且不自动再次删除，避免旧receipt成为后续删除授权。

边界：retirement只清理完成final verification的canonical provisioned workspace；repo-local retirement intent/receipt作为最终证据保留。不merge/cleanup candidate，不写facts/authority/confirmed，不执行heavy tool，不新增PowerShell runtime logic。

验证结果：focused retirement、CLI nested cwd/no `Target`/no `Pack` product path与release/status lifecycle tests已通过；独立审查发现并修复final-proof replay漏验、按名称删除replacement TOCTOU、随机quarantine无法跨进程resume、workspace整体换绑跨identity删除窗口，以及release/status把不可恢复tree drift误报为in-progress。当前实现以deterministic owned quarantine支持leaf/directory/root post-rename crash resume，通过single-prepare parent-bound apply将fresh/attached roots绑定同一pinned workspace identity，并在completed replay与intent resume重验final proof。完整本地release minimum已通过：`go run ./cmd/rekit -- -Command release-check -Format json`返回`ready=true`，`go run ./cmd/rekit -- -Command status`、`go run ./cmd/rekit -- -Command packs`、`go run ./cmd/rekit -- -Command doctor`、`go test ./...`、`go vet ./...`、`git diff --check`均通过；Linux sync/promote/CLI test binaries交叉编译也通过并已清理。implementation commit `303414e`已推送；remote run `30021860514`三平台jobs均在执行任何step前失败，`runner_id=0`且`steps=[]`，未提供不同于既有runner/billing blocker的新代码失败信号，当前不能声明remote CI green。

上一批摘要：Batch 559已完成pack-memory candidate verification case provisioning closure，implementation commit `c65f511 Provision candidate verification cases`已推送；对应run `30012510308`三平台jobs均completed failure且`steps=[]`，仍为既有runner/billing blocker。

### Batch 559：pack-memory candidate verification case provisioning closure

状态：已完成runtime/CLI/release handoff/tests/docs implementation、多轮独立审查修复与最终bounded验收、focused/package/nested case product-path validation、完整本地release minimum、implementation commit `c65f511 Provision candidate verification cases`/push与远程release-gate inspection。对应run `30012510308` completed failure；Linux/Windows/macOS jobs均completed failure且`steps=[]`，仍属既有runner/billing blocker，不能声明remote CI green。

目标：关闭accepted pack-memory candidate decision之后，Mission Commander必须手工创建并管理两个验证case才能进入既有`VerifyCandidateDecision`的operational断点。由Go-native runtime提供source-case-local canonical workspace、两个distinct no-overwrite case的WhatIf→expected-hash Apply、durable exact/partial replay，并返回既有final verification WhatIf handoff。

已完成内容：

- accepted decision receipt新增canonical verification workspace与concrete provisioning command。`promote -ProvisionCandidateVerificationCases`要求packet/decision、两个workspace direct-child roots以及exactly one of WhatIf/Apply；WhatIf返回完整write sets与provision hash，Apply锁内重读并要求`ExpectedProvisionSha256`。
- 新增deterministic exclusive init package，生成attached metadata/thin shim/managed+template files/state/verification role marker，所有leaf使用exclusive create；different bytes、symlink/non-regular/unplanned object fail-closed，exact replay幂等并支持partial exact completion。
- provisioning写source-case-local `provision.intent.json`与`provision.receipt.json`，运行两个case doctor后只返回现有`VerifyCandidateDecision -WhatIf`命令，不创建final repo-local verification proof。release/status handoff投影workspace、provision command与final verification command。
- CLI产品路径从candidate decision Apply继续到nested source case cwd、无`Target`/无`Pack`的provision WhatIf→expected-hash Apply→exact replay text→VerifyCandidateDecision WhatIf；package tests覆盖preview no-write、Apply/replay、root geometry/collision与final proof absence。

边界：provisioning只创建两个source-case-local verification cases和durable intent/receipt；不merge/cleanup candidate，不执行final verification或heavy tool，不写facts/authority/confirmed，不自动删除cases，不新增PowerShell runtime logic。

验证结果：focused `go test ./internal/rekit/sync ./internal/rekit/promote ./internal/rekit/cli ./internal/rekit/releasecheck -count=1`与CLI nested product path、release/status projection已通过。首轮独立审查识别并修复authoritative decision receipt binding、intent crash resume、双root first-leaf preflight、workspace symlink/rebind与provision receipt exact Cases/Writes binding；第二轮复核进一步要求action-derived decision counts、reservation→Apply之间全root preflight、source/workspace namespace与receipt后置重验；第三轮及终局复核继续发现reserved root未固定filesystem identity、workspace/child root在doctor与receipt阶段可换绑、final leaf中断写不可恢复、receipt写后未精确重验刚发布leaf，以及decision/action覆盖可在mutation前或空pending path下绕过。现在exclusive init以Go 1.26 `os.Root` handle端到端固定workspace、parent与reserved roots，所有replay扫描、目录创建、doctor前后identity检查及leaf写入都相对pinned namespace；leaf和receipt均先完整写入deterministic owned temp，再用handle-relative no-replace link发布，支持publish前crash恢复且不暴露半成品。receipt发布后要求同一leaf identity、exact newline bytes、root/workspace/intent仍current；共享normalized review/decision/action validator拒绝重复、空pending path与不完整覆盖，均有并发换绑、crash resume、truncate/replace、duplicate-one/omit-another和pre-mutation no-write回归。完整本地release minimum（`release-check -Format json` ready、`status`、`packs`、`doctor`、`go test ./...`、`go vet ./...`、`git diff --check`）与Linux sync/promote test binary交叉编译已通过；直接设置`GOOS=linux`运行`go test`会尝试在Windows执行Linux binary并按预期失败，已改用`go test -c`验证并清理产物。最终bounded验收专门复核post-link crash temp reconciliation，确认completed leaf/receipt replay会在regular、exact bytes与`os.SameFile`全部成立后清理owned temp，different identity或bytes drift fail-closed，无剩余高置信finding。implementation commit `c65f511`已推送；remote run `30012510308`三平台jobs均在执行任何step前失败，`runner_id=0`且`steps=[]`，未提供不同于既有runner/billing blocker的新代码失败信号。

上一批摘要：Batch 558已完成reviewer result staging / candidate publication closure，implementation commit `8e79971 Stage reviewer results safely`与release inspection commit `9c1fedc Record Batch 558 release gate inspection`已推送；对应run `30000837518`三平台jobs均completed failure且`steps=[]`，仍为既有runner/billing blocker。

### Batch 558：reviewer result staging / candidate publication closure

状态：已完成implementation、focused/package/product-path validation、独立审查修复、最终复核、完整本地release minimum、implementation commit `8e79971 Stage reviewer results safely`/push与远程release-gate inspection。对应run `30000837518` completed failure；Linux/macOS/Windows jobs均completed failure且`steps=[]`，仍属既有runner/billing blocker，不能声明remote CI green。

目标：关闭主Agent必须把read-only reviewer返回JSON直接写入packet-derived canonical candidate path的产品断点。允许先保存到任意case-local source，再以Go-owned WhatIf→expected-source-hash Apply严格验证并no-overwrite发布candidate，随后进入既有collection→intake。

已完成内容：

- 新增`plan-subagents -StageReviewerResult -ReviewerResultSourcePath ... -ShardId ... -WhatIf/-Apply`。WhatIf stable读取symlink-free case-local non-empty regular source，复用collection authoritative validator校验packet integrity、route/shard/items、routeOutput、decision/evidence/blockers，并返回source SHA-256/size及packet-derived candidate target；Apply在共享packet/shard mutation lock内重读并绑定expected source hash。
- staging通过既有temp file + Sync + hard-link no-replace publication发布exact source bytes；different/obstructed candidate不覆盖，exact replay幂等，source保持不变。candidate parent仅在canonical result namespace内按需创建并重验symlink-free geometry。
- fresh planning packet/shard handoff、terminal text与durable workstream投影typed staging preview template；主Agent路径变为read-only Agent JSON → bounded case-local source → staging WhatIf/Apply → collection WhatIf/Apply → packet batch intake。legacy packet缺staging字段仍保留collection capability并由workstream从canonical bindings重建命令。
- package与CLI product-path tests覆盖WhatIf no-write、expected-hash Apply、source drift、candidate collision、out-of-case与case-internal symlink source、candidate directory换绑、idempotent replay，以及nested case cwd/no Target/no Pack staging→collection→batch intake。

边界：staging不删除或修改source，不覆盖different candidate，不自动collection/intake，不spawn/stop/poll/monitor reviewer，不执行heavy-tool，不写facts/authority/confirmed，不新增PowerShell runtime logic。

验证结果：focused `go test ./internal/rekit/subagents ./internal/rekit/workstream ./internal/rekit/cli`已通过。独立审查发现并修复source/candidate publication祖先namespace换绑竞态与staging flags在非`plan-subagents`命令被静默忽略；复核进一步发现`os.Root`会跟随case内symlink，以及packet validation与publication之间仍可整体换绑普通namespace。现已用同一逐组件no-follow parent handle读取packet+integrity并绑定identity，保存validated result-root identity供publication重验，在link前后确认canonical result/candidates paths仍绑定prepared handles。case-internal source symlink、同步candidate directory换绑与prepare→publication result namespace替换测试证明错误namespace无写入、不会误报staged。完整本地release minimum（`release-check -Format json` ready、`status`、`packs`、`doctor`、`go test ./...`、`go vet ./...`、`git diff --check`）与Linux package交叉编译已通过。最终独立复核确认所有post-Link失败出口均执行SameFile限定清理，candidate bytes、result/candidates namespace与packet snapshot全部通过后才返回`staged`，无剩余高置信finding。implementation commit `8e79971`已推送；远程run `30000837518`三平台jobs均在执行任何step前失败，未提供不同于既有runner/billing blocker的新代码失败信号。

上一批摘要：Batch 557已完成ambiguous recovery disposition closure，implementation commit `3f932d9 Resolve ambiguous reviewer recovery state`与release inspection commit `d9b8a15 Record Batch 557 release gate inspection`已推送；对应run `29994310884`仍为既有`steps=[]` runner/billing blocker。

### Batch 557：ambiguous reviewer recovery disposition closure

状态：已完成implementation、focused/package validation、独立审查修复、完整本地release minimum、implementation commit `3f932d9 Resolve ambiguous reviewer recovery state`/push与远程release-gate inspection。对应run `29994310884` completed failure；Windows/Linux jobs completed failure且`steps=[]`，macOS在run完成后仍queued且`steps=[]`，仍属既有runner/billing blocker，不能声明remote CI green。

目标：把Batch 556的`reviewer-result-recovery-ambiguous`硬阻断收口为显式Mission Commander review-first路径。当unfinished strict intent+exact quarantine与canonical exact reviewed candidate同时存在时，主Agent可WhatIf复核intent/canonical hashes，再Apply写`retain-canonical` disposition receipt；不删除或修改canonical、intent或quarantine，receipt current时恢复collection→intake。

已完成内容：

- 新增`-RetireReviewerResultRecovery -ShardId ... -WhatIf/-Apply`。strict disposition绑定repo/case/pack、packet/shard/lane、candidate/canonical/intent exact hash+size、canonical paths、quarantine、actor/reason/timestamp和no-delete/no-facts/no-heavy/no-authority flags；Apply在共享packet/shard mutation lock内重读全部bindings，以hard-link no-replace发布receipt，exact replay幂等。
- disposition仅允许canonical regular bytes exact等于reviewed candidate、intent unfinished且strict current、exact quarantine仍current；任一bytes/path/receipt drift均fail-closed。collection与direct/batch intake只在current disposition存在时忽略unfinished intent，并继续既有strict candidate/canonical validation；canonical后续变化恢复blocked。
- durable workstream在ambiguous state提升typed disposition WhatIf而非不可执行finalize；新增disposition command/path。package tests覆盖preview no-write、expected-hash Apply、canonical/quarantine保持不变、collection恢复与canonical drift重新blocked。

边界：disposition不删除、覆盖、移动或清理任何result/intent/quarantine，不写verdict/facts/authority/confirmed，不执行heavy-tool，不新增PowerShell runtime logic。

验证结果：related `go test ./internal/rekit/subagents ./internal/rekit/workstream ./internal/rekit/cli -count=1`已通过。独立审查发现并修复workstream不读取disposition导致Apply后仍循环ambiguous，以及receipt可指向alternate intent / intent repo-pack drift两项问题；现在canonical intent path、attached repo/pack、packet与disposition provenance统一strict绑定，Apply后workstream可前进，forged/drifted receipt fail-closed。完整本地release minimum（`release-check -Format json` ready、`status`、`packs`、`doctor`、`go test ./...`、`go vet ./...`、`git diff --check`）与Linux package交叉编译已通过。implementation commit `3f932d9`已推送；远程run `29994310884`中Windows/Linux completed failure且`steps=[]`，macOS在run完成后仍queued且`steps=[]`，未出现不同于既有runner/billing blocker的新远程信号。

上一批摘要：Batch 556已完成recovery ambiguity guard，implementation commit `2c0867a Block ambiguous reviewer recovery retries`与release inspection commit `8cd7a01 Record Batch 556 release gate inspection`已推送；implementation SHA未生成GitHub Actions run/check suite，已如实记录为release-gate-not-created。

### Batch 556：reviewer result recovery ambiguity guard closure

状态：已完成implementation、focused validation、独立审查修复、完整本地release minimum、implementation commit `2c0867a Block ambiguous reviewer recovery retries`/push与远程release-gate inspection。inspection时远程`main`已指向该commit，但GitHub Actions尚未为该SHA生成release-gate run或check suite；不能声明remote CI green。

目标：关闭recovery intent与exact quarantine已存在、但canonical reviewer result path又被同bytes regular file或同snapshot obstruction占据时，runtime无法证明两者是同一filesystem object却会自动删除canonical对象的断点。将该状态明确视为ambiguous并fail-closed，保留canonical对象供主Agent复核；真正post-move/pre-receipt crash在canonical missing时仍可确定性finalize。

已完成内容：

- regular-byte与typed obstruction quarantine分支在发现exact quarantine已存在且canonical path仍occupied时，不再按content/snapshot相等推断已移动对象并删除canonical leaf，而是返回`cannot prove` typed error。这样concurrent recreation或同内容替换不会被recovery重试静默删除。
- recovery intent保持unfinished；collection、direct/batch intake与durable workstream继续通过既有intent/receipt/quarantine guard保持blocked。canonical missing + exact intent/quarantine仍由`resumeReviewerResultRecovery` WhatIf→Apply写committed receipt，不改变Batch 553-555 crash-finalize语义。
- 新增regular bytes与empty-file obstruction重现测试，证明ambiguous retry失败且canonical对象保持原状。

边界：不自动删除、覆盖或替换ambiguous canonical object；不改变reviewer verdict/facts/authority/confirmed，不执行heavy-tool，不新增PowerShell runtime logic。

验证结果：focused/related `go test ./internal/rekit/subagents ./internal/rekit/workstream ./internal/rekit/cli -count=1`已通过。独立审查发现workstream仍将ambiguous状态误投影为可执行finalize；已新增`reviewer-result-recovery-ambiguous` blocked state、移除Apply command并提供`cannot prove`人工复核提示。完整本地release minimum（`release-check -Format json` ready、`status`、`packs`、`doctor`、`go test ./...`、`go vet ./...`、`git diff --check`）已通过。implementation commit `2c0867a`已推送；inspection时`git ls-remote`确认remote `main`为exact SHA，但`gh run list --commit`、Actions runs API与commit check-runs均为空，未生成可检查的implementation run；这是不同于既有`steps=[]`的remote signal，按事实记录为release-gate-not-created。

上一批摘要：Batch 555已完成Windows file-symlink obstruction recovery，implementation commit `14585d1 Recover symlink reviewer result obstructions`与release inspection commit `0f01b57 Record Batch 555 release gate inspection`已推送；对应run `29991470157`三平台jobs均completed failure且`steps=[]`，仍为既有runner/billing blocker。

### Batch 555：Windows symlink reviewer result recovery closure

状态：已完成implementation、focused validation、独立审查修复、完整本地release minimum、implementation commit `14585d1 Recover symlink reviewer result obstructions`/push与远程release-gate inspection。对应run `29991470157` completed failure；Linux/macOS/Windows jobs均completed failure且`steps=[]`，仍为既有runner/billing blocker，不能声明remote CI green。

目标：在Batch 554已验证的Windows source/destination handle与durable intent namespace guard基础上，关闭canonical reviewer result path被file symlink占据时仍需手工处置的断点。将expected symlink snapshot绑定到no-follow source handle所固定的canonical leaf，移动link本体而不打开或修改target；directory、其它non-regular object和非Windows仍保持fail-closed。

已完成内容：

- Windows move helper接收typed expected snapshot；source handle用`FILE_OPEN_REPARSE_POINT`打开且不共享write/delete，guard-first固定case-local destination namespace，source final path验证后要求reparse、non-directory shape，再通过被source handle固定的canonical leaf重验anchored `Lstat`/`Readlink` snapshot，最后执行handle-relative no-replace quarantine move。
- `-RecoverReviewerResult`与durable workstream恢复Windows symlink runnable WhatIf；existing empty-file recovery、shared mutation lock、intent/receipt crash-finalize、collection/intake unfinished-recovery guard与no-verdict/no-facts/no-heavy/no-authority边界不变。focused `subagents`与`workstream` tests覆盖symlink target保持不变及exact quarantine。

边界：不打开或修改symlink target；directory与其它non-regular object不自动移动或递归处理；非Windows不提升runnable recovery；runtime不spawn/monitor reviewer、不执行heavy-tool、不写facts/authority/confirmed，不新增PowerShell runtime logic。

验证结果：focused/related `go test ./internal/rekit/subagents ./internal/rekit/workstream ./internal/rekit/cli -count=1`与Linux package交叉编译已通过。独立审查确认Windows source snapshot/namespace/no-follow/no-replace组合有效，并发现、修复非Windows workstream曾发布不可执行symlink recovery命令的问题；完整本地release minimum（`release-check -Format json` ready、`status`、`packs`、`doctor`、`go test ./...`、`go vet ./...`、`git diff --check`）已通过。implementation commit `14585d1`已推送；远程run `29991470157`三平台jobs均completed failure且`steps=[]`，未出现不同于既有runner/billing blocker的新远程信号。

上一批摘要：Batch 554已完成Windows empty regular-file typed recovery，implementation commit `18e6325 Recover obstructed reviewer result files`与release inspection commit `3722ceb Record Batch 554 release gate inspection`已推送。对应run `29990736280`中Linux/Windows completed failure且`steps=[]`，macOS在run完成后仍queued且`steps=[]`，仍为既有runner/billing blocker。

### Batch 554：canonical reviewer result filesystem obstruction recovery closure

状态：已完成implementation、focused/package/product-path validation、独立审查修复、完整本地release minimum、implementation commit `18e6325 Recover obstructed reviewer result files`/push与远程release-gate inspection。对应run `29990736280` completed failure；Linux/Windows jobs completed failure且`steps=[]`，macOS job在run完成后仍显示queued且`steps=[]`，仍属既有runner/billing blocker，不能声明remote CI green。

目标：关闭strict reviewer candidate已正确生成、但canonical reviewer result path被empty regular file占据时，Mission Commander仍只能要求手工删除/修复的operational断点。把Batch 553 exact conflict recovery扩展为typed object snapshot与Windows handle-bound no-replace quarantine；symlink、directory与其它non-regular object保持typed fail-closed，直到其snapshot能够直接从source handle验证。

已完成内容：

- `-RecoverReviewerResult`现在用anchored `os.Root` + `Lstat`区分`regular-file`、`empty-file`、`symlink`、`directory`与`non-regular`；obstruction fingerprint绑定kind、mode、size与symlink target文本。WhatIf返回kind/fingerprint/mode/link target及expected-hash Apply command，Apply在共享packet/shard mutation lock内重读同一object snapshot，漂移即拒绝。
- Windows runnable obstruction recovery收窄为empty regular file：source以`FILE_OPEN_REPARSE_POINT` object handle打开，并用handle attributes绑定non-directory、non-reparse、size-zero shape；durable intent先作为no-delete-share namespace guard打开并验证canonical final path，再打开destination parent handle，以`NtSetInformationFile(FileRenameInformation=10)`执行handle-relative no-replace move。同步测试证明guard持有期间recoveries namespace不能被移走，而move仍可成功。symlink、directory及其它non-regular object均typed识别但fail-closed，不跟随link target、不自动移动或递归删除；非Windows平台也不提升runnable recovery。strict intent/receipt与hash-addressed quarantine保留Batch 553 crash-finalize、exact replay、collection guard及verification/decision writeback prohibition语义。
- recovery、collection与direct/batch intake共享同一packet/shard mutation lock；intake在锁内重读exact canonical result并重新检查intent/receipt/quarantine，workstream也在canonical path重现时优先投影unfinished recovery。durable workstream只为Windows empty-file blocker提升typed WhatIf；symlink、directory与其它non-regular object不生成runnable recovery command。
- CLI text显示canonical kind/mode/link target；nested case cwd/no Target/no Pack product path覆盖planning→candidate→empty file obstruction→recovery WhatIf JSON/text→expected-hash Apply→collection WhatIf/Apply→batch intake WhatIf。focused `subagents`、`workstream`、`cli` tests已通过。

边界：recovery不跟随或修改symlink target，不自动移动或递归处理任何directory，不spawn/stop/poll/monitor reviewer，不执行heavy-tool，不写或撤销facts/authority/confirmed；collection与intake继续分别要求显式WhatIf→Apply。禁止新增PowerShell runtime logic。

验证结果：focused `go test ./internal/rekit/workstream ./internal/rekit/subagents ./internal/rekit/cli -count=1`、Windows namespace-guard同步测试、obstruction case-local CLI product path与Linux package交叉编译已通过；完整本地release minimum（`release-check -Format json` ready、`status`、`packs`、`doctor`、`go test ./...`、`go vet ./...`、`git diff --check`）已通过。独立审查发现并修复object snapshot重验与move间漂移、pre-quarantine intent重试CreatedAt不一致、unfinished recovery可被direct/batch intake与workstream绕过、destination parent可被移出case及old-parent/new-guard split；最终将无法直接从source handle验证snapshot的symlink/directory/其它non-regular recovery收窄为typed fail-closed，仅保留handle attributes可绑定的Windows empty-file runnable recovery，并统一mutation lock、durable intent guard、锁内result重读及preview/Apply双重intake guard。implementation commit `18e6325`已推送；远程run `29990736280`中Linux/Windows completed failure且`steps=[]`，macOS在run完成后仍queued且`steps=[]`，未出现不同于既有runner/billing blocker的新远程信号。

上一批摘要：Batch 553已完成canonical reviewer result regular-byte conflict recovery，implementation commit `e351c3d Recover conflicting reviewer results`与release inspection commit `9955561 Record Batch 553 release gate inspection`已推送；对应run `29985101781`三平台jobs均completed failure且`steps=[]`，仍为既有runner/billing blocker。

### Batch 553：canonical reviewer result recovery / quarantine closure

状态：已完成implementation、focused/package/product-path validation、两轮独立审查修复、完整本地release minimum、implementation commit `e351c3d Recover conflicting reviewer results`/push与远程release-gate inspection。对应run `29985101781` completed failure；Linux/macOS/Windows jobs均`steps=[]`，仍为既有runner/billing blocker，不能声明remote CI green。

目标：关闭read-only reviewer candidate已正确生成、但canonical reviewer result已存在不同损坏或冲突bytes时，immutable collection拒绝覆盖且Mission Commander只能要求手工删除/修补result的operational断点。提供explicit WhatIf→Apply exact recovery，保留冲突bytes的可审计quarantine，再恢复collection→intake路径。

已完成内容：

- 新增`plan-subagents -RecoverReviewerResult -PacketPath ... -ShardId ... -Lane ... -Actor ... -Reason ... -WhatIf/-Apply`。WhatIf复用strict collection candidate、packet integrity、route/items/evidence/blocked-output validation，返回candidate与conflicting canonical result的exact SHA-256/size及expected-hash绑定Apply command；Apply在同一packet/shard collection lock内重读、重验并拒绝preview drift。
- recovery在`results/recoveries/`先exclusive durable写strict intent，再把exact conflicting canonical bytes原样移动到hash-addressed quarantine，最后写committed receipt；若进程在quarantine与receipt之间中断，后续WhatIf投影finalize action，expected-hash Apply验证current candidate、intent和quarantine exact bytes后补齐receipt，不形成手工删除dead end。exact completed replay幂等。
- receipt/intent strict绑定repo/case/pack、packet/route shard/lane、candidate/result/quarantine canonical paths、hash/size、actor/reason/timestamp与no-verdict/no-facts/no-heavy/no-authority flags；unknown/trailing JSON、invalid hash/size、forged path、symlink/non-regular quarantine或bytes drift fail-closed。已存在verification或decision writeback时禁止recovery，不撤销facts或伪造verdict。
- durable workstream在candidate与regular canonical result bytes冲突且尚无writeback时投影`reviewer-result-recovery-required`与typed WhatIf action；collection仍不覆盖different bytes。nested case cwd/no Target/no Pack CLI product path覆盖planning→candidate→conflict→recovery WhatIf/text/Apply→collection WhatIf/Apply→batch intake WhatIf。

边界：recovery只隔离一份exact conflicting regular canonical result，不自动spawn/stop/poll/monitor reviewer，不执行heavy-tool，不写facts/authority/confirmed，不替换candidate、不撤销既有verification/decision；collection与intake仍分别要求显式WhatIf→Apply。禁止新增PowerShell runtime logic。

验证结果：focused `subagents` / `workstream` / `cli` tests与case-local nested CLI product path已通过；完整本地release minimum（`release-check -Format json` ready、`status`、`packs`、`doctor`、`go test ./...`、`go vet ./...`、`git diff --check`）已通过。两轮独立审查发现并修复最终路径直接写导致截断intent/receipt dead end、interrupted recovery可被collection绕过、actor/reason drift、workstream未strict区分intent/committed receipt四项问题；复核确认核心执行端与strict projection闭环。远程run `29985101781`三平台jobs均在执行任何step前失败，未提供新的代码失败信号。

上一批摘要：Batch 552已完成invalid reviewer packet recovery / exact retirement closure，implementation commit `26d9061 Retire invalid reviewer packets safely`与release inspection commit `b9f8e46 Record Batch 552 release gate inspection`已推送；对应run `29981677697`的Linux/macOS/Windows jobs均completed failure且`steps=[]`，仍为既有runner/billing blocker。

### Batch 552：invalid reviewer packet recovery / exact retirement closure

状态：已完成implementation、focused/package validation、两轮独立审查修复、完整本地release minimum、implementation commit `26d9061 Retire invalid reviewer packets safely`/push与远程release-gate inspection。对应run `29981677697` completed failure；Linux/macOS/Windows jobs均`steps=[]`，仍为既有runner/billing blocker，不能声明remote CI green。

目标：关闭`reviewer-packet-integrity-invalid`只有“重新生成packet”提示、却没有durable且可审计地解除exact corrupted packet blocker的operational断点。让主Agent在strict sidecar仍提供可信lane provenance时显式WhatIf→Apply retirement，而不是删除、覆盖或手工修补packet/sidecar。

已完成内容：

- 新增`plan-subagents -RetireInvalidReviewerPacket -PacketPath ... -Lane ... -Actor ... -Reason ... -WhatIf/-Apply`。WhatIf返回exact invalid reason、packet/integrity hash与size、receipt path及携带expected hashes的Mission Commander apply action；Apply要求这两个preview hash，在canonical packet-path派生lock内重读并重验同一snapshot与strict integrity sidecar后写`packet.retirement.json`。
- receipt绑定repo/case/pack、packetId/lane/canonical paths、exact packet与integrity hash/size、actor/reason、RFC3339Nano timestamp以及no-delete/no-heavy/no-authority标志；相同snapshot与decision的Apply replay幂等，不同decision或forged existing receipt fail-closed。
- durable workstream只有在receipt strict JSON、attached metadata binding、timestamp、paths、hashes/sizes、provenance和boundary全部匹配当前bytes时才停止投影该invalid blocker；packet或sidecar bytes变化、receipt篡改、symlink/non-regular path会恢复`reviewer-packet-integrity-invalid`。
- JSON/text CLI与case-local product-path coverage验证WhatIf no-write、Apply closure、exact replay、receipt forgery与packet drift。missing/malformed integrity sidecar因缺少可信lane provenance继续拒绝retirement，只能重新生成canonical packet。

边界：retirement不删除、覆盖或修补packet/integrity，不spawn/stop/poll/monitor reviewer，不执行heavy-tool，不写facts/authority/confirmed。只支持strict sidecar可读但packet无效的exact snapshot recovery；禁止新增PowerShell runtime logic。

验证结果：focused `subagents` / `workstream` / `cli` tests与case-local CLI WhatIf/text/Apply product path已通过；完整本地release minimum（`release-check -Format json` ready、`status`、`packs`、`doctor`、`go test ./...`、`go vet ./...`、`git diff --check`）已通过。独立审查发现并修复semantic-invalid sidecar provenance、WhatIf未绑定exact snapshot/锁外stale result、workstream无界non-regular读取，以及core mutation API未强制preview hashes四项问题；复核通过。远程run `29981677697`三平台jobs均在执行任何step前失败，未提供新的代码失败信号。

上一批摘要：Batch 551已完成reviewer packet integrity / durable fail-closed closure，implementation commit `0d1f518 Attest durable reviewer packets`与release inspection commit `d01f105 Record Batch 551 release gate inspection`已推送；对应run `29979488713`的Linux/macOS/Windows jobs均completed failure且`steps=[]`，仍为既有runner/billing blocker。

### Batch 551：reviewer packet integrity / durable fail-closed closure

状态：已完成implementation、focused/package validation、独立审查修复、完整本地release minimum、implementation commit `0d1f518 Attest durable reviewer packets`/push与远程release-gate inspection。对应run `29979488713` completed failure；Linux/macOS/Windows jobs均`steps=[]`，仍为既有runner/billing blocker，不能声明remote CI green。

目标：关闭canonical reviewer packet在生成后被截断、篡改或与其durable bindings漂移时，workstream宽松decode/静默跳过会让Mission Commander误以为reviewer work消失的operational断点。让新packet携带exact-bytes integrity receipt，并在status/handoff/start/reconcile/continue共同复用的workstream入口中fail-closed为lane-scoped blocked repair action。

已完成内容：

- attached canonical planning写入`packet.integrity.json`，receipt绑定schema/kind/sha256、packetId、targetLane、canonical packet path、exact bytes hash与size；packet内只保存canonical algorithm/path reference，identity覆盖该reference，legacy/custom/out-of-case packet不强制新增sidecar。
- durable workstream在提升任何dispatch/collection/intake/adoption command前验证canonical sidecar与exact packet bytes/bindings；hash/size/path/id/lane drift、missing/unknown/trailing sidecar或sidecar存在但packet移除reference均进入`reviewer-packet-integrity-invalid`，不投影runnable commands。adoption/direct/batch intake与collection执行端也在WhatIf/Apply路径复用stable packet reader与同一integrity validation，Apply mutation lock内再次校验exact packet。
- packet被截断到无法decode时，workstream从strict sidecar保留packetId/targetLane provenance并生成同一blocked handoff，防止active reviewer work静默消失；Mission Commander queue阻止普通lane continuation并只建议重新生成canonical packet，不建议手工修补packet或receipt。
- fresh canonical packet在candidate目录尚未创建时仍保持typed candidate/collection capability；legacy无sidecar packet继续按既有direct/batch intake兼容路径投影。

边界：integrity receipt不是外部授权证明，只用于repo-local durable packet完整性；不自动修复或覆盖packet/receipt，不spawn/stop/poll/monitor reviewer，不执行heavy tool，不写facts/authority/confirmed；collection与intake仍分别要求WhatIf→Apply。禁止新增PowerShell runtime logic。

验证结果：focused `subagents` / `workstream` / `cli` package tests已通过，覆盖exact sidecar binding、fresh planning→durable projection、lane drift与malformed/truncated packet fail-closed、removed-reference downgrade拒绝、执行入口integrity enforcement、blocked Mission Commander action与legacy compatibility。独立审查发现的reference downgrade、lane filter ordering与execution-path bypass均已修复；完整本地release minimum（`release-check` ready、`status`、`packs`、`doctor`、`go test ./...`、`go vet ./...`、`git diff --check`）已通过。远程run `29979488713`三平台jobs均在执行任何step前失败，未提供新的代码失败信号。

上一批摘要：Batch 550已完成reviewer collection capability gating closure，implementation commit `b0ea8c7 Gate reviewer collection capabilities`与release inspection commit `b995975 Record Batch 550 release gate inspection`已推送；对应run `29977522917`的Linux/macOS/Windows jobs均completed failure且`steps=[]`，仍为既有runner/billing blocker。

### Batch 550：reviewer collection capability gating closure

状态：已完成implementation、focused/package validation、两轮独立审查修复、完整本地release minimum、implementation commit `b0ea8c7 Gate reviewer collection capabilities`/push与远程release-gate inspection。对应run `29977522917` completed failure；Linux/macOS/Windows jobs均`steps=[]`，仍为既有runner/billing blocker，不能声明remote CI green。

目标：关闭Batch 549 collection执行端只接受canonical case-local review namespace，但planning仍会为attached custom `-ReviewOutputDir` / explicit `-PacketPath`生成必然失败collection commands的真实性断点。让planning、collection runtime与durable workstream共享同一canonical geometry，并把direct/batch intake能力与collection mutation能力显式分离。

已完成内容：

- 新增共享canonical review namespace helper：只接受`<case>/.rekit/reviews/<单层review>/packet.json`，并严格派生`results/candidates/<shard>.json`与`results/<shard>.json`；planning与collection runtime复用同一定义，nested/custom packet不再被视为collection-capable。
- `reviewerOrchestration.summary`新增`collectionAvailable`，与`intakeAvailable`/`dispatchOnly`分离。attached canonical packet继续生成typed candidate与collection WhatIf/Apply；attached custom/noncanonical artifact保持strict direct与packet batch intake，但省略candidate/collection commands并改用direct-result guidance；out-of-case packet保持纯dispatch-only，attach/init后要求重新生成canonical packet。
- durable workstream以实际扫描到的packet path为权威，并重新绑定embedded packet path、result root、candidate/result paths和collection candidate path；任一geometry不一致即抑制collection command/state/summary，不让forged packet进入Mission Commander collection queue，同时保留legacy direct intake fallback。
- collection执行端要求单层canonical packet，并从case root检查packet、result、candidate与publication parent的完整祖先链；planning在创建canonical review artifact目录前拒绝symlink traversal，collection在首次读取packet前拒绝`.rekit`或review root symlink。现有strict candidate validation、WhatIf→Apply、exact bytes/no-overwrite publication与idempotent replay保持不变。

边界：不禁止custom review artifacts，不删除legacy direct/batch intake。runtime不spawn、stop、poll或monitor reviewer；collection不写facts、不执行heavy tool、不修改managed/project source、不写authority/confirmed；intake仍独立要求WhatIf→Apply。禁止新增PowerShell runtime logic。

验证结果：focused `reviewpath` / `subagents` / `workstream` / `cli` tests已通过，覆盖default/custom canonical正例、case-local/external/custom-name/nested/case-variant packet capability suppression、nested runtime rejection、forged downstream geometry suppression、forged collection command canonical rebuild、missing candidate directory capability保留、canonical review symlink pre-write拒绝与`.rekit` symlink collection WhatIf/Apply no-publish。完整本地release minimum（`release-check` ready、`status`、`packs`、`doctor`、`go test ./...`、`go vet ./...`、`git diff --check`）已通过；两轮独立审查发现的case-root symlink祖先、fresh packet candidate parent、case-sensitive packet name与embedded command trust缺口均已修复。远程run `29977522917`三平台jobs均在执行任何step前失败，未提供新的代码失败信号。

上一批摘要：Batch 549已完成reviewer Agent handoff / immutable result collection closure，implementation commit `ae6b8bd Close reviewer result collection handoff`与release inspection commit `7aa1a7d Record Batch 549 release gate inspection`已推送；对应run `29974679916`的Linux/macOS/Windows jobs均completed failure且`steps=[]`，仍为既有runner/billing blocker。

### Batch 549：reviewer Agent handoff / immutable result collection closure

状态：已完成 implementation、focused/package validation、独立审查修复、完整本地 release minimum、implementation commit `ae6b8bd Close reviewer result collection handoff`/push 与远程 release-gate inspection。对应run `29974679916` completed failure；Linux/macOS/Windows jobs均`steps=[]`，仍为既有runner/billing blocker，不能声明remote CI green。

目标：关闭 `plan-subagents` packet 已能描述 reviewer task、strict intake 已能消费 canonical result，但主 Agent仍需手工拼 Claude Code Agent调用并直接写 canonical `reviewerResultPath` 的 operational 断点。让 packet提供 typed Agent tool request与独立 candidate path，并由 Go-native collection WhatIf→Apply严格验证和不可覆盖地发布exact result bytes，再接续既有 packet-level batch intake。

已完成内容：

- 每个 shard handoff与 reviewer orchestration dispatch新增 typed `agentToolRequest`（Claude Code Agent、read-only reviewer、prompt、exact single-JSON output contract）、packet派生 `reviewerResultCandidatePath` 与 collection preview/apply commands；runtime仍不启动、停止、轮询或监控 reviewer。
- 新增 `plan-subagents -CollectReviewerResult -ShardId ... -WhatIf/-Apply`：strict重读immutable packet与candidate，绑定repo/case/pack/lane、packet/route/shard/items、route output、evidence/conflict/write/heavy/authority blockers；WhatIf不创建canonical result，Apply持有packet/shard lock后重验并以temporary file + no-replace hard link发布exact bytes。
- canonical target只能来自packet handoff；different existing bytes fail-closed，exact replay返回`already-collected`；candidate/canonical intermediate或leaf symlink、empty/directory/oversize/trailing/unknown JSON、forged binding均拒绝。collection完成后若owner stale则先要求packet adoption，否则提升packet-level ready-result batch intake preview。
- status/overview/handoff/start/reconcile/continue与durable resume/checkpoint/digest复用 reviewer dispatch handoff中的candidate、typed Agent metadata和collection commands；candidate missing继续要求主Agent dispatch，non-regular candidate显式invalid，regular candidate提升collection WhatIf，canonical到位后提升batch intake。所有open阶段继续阻止lane continuation mutation。
- Windows本机 nested case/no-target/no-pack product path覆盖 planning→candidate write→collection WhatIf no-write→Apply publication→batch intake WhatIf→Apply；legacy direct `-ReviewerResultPath`与旧packet fallback继续保留。

边界：reviewer只返回JSON、不写文件/ledger；主Agent保存candidate并显式执行collection WhatIf→Apply。collection不写facts、不执行heavy tool、不修改managed/project source、不写authority/confirmed；intake仍是独立WhatIf→Apply。禁止新增PowerShell runtime logic。

验证结果：focused `subagents` / `workstream` / `cli` packages与case-local product-path tests已通过；独立审查发现并修复collection可被重算packetId引向case review namespace外、WhatIf遗漏canonical collision/non-regular preflight、`-ShardId`在非collection batch Apply被静默忽略三项问题，并补充forged namespace、WhatIf/Apply collision、invalid canonical与CLI no-write guards。完整本地release minimum已通过`release-check -Format json`（`ready=true`）、`status`、`packs`、`doctor`、`go test ./...`、`go vet ./...`与`git diff --check`。implementation run `29974679916`的Linux/macOS/Windows jobs均completed failure且`steps=[]`，仍为既有runner/billing blocker。

上一批摘要：Batch 548已完成reviewer packet adoption / continuation preflight closure，implementation commit `cc451d0 Close reviewer packet continuation preflight`已推送；对应run `29971220679`的Linux/macOS/Windows jobs均completed failure且`steps=[]`，仍为既有runner/billing blocker。

### Batch 548：reviewer packet adoption / continuation preflight closure

状态：已完成 implementation、focused/package validation、独立审查修复、完整本地 release minimum、implementation commit `cc451d0 Close reviewer packet continuation preflight`/push 与远程 release-gate inspection。对应run `29971220679` completed failure；Linux/macOS/Windows jobs均`steps=[]`，仍为既有runner/billing blocker，不能声明remote CI green。

目标：关闭 reviewer packet 由旧 executor/generation 创建、reviewer result 到达后 lane 被 replacement executor takeover 时，status/handoff仍可能看似可继续、strict intake却因 stale owner fail-closed 的 operational 断点。保持 packet与result binding immutable，用 case-local durable adoption receipt显式转移 strict intake ownership，并让 waiting/ready/partial/stale/blocked reviewer work统一阻止 lane continuation mutation。

已完成内容：

- 新增显式 reviewer packet owner adoption WhatIf→Apply product path，在`.rekit/reviewer-adoptions/<packetId>.json`写strict receipt，绑定exact packet SHA-256、repo/case/pack/lane、dispatched/adopted executor generation、actor/reason与no-spawn/no-heavy/no-authority boundary；packet bytes及既有reviewer result `packetId` binding保持不变，后续再次takeover会使receipt失效。
- status/overview、project/lane handoff、start/reconcile、continue、lane`RESUME.md`/checkpoint复用同一reviewer-aware Mission Commander queue；stale adoption、result symlink/attach repair、ready intake preview与waiting state按packet identity去重和排序，start/reconcile本次bounded Apply与execution evidence main escalation仍保持更高优先级。
- active reviewer work使continue WhatIf/Apply fail-closed：不创建run directory、不写facts、不刷新resume/checkpoint/board；partial verification-before-decision写回仍保持open并可通过deterministic event id重试。start/reconcile takeover先刷新board再生成durable resume/checkpoint，避免新executor owner快照滞后；handoff/resume prose不再把reviewer-blocked lane描述为ready continue。
- owner stale判定覆盖packet创建时unassigned/generation 0、之后首次claim executor的场景；status侧receipt validator strict拒绝unknown/trailing JSON及错误case/pack/path/hash/owner/boundary，adoption与intake lock路径拒绝metadata root/intermediate/leaf symlink。

边界：adoption只转移strict intake ownership，不修改原packet或reviewer result、不spawn/monitor reviewer、不执行heavy tool、不写authority/confirmed。reviewer intake仍由主Agent显式WhatIf后Apply；queue只排序和投影typed actions。禁止新增PowerShell runtime logic。

验证结果：focused adoption、stale owner/re-adoption history、effective owner writeback、forged receipt、symlink lock/path、queue priority、多packet identity、partial intake与case-local product-path coverage，以及`go test ./internal/rekit/workstream ./internal/rekit/subagents ./internal/rekit/overview ./internal/rekit/cli -count=1`均通过。独立审查发现并修复authoritative intake receipt contract弱于status validator、第二次takeover无法再次adopt、ledger仍记录旧owner三项问题。完整本地release minimum已通过`release-check -Format json`（`ready=true`）、`status`、`packs`、`doctor`、`go test ./...`、`go vet ./...`与`git diff --check`。implementation run `29971220679`的Linux/macOS/Windows jobs均completed failure且`steps=[]`，仍为既有runner/billing blocker。

上一批摘要：Batch 547已完成authorized adapter Mission Commander action queue closure，implementation commit `889c421 Close authorized adapter action queue`已推送；对应run `29967797366`的Linux/macOS/Windows jobs均completed failure且`steps=[]`，仍为既有runner/billing blocker。

### Batch 547：authorized adapter Mission Commander action queue closure

状态：已完成 implementation、focused/package validation、三轮独立审查修复、完整本地 release minimum、implementation commit `889c421 Close authorized adapter action queue`/push 与远程 release-gate inspection。对应 run `29967797366` completed failure；Linux/macOS/Windows jobs均`steps=[]`，仍为既有 runner/billing blocker，不能声明 remote CI green。

目标：关闭canonical adapter live state虽已进入status/handoff/continue独立字段、但replacement executor仍需把repair/record/escalation与普通lane continue手工拼接优先级的operational断点。把missing validation、invalid repair、valid record-ready、recorded evidence review与recorded main escalation收口到统一`MissionCommanderNextActions[]` / `MissionCommanderActionQueue`及durable handoff。

已完成内容：

- authorized-gate handoff保留contract/live validation产生的typed Mission Commander actions，并通过共享builder与lane/evidence actions合并；`GateEventID`进入typed action identity与去重键，多个gate的recorded/escalated/repair/record动作不再因同lane、同command互相吞并。
- exact recorded evidence逐gate去重adapter record动作；changed sidecar可替换同gate旧evidence；missing contract-only sidecar不会误删已有evidence。main escalation evidence排在普通evidence review之前成为current，其它gate adapter actions继续可见但blocked，并停止自主continue。
- status/overview、project/lane handoff、start、reconcile、continue preview/apply/blocked path、lane`RESUME.md`/checkpoint与durable handoff均复用统一queue；invalid sidecar repair与valid sidecar显式record可成为review-owned current action，而不是被普通lane continue抢占。start/reconcile显式preview Apply保持command-local priority，blocked lane takeover不会被adapter action抢占。
- product-path与unit coverage锁定invalid repair current、valid record-ready current、recorded evidence去重、same-path changed sidecar、多gate identity/main escalation priority、missing sidecar evidence preservation、blocked start takeover priority，以及status/handoff/continue只读状态不写`.rekit`。

边界：queue只排序并投影已有typed commands；不执行adapter/heavy-tool、不自动record observation、不写authority/confirmed。record仍要求主Agent显式复核并执行；不新增PowerShell runtime logic。

验证结果：focused adapter product-path、Mission Commander multi-gate identity/main escalation、missing sidecar preservation、blocked start takeover priority tests，以及`go test ./internal/rekit/mission ./internal/rekit/workstream ./internal/rekit/cli -count=1`均通过；三轮独立审查发现并修复same-path changed sidecar误去重、跨gate actions消失、start/reconcile preview优先级、多gate identity去重与main escalation current排序。完整本地release minimum已通过`release-check -Format json`（`ready=true`）、`status`、`packs`、`doctor`、`go test ./...`、`go vet ./...`与`git diff --check`。implementation run `29967797366`的Linux/macOS/Windows jobs均completed failure且`steps=[]`，仍为既有runner/billing blocker。

上一批摘要：Batch 546已完成canonical adapter sidecar live Mission Commander closure，implementation commits `fb1bd07 Close adapter sidecar live handoff`、`7038f04 Align recorded adapter handoff state`与release inspection commit `ca976d4 Record Batch 546 release gate inspection`已推送；最终implementation run `29964742726`的Linux/macOS/Windows jobs均completed failure且`steps=[]`，仍为既有runner/billing blocker。

### Batch 546：canonical adapter sidecar live Mission Commander closure

状态：已完成 implementation、focused/package validation、独立审查修复、完整本地 release minimum、implementation commits/push 与远程 release-gate inspection；最终 implementation commit `7038f04 Align recorded adapter handoff state` 已推送。对应 run `29964742726` completed failure；Linux/macOS/Windows jobs均`steps=[]`，仍为既有runner/billing blocker，不能声明remote CI green。

目标：关闭 adapter sidecar已写入canonical/default path后，replacement executor仍需显式运行另一条validation命令并手工判断是否可record、是否已record的operational断点。让只读`status`、`handoff`与`continue`自动复用strict validation，投影missing / invalid repair / valid record-ready / exact evidence already-recorded状态，同时保持record显式、无adapter/heavy replay。

已完成内容：

- authorized-gate compact handoff在canonical sidecar存在时复用`ValidateAdapterExecutionReport`，把真实`reportSummary`、selected adapter、repair hints、next steps与validation error投影到status、overview、handoff、start/reconcile、continue及durable resume/checkpoint/digest；sidecar缺失时保持contract-only状态。
- exact recorded detection绑定gate/report path和sidecar完整执行身份（adapter/status/budget/refs/boundary/escalation/summary），旧observation不会把同路径新内容误判为已记录；exact evidence到位后切换为`evidence-already-recorded`并只推荐evidence handoff/review，不再推荐record/replay。
- strict reader拒绝leaf或intermediate symlink、directory与其它non-regular sidecar，open后重新`f.Stat`并绑定同一file identity，避免只读status跟随symlink、阻塞特殊文件或在read窗口接受替换对象；新增`report-not-regular`failure/repair taxonomy。
- focused/package与nested case-local product-path coverage锁定invalid→repair、valid→record-ready、recorded→evidence review、same-path changed report、status/handoff/continue no-write及symlink fail-closed。

边界：live projection只读取并strict validate已存在canonical sidecar；不执行adapter/heavy-tool、不自动record observation、不写authority/confirmed。record仍要求主Agent复核`valid=true`后显式执行`gate -Apply ... -Actor ...`；不新增PowerShell runtime logic。

验证结果：focused gate identity/symlink/malformed-arrival/recorded-boundary tests、`go test ./internal/rekit/gate ./internal/rekit/workstream ./internal/rekit/overview ./internal/rekit/cli -count=1`、三轮独立审查及其record/replay suppression、record-ready queue replacement、recorded main-escalation preservation、malformed presence、text repair handoff修复均通过；完整本地release minimum已通过`release-check -Format json`（`ready=true`）、`status`、`packs`、`doctor`、`go test ./...`、`go vet ./...`与`git diff --check`。implementation commits `fb1bd07 Close adapter sidecar live handoff`、`7038f04 Align recorded adapter handoff state` 已推送；最终implementation run `29964742726` 的Linux/macOS/Windows jobs均completed failure且`steps=[]`，仍为既有runner/billing blocker。

上一批摘要：Batch 545已完成terminal decision receipt attestation，implementation commit `a7681b4 Attest terminal candidate decisions` 与 release inspection commit `482e19f Record Batch 545 release gate inspection` 已推送；implementation run `29961478940` 的Linux/macOS/Windows jobs均completed failure且`steps=[]`，仍为既有runner/billing blocker。

### Batch 545：pack-memory terminal decision receipt attestation closure

状态：已完成；implementation commit `a7681b4 Attest terminal candidate decisions` 已推送。对应 release-gate run `29961478940` completed failure；Linux/macOS/Windows jobs均 `steps=[]`，仍为既有 runner/billing blocker，不能声明 remote CI green。

目标：关闭 Batch 544 tooling reject/superseded无需reconsume时仅靠 receipt形状与非空 committed marker即可清除release blocker的真实性断点。让 release/status在把terminal receipt视为closed前，重新绑定exact packet/decision、transaction journal、committed result、candidate backup和decision evidence，确保 replacement executor接手的是可审计完成态。

已完成内容：

- release scanner对 `verificationPending=false` receipt执行strict terminal attestation：重算packet/decision SHA-256，strict decode decision/transaction/committed result，逐项绑定repo/case/pack、backup/index、counts与actions。
- 每个terminal action重新绑定decision outcome、candidate hash、reason/actor、evidence path/hash和staged candidate backup；decision/evidence/backup/transaction/marker drift均fail-closed。
- pending accepted receipt继续由Batch 543 strict verification proof路径收口，避免在proof产生前重复要求case-local packet artifact必须持续可读；proof仍绑定receipt hash与完整actions。
- `docs/agent-team-usage.md`补齐mixed与tooling-only WhatIf→Apply/terminal receipt语义；README、根/项目CLAUDE、pack reference、manifest/config与case-shim均无需改动，因为公共入口、schema、pack配置和shim边界未变化。

边界：本批只加强repo-local release/status只读attestation，不执行candidate decision、init/sync/reconsume、tooling merge或heavy action，不写authority/confirmed，不新增PowerShell runtime logic。

验证结果：focused terminal receipt valid/canonical path/decision drift/backup drift/forged tooling accept与existing pending proof测试、独立两轮审查、`go test ./...`、`go vet ./...`、`git diff --check`、`release-check -Format json`（`ready=true`）、`status`、`packs`与`doctor`均通过。implementation run `29961478940` 的Linux/macOS/Windows jobs均 completed failure且`steps=[]`，仍为既有runner/billing blocker。

上一批摘要：Batch 544已完成mixed candidate decision closure，implementation commit `1f36471 Close mixed candidate decisions` 与 release inspection commit `99f5474 Record Batch 544 release gate inspection` 已推送；implementation run `29959376352` 的Linux/macOS/Windows jobs均 completed failure且`steps=[]`，仍为既有runner/billing blocker。

### Batch 544：pack-memory mixed candidate decision closure

状态：已完成；implementation commit `1f36471 Close mixed candidate decisions` 已推送。对应 release-gate run `29959376352` completed failure；Linux/macOS/Windows jobs均 `steps=[]`，仍为既有 runner/billing blocker，不能声明 remote CI green。

目标：关闭 Batch 543 strict receipt consumer暴露的合法多决策断点：tooling-only reject/superseded packet没有 managed index时不能Apply，managed accept与 tooling reject混合 receipt在verification/release scanner中可能因candidate root不同而 fail-closed。让 replacement executor可在同一 reviewed packet内完成不同kind/outcome，并在无需reconsume时自动清除release阻塞。

已完成内容：

- candidate planning允许packet没有 managed pending item时使用 canonical missing index；WhatIf在全新 pack上不创建 candidate root/index/lock，Apply才安全建立 canonical candidate lock root；receipt始终记录canonical index path。
- receipt/verification按 action kind选择 managed `promote-candidates` 或 `tooling/candidates` root；混合 managed accept + tooling reject仍严格绑定 transaction、backup、decision hash、evidence与 accepted reconsume proof；tooling auto-accept继续拒绝。
- tooling-only reject生成非 pending receipt，清理candidate后不要求无意义的 fresh/attached verification，release/status scanner接受该完整receipt并保持ready。
- focused coverage新增混合 accept/reject product path、tooling-only reject和 completed non-verification receipt release handoff。

边界：所有decision仍必须来自exact durable packet并先WhatIf后Apply；tooling candidate只能reject/superseded或由人工另行review/merge，runtime不自动accept或写tooling catalog，不执行heavy-tool、不写authority/confirmed、不新增PowerShell runtime logic。

验证结果：focused promote mixed/tooling-only WhatIf→Apply与releasecheck completed/forged receipt测试、`go test ./...`、`go vet ./...`、`git diff --check`、`release-check -Format json`（`ready=true`）、`status`、`packs`与`doctor`均通过。implementation run `29959376352` 的 Linux/macOS/Windows jobs均 completed failure且 `steps=[]`；这是既有 GitHub Actions runner/billing blocker，不是远程测试执行失败。

上一批摘要：Batch 543已完成 candidate decision receipt/reconsume verification closure，implementation commit `8458ec7 Close candidate decision verification` 与 release inspection commit `12ab902 Record Batch 543 release gate inspection` 已推送；implementation run `29958441694` 的 Linux/macOS/Windows jobs均 completed failure且 `steps=[]`，仍为既有 runner/billing blocker。

### Batch 543：pack-memory candidate decision receipt / reconsume verification closure

状态：已完成；implementation commit `8458ec7 Close candidate decision verification` 已推送。对应 release-gate run `29958441694` completed failure；Linux/macOS/Windows jobs均 `steps=[]`，仍为既有 runner/billing blocker，不能声明 remote CI green。

目标：补齐 Batch 542 Apply后 candidate/index消失导致 replacement executor与 release/status丢失后续 doctor/reconsume工作的 operational断点。让 reviewed decision留下 durable receipt，并提供严格、显式、可幂等的 pack/fresh/attached verification product path；release/status在 candidate residue清零后仍能交付 pending verification，proof完成后自动闭环。

已完成内容：

- candidate decision Apply在 canonical candidate root的 `review-artifacts` 写单一 durable receipt，绑定 repo/case/pack、packet/decision hashes、transaction backup/index、actions/evidence与 verification proof path；accepted receipt给出包含 `<fresh-case>` / `<attached-case>` 必需输入的 concrete WhatIf命令。receipt写失败仍走既有 target/index/candidate rollback。
- 新增 `-VerifyCandidateDecision -FreshCaseRoot ... -AttachedCaseRoot ... -WhatIf/-Apply`：严格重读 packet/decision/receipt/transaction/committed marker，检查 receipt、transaction与 marker的 action/count/path bindings，reviewed candidate backup仍匹配 decision hash，accepted target与 staged reviewed candidate backup一致，candidate缺失、index不再引用 candidate、source/fresh/attached roots互异，并运行 pack/fresh/attached doctor和 fresh/attached accepted managed-content hash验证。WhatIf no-write；Apply持有 decision lock且在proof commit前重验状态，只写 repo-local durable proof，相同验证结果幂等 replay。
- release/status扫描 strict repo-local decision receipts；candidate/index已清理但 accepted verification尚无 proof时继续投影 open work、pending count、concrete command和 proof path。proof必须严格绑定 schema/kind、pack、packet/decision hashes、source/fresh/attached roots、receipt/proof paths、doctor rows、mutation/applied/ready状态和完整 actions；malformed、trailing、unknown-field或错误绑定均 fail-closed，合法 proof到位后该 receipt不再阻塞。CLI text与 JSON同步输出 receipt/pending/completed/verification状态。
- package/CLI coverage验证 receipt命令与 durable path、verification WhatIf no-write、Apply/replay、missing/same/source roots、stale fresh content、target drift、candidate/index重新出现、existing proof drift、malformed/wrong-hash/wrong-receipt/wrong-actions proof、release handoff pending/complete转换，以及 case-local CLI decision→fresh/attached init→verification全闭环。

边界：runtime不创建或初始化 fresh/attached cases，不执行 sync、heavy-tool或 authority/confirmed写入；调用者必须先准备两个不同的 attached cases，并先审查 WhatIf再Apply。verification只读取 pack/cases并写 repo-local proof；tooling candidate仍不允许自动accept；不新增 PowerShell runtime logic。

验证结果：focused candidate promote/releasecheck/CLI测试、`go test ./...`、`go vet ./...`、`git diff --check`、`release-check -Format json`（`ready=true`）、`status`、`packs`与`doctor`均通过。implementation run `29958441694` 的 Linux/macOS/Windows jobs均 completed failure且 `steps=[]`；这是既有 GitHub Actions runner/billing blocker，不是远程测试执行失败。

上一批摘要：Batch 542已完成 reviewed candidate decision/cleanup product path，implementation commit `882a553 Close pack memory candidate decisions` 与 release inspection commit `32650b9 Record Batch 542 release gate inspection` 已推送；implementation run `29953603900` 的 Linux/macOS/Windows jobs均 completed failure且 `steps=[]`，仍为既有 runner/billing blocker，不能声明 remote CI green。

### Batch 542：pack-memory reviewed candidate decision / cleanup product-path closure

状态：已完成 implementation、focused/package validation、完整本地 release minimum、implementation commit/push 与远程 release-gate inspection；implementation commit `882a553 Close pack memory candidate decisions` 已推送到 `main`。远程 release-gate run `29953603900` completed failure，Linux/macOS/Windows jobs 均 completed failure 且 `steps=[]`，仍是既有 GitHub Actions runner/billing blocker，不能声明 remote CI green。按 cadence 只记录 implementation commit触发的远程 run；不为本 release inspection commit自身触发的 CI追加第三个记录提交，除非出现不同于既有 `steps=[]` blocker的新信号。

目标：解决 Batch 538 durable candidate review workspace 之后仍需主 Agent手工复制 candidate到 pack target、删除 candidate并编辑 index 的 operational 断点。提供一个 strict、review-first、Go-native `promote -PacketPath ... -CandidateDecisionPath ... -WhatIf/-Apply` 闭环，让 replacement executor可从同一 durable packet预览并执行 reviewed accept/reject/superseded，同时保持 tooling merge、authority/confirmed与 heavy action边界。

已完成内容：

- 新增 strict candidate decision schema和 CLI入口；decision绑定 exact packet SHA-256、attached repo/case/pack、canonical candidate/tooling roots与 index、manifest managed target、packet pending review item、`create-candidate` write、managed index mapping、candidate/target hashes，以及每个 evidence ref 的 path + SHA-256；拒绝 unknown fields、trailing JSON、hash drift、forged roots/items/writes/index、path escape和 symlink traversal。
- `accept` 仅允许 managed-doc并写 exact packet/manifest target；tooling candidate auto-accept fail-closed，继续要求人工 catalog/recipe review。`reject` / `superseded` 只删除 reviewed candidate并移除对应 index entry。
- Apply在 canonical candidate root持有 pack-scoped exclusive lock，mutation前重新验证 packet/decision/index/candidate/target/evidence，在经过 symlink检查的随机 transaction目录中 exclusive stage candidate/target/index backups并同步写 durable journal；namespace中若存在其它 unfinished decision会 fail-closed，使用原 packet+decision重试时先严格校验 journal/backup/target绑定并执行 deterministic rollback。正常错误也会逆序恢复 target/index和已删除 candidate。CLI通过 typed error保留 `failedAction`、`rolledBack`、`recoveryRequired`、backup paths与 recovery actions；Windows路径不依赖 `os.Rename` 提供 crash-atomic保证；journal/recovery明确覆盖进程中断恢复，Unix额外同步 transaction目录项，系统断电级保证仍受平台文件系统语义限制。
- package/CLI coverage验证 WhatIf no-write、managed accept/backup/cleanup、case-local nested cwd product path、candidate/evidence hash drift、canonical root/manifest target/packet write/Apply recovery伪造、tooling auto-accept拒绝、candidate/backup-root symlink拒绝、live/stale/malformed lock、进程中断journal recovery与 cleanup failure rollback。

边界：本批只执行已由主 Agent在 durable packet上明确记录的 reviewed candidate decision；必须先 WhatIf再显式 Apply。runtime不自动决定 accept/reject/superseded，不自动 accept tooling candidate，不运行 doctor/init/reconsume，不创建 expected proof，不写 authority/confirmed、不执行 heavy-tool、不新增 PowerShell runtime logic。

验证结果：focused `go test ./internal/rekit/promote ./internal/rekit/cli -run "TestApplyCandidateDecisions|TestRunPromoteCandidateDecision" -count=1` 与 package `go test ./internal/rekit/promote -count=1` 已通过；完整本地 release minimum 已通过 `go run ./cmd/rekit -- -Command release-check -Format json`（ready=true / release gate inventory ok）、`go run ./cmd/rekit -- -Command status -Format text`、`go run ./cmd/rekit -- -Command packs -Format text`、`go run ./cmd/rekit -- -Command doctor -Format text`、`go test ./...`、`go vet ./...`、`git diff --check`（仅 LF→CRLF working-copy warning，无 whitespace error）。implementation commit `882a553 Close pack memory candidate decisions` 已推送；远程 release-gate run `29953603900` 已检查，Linux/macOS/Windows jobs均 completed failure且 `steps=[]`，仍符合既有 runner/billing blocker，不能声明 remote CI green。

上一批摘要：Batch 541 已完成 reviewer batch Mission Commander / durable handoff closure，implementation commit `5a128f9 Close reviewer batch commander handoff` 与 release inspection commit `ff077a8 Record Batch 541 release gate inspection` 已推送；implementation run `29949565383` completed failure，Linux/macOS/Windows jobs均 `steps=[]`，仍是既有 GitHub Actions runner/billing blocker，不能声明 remote CI green。

### Batch 541：reviewer batch Mission Commander / durable handoff closure

状态：已完成 implementation、focused/package validation、完整本地 release minimum、implementation commit/push 与远程 release-gate inspection；implementation commit `5a128f9 Close reviewer batch commander handoff` 已推送到 `main`。远程 release-gate run `29949565383` completed failure，Linux/macOS/Windows jobs 均 completed failure 且 `steps=[]`，仍是既有 GitHub Actions runner/billing blocker，不能声明 remote CI green。按 cadence 只记录 implementation commit 触发的远程 run；不为本 release inspection commit 自身触发的 CI 追加第三个记录提交，除非出现不同于既有 `steps=[]` blocker 的新信号。

目标：补齐 Batch 539 的真实 operational handoff：runtime 已支持 `-ReadyReviewerResults`，但 planning packet、Mission Commander action queue 与 downstream durable handoff 仍主要暴露逐 shard `-ReviewerResultPath ... -WhatIf/-Apply`，replacement executor 仍需手工发现并拼接 batch intake。让 attached-case reviewer planning、action queue、status/handoff/continue 与 durable artifacts 直接交付 packet-level batch preview/apply，同时保留旧 packet和 out-of-case安全语义。

已完成内容：

- attached-case `plan-subagents` 的 `reviewerOrchestration` / compact summary 新增 `batchPreviewCommand` 与 `batchApplyCommand`；`packet.json`、`summary.md` 和 CLI text 同步输出可复制的 `-ReadyReviewerResults` commands。out-of-case dispatch-only planning 保持 batch commands 为空。
- Mission Commander action queue 保留每个 shard 的 read-only dispatch，但将每 shard preview/apply 收口为 packet-level batch preview/apply；两 shard queue 从 6 个 action 收敛为 4 个，batch Apply 仍 blocked/requiresReview，直到 reviewer JSON、WhatIf 与 evidence review通过。
- downstream reviewer dispatch intake handoff / summary 现在携带 batch commands，status/handoff/continue、run status/digest、lane `RESUME.md` 与 checkpoint 可直接接手；ready 判定与 strict batch intake 共享 non-empty regular-file classifier，empty/directory 保持 waiting、symlink 显式 blocked；存在 ready shard 时 summary 选择该 ready packet 的 batch preview，不被最后一个 waiting shard或 5-row detail cap 覆盖。legacy packet无 batch fields时回退 single-result preview。
- package/CLI coverage锁定 attached/out-of-case command生成、action queue、summary artifact/text、ready+waiting packet选择、empty/directory/symlink classification、detail-cap ready preservation、legacy fallback，以及 status/continue/durable projection。

边界：本批只增强 reviewer orchestration command handoff 与 Mission Commander ordering；不自动 spawn、轮询或监控 reviewer，不创建 result，不绕过 strict batch intake、packet order、waiting/evidence/blocker review 或 verification-before-decision；不执行 heavy-tool、不写 authority/confirmed、不新增 PowerShell runtime logic。

验证结果：已通过 `gofmt`、`go test ./internal/rekit/subagents ./internal/rekit/workstream ./internal/rekit/cli -count=1`；完整本地 release minimum 已通过 `go run ./cmd/rekit -- -Command release-check -Format json`（ready=true / release gate inventory ok）、`go run ./cmd/rekit -- -Command status -Format text`、`go run ./cmd/rekit -- -Command packs -Format text`、`go run ./cmd/rekit -- -Command doctor -Format text`、`go test ./...`、`go vet ./...`、`git diff --check`（仅 LF→CRLF working-copy warning，无 whitespace error）。implementation commit `5a128f9 Close reviewer batch commander handoff` 已推送；远程 release-gate run `29949565383` 已检查，Linux/macOS/Windows jobs 均 completed failure 且 `steps=[]`，仍符合既有 runner/billing blocker，不能声明 remote CI green。

上一批摘要：Batch 540 已完成 release handoff current-batch truthfulness repair，implementation commit `74327c3 Fix current batch release handoff` 与 release inspection commit `d4e65f2 Record Batch 540 release gate inspection` 已推送；implementation run `29946738172` completed failure，Linux/macOS/Windows jobs 均 `steps=[]`，仍是既有 GitHub Actions runner/billing blocker，不能声明 remote CI green。

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

Batch 630 强制选择端到端能力闭环：不要再把单字段、summary、text line、first-screen 或 handoff 可见性投影单独立批；若需要投影字段，必须作为更大 runtime/writeback/executor/reviewer/adapter/pack-memory E2E 的支撑，并通过 CLI/product-path/E2E 验证实际完成一个 Mission Commander / replacement executor 可感知的状态转换。

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
