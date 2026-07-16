# Release readiness checklist

## 读取指南

本文件是发布前和新会话接手时的一页门禁说明，配合 `docs/mission-control-product-direction.md`、`docs/autonomous-goal.md`、`docs/go-first-convergence-plan.md` 与 `docs/powershell-deprecation.md` 使用。维护者在准备 release、合并大型批次、或需要判断当前 runtime owner 时先读本文件顶部区域；需要产品北极星时读 `docs/mission-control-product-direction.md`，需要长期自主推进 goal 时读 `docs/autonomous-goal.md`，需要历史路线时再读 go-first convergence、runtime migration 和 batch-plan。

本文件不替代 `rekit/tests/README.md` 的 smoke 选择指南，也不是要求每次改动都运行全量 PowerShell matrix。它定义轻量、可重复的 release gate 和当前 known gaps。

## 实施摘要

当前 release readiness 状态：

- Go backend 已是多数确定性 runtime 路径的 owner：`status`、`packs`、`doctor/validate`、case lifecycle、sync/promote、overview、note、gate、start/handoff、continue preview/apply safe subset，以及 `plan-subagents` review artifacts。
- PowerShell `rekit/rekit.ps1` 仍是迁移期公共 legacy façade：负责参数兼容、Go binary 查找、少量 parity smoke 和 retired fallback error；`release-check`、`status`、`packs`、`doctor`、`validate`、`attach`、`repair`、`init`、`bootstrap`、`sync`、`update`、`promote`、`overview`、`note`、`gate`、`start`、`handoff`、`continue` 与 `plan-subagents` 已进入 no-fallback Go-owned 路径，Batch 234 后 `rekit.ps1` 不再 dot-source legacy runtime modules 或保留 command switch fallback dispatcher，Batch 240 后 `rekit/lib/*.ps1` legacy runtime modules 已物理删除。当前阶段默认向 PowerShell-free / Go-native / 跨平台路径收敛，后续不应新增 PowerShell runtime logic。
- Agent Team dry-run 已从 `_template` package E2E 扩展到 `generic-binary-re`、`web-security` package E2E，并新增 `web-security`、`generic-binary-re`、`malware-analysis`、`vuln-research`、`ctf`、`unpack-pe`、`ollvm`、`android-native` 真实临时 case smoke。
- 发布门禁默认依赖 Go-owned `release-check` inventory、Go-native `status` / `packs` / `doctor`、Go tests 与 `go vet`；Batch 139 已让 `release-check` 输出 `gateProfile`，将 recommended minimum 解析为本机/CI 可消费的 step kind、repo-local path、present/resolved 状态；Batch 140 已新增 `.github/workflows/release-gate.yml`，Batch 217 将默认 CI / recommended minimum 收敛为 Linux、Windows、macOS 三平台 Go-native release checks，不再把 PowerShell façade smoke 或 `rekit.ps1 doctor` 放在默认 release gate 内；Batch 142 已让 `release-check` 输出 `powerShellDeprecation` inventory，解析 PowerShell 命令归属、模块状态、freeze gates 与 blocked migrations，作为 PowerShell 收缩的确定性前置检查；Batch 144 已让 `release-check` 输出 `ciReleaseGate` inventory，对照 GitHub Actions workflow 的 job、required commands 与 forbidden broad/heavy steps 发现漂移；Batch 146 已让 `release-check` 输出 `releaseHandoff`，把新会话 read-first 文档、关键 readiness signals、latest batch 摘要、验证命令和下一步接手动作放进同一个 Go-owned envelope；Batch 147 已让 `releaseHandoff.releaseNotes` 对照最新 batch 与 CHANGELOG `Unreleased`，防止完成批次但漏写 release note；Batch 148 已让 `releaseHandoff.knownGaps[]` 汇总 `docs/release-readiness.md` Known gaps 的 category/summary，便于新会话先看机器可读缺口再按需读长文档；Batch 149 已让 `releaseHandoff.packMaturity` 汇总 pack maturity、schema validity 与每 pack heavy-tool gate readiness，Batch 162 已进一步输出 pack manifest `schemaVersion` 与 `schemaVersionReady`，避免接手时必须遍历完整 `packs[]` 或 manifest 才能判断 pack 覆盖和 schema contract 版本状态。
- Mission Control 产品北极星已集中到 `docs/mission-control-product-direction.md`：后续 release readiness 不应偏回“命令大全”、固定旧会话、短命 subagent-only 或用户盯多个会话的路线。
- 长期自主推进和新会话接手 guidance 已集中到 `docs/autonomous-goal.md`；它保留 Mission Control 大方向、自主推进循环和可复制 goal 语句，并把当前阶段重点更新为 PowerShell-free / Go-native / 跨平台收敛，避免 release readiness 批次退回微批次或 PowerShell smoke/catalog 扩张，也避免用过长约束束缚模型发挥。

