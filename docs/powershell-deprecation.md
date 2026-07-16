# PowerShell-free Go-native convergence roadmap

## 读取指南

本文件定义 `rekit` PowerShell 层的冻结、替换、迁移与删除策略，配合 `docs/autonomous-goal.md`、`docs/release-readiness.md` 和 `docs/go-first-convergence-plan.md` 使用。维护者在修改 `rekit/rekit.ps1`、`rekit/lib/*.ps1`、Go façade 委托集合、fallback 逻辑、旧文本 flow、默认验证路径或跨平台入口前应先读本文件顶部区域。

当前阶段目标已经从单纯 **PowerShell runtime deprecation strategy** 升级为 **PowerShell-free / Go-native / 跨平台 convergence**：Go CLI/backend 是 canonical runtime；PowerShell 只作为迁移期 legacy façade / fallback / parity residue。默认入口、默认验证、release gate、case shim 和文档路径应逐步不依赖 PowerShell。删除 PowerShell 代码不再因为“删除”本身停下询问，但必须有 Go-native 替代、文档、测试和恢复边界；删除公共入口且没有替代路径时仍需升级。

## 实施摘要

当前策略：

- **Go-owned**：确定性状态、结构化输出、低风险写入、`release-check` inventory、release invariant、pack-neutral contract 和跨平台路径优先由 `cmd/rekit/**` 与 `internal/rekit/**` 维护。
- **PowerShell façade**：`rekit/rekit.ps1` 只是迁移期公共兼容入口，负责 Go binary 查找、参数兼容、环境变量开关和旧 case 用户体验；默认 Go 委托与 no-fallback guard 后不再 dot-source legacy runtime modules，也不再保留 command switch fallback dispatcher；不承载新的业务语义。
- **Legacy-only**：内部命令和非 Go-owned 写入路径暂时保留，禁止扩展新能力；`start` / `handoff` / `continue` 的文本/default 工作线 flow 已由 Go 接管，不再作为 PowerShell fallback 保留。
- **Parity smoke**：少量 PowerShell smoke 仅保留为 Windows façade / fallback 回归，不进入默认 release gate；后续应迁移到 Go CLI E2E、Go package tests 或跨平台测试。
- **Release inventory**：Go-owned `release-check -Format json` 输出 `powerShellDeprecation`、`ciReleaseGate`、`caseShim` 与 `publicDefaultDocs`，解析本文件中的命令归属、模块状态、freeze gates 和 blocked migrations，并对照 `rekit/rekit.ps1` 默认委托集合、`rekit/lib/*.ps1` 实际模块清单、默认 CI workflow、case-local thin shim 模板与公开默认文档入口发现漂移；其中 `powerShellDeprecation.fallbackRetirement` 进一步列出 Go-default commands、no-fallback commands、fallback retirement candidate commands、blocked commands 与 removal-candidate modules，作为后续 fallback removal batch 的机器可读前置库存。
- **删除前置条件**：对应命令已有 Go owner、Go-native 文档入口、release invariant、测试覆盖、fallback 替代或明确删除条件后，即可按独立 batch 删除或降级 PowerShell 实现。
- **最终状态**：PowerShell 不再是默认 runtime、默认 public entrypoint、默认验证路径、release gate 依赖或 case shim 依赖；macOS/Linux/Windows 默认路径均由 Go-native runtime 支撑。

## 执行清单

修改 PowerShell 或新增 Go-owned 路径时按以下流程执行：

1. 判断目标属于 **Go-owned**、**façade-only**、**legacy-only**、**removal-candidate** 还是 **blocked**。
2. Go-owned 命令的新逻辑优先进入 Go package；PowerShell 只改委托、参数透传、compatibility 或删除路径。
3. Legacy-only 路径只做 bug fix、安全边界修复、兼容性维护或替换准备；不新增功能面。
4. 禁止新增 PowerShell runtime logic；如必须保留 PowerShell，必须写明依赖方、阻塞原因和删除条件。
5. 默认委托变化或 fallback 删除必须同步更新本矩阵、release invariants、release-check inventory、验证说明与相关 smoke。
6. 删除 PowerShell 代码前，先确认 Go-native 路径通过 release gate，且 README、CLAUDE、skill、case shim、tests、CI/release-check 不再把被删路径作为默认入口。
7. 每批更新 `docs/batch-plan.md`、`docs/release-readiness.md`、`docs/go-first-convergence-plan.md` 或本文件，避免只留在聊天上下文中。

## 验证标准

PowerShell-free / Go-native convergence 相关变更至少满足：

