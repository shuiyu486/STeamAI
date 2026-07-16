# Release readiness checklist

## 读取指南

本文件是发布前和新会话接手时的一页门禁说明，配合 `docs/mission-control-product-direction.md`、`docs/autonomous-goal.md`、`docs/go-first-convergence-plan.md` 与 `docs/powershell-deprecation.md` 使用。维护者在准备 release、合并大型批次、或需要判断当前 runtime owner 时先读本文件顶部区域；需要产品北极星时读 `docs/mission-control-product-direction.md`，需要长期自主推进 goal 时读 `docs/autonomous-goal.md`，需要历史路线时再读 go-first convergence、runtime migration 和 batch-plan。

本文件不替代 `rekit/tests/README.md` 的 smoke 选择指南，也不是要求每次改动都运行全量 PowerShell matrix。它定义轻量、可重复的 release gate 和当前 known gaps。

## 实施摘要

当前 release readiness 状态：

- Go backend 已是多数确定性 runtime 路径的 owner：`status`、`packs`、`doctor/validate`、case lifecycle、sync/promote、overview、note、gate、start/handoff、continue preview/apply safe subset，以及 `plan-subagents` review artifacts。
- PowerShell `rekit/rekit.ps1` 仍是迁移期公共 legacy façade：负责参数兼容、旧文本 flow、剩余 compatibility fallback、少量 parity smoke 和仍未退休命令的 `REKIT_GO_DISABLE=1` 回退；`release-check`、`status`、`packs`、`doctor` 与 `validate` 已进入 no-fallback Go-owned 路径。当前阶段默认向 PowerShell-free / Go-native / 跨平台路径收敛，后续不应新增 PowerShell runtime logic。
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

`release-check -Format json` 会同时输出 `gateProfile.name=local-ci-minimum`、`defaultFor=[local,ci]`、`stepCount`、`largeMatrixDefault=false` 和每个 step 的 `kind` / `repoPath` / `present` / `resolved`，供本机脚本或 CI 在真正执行命令前先做确定性 inventory 检查；同时输出 `ciReleaseGate.ready`、`workflowPath`、`jobs[]`、`requiredCommands[]`、`forbiddenStrings[]`，验证 `.github/workflows/release-gate.yml` 仍只包含 Linux/Windows/macOS Go-native release checks，未漂移回 `rekit.ps1`、façade smoke、大型 matrix、真实临时 case smoke 或 heavy-tool 相关步骤；同时输出 `powerShellDeprecation.ready`、`commandOwnership[]`、`moduleStatus[]`、`freezeGates[]`、`blockedMigrations[]`、`fallbackRetirement` 与 `heavyToolGateActions[]`，在 PowerShell façade / fallback 收缩前验证 deprecation 文档、实际 façade 默认委托和 `rekit/lib/*.ps1` 模块清单没有漂移，也能看到 Go-default commands、no-fallback commands、fallback retirement candidate commands、blocked commands、removal-candidate modules 和当前 pack manifest 声明的 heavy-tool gate action 集合；同时输出 `caseShim.ready`、`requiredPhrases[]`、`canonicalSkillPhrases[]` 与 `forbiddenStrings[]`，确保 case-local thin shim 只跳转到 canonical skill / Go-native backend，不把 PowerShell façade、raw Go CLI 命令或 fallback 开关变回默认入口；同时输出 `publicDefaultDocs.ready`、`documents[]`、`requiredPhrases[]`、`forbiddenCommands[]` 与 `forbiddenShellFences[]`，确保 README、canonical `/rekit` skill、项目 CLAUDE、Mission Control 产品方向、autonomous goal、本 release readiness checklist、Go-first convergence plan、Go runtime migration plan、PowerShell deprecation roadmap、vision、reference absorption、Agent Team rollout plan 与 tests guide 继续把 Mission Control / `/rekit` / Go-native backend 作为默认公开路径，不把 `rekit.ps1` 命令片段或 PowerShell shell fence 重新展示成用户默认入口；同时输出 `releaseHandoff.ready`、`readFirst[]`、`signals[]`、`latestBatch`、`releaseNotes.covered`、`knownGaps[]`、`packMaturity`、`validation[]` 与 `nextActions[]`，供新会话和 release maintainer 不读完整长文档也能先定位当前状态、必读入口、最新批次、release note freshness、当前 known gaps、pack schema version readiness 和最小验证。

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