## 执行清单

### CI release gate

`.github/workflows/release-gate.yml` 当前是默认轻量 CI 门禁：

- `Go release checks (Linux)` 在 `ubuntu-latest` 上运行 `release-check -Format json`、`status`、`packs`、`doctor`、`go test ./...` 与 `go vet ./...`。
- `Go release checks (Windows)` 在 `windows-latest` 上运行同一组 Go-native checks，验证 Windows 默认路径不依赖 PowerShell façade。
- `Go release checks (macOS)` 在 `macos-latest` 上运行同一组 Go-native checks，验证 macOS 默认路径。
- CI 不默认运行 `rekit.ps1`、`facade-smoke.ps1`、`pack-smoke-matrix.ps1`、真实临时 case Agent Team smoke 或 heavy-tool gate；这些仍按下面“选择性追加”规则手动选择。

### 本机 release gate（推荐最小集）

```text
go run ./cmd/rekit -- -Command release-check -Format json
go run ./cmd/rekit -- -Command status
go run ./cmd/rekit -- -Command packs
go run ./cmd/rekit -- -Command doctor
go test ./...
go vet ./...
git diff --check
```

`release-check -Format json` 会同时输出 `gateProfile.name=local-ci-minimum`、`defaultFor=[local,ci]`、`stepCount`、`largeMatrixDefault=false` 和每个 step 的 `kind` / `repoPath` / `present` / `resolved`，供本机脚本或 CI 在真正执行命令前先做确定性 inventory 检查；同时输出 `ciReleaseGate.ready`、`workflowPath`、`jobs[]`、`requiredCommands[]`、`forbiddenStrings[]`，验证 `.github/workflows/release-gate.yml` 仍只包含 Linux/Windows/macOS Go-native release checks，未漂移回 `rekit.ps1`、façade smoke、大型 matrix、真实临时 case smoke 或 heavy-tool 相关步骤；同时输出 `powerShellDeprecation.ready`、`commandOwnership[]`、`moduleStatus[]`、`freezeGates[]`、`blockedMigrations[]`、`fallbackRetirement`、`publicFacade` 与 `heavyToolGateActions[]`，在 PowerShell façade / fallback 收缩后验证 deprecation 文档、实际 façade 默认委托、公共 façade retained/no-fallback 命令面、legacy `rekit/lib/*.ps1` retired/removed 基线和回归防护没有漂移，也能看到 Go-default commands、no-fallback commands、fallback retirement candidate commands、blocked commands、removal-candidate / retired modules 和当前 pack manifest 声明的 heavy-tool gate action 集合；同时输出 `caseShim.ready`、`requiredPhrases[]`、`canonicalSkillPhrases[]` 与 `forbiddenStrings[]`，确保 case-local thin shim 只跳转到 canonical skill / Go-native backend，不把 PowerShell façade、raw Go CLI 命令或 fallback 开关变回默认入口；同时输出 `publicDefaultDocs.ready`、`documents[]`、`requiredPhrases[]`、`forbiddenCommands[]` 与 `forbiddenShellFences[]`，确保 README、canonical `/rekit` skill、项目 CLAUDE、Mission Control 产品方向、autonomous goal、本 release readiness checklist、Go-first convergence plan、Go runtime migration plan、PowerShell deprecation roadmap、vision、reference absorption、Agent Team rollout plan 与 tests guide 继续把 Mission Control / `/rekit` / Go-native backend 作为默认公开路径，不把 `rekit.ps1` 命令片段或 PowerShell shell fence 重新展示成用户默认入口；同时输出 `releaseHandoff.ready`、`readFirst[]`、`signals[]`、`latestBatch`、`releaseNotes.covered`、`knownGaps[]`、`packMaturity`、`validation[]` 与 `nextActions[]`，供新会话和 release maintainer 不读完整长文档也能先定位当前状态、必读入口、最新批次、release note freshness、当前 known gaps、pack schema version readiness 和最小验证。