- Go package tests 覆盖新 owner 的 deterministic 行为。
- `go run ./cmd/rekit -- -Command release-check -Format json` 输出 `ready=true`。
- `go run ./cmd/rekit -- -Command status`、`packs` 与 `doctor` 通过。
- `go test ./...` 与 `go vet ./...` 通过。
- 默认 CI / recommended minimum 不要求 `rekit.ps1` 或 `facade-smoke.ps1`；PowerShell smoke 仅在改 façade/fallback 时按需追加。
- 涉及迁移期 façade 委托时运行 `./rekit/tests/facade-smoke.ps1`；涉及 legacy fallback 时显式验证 `REKIT_GO_DISABLE=1`，直到该 fallback 被正式删除。
- 涉及 workstream / ledger / gate / sync / promote 写入时使用临时 case 验证 containment、backup、review-first 和 no authority/confirmed 边界。
- `git diff --check` 无 whitespace error。

## 风险与注意事项

- 不把 PowerShell 删除当作 release readiness 的前提；目标是先让默认路径不依赖 PowerShell，再按批次删除遗留入口。
- 不复制 runtime 逻辑到 case-local thin shim；case-local `/rekit` 必须继续回到 kit 仓库 canonical Go runtime。
- 不用 Go 默认委托绕过 review-first、backup、deny、restore、gate 或人工确认。
- 不把 actual heavy-tool、authority/confirmed 写入、policy schema 迁移、外部副作用纳入自动迁移。
- 不默认运行大型 matrix；只在 pack helper/matrix 或跨 pack skeleton 变更时选择运行。
- 不为了保留 Windows 兼容而牺牲 macOS/Linux 默认路径；Windows 兼容应由 Go runtime 或明确 legacy shim 支撑。

## 命令归属矩阵

| 区域 | 当前 owner | PowerShell 状态 | 冻结/删除策略 |
|---|---|---|---|
| `release-check` | Go default | façade delegate + no PowerShell fallback | 只输出 release gate inventory，不执行测试、不写状态；保持 Go-owned。 |
| `status` / `packs` / `doctor` / `validate` | Go default | façade delegate + no PowerShell fallback | 默认本机验证路径已由 Go-native runtime 覆盖；PowerShell fallback 已退休，使用 Go release gate / package tests 维护 parity。 |
| case lifecycle `attach` / `repair` / `init` / `bootstrap` preview/apply | Go default | façade delegate + no PowerShell fallback | attach/repair/init/bootstrap 的预览与显式 `-Apply` 均由 Go-owned/no-fallback 边界维护；裸 lifecycle 命令和 `REKIT_GO_DISABLE=1` 不再进入 PowerShell fallback。 |
| `sync` / `update` review/apply/JSON preview | Go default | façade delegate + no PowerShell fallback | `update` 作为 sync alias 同属 Go-default façade；PowerShell fallback 已退休，review、JSON preview 与显式 apply 由 Go 维护，继续保持 review-first 与 explicit apply 边界。 |
| `promote` review/artifacts/candidates/apply/JSON preview | Go default | façade delegate + no PowerShell fallback | promote review、review artifacts、candidate/apply、JSON preview 与文本 what-if 均由 Go-owned/no-fallback 边界维护；pack source 写入仍要求 review-first/backup/deny/restore。 |
| `overview` text/JSON 与缺 board 初始化 | Go default | façade delegate + no PowerShell fallback | Go overview 是 canonical owner；PowerShell fallback 已退休，缺 board 初始化仍只写 case-local scaffold。 |
| `note -List` text/table/tsv/JSON、`note` append、`note -WhatIf` | Go default | façade delegate + no PowerShell fallback | 新 ledger schema 校验由 Go 维护；PowerShell fallback 已退休，append 仍只写 facts JSONL、不写 authority/confirmed。 |
| `gate -WhatIf` / `gate -Apply` pending-gate | Go default | façade delegate + no PowerShell fallback | 只预览或写 pending-gate request；不执行 heavy-tool；PowerShell fallback 已退休。 |
| `start` / `handoff` preview/apply/text/default | Go default | façade delegate + no PowerShell fallback | Batch 232 起 JSON preview、explicit apply、文本 preview 与 bare/default 工作线 flow 均由 Go-owned 路径接管；façade 对非 `-Apply` 且空 `-Format` 明确转为 Go text output，同时 direct Go CLI 继续保持默认 JSON contract。 |
| `continue -WhatIf` / explicit `continue -Apply` text/JSON | Go default | façade delegate + no PowerShell fallback | Batch 232 起 JSON preview、explicit apply 与文本/default preview 均由 Go-owned 路径接管；`continue -Apply` 仍只写 case-local facts/routing/run digest/lane resume/checkpoint/board，不写 authority/confirmed、不执行 heavy-tool。 |
| `plan-subagents` review artifacts | Go default | façade delegate + no PowerShell fallback | 只写 review packet / summary / combined diff 路径；不自动 spawn agent；PowerShell fallback 已退休，使用 Go package tests 与 `plan-subagents-smoke.ps1` 维护 parity。 |
| actual heavy-tool 执行 | 未迁移 | blocked / manual gate | 不自动迁移；需要用户确认和单独设计。 |
| authority/confirmed 写入 | 人工 gate / legacy guarded | blocked | 不由 Go apply 自动执行；删除/迁移需 schema 与恢复策略。 |

