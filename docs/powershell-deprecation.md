# PowerShell-free Go-native convergence roadmap

## 读取指南

本文件定义 `rekit` PowerShell 层的冻结、替换、迁移与删除策略，配合 `docs/autonomous-goal.md`、`docs/release-readiness.md` 和 `docs/go-first-convergence-plan.md` 使用。维护者在修改 `rekit/rekit.ps1`、`rekit/lib/*.ps1`、Go façade 委托集合、fallback 逻辑、旧文本 flow、默认验证路径或跨平台入口前应先读本文件顶部区域。

当前阶段目标已经从单纯 **PowerShell runtime deprecation strategy** 升级为 **PowerShell-free / Go-native / 跨平台 convergence**：Go CLI/backend 是 canonical runtime；PowerShell 只作为迁移期 legacy façade / fallback / parity residue。默认入口、默认验证、release gate、case shim 和文档路径应逐步不依赖 PowerShell。删除 PowerShell 代码不再因为“删除”本身停下询问，但必须有 Go-native 替代、文档、测试和恢复边界；删除公共入口且没有替代路径时仍需升级。

## 实施摘要

当前策略：

- **Go-owned**：确定性状态、结构化输出、低风险写入、`release-check` inventory、release invariant、pack-neutral contract 和跨平台路径优先由 `cmd/rekit/**` 与 `internal/rekit/**` 维护。
- **PowerShell façade**：`rekit/rekit.ps1` 只是迁移期公共兼容入口，负责 Go binary 查找、参数兼容、fallback、环境变量开关和旧 case 用户体验；不承载新的业务语义。
- **Legacy-only**：无 `-Apply` 的文本工作线 flow、文本 sync/promote what-if、内部命令和非 Go-owned 写入路径暂时保留，禁止扩展新能力。
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
| case lifecycle `attach` / `repair` / `init` / `bootstrap` preview/apply | Go default | façade + fallback | 保留 `REKIT_GO_DISABLE=1` fallback 到旧 case smoke 和 Go-native case shim 验证完成；随后按 removal batch 删除。 |
| `sync` / `update` review/apply/JSON preview | Go default | façade delegate + no PowerShell fallback | `update` 作为 sync alias 同属 Go-default façade；PowerShell fallback 已退休，review、JSON preview 与显式 apply 由 Go 维护，继续保持 review-first 与 explicit apply 边界。 |
| `promote` review/artifacts/candidates/apply/JSON preview | Go default | façade delegate + no PowerShell fallback | promote review、review artifacts、candidate/apply、JSON preview 与文本 what-if 均由 Go-owned/no-fallback 边界维护；pack source 写入仍要求 review-first/backup/deny/restore。 |
| `overview` text/JSON 与缺 board 初始化 | Go default | façade delegate + no PowerShell fallback | Go overview 是 canonical owner；PowerShell fallback 已退休，缺 board 初始化仍只写 case-local scaffold。 |
| `note -List` text/table/tsv/JSON、`note` append、`note -WhatIf` | Go default | façade delegate + no PowerShell fallback | 新 ledger schema 校验由 Go 维护；PowerShell fallback 已退休，append 仍只写 facts JSONL、不写 authority/confirmed。 |
| `gate -WhatIf` / `gate -Apply` pending-gate | Go default | façade delegate + no PowerShell fallback | 只预览或写 pending-gate request；不执行 heavy-tool；PowerShell fallback 已退休。 |
| `start` / `handoff` JSON preview/apply | Go default | façade + text fallback | 文本 preview legacy-only；结构化语义以 Go 为准；Go-native default path 文档化后删除 fallback。 |
| `continue -WhatIf -Format json` / explicit `continue -Apply` | Go default safe subset | façade + fallback | `continue -Apply` 不写 authority/confirmed；text flow legacy-only，后续以 Go-native resume/handoff 取代。 |
| `plan-subagents` review artifacts | Go default | façade delegate + no PowerShell fallback | 只写 review packet / summary / combined diff 路径；不自动 spawn agent；PowerShell fallback 已退休，使用 Go package tests 与 `plan-subagents-smoke.ps1` 维护 parity。 |
| 无 `-Apply` 的文本工作线 flow | PowerShell legacy | legacy-only | 冻结语义；只修 bug，不新增状态模型；由 Go-native lane handoff/resume/continue 取代后删除。 |
| actual heavy-tool 执行 | 未迁移 | blocked / manual gate | 不自动迁移；需要用户确认和单独设计。 |
| authority/confirmed 写入 | 人工 gate / legacy guarded | blocked | 不由 Go apply 自动执行；删除/迁移需 schema 与恢复策略。 |

## PowerShell 模块状态

