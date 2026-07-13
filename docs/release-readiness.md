# Release readiness checklist

## 读取指南

本文件是发布前和新会话接手时的一页门禁说明，配合 `docs/go-first-convergence-plan.md` 与 `docs/powershell-deprecation.md` 使用。维护者在准备 release、合并大型批次、或需要判断当前 runtime owner 时先读本文件顶部区域；需要历史路线时再读 go-first convergence、runtime migration 和 batch-plan。

本文件不替代 `rekit/tests/README.md` 的 smoke 选择指南，也不是要求每次改动都运行全量 PowerShell matrix。它定义轻量、可重复的 release gate 和当前 known gaps。

## 实施摘要

当前 release readiness 状态：

- Go backend 已是多数确定性 runtime 路径的 owner：`status`、`packs`、`doctor/validate`、case lifecycle、sync/promote、overview、note、gate、start/handoff、continue preview/apply safe subset。
- PowerShell `rekit/rekit.ps1` 仍是公共 façade：负责参数兼容、旧文本 flow、fallback、少量 parity smoke 和 `REKIT_GO_DISABLE=1` 回退。
- Agent Team dry-run 已从 `_template` package E2E 扩展到 `generic-binary-re`、`web-security` package E2E，并新增 `web-security`、`generic-binary-re`、`malware-analysis`、`vuln-research`、`ctf`、`unpack-pe` 真实临时 case smoke。
- 发布门禁应优先依赖 Go-owned `release-check` inventory、Go tests / `go vet` / doctor / 少量 Windows façade smoke；不要把大型 pack matrix 作为默认必跑。

## 执行清单

### 本机 release gate（推荐最小集）

```powershell
go run ./cmd/rekit -- -Command release-check -Format json
go test ./...
go vet ./...
./rekit/rekit.ps1 -Command doctor
./rekit/tests/facade-smoke.ps1
git diff --check
```

机器可读 smoke catalog 的 `recommendedMinimum` 当前为：

```text
go run ./cmd/rekit -- -Command release-check -Format json
facade-smoke.ps1
catalog-smoke.ps1
pack-smoke-matrix-selftest.ps1
pack-smoke-matrix.ps1 -DiscoveryOnly
pack-inventory-smoke.ps1
go test ./...
go vet ./...
rekit/rekit.ps1 -Command doctor
git diff --check
```

选择性追加：

- 改 catalog 或 smoke metadata：运行 `catalog-smoke.ps1`、`pack-smoke-matrix-selftest.ps1`、`pack-smoke-matrix.ps1 -DiscoveryOnly` 与 `pack-inventory-smoke.ps1`。
- 改 façade 委托：追加 `facade-smoke.ps1`。
- 改某个 pack skeleton：运行对应 `*-pack-smoke.ps1`；跨 pack helper 改动才运行 matrix 子集或 discovery。
- 改 sync/promote 写入：运行对应 preflight/apply smoke，并确认 backup、deny、restore 与 pack-root containment。
- 改 Agent Team / workstream / ledger / gate：运行对应临时 case smoke，例如 `agent-team-review-loop-smoke.ps1`、`gate-parity-smoke.ps1`、`web-security-agent-team-dryrun-smoke.ps1`、`vuln-research-agent-team-dryrun-smoke.ps1`、`unpack-pe-agent-team-dryrun-smoke.ps1`。

### 发布前人工检查

1. `CHANGELOG.md` 顶部 `Unreleased` 已描述本轮用户可见变化、边界和验证结果。
2. `docs/batch-plan.md` 最新 batch 状态为已完成，并列出实际执行过的验证。
3. `docs/go-first-convergence-plan.md` 未把阶段性进展误写成全局完成；Stage 1-8 未完成项仍保留。
4. `rekit/tests/catalog.json` 与 `rekit/tests/README.md` 一致；新增 `*.ps1` 已被 catalog 收录。
5. `git status --short` 干净后再打 tag 或发 release。

## 验证标准

Release gate 通过的最低标准：

- `go run ./cmd/rekit -- -Command release-check -Format json` 输出 `ready=true`，且 required commands、必备文档、pack schema、边界与 known gaps 清单完整。
- `go test ./...` 通过，包含 `internal/rekit/manifest` release invariants。
- `go vet ./...` 无输出或无错误退出。
- `./rekit/rekit.ps1 -Command doctor` 输出 pack validation ok。
- `./rekit/tests/facade-smoke.ps1` 验证默认 Go façade 委托和 `REKIT_GO_DISABLE=1` fallback 不回归。
- `git diff --check` 没有 whitespace error；Windows LF/CRLF warning 可记录。
- 涉及临时 case 的 smoke 必须清理自身 case root 和 review artifact。