## PowerShell 模块状态

| 模块 | 状态 | 说明 |
|---|---|---|
| `rekit/rekit.ps1` | façade-stable / retained | 迁移期公共入口、参数兼容、Go binary 查找和错误透传；Batch 234 后不再 dot-source `rekit/lib/*.ps1` 或保留 command switch fallback dispatcher；Batch 240 后作为唯一保留 PowerShell façade，不承载业务 runtime。 |
| `rekit/lib/Manifest.ps1` | retired / removed | manifest 读取/兼容 helper 已由 Go manifest package 接管；Batch 240 物理删除。 |
| `rekit/lib/Validate.ps1` | retired / removed | pack/case doctor 兼容层已由 Go doctor 默认路径接管；Batch 240 物理删除。 |
| `rekit/lib/Instance.ps1` | retired / removed | case instance/shim 兼容已由 Go case lifecycle 接管；Batch 240 物理删除。 |
| `rekit/lib/Sync.ps1` | retired / removed | sync/update fallback 已退休；写入语义 Go-owned；Batch 240 物理删除。 |
| `rekit/lib/Promote.ps1` | retired / removed | promote fallback 已退休；candidate/apply 语义 Go-owned；Batch 240 物理删除。 |
| `rekit/lib/Review.ps1` | retired / removed | review artifact helper 已由 Go review/promote/sync paths 接管；Batch 240 物理删除。 |
| `rekit/lib/B3.Core.ps1` | retired / removed | PowerShell B3 基础 helper 已无默认 runtime 依赖；Batch 240 物理删除。 |
| `rekit/lib/B3.State.ps1` | retired / removed | board/facts/lane 状态兼容已由 Go Mission Control/workstream packages 接管；Batch 240 物理删除。 |
| `rekit/lib/B3.Policy.ps1` | retired / removed | policy helper 不再作为默认 runtime；blocked policy schema migration 仍需单独 gate；Batch 240 物理删除。 |
| `rekit/lib/B3.Lane.ps1` | retired / removed | lane prompt/workspace helper 已由 Go start/handoff/continue paths 接管；Batch 240 物理删除。 |
| `rekit/lib/B3.Auto.ps1` | retired / removed | continue auto / authority guard 旧实现已由 Go continue guard 和人工 authority gate 取代；Batch 240 物理删除。 |
| `rekit/lib/B3.Handoff.ps1` | retired / removed | handoff fallback 展示已由 Go handoff 接管；Batch 240 物理删除。 |
| `rekit/lib/B3.Commands.ps1` | retired / removed | 文本工作线和内部命令入口 fallback 已退休；Batch 240 物理删除。 |

## Façade runtime dependency inventory

`release-check -Format json` 的 `powerShellDeprecation.facadeRuntime` 是 Batch 234 删除公共 façade legacy dispatcher 后的确定性回归库存：

- `legacyModuleImportsPresent=false` 表示 `rekit/rekit.ps1` 不再 dot-source `rekit/lib/*.ps1` runtime modules。
- `commandDispatcherPresent=false` 表示公共 façade 不再包含 legacy command switch fallback dispatcher 的代表性调用或 `switch ($Command)` dispatcher。
- `noFallbackGuardPresent=true`、`goDelegationPresent=true` 与 `retiredDispatcherError=true` 表示公共 façade 仍保留 Go delegation、no-fallback guard 与 retired dispatcher error。
- `forbiddenPatterns[]` 与 `requiredPatterns[]` 是后续删除/归档 batch 的机器可读审计清单；任一 forbidden pattern 回归或 required pattern 缺失都会让 `powerShellDeprecation.ready=false`。

该库存只审计公共 façade 是否重新依赖 legacy runtime；Batch 240 后 `rekit/lib/*.ps1` 已删除，该库存继续防止 façade 重新加载已删除 legacy modules，也不表示 blocked heavy-tool、authority/confirmed 或 policy schema 迁移可以顺手迁入。

## Public façade retention inventory

`release-check -Format json` 的 `powerShellDeprecation.publicFacade` 是 Batch 241 为保留公共迁移期 façade 新增的只读库存：

