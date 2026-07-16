# rekit tests 维护指南

## 读取指南

本文件给维护者选择 `rekit/tests/*.ps1` smoke 时使用。日常使用 `/rekit` 不需要阅读本文件；只有维护 runtime、pack、Go façade、sync/promote 或 Agent Team ledger/workstream 行为时再按需查阅。机器可读导航见 `rekit/tests/catalog.json`，它不是自动执行器。

## 实施摘要

`rekit/tests` 里的脚本都是仓库维护验证入口，默认使用临时 case 或只读仓库状态，目标是锁定 review-first、no-write、Go façade parity、remaining PowerShell compatibility 和 pack skeleton 边界。`catalog.json` 用相同分类记录全部 `*.ps1` smoke/helper 的 `category`、`purpose`、`recommendedFor`、`supportsWorkRoot` 和 `riskBoundary`，供后续自动测试选择器或 CI 读取。

Go-first release gate 优先由 Go-owned `release-check` inventory、Go-native `status` / `packs` / `doctor`、`go test ./...` 与 `go vet ./...` 捕获确定性 invariant；默认远程 CI 见 `.github/workflows/release-gate.yml`，在 Linux、Windows、macOS 上运行同一组 Go-native release checks；`facade-smoke.ps1`、`catalog-smoke.ps1`、`pack-smoke-matrix-selftest.ps1` 与 pack matrix 保留为按需 PowerShell compatibility / parity 层，不继续扩张成新的 runtime owner。

推荐最小回归组合：

```text
go run ./cmd/rekit -- -Command release-check -Format json
go run ./cmd/rekit -- -Command status
go run ./cmd/rekit -- -Command packs
go run ./cmd/rekit -- -Command doctor
go test ./...
go vet ./...
git diff --check
```

## 执行清单

1. 先判断改动层：façade、Go backend、sync/promote、workstream/ledger、pack skeleton 或文档。
2. 选择下面对应 smoke；不要无脑跑真实 case。
3. 需要 case 行为时使用默认 `WorkRoot` 下的临时 case，或显式传入确认过的 dryrun case。
4. 验证后确认临时目录已清理，kit 仓库没有真实 sample、trace、dump、capture、artifact 或 case-specific state。
5. 提交前跑 `git diff --check`；Windows 上 LF/CRLF warning 可记录，不能有 whitespace error。

## 验证标准

- smoke 失败时先修 root cause，不跳过验证。
- pack smoke 必须覆盖 Go/PowerShell doctor、Go init、case doctor、`plan-subagents` route packet、promote review managed-doc candidate 和 no-write 边界。
- façade smoke 必须证明默认 Go delegation、已退休 fallback 的 no-fallback 边界、显式 Go enable、Go disable 优先级和剩余 write/text compatibility fallback 不回归。
- sync/promote apply smoke 只能使用临时 case / pack-safe fixture，并验证 backup、deny、pack-root containment 和 cleanup。
- workstream / ledger smoke 不写 confirmed/authority，除非脚本专门验证 gate 且使用临时 case。

## 风险与注意事项

- 不要在 kit 仓库内伪造真实 case state。
- 不要把真实样本、目标、hash、IOC、flag、payload、trace、dump、pcap、crash、客户信息或绝对 case 路径写入测试 fixture。
- 不要让 smoke 执行网络请求、扫描、fuzz、exploit replay、debug、dump、patch、hook、设备连接或外部副作用。
- `sync -Apply`、`promote -Apply`、candidate 写入、authority/confirmed 写入相关测试必须保持临时目录、backup 和 cleanup 边界。

## 常用 smoke 分组

### Façade / inventory / pack matrix