## 风险与注意事项

- 不默认运行大型 PowerShell matrix；只有改 pack helper、matrix 输出或跨 pack skeleton 时才运行全量或子集 matrix。
- 不把真实样本、trace、dump、capture、pcap、crash、payload、客户信息、flag、IOC、绝对 case 路径或 case-specific 进度写入本仓库。
- 不执行真实网络请求、扫描、fuzz、exploit replay、debug、dump、patch、hook、设备连接或其它外部副作用。
- `gate -WhatIf` 只预览 pending-gate request；`gate -Apply` 只写 pending-gate request，不执行 heavy-tool。
- `continue -Apply` 只写 case-local facts/routing/run digest/lane resume/checkpoint/board；不写 authority/confirmed。
- `sync/promote` 保持 review-first；实际写入前必须确认范围，pack source 写入依赖 backup/deny/restore。
- PowerShell fallback 仍存在；删除或冻结 PowerShell runtime 前必须有单独 deprecation batch、兼容验证和文档说明。

## 当前 pack maturity matrix

| pack | maturity | authority lane | release 备注 |
|---|---|---|---|
| `_template` | template | main | pack authoring template，不作为 skeleton matrix 成员。 |
| `android-native` | skeleton | main | 不连接设备、不 attach Frida、不执行 hook。 |
| `ctf` | skeleton | main | 已有真实临时 case dry-run；不远程连接、不 brute force、不写 flag/authority/confirmed。 |
| `generic-binary-re` | skeleton | main | 已有 package E2E 与真实临时 case dry-run；不执行样本/debug/trace/dump/patch。 |
| `malware-analysis` | skeleton | main | 已有真实临时 case dry-run；不执行样本、不联网、不 sandbox detonation。 |
| `ollvm` | skeleton | main | 不执行样本、不 trace/dump/patch。 |
| `unpack-pe` | skeleton | main | 已有真实临时 case dry-run；不执行样本、不 debug/dump/patch、不写 unpacked binary。 |
| `vmp-re` | mature | devirt-main | 首个 mature pack；仍需 heavy-tool/authority/confirmed 人工 gate。 |
| `vuln-research` | skeleton | main | 已有真实临时 case dry-run；不连接 live target、不主动扫描、不 fuzz、不 replay exploit。 |
| `web-security` | skeleton | main | 已有 package E2E 与真实临时 case dry-run；不发真实网络请求。 |

## Go-owned 与 PowerShell legacy 状态

Go-owned / Go-default 路径：

- `release-check` release gate inventory。
- `status`、`packs`、`doctor/validate`。
- attached case 的 `overview` 文本/JSON 与缺 board 初始化。
- `note -List` 文本/table/tsv/JSON、`note` append、`note -WhatIf`。
- `gate -WhatIf`、`gate -Apply` pending-gate request。
- `start -WhatIf -Format json`、`start -Apply`、`handoff -WhatIf -Format json`、`handoff -Apply`。
- `continue -WhatIf -Format json`、explicit `continue -Apply` safe subset。
- case lifecycle `attach`、`repair`、`init/bootstrap` preview/apply。
- `sync` review/apply/JSON preview 与 `promote` review/artifacts/candidates/apply/JSON preview。

PowerShell legacy / fallback 路径（详细冻结/删除策略见 `docs/powershell-deprecation.md`）：

- 无 `-Apply` 的文本工作线 flow。
- 文本 `sync -Apply -WhatIf` 与文本 promote what-if。
- 内部命令和非 note/gate/continue apply 的其它 ledger 写入命令。
- `REKIT_GO_DISABLE=1` fallback。
- 少量 Windows façade parity smoke。

## Known gaps

- bounded dispatch 仍不自动 spawn reviewer；runtime 只生成 review packet 和 observability。
- actual heavy-tool 执行未迁入 deterministic runtime；full-trace/debug/inject/patch/dump/network 仍必须显式 gate。
- authority/confirmed 写入仍需人工确认，不由 Go `continue -Apply` 自动执行。
- policy schema 迁移、PowerShell runtime deprecation 的实际删除批次和 CI workflow 尚未进入默认发布路径；Batch 129 已新增 `docs/powershell-deprecation.md` 作为策略入口，Batch 130 已新增 Go-owned `release-check` inventory 作为本机/CI release gate 前置检查，Batch 131 已新增 façade freeze invariant 防止默认委托集合和 blocked 边界漂移。
- 目前 `generic-binary-re`、`web-security`、`malware-analysis`、`vuln-research`、`ctf` 与 `unpack-pe` 已有真实临时 case dry-run；其它 skeleton pack 仍主要依赖 pack smoke 和 package/route 覆盖。