- `present=true` 与 `retained=true` 表示 `rekit/rekit.ps1` 仍作为迁移期公共 façade 明确保留，并在模块状态矩阵中标记为 retained，而不是被误当作 legacy runtime module 删除。
- `commandSurface[]` 来自 `rekit/rekit.ps1` 的 `ValidateSet`，当前基线为 19；每个公开 façade command 都必须同时出现在 `goDefaultCommands[]` 与 `noFallbackCommands[]` 中，确保公共 façade 只透传 Go-owned / no-fallback 命令面。
- `goNativeAlternative` 当前为 `go run ./cmd/rekit -- -Command <command>`，记录公共 façade 删除或失效时的底层 Go-native 替代路径；公开用户默认路径仍由 `/rekit` skill / Mission Control 负责，不把 raw Go CLI 变成用户主要交互界面。
- `migrationBoundaryDocumented=true` 与 `removalBoundaryDocumented=true` 分别要求本文件说明 façade 是迁移期公共入口、不承载业务 runtime，并要求未来删除公共 `rekit/rekit.ps1` façade 时必须是独立 removal batch，包含替代入口、恢复计划、验证和文档。

该库存只锁定公共 façade 的“保留但非 runtime owner”状态；它不鼓励新增 PowerShell 逻辑，也不把公共 façade 删除并入普通 legacy module removal。若新增 façade command 没有 Go-default/no-fallback owner，或文档未声明删除边界，`powerShellDeprecation.ready=false`。

## Go-native public surface inventory

`release-check -Format json` 的顶层 `goNativePublicSurface` 是 Batch 242 为公共 façade 删除准备新增的只读库存：

- `entrypoint=cmd/rekit` 与 `entrypointPresent=true` 表示底层 Go-native entrypoint 存在；`alternativePattern=go run ./cmd/rekit -- -Command <command>` 与 `powerShellDeprecation.publicFacade.goNativeAlternative` 保持一致。
- `commandCatalogPath=internal/rekit/commands/commands.go` 与 `commands[]` 是 Go-owned public command catalog，当前基线为 19；`handlerCommands[]` 来自 Go CLI dispatcher 的 `commands.*` handler case，`symbolCommands{}` 来自 public command symbol catalog；`commandProfiles[]` 与 `mutationBoundaries[]` 记录每个 public command 的 read-only、case-local append/apply/bootstrap、review artifact、review-first 或 kit review-first 边界；`commandProfileSummary` 汇总并校验 `total=19`、`readOnly=5`、`mutating=14`、`writesCase=13`、`writesKit=1`、`reviewFirst=3`、`applyRequired=11`、`heavyTool=0` 与 `authorityConfirmed=0`；`commandProfileGroups` 将这些 counts 反向展开为 read-only、mutating、case write、kit write、review-first、apply-required、heavy-tool、authority/confirmed 和 per-boundary command sets；`commandProfileBoundaries[]` 将 7 个 mutation/read-only boundary 输出为 `boundary`、`count` 与 `commands[]` rows，并与 summary / groups 互相校验；`commandProfilePolicies[]` 将 no heavy-tool、no authority/confirmed、kit write review-first、review-first apply-required 与 known mutation boundary guardrails 输出为 policy rows，并要求 violation count 为 0。release inventory 要求 catalog、dispatcher handlers、symbols 与 command profiles 均覆盖同一 19 个 public commands，并拒绝 public command profile 标记 actual heavy-tool 或 authority/confirmed 写入，避免 public façade command surface、Go-native catalog、CLI handler coverage、mutation boundary catalog、summary counts、command groups、boundary rows 和 policy rows 各自漂移。
- `facadeRemovalReady=true` 与 `facadeRemovalPrerequisites[]` 是 Go-native public surface 的公共 façade 删除前置库存，不代表本批删除 façade；当前 5 条 prerequisites 分别覆盖 `entrypoint`、`catalog-handler-symbol-profile-coverage`、`mutation-boundary-inventory`、`profile-policy-guards` 与 `unsupported-command-diagnostic`。顶层 `publicFacadeRemoval` 进一步汇总跨 inventory readiness，要求 public façade retained/boundary documented、façade command surface 全部 Go-default/no-fallback、Go-native public surface ready、legacy runtime detached、legacy module removal settled、module reference blockers clear 六条 prerequisites 全部 ready 后，才能把公共 façade 删除作为后续独立 removal batch 评估。
- `unsupportedCommandDiagnosticPresent=true` 要求 direct Go CLI 的未知命令错误列出 supported commands 与 Go-native alternative pattern，确保公共 façade 删除或失效时仍有机器可读、可诊断的底层替代路径。
- release inventory 会双向校验 `goNativePublicSurface.commands[]` 与 `powerShellDeprecation.publicFacade.commandSurface[]`；任一公开 façade command 未进入 Go-native catalog，或 Go-native public command 未出现在 façade command surface，都会让 `release-check` 失败。

该库存不把 raw Go CLI 变成用户主要交互界面；公开用户默认路径仍由 `/rekit` skill / Mission Control 负责。它只为未来独立删除公共 `rekit/rekit.ps1` façade 时提供确定性替代路径、命令面和诊断基线。

## Public façade removal plan inventory

`release-check -Format json` 的顶层 `publicFacadeRemoval` 是后续删除公共 `rekit/rekit.ps1` façade 前必须通过的跨库存 readiness：