| 脚本 | 什么时候跑 | 覆盖重点 |
|---|---|---|
| `facade-smoke.ps1` | 改 `rekit.ps1`、Go façade 委托集合或 JSON preview/read-only 委托 | 只读命令、release-check inventory 默认委托、已退休 fallback 的 no-fallback 边界、overview 文本/JSON 与 note 文本/JSON 只读默认委托、note append/what-if 默认委托、gate what-if/apply 默认委托、start/handoff JSON preview/apply/text 默认委托、continue JSON preview/apply/text 默认委托、case lifecycle、sync 与 promote review/candidate/apply 默认 Go 委托、disable 优先级。 |
| `pack-inventory-smoke.ps1` | 改 pack manifest、maturity、inventory、status/doctor JSON | `/rekit packs/status/doctor/validate` text+JSON parity、默认 Go/facade 委托，以及 read-only fallback 退休后的 disable no-fallback 边界。 |
| `catalog-smoke.ps1` | 改 `catalog.json` 或测试导航字段 | catalog schema、唯一 id、全部 `*.ps1` 覆盖、脚本/文档存在性、pack smoke 与 discovery 对齐。 |
| `pack-smoke-matrix.ps1 -DiscoveryOnly` | 新增/删除 skeleton pack 或 pack smoke wrapper | inventory 中 schema-valid skeleton pack 与 matrix 清单/wrapper 一致性。 |
| `pack-smoke-matrix.ps1` | 需要全量 pack skeleton smoke 回归 | 串行运行全部安全领域 skeleton pack smoke。 |
| `pack-smoke-matrix.ps1 -Packs web-security,generic-binary-re -Format json` | 需要快速子集或机器可读结果 | 子集 pack smoke + JSON envelope。 |
| `pack-smoke-matrix-selftest.ps1` | 改 matrix 输出、discovery、参数或 guard | discovery text/JSON、matrix JSON、去重、文本输出、unknown pack guard。 |

### Pack skeleton smoke

这些脚本是 thin wrapper，复用 `pack-smoke-lib.ps1`：

```text
web-security-pack-smoke.ps1
malware-analysis-pack-smoke.ps1
vuln-research-pack-smoke.ps1
ctf-pack-smoke.ps1
unpack-pe-pack-smoke.ps1
ollvm-pack-smoke.ps1
android-native-pack-smoke.ps1
generic-binary-re-pack-smoke.ps1
```

单个 pack 修改时先跑对应脚本；跨 pack helper 或 route 逻辑修改时跑 matrix。

### Sync / promote / init / attach

| 脚本 | 什么时候跑 | 覆盖重点 |
|---|---|---|
| `init-bootstrap-smoke.ps1` | 改 Go init/bootstrap、case scaffold 或 lifecycle façade 委托 | preview/apply、managed docs、template、managed block、state、doctor、默认 façade 委托、disabled no-fallback。 |
| `sync-review-parity-smoke.ps1` | 改 sync review 或 bounded diff | 默认 façade/Go sync review action 和 diff parity。 |
| `sync-apply-smoke.ps1` / `go test ./internal/rekit/sync` | 改 Go sync apply、sync package helper 或 façade sync 默认委托 | 临时 case apply、backup、managed block、template force、state、backup escape guard、默认 façade 委托、disabled no-fallback、Go/façade doctor。 |
| `sync-apply-parity-smoke.ps1` | 改 sync apply force/parity | 默认 façade 与直接 Go apply/force 后 managed docs、metadata/shim、state 对比，并验证 disabled no-fallback。 |
| `promote-candidates-preflight-smoke.ps1` | 改 promote candidates preview/review 或 façade promote 默认委托 | promote text/no-fallback、Go review artifact/sanitized preview、default JSON preview/review/create-candidates delegation、write guard、disabled no-fallback。 |
| `promote-candidates-apply-smoke.ps1` / `go test ./internal/rekit/promote` | 改 Go candidate 写入、façade candidate 写入委托或 promote package helper | 默认 façade candidate 写入、candidate/index/tooling candidate、deny、sanitization、unique candidate path、restore helper、pack-root containment、cleanup。 |
| `promote-apply-preflight-smoke.ps1` | 改 promote apply preview 或 façade promote 默认委托 | promote text/disabled no-fallback、backup、deny、Go apply what-if、default JSON preview delegation、actual apply default delegation。 |
| `promote-apply-smoke.ps1` / `go test ./internal/rekit/promote` | 改 Go promote apply、façade promote apply 委托或 promote apply package helper | 默认 façade pack managed docs writeback、what-if no-write、backup、blocked deny、validation rows、validation failure restore、cleanup。 |