机器可读 smoke catalog 的 `recommendedMinimum` 当前为：

```text
go run ./cmd/rekit -- -Command release-check -Format json
go run ./cmd/rekit -- -Command status
go run ./cmd/rekit -- -Command packs
go run ./cmd/rekit -- -Command doctor
go test ./...
go vet ./...
git diff --check
```

选择性追加：

- 改 catalog 或 smoke metadata：运行 `catalog-smoke.ps1`、`pack-smoke-matrix-selftest.ps1`、`pack-smoke-matrix.ps1 -DiscoveryOnly` 与 `pack-inventory-smoke.ps1`。
- 改 façade 委托：追加 `facade-smoke.ps1`。
- 改某个 pack skeleton：运行对应 `*-pack-smoke.ps1`；跨 pack helper 改动才运行 matrix 子集或 discovery。
- 改 sync/promote 写入：运行对应 preflight/apply smoke，并确认 backup、deny、restore 与 pack-root containment。
- 改 Agent Team / workstream / ledger / gate：运行对应临时 case smoke，例如 `agent-team-review-loop-smoke.ps1`、`gate-parity-smoke.ps1`、`web-security-agent-team-dryrun-smoke.ps1`、`vuln-research-agent-team-dryrun-smoke.ps1`、`unpack-pe-agent-team-dryrun-smoke.ps1`、`ollvm-agent-team-dryrun-smoke.ps1`、`android-native-agent-team-dryrun-smoke.ps1`。

### 发布前人工检查

1. `CHANGELOG.md` 顶部 `Unreleased` 已描述本轮用户可见变化、边界和验证结果。
2. `docs/batch-plan.md` 最新 batch 状态为已完成，并列出实际执行过的验证。
3. `docs/go-first-convergence-plan.md` 未把阶段性进展误写成全局完成；Stage 1-8 未完成项仍保留。
4. `docs/mission-control-product-direction.md` 的产品北极星与 README、CLAUDE、vision、design 一致；`docs/autonomous-goal.md` 的长期 goal、停止条件和中大型 vertical slice guidance 与本文件、go-first convergence 计划一致。
5. `rekit/tests/catalog.json` 与 `rekit/tests/README.md` 一致；新增 `*.ps1` 已被 catalog 收录。
6. `git status --short` 干净后再打 tag 或发 release。

## 验证标准

Release gate 通过的最低标准：