- `public-facade-retained-boundary` 要求公共 façade 当前仍存在、仍被文档标记为 retained / migration boundary，并继续声明删除公共 façade 必须作为独立 removal batch。
- `facade-command-surface-no-fallback` 要求 19 个 façade commands 与 Go-default / no-fallback command surface 完全一致，不能留下隐式 PowerShell fallback。
- `go-native-public-surface` 要求 `goNativePublicSurface.ready=true` 且 Go-native surface 自身的 `facadeRemovalReady=true`。
- `legacy-runtime-detached`、`legacy-module-removal-settled` 与 `module-reference-blockers-clear` 要求公共 façade 不再加载 legacy runtime、不再包含 legacy dispatcher、legacy modules 已 settled，且 active test dependencies、compatibility fixtures、removal blockers 与 unclassified references 均为空。
- `removal-plan-documented` 要求本节持续记录后续独立 removal batch 的替代入口、恢复计划、验证命令、文档同步、CHANGELOG、当前授权边界、不新增 PowerShell runtime logic，以及不触碰 actual heavy-tool、authority/confirmed 的边界；`publicFacadeRemoval.removalPlan.replacementEntrypoints[]` 必须把 canonical `/rekit` skill、case-local thin shim、direct Go CLI 与 cross-platform release gate 四个替代入口纳入机器可读清单，并要求每项带 audience、purpose、Go-native backed / user-facing 标记与 validation commands；`publicFacadeRemoval.removalPlan.deletionGates[]` 必须把 go-native-alternatives-ready、public-references-migrated、facade-smoke-retired、recovery-path-ready 与 release-gate-green 五个 blocking gates 纳入机器可读删除门禁，并要求每项带 blocked execution steps、input inventory、exit criteria、failure signals、escalation triggers、escalation evidence、escalation recipients、escalation handoff steps、escalation decision options、escalation retry conditions、escalation stop conditions、escalation resolution artifacts、escalation closure checks、escalation reopen conditions、escalation ledger events、escalation state transitions、escalation boundary guards、escalation audit checks、verification artifacts、remediation actions 与 validation commands，release-check / handoff 固定展示 `deletionGates=5 deletionGateValidationCommands=40 deletionGateExitCriteria=15 deletionGateFailureSignals=15 deletionGateEscalationTriggers=15 deletionGateEscalationEvidence=15 deletionGateEscalationRecipients=15 deletionGateEscalationHandoffSteps=15 deletionGateEscalationDecisionOptions=15 deletionGateEscalationRetryConditions=15 deletionGateEscalationStopConditions=15 deletionGateEscalationResolutionArtifacts=15 deletionGateEscalationClosureChecks=15 deletionGateEscalationReopenConditions=15 deletionGateEscalationLedgerEvents=15 deletionGateEscalationStateTransitions=15 deletionGateEscalationBoundaryGuards=15 deletionGateEscalationAuditChecks=15 deletionGateVerificationArtifacts=15 deletionGateBlockedExecutionSteps=10 deletionGateRemediationActions=15`；`publicFacadeRemoval.removalPlan.executionSteps[]` 必须把 verify Go-native alternative、迁移 public references、retire façade smoke、delete public façade 与 rerun release gate 纳入机器可读执行步骤，并要求每步绑定 dependencies、input inventory、output artifacts、failure signals、remediation actions、verification artifacts、ledger events、state transitions、escalation triggers、escalation evidence、escalation recipients、escalation handoff steps、escalation decision options、escalation retry conditions、escalation stop conditions、escalation resolution artifacts、escalation closure checks、escalation reopen conditions、escalation ledger events、boundary guards、audit checks、validation commands、no PowerShell runtime logic 与 no external effects 边界；`publicFacadeRemoval.removalPlan.boundaryChecks[]` 必须把 no PowerShell runtime logic、no actual heavy-tool、no authority/confirmed write、sync/promote review-first、case-local write semantics 与 no external effects 六个 preserved required boundaries 纳入机器可读边界检查，并要求每项带 evidence 与 validation commands；`publicFacadeRemoval.removalPlan.recoverySteps[]` 必须把 separate revertable commit、公共 façade 文件恢复、同步文档/release notes 恢复与 Go-native release gate 重新验证纳入机器可读恢复步骤；`publicFacadeRemoval.removalPlan.documentationTargets[]` 必须把未来 removal batch 必须同步的文档路径、purpose、action 与 validation commands 纳入机器可读清单。
- `removal-impact-inventoried` 要求 `publicFacadeRemoval.removalImpact` 持续扫描并分类仓库内 `rekit/rekit.ps1` / `rekit.ps1` / `facade-smoke.ps1` 引用面，确保公共 façade 删除前已看到 public façade entrypoint、facade compatibility smoke、facade-dependent smoke、pack wrapper compatibility、public default docs、roadmap/history docs 与 release inventory/test 影响面，且没有 unclassified references；`removalImpact.workItems[]` 必须覆盖每个 reference category，并给出后续 removal batch 的 required action、count、paths 与 `validationCommands[]`；`removalImpact.migrationTargets[]` 必须把全部 public façade references 转为 required、Go-native preferred、带 `validationCommands[]` 的迁移目标，并对 roadmap/history docs 保留 historical context；`removalImpact.smokeMigrationTargets[]` 必须把 façade compatibility smoke 与 façade-dependent smoke 标记为 Go-native preferred、no facade compatibility、retire façade assertions 的迁移/退休目标。