### Workstream / ledger / gate / Agent Team

| 脚本 | 什么时候跑 | 覆盖重点 |
|---|---|---|
| `plan-subagents-smoke.ps1` / `go test ./internal/rekit/cli -run 'TestRunGo(GateDispatchE2EPlanGateOverviewHandoff|GenericBinaryPackNeutralE2EStartPlanGateOverviewHandoff|WebSecurityPackNeutralE2EStartPlanGateOverviewHandoff)'` | 改 `plan-subagents`、subagent routes、review packet、plan-subagents façade 委托、gate/dispatch 闭环或 pack-neutral route 闭环 | route/taskType、Items/ItemsFile、observability、out-of-case guard、默认 façade Go JSON result、`REKIT_GO_DISABLE=1` no-fallback 边界；Go package E2E 额外覆盖 `_template` pack plan-subagents → gate request → overview/handoff 可见性链路、`generic-binary-re:binary-analysis` non-feature lane / pack-specific route 可见性链路，以及 `web-security:feature-analysis` Web/API route 可见性链路。 |
| `agent-team-review-loop-smoke.ps1` / `go test ./internal/rekit/cli -run 'TestRunGo(ReviewerDecisionE2ENoteOverviewHandoff|GenericBinaryPackNeutralE2EStartPlanGateOverviewHandoff|WebSecurityPackNeutralE2EStartPlanGateOverviewHandoff)'` | 改 review loop、verification/decision 展示、note list 委托、note append façade、reviewer/decision 闭环或 pack-neutral ledger 闭环 | packet -> note what-if no-write -> verification append -> decision append -> note/list -> overview/handoff 最小闭环，含默认 façade note append/what-if、note list 文本/JSON 委托与 `REKIT_GO_DISABLE=1` no-fallback 边界；Go package E2E 额外覆盖 `_template` pack candidate → verification → decision → overview/handoff 可见性链路、`generic-binary-re` non-feature lane 的 candidate/verification/decision → overview/handoff 可见性链路，以及 `web-security` endpoint candidate/verification/decision → overview/handoff 可见性链路。 |
| `agent-team-d5-dryrun-smoke.ps1` | 改 batch/intervention/rollback 展示 | candidate、verification、decision、batch、intervention/rollback、handoff。 |
| `web-security-agent-team-dryrun-smoke.ps1` | 改 web-security Agent Team dry-run、Web/API route、network gate、真实临时 case flow 或 pack-neutral handoff | 公共 `/rekit` façade 下的临时 case：`init -Apply`、`start -Apply`、`plan-subagents` review packet、candidate/verification/decision `note` append、`gate -WhatIf/-Apply -Action network`、`note -List`、`overview` text/JSON、lane `handoff -Apply` 与 `doctor`；不自动 spawn agent、不联网、不写 authority/confirmed。 |
| `generic-binary-agent-team-dryrun-smoke.ps1` | 改 generic-binary-re Agent Team dry-run、binary-analysis route、debug gate、真实临时 case flow 或 pack-neutral handoff | 公共 `/rekit` façade 下的临时 case：`init -Apply`、`start -Apply`、`plan-subagents` review packet、candidate/verification/decision `note` append、`gate -WhatIf/-Apply -Action debug`、`note -List`、`overview` text/JSON、lane `handoff -Apply` 与 `doctor`；不自动 spawn agent、不执行样本/debug/trace/dump/patch、不写 authority/confirmed。 |
| `malware-analysis-agent-team-dryrun-smoke.ps1` | 改 malware-analysis Agent Team dry-run、sample-analysis route、network/sandbox gate、真实临时 case flow 或 pack-neutral handoff | 公共 `/rekit` façade 下的临时 case：`init -Apply`、`start -Apply`、`plan-subagents` review packet、candidate/verification/decision `note` append、`gate -WhatIf/-Apply -Action network`、`note -List`、`overview` text/JSON、lane `handoff -Apply` 与 `doctor`；不自动 spawn agent、不执行样本/sandbox/network detonation/debug/dump/patch、不写 authority/confirmed。 |
| `vuln-research-agent-team-dryrun-smoke.ps1` | 改 vuln-research Agent Team dry-run、vuln-analysis route、debug/repro gate、真实临时 case flow 或 pack-neutral handoff | 公共 `/rekit` façade 下的临时 case：`init -Apply`、`start -Apply`、`plan-subagents` review packet、candidate/verification/decision `note` append、`gate -WhatIf/-Apply -Action debug`、`note -List`、`overview` text/JSON、lane `handoff -Apply` 与 `doctor`；不自动 spawn agent、不连接 live target、不扫描、不 fuzz、不 replay exploit、不执行 debug/dump/patch、不写 authority/confirmed。 |
| `ctf-agent-team-dryrun-smoke.ps1` | 改 ctf Agent Team dry-run、challenge-analysis route、network/remote gate、真实临时 case flow 或 pack-neutral handoff | 公共 `/rekit` façade 下的临时 case：`init -Apply`、`start -Apply`、`plan-subagents` review packet、candidate/verification/decision `note` append、`gate -WhatIf/-Apply -Action network`、`note -List`、`overview` text/JSON、lane `handoff -Apply` 与 `doctor`；不自动 spawn agent、不远程连接、不 brute force、不 fuzz、不 replay exploit、不执行 debug/dump/patch、不写 flag/authority/confirmed。 |
| `unpack-pe-agent-team-dryrun-smoke.ps1` | 改 unpack-pe Agent Team dry-run、unpack-analysis route、dump/unpack gate、真实临时 case flow 或 pack-neutral handoff | 公共 `/rekit` façade 下的临时 case：`init -Apply`、`start -Apply`、`plan-subagents` review packet、candidate/verification/decision `note` append、`gate -WhatIf/-Apply -Action dump`、`note -List`、`overview` text/JSON、lane `handoff -Apply` 与 `doctor`；不自动 spawn agent、不执行样本/debug/dump/patch、不写 unpacked binary、不写 authority/confirmed。 |
| `ollvm-agent-team-dryrun-smoke.ps1` | 改 ollvm Agent Team dry-run、obfuscation-analysis route、full-trace/CFG gate、真实临时 case flow 或 pack-neutral handoff | 公共 `/rekit` façade 下的临时 case：`init -Apply`、`start -Apply`、`plan-subagents` review packet、candidate/verification/decision `note` append、`gate -WhatIf/-Apply -Action full-trace`、`note -List`、`overview` text/JSON、lane `handoff -Apply` 与 `doctor`；不自动 spawn agent、不执行样本/full-trace/dump/patch、不写 deobfuscated binary、不写 authority/confirmed。 |
| `android-native-agent-team-dryrun-smoke.ps1` | 改 android-native Agent Team dry-run、native-analysis route、inject/hook gate、真实临时 case flow 或 pack-neutral handoff | 公共 `/rekit` façade 下的临时 case：`init -Apply`、`start -Apply`、`plan-subagents` review packet、candidate/verification/decision `note` append、`gate -WhatIf/-Apply -Action inject`、`note -List`、`overview` text/JSON、lane `handoff -Apply` 与 `doctor`；不自动 spawn agent、不连接设备、不 attach Frida、不执行 hook/inject/dump/patch、不写 authority/confirmed。 |
| `gate-parity-smoke.ps1` / `go test ./internal/rekit/cli -run 'TestRunGo(GateDispatchE2EPlanGateOverviewHandoff|GenericBinaryPackNeutralE2EStartPlanGateOverviewHandoff|WebSecurityPackNeutralE2EStartPlanGateOverviewHandoff)'` | 改 heavy-tool gate request schema、gate façade 委托、PowerShell 读层、gate/dispatch 闭环或 pack-neutral gate 展示 | 默认 façade gate what-if no-write、默认 façade gate apply request 写入 + overview/note/handoff 展示 parity、`REKIT_GO_DISABLE=1` no-fallback 边界；Go package E2E 额外覆盖 `_template` pack pending-gate request 在 Go overview/handoff 中的可见性、`generic-binary-re` non-feature lane pending-gate 的 overview/handoff 可见性，以及 `web-security` network pending-gate 的 overview/handoff 可见性。 |
| `overview-readonly-smoke.ps1` | 改 Go overview 或 façade overview 文本/JSON | 缺 board 初始化、后续只读 overview、Go gate request 展示、默认 façade 文本/JSON 委托、`REKIT_GO_DISABLE=1` no-fallback 边界。 |
| `start-apply-smoke.ps1` | 改 Go start 或 start façade 委托 | preview/apply scaffold、board/facts/policy/lane/workspace、默认 façade JSON preview/apply/text 委托、disabled text/JSON no-fallback、doctor。 |
| `handoff-apply-smoke.ps1` | 改 Go handoff 或 handoff façade 委托 | project/lane handoff preview/apply、resume/checkpoint、ledger 区段、默认 façade JSON preview/apply/text 委托、disabled text/JSON no-fallback。 |
| `continue-preflight-smoke.ps1` | 改 PowerShell continue authority gate | authority append gate matrix、backup/diff、CSV recovery、routing、digest/status、WhatIf no-write。 |
| `continue-whatif-smoke.ps1` / `go test ./internal/rekit/cli -run TestRunGoWorkstreamE2EStartNoteContinueHandoff` | 改 Go continue preview/apply、continue JSON/text preview/apply façade 委托或 Go workstream 闭环 | non-write preview、case-local apply writes、default façade JSON preview/apply/text delegation、disabled text/JSON no-fallback、wouldWrites/writes、blocked actions、authority guard；Go package E2E 额外覆盖 `_template` pack start → note → continue apply → handoff 闭环。 |
| `continue-digest-smoke.ps1` | 改 continue digest/status | structured digest inputs/route/packet refs/outputs/decisions/open risks。 |