| 模块 | 状态 | 说明 |
|---|---|---|
| `rekit/rekit.ps1` | façade-stable / removal-candidate | 迁移期公共入口、参数兼容、Go binary 查找、fallback 和错误透传；不放业务语义；Go-native public entrypoint 文档化后删除或归档。 |
| `rekit/lib/Manifest.ps1` | compatibility / removal-candidate | manifest 读取/兼容 helper；Go manifest package 是 release invariant owner。 |
| `rekit/lib/Validate.ps1` | parity / compatibility / removal-candidate | pack/case doctor 兼容层；Go doctor 是默认路径。 |
| `rekit/lib/Instance.ps1` | compatibility / removal-candidate | case instance/shim 兼容；case lifecycle Go-owned。 |
| `rekit/lib/Sync.ps1` | retired fallback / removal-candidate | sync/update fallback 已退休；写入语义 Go-owned，模块仅作为后续文件删除/历史兼容审查对象。 |
| `rekit/lib/Promote.ps1` | retired fallback / removal-candidate | promote fallback 已退休；candidate/apply 语义 Go-owned，模块仅作为后续文件删除/历史兼容审查对象。 |
| `rekit/lib/Review.ps1` | compatibility / removal-candidate | review artifact helper；新增确定性 review 语义优先 Go。 |
| `rekit/lib/B3.Core.ps1` | shared compatibility / removal-candidate | PowerShell B3 基础 helper；只修兼容和安全边界。 |
| `rekit/lib/B3.State.ps1` | legacy / compatibility / removal-candidate | board/facts/lane 状态兼容；Go workstream 是新 owner。 |
| `rekit/lib/B3.Policy.ps1` | legacy / compatibility / removal-candidate | policy helper；schema 迁移需单独 gate。 |
| `rekit/lib/B3.Lane.ps1` | legacy text flow / removal-candidate | lane prompt/workspace helper；结构化 start/handoff/continue 以 Go 为准。 |
| `rekit/lib/B3.Auto.ps1` | legacy guarded / removal-candidate | continue auto / authority guard 旧实现；不扩 authority/confirmed 自动写入。 |
| `rekit/lib/B3.Handoff.ps1` | legacy display fallback / removal-candidate | handoff fallback 展示；Go handoff 是默认结构化 owner。 |
| `rekit/lib/B3.Commands.ps1` | legacy command layer / removal-candidate | 文本工作线和内部命令入口；冻结语义，避免新增 runtime owner。 |

## Fallback retirement inventory

`release-check -Format json` 的 `powerShellDeprecation.fallbackRetirement` 是后续 removal batch 的确定性前置库存：

- `goDefaultCommands[]` 来自 `rekit/rekit.ps1` 的默认 Go delegation 集合，用于确认 façade 默认路径已经由 Go owner 覆盖。
- `noFallbackCommands[]` 表示已经没有 PowerShell fallback 的 Go-default 命令；当前基线包含 `release-check`、`status`、`packs`、`doctor`、`validate`、`sync`、`update`、`promote`、`overview`、`note`、`gate` 与 `plan-subagents`。
- `candidateCommands[]` 表示 Go-default 但仍有 legacy / fallback / removal-candidate 语义的命令行，是后续 fallback removal batch 的候选工作清单。
- `blockedCommands[]` 保留 actual heavy-tool、authority/confirmed 等不得普通迁移的 command rows；这些 row 不应进入自动 removal batch。
- `removalCandidateModules[]` 来自模块状态矩阵中的 removal-candidate `.ps1` 文件，用于决定独立删除批次的 review 范围。

该库存只分类和报警；真正 fallback 退休或文件删除仍必须按单独 removal batch 执行，包含恢复计划、验证和文档。当前已退休的 no-fallback 命令即使设置 `REKIT_GO_DISABLE=1` 也不会回落到 PowerShell 业务实现；仍在 `candidateCommands[]` 中的命令保留迁移期 compatibility fallback。

## Freeze / deprecation gates

1. **Documented**：本文件列出命令和模块状态。
2. **Go-owned**：Go package tests 和 CLI tests 覆盖 deterministic 行为。
3. **Façade default**：公共 `/rekit` 默认委托 Go，并保留 `REKIT_GO_DISABLE=1` fallback 到对应 removal batch 明确删除。
4. **Release inventory**：`release-check` 的 `powerShellDeprecation`、`caseShim` 与 `publicDefaultDocs` inventory 必须 `ready=true`，并能解析命令归属、模块状态、freeze gates、blocked migrations、默认委托、actual Go-default fallback retirement 分类、实际 `.ps1` 模块清单、case-local thin shim 是否保持无 PowerShell / raw CLI 默认入口，以及 README / `/rekit` skill / CLAUDE / Mission Control product direction / autonomous goal / release readiness / Go-first plan / Go runtime migration / deprecation roadmap / vision / reference absorption / Agent Team rollout / tests guide 是否继续把 Mission Control、`/rekit` 和 Go-native backend 作为默认公开路径。
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