后续真正删除公共 façade 时，替代入口仍以 `/rekit` skill / Mission Control 为用户默认路径，底层 deterministic alternative 为 `go run ./cmd/rekit -- -Command <command>`；恢复计划至少包括保留可 revert 的单独 commit、在失败时恢复 `rekit/rekit.ps1` 公共 façade 文件与相关 docs，并重新运行本节列出的验证命令。验证命令至少覆盖 `go run ./cmd/rekit -- -Command release-check -Format json`、`go run ./cmd/rekit -- -Command release-check`、`go run ./cmd/rekit -- -Command status`、`go run ./cmd/rekit -- -Command packs`、`go run ./cmd/rekit -- -Command doctor`、`go test ./...`、`go vet ./...` 与 `git diff --check`。文档同步必须覆盖 README / CLAUDE / canonical `/rekit` skill / case shim / release readiness / PowerShell deprecation / Go-first convergence plan / batch plan / CHANGELOG，并在 `documentationTargets[]` 中保留每个 target 的 purpose、action 与 `validationCommands[]`；影响面盘点必须处理 `publicFacadeRemoval.removalImpact.referenceCategories[]` 与 `workItems[]` 中列出的 façade entrypoint、compatibility smoke、facade-dependent smoke、pack wrapper、public docs、roadmap/history docs 与 release inventory/test rows，并按每个 work item 的 `validationCommands[]` 记录后续 removal batch 的验证覆盖；替代入口盘点必须处理 `publicFacadeRemoval.removalPlan.replacementEntrypoints[]` 中列出的 canonical `/rekit` skill、case-local thin shim、direct Go CLI 与 cross-platform release gate，确认 user-facing 路径仍是 `/rekit` / Mission Control、底层 deterministic alternative 仍是 Go-native、CI/release gate 不依赖公共 façade；删除门禁盘点必须处理 `publicFacadeRemoval.removalPlan.deletionGates[]` 中列出的 go-native-alternatives-ready、public-references-migrated、facade-smoke-retired、recovery-path-ready 与 release-gate-green，确认 Go-native alternatives、public references、façade smoke、recovery path 与 release gate 均在删除前阻断，且每个 gate 的 `blockedExecutionSteps[]` 指向 delete-public-facade / rerun-release-gate 等被阻断步骤、`exitCriteria[]` 已给出可判定退出条件、`failureSignals[]` 已列出 gate 未满足时可观测的失败信号、`escalationTriggers[]` 已列出必须提升到主 Agent / 用户决策的触发条件、`escalationEvidence[]` 已列出触发升级时必须附带的证据、`escalationRecipients[]` 已列出升级路由接收者、`escalationHandoffSteps[]` 已列出升级交接步骤、`escalationDecisionOptions[]` 已列出升级后可选决策、`escalationRetryConditions[]` 已列出升级后允许重试的条件、`escalationStopConditions[]` 已列出升级后必须停止而不能重试的条件、`escalationResolutionArtifacts[]` 已列出升级收束后必须记录的决策/证据产物、`escalationClosureChecks[]` 已列出升级关闭前必须执行的检查、`escalationReopenConditions[]` 已列出升级关闭后必须重新打开的条件、`escalationLedgerEvents[]` 已列出升级过程必须记录的账本事件、`escalationStateTransitions[]` 已列出升级过程允许的状态转换、`escalationBoundaryGuards[]` 已列出升级过程必须遵守的边界保护、`escalationAuditChecks[]` 已列出升级结果必须通过的审计检查、`verificationArtifacts[]` 已列出必须产出或检查的证据、`remediationActions[]` 已列出 gate 未满足时先执行的修复动作；执行步骤盘点必须处理 `publicFacadeRemoval.removalPlan.executionSteps[]` 中列出的 verify Go-native alternative、migrate public references、retire façade smoke、delete public façade 与 rerun release gate 步骤，确认 dependencies、input inventory、output artifacts、failureSignals[]、remediationActions[]、verificationArtifacts[]、ledgerEvents[]、stateTransitions[]、escalationTriggers[]、escalationEvidence[]、escalationRecipients[]、escalationHandoffSteps[]、escalationDecisionOptions[]、escalationRetryConditions[]、escalationStopConditions[]、escalationResolutionArtifacts[]、escalationClosureChecks[]、escalationReopenConditions[]、escalationLedgerEvents[]、boundaryGuards[]、auditChecks[]、validation commands、no PowerShell runtime logic 与 no external effects 边界完整；边界检查盘点必须处理 `publicFacadeRemoval.removalPlan.boundaryChecks[]` 中列出的 no PowerShell runtime logic、no actual heavy-tool、no authority/confirmed write、sync/promote review-first、case-local write semantics 与 no external effects 六个 preserved required boundaries，确认每项 evidence、validation commands 与当前授权边界完整；通用迁移目标盘点必须处理 `publicFacadeRemoval.removalImpact.migrationTargets[]` 中全部 public façade references，保持 `required=true`、`goNativePreferred=true`、每项绑定 Go-native release gate 验证命令，并对 roadmap/history docs 设置 historical context 保留边界；smoke 迁移盘点必须处理 `publicFacadeRemoval.removalImpact.smokeMigrationTargets[]` 中列出的 façade compatibility smoke 与 façade-dependent smoke，保持 Go-native preferred、no facade compatibility 与 retire façade assertions 边界。恢复计划盘点必须处理 `publicFacadeRemoval.removalPlan.recoverySteps[]` 中列出的 separate revertable commit、restore public façade、restore synchronized docs 与 rerun Go-native release gate 步骤。文档目标盘点必须处理 `publicFacadeRemoval.removalPlan.documentationTargets[]` 中列出的 README、CLAUDE、canonical `/rekit` skill、case shim、release readiness、PowerShell deprecation、Go-first plan、batch-plan 与 CHANGELOG。该 removal batch 不得新增 PowerShell runtime logic，不得执行 actual heavy-tool、authority/confirmed 写入或改变 sync/promote review-first 与 case-local write semantics。