- `go run ./cmd/rekit -- -Command release-check -Format json` 输出 `ready=true`，且 `gateProfile.ready=true`、`ciReleaseGate.ready=true`、`powerShellDeprecation.ready=true`、`powerShellDeprecation.fallbackRetirement.ready=true`、`powerShellDeprecation.fallbackRetirement.noFallbackCommands[]` 与 `candidateCommands[]` 非空、`caseShim.ready=true`、`publicDefaultDocs.ready=true`、`publicDefaultDocs.forbiddenShellFences[]` 为空、`releaseHandoff.ready=true`、`releaseHandoff.releaseNotes.covered=true`、`releaseHandoff.knownGaps[]` 非空、`releaseHandoff.packMaturity.total` 覆盖所有 pack、`releaseHandoff.packMaturity.schemaVersionReady=true`、`releaseHandoff.packMaturity.heavyToolGateReady=true`、`heavyToolGateActions[]` 非空，required commands、必备文档、pack schema、CI release gate inventory、PowerShell deprecation inventory、fallback retirement inventory、case shim readiness、public default docs readiness、release handoff summary、release notes freshness、manifest heavy-tool gate action、边界与 known gaps 清单完整；若 inventory `ready=false`，命令必须仍输出完整 JSON/text 诊断并以非零退出，避免 CI 只看日志不失败。
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
- PowerShell fallback 仍存在于 candidate commands；`release-check`、`status`、`packs`、`doctor` 与 `validate` 的 read-only fallback 已退休。删除或冻结剩余 PowerShell runtime 前必须有单独 deprecation batch、兼容验证和文档说明。

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
- attached case 的 `overview` 文本/JSON 与缺 board 初始化。
- `note -List` 文本/table/tsv/JSON、`note` append、`note -WhatIf`。
- `gate -WhatIf`、`gate -Apply` pending-gate request。
- `start -WhatIf -Format json`、`start -Apply`、`handoff -WhatIf -Format json`、`handoff -Apply`。
- `continue -WhatIf -Format json`、explicit `continue -Apply` safe subset。
- case lifecycle `attach`、`repair`、`init/bootstrap` preview/apply。
- `sync` / `update` review/apply/JSON preview 与 `promote` review/artifacts/candidates/apply/JSON preview。
- `plan-subagents` review artifact 写入：只生成 packet / summary / combined diff 路径，不自动 spawn reviewer。

PowerShell legacy / fallback 路径（详细冻结/删除策略见 `docs/powershell-deprecation.md`）：

- 无 `-Apply` 的文本工作线 flow。
- 文本 `sync -Apply -WhatIf` 与文本 promote what-if。
- 非 note/gate/continue apply 的其它 ledger 写入命令。
- 仍未退休 candidate commands 的 `REKIT_GO_DISABLE=1` compatibility fallback；`release-check`、`status`、`packs`、`doctor` 与 `validate` 已 no-fallback。
- 少量 Windows façade parity smoke，默认 release gate 已改为三平台 Go-native checks；只有改 façade/fallback 时按需运行。

## Known gaps

- bounded dispatch 仍不自动 spawn reviewer；runtime 只生成 review packet 和 observability。
- actual heavy-tool 执行未迁入 deterministic runtime；full-trace/debug/inject/patch/dump/network/symex 仍必须显式 gate，且 `gate` 只接受 pack manifest `heavyToolGates` 声明的 action。
- authority/confirmed 写入仍需人工确认，不由 Go `continue -Apply` 自动执行。
- policy schema 迁移与 PowerShell-free / Go-native 收敛的实际删除批次尚未进入默认发布路径；Batch 129 已新增 `docs/powershell-deprecation.md` 作为策略入口，Batch 130 已新增 Go-owned `release-check` inventory 作为本机/CI release gate 前置检查，Batch 131 已新增 façade freeze invariant 防止默认委托集合和 blocked 边界漂移，Batch 139 已新增 `release-check` gate profile 以便 CI/本机在执行前解析 recommended minimum，Batch 140 已新增轻量 CI workflow，Batch 142 已新增 `powerShellDeprecation` inventory 用于在删除前发现命令归属/模块清单漂移，Batch 144 已新增 `ciReleaseGate` inventory 用于发现 GitHub Actions release gate job/command/forbidden step 漂移，Batch 216 已把接手 goal 与 roadmap 调整为默认路径逐步不依赖 PowerShell、面向 macOS/Linux/Windows 的 Go-native convergence，Batch 217 已将默认 recommended minimum 与 CI release gate 收敛为 Linux/Windows/macOS 三平台 Go-native checks，Batch 223 已新增 `powerShellDeprecation.fallbackRetirement` 子库存用于区分 no-fallback、fallback removal candidate、blocked command 与 removal-candidate module，Batch 224 已退休 `status` / `packs` / `doctor` / `validate` read-only PowerShell fallback，使 no-fallback baseline 扩展到 5 个命令且剩余 candidate fallback 收窄到 9 个 command rows；PowerShell façade smoke 转为按需 compatibility 验证。
- 目前安全领域 skeleton pack 均已有真实临时 case dry-run；后续缺口转向 release readiness/CI 门禁、policy schema migration 和 PowerShell runtime deprecation。