## WorkRoot 约定

多数 case smoke 默认使用：

```text
C:\AI\m_projects\RE\_dryrun_cases
```

可以用 `-WorkRoot <path>` 指定临时工作根。除非脚本明确支持 `-CaseRoot` 并且你确认目标是长期 dryrun case，否则不要指向真实 case。

## 推荐组合

| 改动类型 | 推荐验证 |
|---|---|
| 新增 skeleton pack | 单 pack smoke -> `pack-smoke-matrix.ps1 -DiscoveryOnly` -> `pack-inventory-smoke.ps1` -> `go test ./...` -> `doctor`。 |
| 改测试导航 catalog | `catalog-smoke.ps1` -> `pack-smoke-matrix.ps1 -DiscoveryOnly` -> `pack-inventory-smoke.ps1`。 |
| 改 pack smoke helper/matrix | `pack-smoke-matrix-selftest.ps1` -> `pack-smoke-matrix.ps1 -DiscoveryOnly` -> 1-2 个代表 pack smoke -> `pack-inventory-smoke.ps1`。 |
| 改 Go façade 委托 | 按需运行 `facade-smoke.ps1` 与相关命令 smoke -> `go test ./...`。 |
| 改 sync/promote 写入 | 对应 preflight/apply smoke -> `doctor` -> `git diff --check`。 |
| 改 workstream/ledger/gate | 对应 workstream/ledger smoke -> `go test ./...` -> `doctor`。 |