## Module removal inventory

`release-check -Format json` 的 `powerShellDeprecation.moduleRemoval` 是 Batch 240 删除 legacy PowerShell runtime modules 后的只读 readiness 库存：

- `candidateModules[]` 来自模块状态矩阵中仍标记为 `removal-candidate` 且仍存在的 `.ps1` 文件；当前基线为 0，表示 `rekit/lib/*.ps1` legacy runtime modules 已无待删候选。
- `retiredModules[]` 来自模块状态矩阵中的 `retired / removed` 行，记录已从磁盘移除的 legacy runtime modules；当前基线为 13，且每项必须 `present=false`。
- `facadeRuntimeDependencies[]` 必须为空，表示公共 façade 当前不依赖候选或已删除 legacy modules；若 `rekit.ps1` 重新加载 legacy runtime modules，则该清单会转为 warning。
- `undocumentedModules[]` 必须为空，表示实际存在的 `rekit/` PowerShell 文件都已在模块状态矩阵中登记；若未来重新引入 `rekit/lib/*.ps1`，必须先显式分类，而不能作为隐式 runtime 回归。

该库存只给出 façade 保留、legacy modules 已删除和回归防护信号；后续若要删除公共 `rekit/rekit.ps1` façade，仍必须按独立 removal batch 执行，包含替代入口、恢复计划、验证和文档。

## Module reference inventory

`release-check -Format json` 的 `powerShellDeprecation.moduleReferences` 是删除或归档 PowerShell 文件前的引用面分类库存：

- `activeTestDependencies[]` 记录仍直接 dot-source `rekit/lib/*.ps1` 的主动测试依赖；当前基线为 0，`continue-preflight-smoke.ps1` 已改为 Go-owned package-test wrapper，不再加载 legacy runtime modules。
- `compatibilityFixtures[]` 记录刻意构造的 compatibility fixture；当前基线为 0，`facade-smoke.ps1` 已改为直接检查 façade source invariant 与默认 Go delegation / no-fallback 行为，不再复制隔离 `lib/Manifest.ps1` sentinel。
- `inventoryGuards[]`、`documentationReferences[]` 与 `historicalReferences[]` 分别记录 Go release inventory / invariant guard、当前说明文档和历史迁移记录中的引用，避免删除批次误把历史说明当成 runtime dependency。
- `removalBlockers[]` 和 `unclassifiedReferences[]` 必须为空；若出现新的 `.ps1` / `.go` 主动依赖或无法分类的 `rekit/lib/*.ps1` 引用，`powerShellDeprecation.ready=false`，删除/归档批次必须先分类或消除该引用。

该库存只分类引用面；Batch 240 后 `rekit/lib/*.ps1` 已删除，历史文档引用不视为 active runtime dependency。若未来重新出现 active test dependency、compatibility fixture 或新的 `rekit/lib/*.ps1` 文件，应先迁移到 Go-native/package-level source invariant coverage，或显式作为 regression/blocker 分类。

## Fallback retirement inventory

`release-check -Format json` 的 `powerShellDeprecation.fallbackRetirement` 是后续 removal batch 的确定性前置库存：