- `go run ./cmd/rekit -- -Command release-check -Format json` 输出 `ready=true`，且 `gateProfile.ready=true`、`ciReleaseGate.ready=true`、`powerShellDeprecation.ready=true`、`powerShellDeprecation.fallbackRetirement.ready=true`、`powerShellDeprecation.fallbackRetirement.noFallbackCommands[]` 非空、`powerShellDeprecation.publicFacade.ready=true`、`powerShellDeprecation.publicFacade.retained=true`、`goNativePublicSurface.ready=true`、`goNativePublicSurface.entrypointPresent=true`、`goNativePublicSurface.commandCatalogPresent=true`、`goNativePublicSurface.commands[]`、`goNativePublicSurface.handlerCommands[]`、`goNativePublicSurface.symbolCommands{}` 与 `goNativePublicSurface.commandProfiles[]` 均覆盖 19 个 public commands，`goNativePublicSurface.mutationBoundaries[]` 覆盖 7 个 public mutation/read-only boundary，`goNativePublicSurface.commandProfileSummary` 锁定 `total=19`、`readOnly=5`、`mutating=14`、`writesCase=13`、`writesKit=1`、`reviewFirst=3`、`applyRequired=11`、`heavyTool=0` 与 `authorityConfirmed=0`，`goNativePublicSurface.commandProfileGroups` 同步暴露并校验 read-only、mutating、case write、kit write、review-first、apply-required、heavy-tool、authority/confirmed 与 per-boundary command sets，`goNativePublicSurface.commandProfileBoundaries[]` 覆盖 7 个 mutation/read-only boundary rows 并与 summary / groups 一致，`goNativePublicSurface.commandProfilePolicies[]` 覆盖 5 个 public profile guardrail rows 且 violation count 为 0，`goNativePublicSurface.facadeRemovalReady=true`，`goNativePublicSurface.facadeRemovalPrerequisites[]` 覆盖 5 个 façade removal prerequisite rows 且全部 ready，`publicFacadeRemoval.ready=true`，`publicFacadeRemoval.prerequisites[]` 覆盖 8 个跨 inventory façade removal prerequisite rows 且全部 ready，`publicFacadeRemoval.removalPlan.ready=true` 且覆盖后续 removal batch 的替代入口、恢复计划、验证命令、文档同步、CHANGELOG 与 no heavy-tool/authority 边界，`publicFacadeRemoval.removalPlan.recoverySteps[]` 覆盖恢复公共 façade、同步文档/release notes 与重新运行 Go-native release gate 的 required steps，`publicFacadeRemoval.removalPlan.documentationTargets[]` 覆盖 README、CLAUDE、canonical `/rekit` skill、case shim、release readiness、PowerShell deprecation、Go-first plan、batch-plan 与 CHANGELOG 且 `documentationValidationCommands=72`，`publicFacadeRemoval.removalImpact.ready=true` 且 `unclassifiedReferences[]` 为空、reference categories 覆盖 façade entrypoint、compatibility smoke、facade-dependent smoke、pack wrapper、public docs、roadmap/history docs 与 release inventory/test 影响面，`publicFacadeRemoval.removalImpact.workItems[]` 覆盖每个 reference category 且 action / `validationCommands[]` 非空，`publicFacadeRemoval.removalImpact.migrationTargets[]` 覆盖全部 74 个 public façade references 且 `migrationValidationCommands=592`、全部 target 为 required / Go-native preferred 并对 roadmap/history docs 保留 historical context，`publicFacadeRemoval.removalImpact.smokeMigrationTargets[]` 覆盖 façade compatibility smoke 与 façade-dependent smoke 且 `smokeMigrationValidationCommands=232`，`goNativePublicSurface.alternativePattern` 与 public façade 替代路径一致、`candidateCommands[]` 允许为空但必须与 removal-candidate modules / blocked commands 一起形成完整库存、`caseShim.ready=true`、`publicDefaultDocs.ready=true`、`publicDefaultDocs.forbiddenShellFences[]` 为空、`releaseHandoff.ready=true`、`releaseHandoff.releaseNotes.covered=true`、`releaseHandoff.knownGaps[]` 非空、`releaseHandoff.packMaturity.total` 覆盖所有 pack、`releaseHandoff.packMaturity.schemaVersionReady=true`、`releaseHandoff.packMaturity.heavyToolGateReady=true`、`heavyToolGateActions[]` 非空，required commands、必备文档、pack schema、CI release gate inventory、PowerShell deprecation inventory、fallback retirement inventory、Go-native public surface inventory、case shim readiness、public default docs readiness、release handoff summary、release notes freshness、manifest heavy-tool gate action、边界与 known gaps 清单完整；若 inventory `ready=false`，命令必须仍输出完整 JSON/text 诊断并以非零退出，避免 CI 只看日志不失败。
- `go test ./...` 通过，包含 `internal/rekit/manifest` release invariants。
- `go vet ./...` 无输出或无错误退出。
- `go run ./cmd/rekit -- -Command status`、`packs` 与 `doctor` 通过，确认默认本机验证路径不依赖 PowerShell。
- `rekit.ps1` doctor compatibility check 与 `facade-smoke.ps1` 不属于默认 release gate；只有改迁移期 façade、fallback 或 Windows compatibility 时按需追加。
- `git diff --check` 没有 whitespace error；Windows LF/CRLF warning 可记录。
- 涉及临时 case 的 smoke 必须清理自身 case root 和 review artifact。