- `goDefaultCommands[]` 来自 `rekit/rekit.ps1` 的默认 Go delegation 集合，用于确认 façade 默认路径已经由 Go owner 覆盖。
- `noFallbackCommands[]` 表示已经没有 PowerShell fallback 的 Go-default 命令；当前基线包含 `release-check`、`status`、`packs`、`doctor`、`validate`、`attach`、`repair`、`init`、`bootstrap`、`sync`、`update`、`promote`、`overview`、`note`、`gate`、`start`、`handoff`、`continue` 与 `plan-subagents`。
- `candidateCommands[]` 表示 Go-default 但仍有 legacy / fallback / removal-candidate 语义的命令行，是后续 fallback removal batch 的候选工作清单；Batch 232 起 `start` / `handoff` / `continue` 的 structured 与文本/default invocation 均不再 fallback，因此当前 candidate 清单为空。
- `blockedCommands[]` 保留 actual heavy-tool、authority/confirmed 等不得普通迁移的 command rows；这些 row 不应进入自动 removal batch。
- `removalCandidateModules[]` 来自模块状态矩阵中仍存在且仍标记为 removal-candidate 的 `.ps1` 文件；Batch 240 后当前基线为 0。
- `retiredModules[]` 来自模块状态矩阵中已标记 retired / removed 且磁盘上不存在的 legacy runtime modules；Batch 240 后当前基线为 13。

该库存只分类和报警；当前已退休的 no-fallback 命令即使设置 `REKIT_GO_DISABLE=1` 也不会回落到 PowerShell 业务实现；`candidateCommands[]` 和 `removalCandidateModules[]` 为空表示 Go-default command rows 与 legacy `rekit/lib/*.ps1` modules 已无普通 fallback/removal 候选，不表示 blocked heavy-tool、authority/confirmed、policy schema 迁移或公共 façade 删除可以顺手迁入。

## Freeze / deprecation gates

1. **Documented**：本文件列出命令和模块状态。
2. **Go-owned**：Go package tests 和 CLI tests 覆盖 deterministic 行为。
3. **Façade default**：公共 `/rekit` 默认委托 Go；已 no-fallback 的 Go-default commands 即使设置 `REKIT_GO_DISABLE=1` 也直接失败，且 `rekit.ps1` 不再加载 legacy runtime modules 或进入 command switch fallback dispatcher。
4. **Release inventory**：`release-check` 的 `powerShellDeprecation`、`caseShim` 与 `publicDefaultDocs` inventory 必须 `ready=true`，并能解析命令归属、模块状态、freeze gates、blocked migrations、默认委托、actual Go-default fallback retirement 分类、公共 façade runtime dependency / legacy dispatcher 状态、PowerShell module removal readiness、PowerShell module reference 分类、实际 `.ps1` 模块清单、case-local thin shim 是否保持无 PowerShell / raw CLI 默认入口，以及 README / `/rekit` skill / CLAUDE / Mission Control product direction / autonomous goal / release readiness / Go-first plan / Go runtime migration / deprecation roadmap / vision / reference absorption / Agent Team rollout / tests guide 是否继续把 Mission Control、`/rekit` 和 Go-native backend 作为默认公开路径。
5. **Release invariant**：Go release invariant 锁定 checklist、边界、known gaps、deprecation 状态或 façade freeze guard；Batch 131 已新增 `TestPowerShellFacadeFreezeInvariants` 锁定默认 Go 委托集合、`release-check` Go-only guard和 blocked heavy-tool/authority/confirmed 不进入默认委托；Batch 190 将 `plan-subagents` review artifacts 纳入默认 Go façade 并继续锁定不自动 spawn agent / 不执行 heavy-tool 边界。
6. **Legacy freeze**：PowerShell 只允许 bug fix / compatibility / safety boundary 修复。
7. **Fallback retirement candidate**：至少一个 release cycle 无 fallback 需求，且旧 case smoke、doctor、facade smoke 或对应 Go-native parity test 通过。
8. **Removal batch**：删除必须是单独批次，含恢复计划、diff review、CHANGELOG、docs、tests 和当前阶段授权范围说明。
9. **PowerShell-free default path**：README、CLAUDE、skill、case shim、release-check 和 CI 默认路径不再要求 PowerShell。
10. **Cross-platform verification**：Go-native default path 可在 Windows/macOS/Linux 语义下运行或被测试覆盖。

## 禁止迁移清单

以下内容不得作为普通 Go-first / PowerShell-free 收口批次顺手迁移：

- actual full-trace/debug/inject/patch/dump/network/symex/heavy-tool 执行。
- authority/confirmed 自动写入。
- policy schema 迁移或历史 case state 自动改写。
- 外部服务发布、真实扫描、fuzz、exploit replay、设备连接、hook。
- 将 runtime 逻辑复制进 case-local shim。