## 风险与注意事项

- 不默认运行 PowerShell façade smoke 或大型 PowerShell matrix；只有改 façade/fallback、pack helper、matrix 输出或跨 pack skeleton 时才运行对应 PowerShell compatibility smoke。
- 不把真实样本、trace、dump、capture、pcap、crash、payload、客户信息、flag、IOC、绝对 case 路径或 case-specific 进度写入本仓库。
- release gate / CI / smoke 不执行真实网络请求、扫描、fuzz、exploit replay、debug、dump、patch、hook、设备连接或其它外部副作用；真实 case 的成员 lane 只有在 lane 文档/packet/autonomy profile 预授权范围内才可自主执行这些动作。
- `gate -WhatIf` 只预览 pending-gate request；`gate -Apply` 只写 pending-gate request，不执行 heavy-tool；实际 heavy action 由 lane executor / tool adapter 在授权 profile 内执行并写回 evidence/ledger。
- `continue -Apply` 只写 case-local facts/routing/run digest/lane resume/checkpoint/board；不写 authority/confirmed。
- `sync/promote` 保持 review-first；实际写入前必须确认范围，pack source 写入依赖 backup/deny/restore。
- 普通 Go-default command fallback candidates 已清空；`release-check`、`status`、`packs`、`doctor`、`validate`、`attach`、`repair`、`init`、`bootstrap`、`sync`、`update`、`promote`、`overview`、`note`、`gate`、`start`、`handoff`、`continue` 与 `plan-subagents` 的 fallback 已退休；`REKIT_GO_DISABLE=1` 只会触发 no-fallback guard，不再回落到这些 PowerShell 业务实现。删除或冻结剩余 PowerShell runtime modules 前必须有单独 deprecation batch、兼容验证和文档说明。

## 当前 pack maturity matrix

| pack | maturity | authority lane | release 备注 |
|---|---|---|---|
| `_template` | template | main | pack authoring template，不作为 skeleton matrix 成员。 |
| `android-native` | skeleton | main | 已有真实临时 case dry-run；不连接设备、不 attach Frida、不执行 hook/inject。 |
| `ctf` | skeleton | main | 已有真实临时 case dry-run；不远程连接、不 brute force、不写 flag/authority/confirmed。 |
| `generic-binary-re` | skeleton | main | 已有 package E2E 与真实临时 case dry-run；不执行样本/debug/trace/dump/patch。 |
| `malware-analysis` | skeleton | main | 已有真实临时 case dry-run；不执行样本、不联网、不 sandbox detonation。 |
| `ollvm` | skeleton | main | 已有真实临时 case dry-run；不执行样本、不 trace/dump/patch、不写 deobfuscated binary。 |
| `unpack-pe` | skeleton | main | 已有真实临时 case dry-run；不执行样本、不 debug/dump/patch、不写 unpacked binary。 |
| `vmp-re` | mature | devirt-main | 首个 mature pack；仍需 heavy-tool/authority/confirmed 人工 gate。 |
| `vuln-research` | skeleton | main | 已有真实临时 case dry-run；不连接 live target、不主动扫描、不 fuzz、不 replay exploit。 |
| `web-security` | skeleton | main | 已有 package E2E 与真实临时 case dry-run；不发真实网络请求。 |

## Go-owned 与 PowerShell legacy 状态

Go-owned / Go-default 路径：

- `release-check` release gate inventory 与本机/CI `gateProfile` 解析。
- `status`、`packs`、`doctor/validate`。
- attached case 的 `overview` 文本/JSON 与缺 board 初始化，PowerShell fallback 已退休。
- `note -List` 文本/table/tsv/JSON、`note` append、`note -WhatIf`，PowerShell fallback 已退休。
- `gate -WhatIf`、`gate -Apply` pending-gate request：只预览或写 pending-gate request，PowerShell fallback 已退休。
- `start` / `handoff` 的 JSON preview、explicit apply、文本 preview 与 bare/default 工作线 flow；façade 非 `-Apply` 且空 `-Format` 会显式转为 Go text output，direct Go CLI 继续保持默认 JSON contract，Go disabled/unavailable 时直接 no-fallback。
- `continue -WhatIf` 与 explicit / bare text `continue` safe subset；JSON preview、explicit apply 和文本/default preview 均由 Go-owned 路径接管，Go disabled/unavailable 时直接 no-fallback，`continue -Apply` 不写 authority/confirmed。
- case lifecycle `attach`、`repair`、`init/bootstrap` preview/apply，PowerShell fallback 已退休。
- `sync` / `update` review/apply/JSON preview，PowerShell fallback 已退休。
- `promote` review/artifacts/candidates/apply/JSON preview。
- `plan-subagents` review artifact 写入：只生成 packet / summary / combined diff 路径，不自动 spawn reviewer，PowerShell fallback 已退休。

PowerShell legacy / fallback 路径（详细冻结/删除策略见 `docs/powershell-deprecation.md`）：

- 非 note/gate/continue apply 的其它 ledger 写入命令。
- 普通 Go-default command fallback candidates 已清空；`release-check`、`status`、`packs`、`doctor`、`validate`、`attach`、`repair`、`init`、`bootstrap`、`sync`、`update`、`promote`、`overview`、`note`、`gate`、`start`、`handoff`、`continue` 与 `plan-subagents` 均已 no-fallback；`powerShellDeprecation.facadeRuntime` 同步锁定公共 façade 不再 dot-source legacy modules 或包含 command switch fallback dispatcher；`powerShellDeprecation.moduleRemoval` 同步锁定 legacy runtime modules 已删除（`candidates=0 retired=13`）、façade dependency 与 undocumented module 状态；`powerShellDeprecation.moduleReferences` 同步分类 active test dependency、compatibility fixture、inventory guard、文档/历史引用，并要求 removal blockers 与 unclassified references 为 0。
- 少量 Windows façade parity smoke，默认 release gate 已改为三平台 Go-native checks；只有改 façade/fallback 时按需运行。

## Known gaps

- bounded dispatch 仍不自动 spawn reviewer；runtime 只生成 review packet 和 observability。
- actual heavy-tool 执行未迁入 deterministic runtime；full-trace/debug/inject/patch/dump/network/symex 仍必须显式 gate，且 `gate` 只接受 pack manifest `heavyToolGates` 声明的 action。
- authority/confirmed 写入仍需人工确认，不由 Go `continue -Apply` 自动执行。
- policy schema 迁移与公共 PowerShell façade 删除仍未进入默认发布路径；PowerShell-free / Go-native 收敛已删除 legacy runtime modules，但仍保留迁移期 façade 与 blocked migration 边界；Batch 129 已新增 `docs/powershell-deprecation.md` 作为策略入口，Batch 130 已新增 Go-owned `release-check` inventory 作为本机/CI release gate 前置检查，Batch 131 已新增 façade freeze invariant 防止默认委托集合和 blocked 边界漂移，Batch 139 已新增 `release-check` gate profile 以便 CI/本机在执行前解析 recommended minimum，Batch 140 已新增轻量 CI workflow，Batch 142 已新增 `powerShellDeprecation` inventory 用于在删除前发现命令归属/模块清单漂移，Batch 144 已新增 `ciReleaseGate` inventory 用于发现 GitHub Actions release gate job/command/forbidden step 漂移，Batch 216 已把接手 goal 与 roadmap 调整为默认路径逐步不依赖 PowerShell、面向 macOS/Linux/Windows 的 Go-native convergence，Batch 217 已将默认 recommended minimum 与 CI release gate 收敛为 Linux/Windows/macOS 三平台 Go-native checks，Batch 223 已新增 `powerShellDeprecation.fallbackRetirement` 子库存用于区分 no-fallback、fallback removal candidate、blocked command 与 removal-candidate module，Batch 224 已退休 `status` / `packs` / `doctor` / `validate` read-only PowerShell fallback，使 no-fallback baseline 扩展到 5 个命令且剩余 candidate fallback 收窄到 9 个 command rows；Batch 225 已退休 `plan-subagents` PowerShell fallback，使 no-fallback baseline 扩展到 6 个命令且剩余 candidate fallback 收窄到 8 个 command rows；Batch 226 已退休 `gate -WhatIf` / `gate -Apply` pending-gate PowerShell fallback，使 no-fallback baseline 扩展到 7 个命令且剩余 candidate fallback 收窄到 7 个 command rows；Batch 227 已退休 `overview` 与 `note` PowerShell fallback，使 no-fallback baseline 扩展到 9 个命令且剩余 candidate fallback 收窄到 5 个 command rows；Batch 228 已退休 `sync` / `update` PowerShell fallback，使 no-fallback baseline 扩展到 11 个命令且剩余 candidate fallback 收窄到 4 个 command rows；Batch 229 退休 `promote` PowerShell fallback，使 no-fallback baseline 扩展到 12 个命令且剩余 candidate fallback 收窄到 3 个 command rows；Batch 230 退休 case lifecycle `attach` / `repair` / `init` / `bootstrap` PowerShell fallback，使 no-fallback baseline 扩展到 16 个命令且剩余 candidate fallback 收窄到 2 个 command rows；Batch 231 退休 `start` / `handoff` / `continue` 已 Go-owned structured invocation 的 disabled/unavailable fallback，保留无 `-Apply` 文本工作线 compatibility，因此 release inventory 仍为 noFallback=16/candidates=2；Batch 232 将这些文本/default 工作线 flow 也交给 Go text output 并退休 whole-command fallback，使 release inventory 变为 noFallback=19/candidates=0；Batch 233 将 legacy PowerShell runtime module dot-source 移到 Go delegation 与 no-fallback guard 之后，使 default/no-fallback façade path 不再预加载 retired modules；Batch 234 删除 `rekit.ps1` 中已不可达的 legacy dot-source block 与 command switch fallback dispatcher，使公共 façade 只保留 Go delegation、no-fallback guard 和 retired dispatcher error；Batch 235 将该 façade runtime dependency 状态纳入 `powerShellDeprecation.facadeRuntime`、release-check text output 与 release handoff signal；Batch 236 将 removal-candidate PowerShell module 清单、公共 façade 依赖和未登记模块状态纳入 `powerShellDeprecation.moduleRemoval`、release-check text output 与 release handoff signal；Batch 237 将 PowerShell module 引用面纳入 `powerShellDeprecation.moduleReferences`，区分 active test dependencies、compatibility fixtures、inventory guards、文档/历史引用、removal blockers 与 unclassified references；Batch 238 已把 `continue-preflight-smoke.ps1` 收敛为 Go-owned package-test wrapper；Batch 239 已移除 `facade-smoke.ps1` 的隔离 legacy runtime sentinel fixture，使 module reference 基线变为 `activeTests=0 fixtures=0 blockers=0 unclassified=0`；Batch 240 已物理删除 `rekit/lib/*.ps1` legacy runtime modules，并将 module removal 基线变为 `candidates=0 retired=13 facadeDeps=0 undocumented=0`；Batch 241 已新增 `powerShellDeprecation.publicFacade`，锁定公共 façade retained、19 个 façade command 均为 Go-default/no-fallback、以及未来删除公共 `rekit/rekit.ps1` 必须作为独立 removal batch；Batch 242 已新增 `goNativePublicSurface` 顶层库存和 `internal/rekit/commands` public command catalog，锁定 `cmd/rekit` entrypoint、19 个 Go-native public commands、unsupported command diagnostic 与 public façade command surface 双向一致；Batch 243 继续新增 `handlerCommands[]` 与 `symbolCommands{}` coverage gate，锁定 Go CLI dispatcher handlers、public command symbols 与 catalog 同步覆盖 19 个 public commands；Batch 244 继续新增 `commandProfiles[]` 与 `mutationBoundaries[]`，把 read-only、case-local apply/append/bootstrap、review artifact、review-first 与 kit review-first 边界纳入同一 Go-native public surface inventory，并拒绝 public command profile 标记 actual heavy-tool 或 authority/confirmed 写入；PowerShell façade smoke 转为按需 compatibility 验证。
- 目前安全领域 skeleton pack 均已有真实临时 case dry-run；后续缺口转向 release readiness/CI 门禁、policy schema migration 和 PowerShell runtime deprecation。
