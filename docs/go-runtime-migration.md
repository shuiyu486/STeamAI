# Go runtime migration plan

## 目的

本文件记录 `/rekit` runtime 从 legacy PowerShell backend / fallback 渐进迁移到 Go backend 的实施方案，避免后续上下文压缩导致路线失真。

核心目标：在不破坏现有 `/rekit`、case-local thin shim、旧 case metadata、pack wrapper 和 review-first 边界的前提下，把越来越复杂的确定性 runtime 逻辑迁移到更易测试、调试和维护的 Go 模块中。当前默认验证应优先运行 Go-native release gate；历史 façade 片段只作为 compatibility / fallback 背景，不作为公开默认命令路径。

## 结论

采用 **`/rekit` public ABI + Go deterministic backend + legacy compatibility façade** 的渐进迁移：

```text
用户 /rekit
  -> canonical `/rekit` skill / case shim
     -> Go CLI/backend deterministic runtime
        -> internal/rekit/*        # Go deterministic runtime modules
     -> legacy façade/fallback compatibility path when explicitly needed
```

不要一次性重写全部 legacy PowerShell runtime。首批 Go 只接管只读或 review-only 的确定性能力；涉及 authority 写入、confirmed CSV、外部工具、副作用和复杂工作线自动化的命令继续留在 legacy guarded path，直到有充分 parity test。

## 非目标

- 不改变用户入口；用户仍只使用 `/rekit`。
- 不让 case-local shim 复制 runtime 逻辑。
- 不在首批 Go backend 中执行 IDA、x64dbg、Frida、trace、patch、dump 或任何外部 RE 工具。
- 不在首批迁移 `continue` 的 authority auto-append。
- 不把 Go migration 作为 manifest schema 破坏性迁移。

## 架构分层

推荐 Go 目录结构：

```text
cmd/rekit/main.go                 # Go backend CLI entrypoint
internal/rekit/cli/               # PowerShell-compatible args and command guards
internal/rekit/runtime/           # repo root/runtime root/clock/stdout context
internal/rekit/fs/                # safe join, UTF-8, hash, atomic-ish writes, relative paths
internal/rekit/manifest/          # PowerShell-compatible manifest subset parser and schema checks
internal/rekit/instance/          # .rekit/instance.yml and legacy .re-template.yml reader
internal/rekit/doctor/            # pack/case validation
internal/rekit/review/            # review paths, packet, bounded diff, summary
internal/rekit/sync/              # sync plan/review/apply; first Go phase is review-only
internal/rekit/promote/           # promote plan/review/sanitize; first Go phase is review-only
internal/rekit/workstream/        # future B3 board/lane/policy/auto migration
internal/rekit/testkit/           # temp case fixtures and golden parity tests
```

`/rekit` remains the public ABI while the default implementation converges to the Go deterministic backend; Go modules should not import pack-specific logic, and pack behavior must come from `packs/<pack>/manifest.yml` and managed docs.

## Manifest parser rule

首批 Go backend 必须兼容现有 PowerShell manifest subset parser，而不是直接替换成严格 YAML parser。

原因：当前 manifest 中的 regex pattern 依赖“去掉首尾引号但不解释 backslash escape”的行为。严格 YAML parser 可能改变 `\.`, `\b`, `\/` 等字符串语义，导致 promote deny pattern 漂移。

首批 parser 支持：

- 顶层 scalar；
- 顶层 list；
- 顶层 map；
- 顶层 object list；
- 简单单双引号剥离；
- 不解释复杂 YAML escape；
- 不支持任意深层嵌套。

## 迁移批次

### G0：契约冻结与 parity harness

目标：先定义 PowerShell 与 Go 的可比对契约，不改变用户行为。

- 固化 manifest parser、safe path、review packet、bounded diff、promote deny/sanitize 的 golden cases。
- 建立临时 case fixture 和 fixed clock 机制。
- 记录哪些字段需要 byte-level parity，哪些字段只需 normalized parity。

验证：`go test ./...`、`git diff --check`、Go-native `doctor`；legacy façade doctor 仅在改 compatibility/fallback 时按需追加。

### G1：Go read-only skeleton

目标：Go backend 能独立完成只读 root/manifest/instance status 和 pack doctor；case doctor 在 parity fixture 完成前显式拒绝，避免误报。

首批实现：

- `go.mod`；
- `cmd/rekit/main.go`；
- `internal/rekit/{cli,runtime,fs,manifest,instance,doctor}`；
- `manifest`、`runtime`、`cli` 的轻量 parity/guard tests；
- Go `status`；
- Go pack `doctor/validate`；
- Go `doctor/validate -Target <case>` 先显式拒绝 case doctor，避免未实现 case validation 时误报 `pack validation ok`；
- 早期保持 legacy façade 不默认委托 Go，降低回归风险；当前默认路径已转向 Go-native release gate。

验证：

```text
go run ./cmd/rekit -- -Command release-check -Format json
go run ./cmd/rekit -- -Command status
go run ./cmd/rekit -- -Command doctor
go run ./cmd/rekit -- -Command doctor -Pack _template
go test ./...
go vet ./...
git diff --check
```

Legacy façade / wrapper compatibility checks are only required when touching the migration façade, fallback, or wrapper layer.

### G2：Go review-only backend

状态：已完成 review-only plan 与 G2.1 artifact 写入；review/artifact 路径已随默认 façade 收口到 Go。

目标：Go 接管 `sync/promote` review-only plan，不写 managed docs、不写 pack、不写 candidates；需要 review packet 时只写 `.rekit/reviews/**` 或显式指定的 artifact 路径。

已实现：

- `internal/rekit/review`：共享 plan/item、hash、deny pattern、managed block helper、bounded diff 与 review artifact writer；
- `internal/rekit/sync`：生成 `sync` review-only plan；G3.4 另支持显式 `sync -Apply` 手动写入路径，Batch 79 支持 `sync -Apply -WhatIf` 非写入 JSON preview，G3.5 支持 `init/bootstrap -WhatIf/-Apply` 手动路径，Batch 106 起 `/rekit sync` review 与 `sync -Apply` 默认经 façade 委托 Go；裸 `sync -WhatIf` 仍拒绝；
- `internal/rekit/promote`：生成 `promote` review-only plan，覆盖 deny pattern、tooling source sanitization metadata 与 sanitized preview 内容；G3.7 另支持显式 `promote -CreateCandidates` 路径（可配 `-WhatIf` 预览），G3.9 支持显式 `promote -Apply/-Apply -WhatIf` 手动路径；Batch 107 起 `/rekit promote` review、review artifact 写入与 JSON what-if preview 默认经 façade 委托 Go；Batch 108 起 `promote -CreateCandidates` 实际候选写入也默认经 façade 委托 Go；Batch 111 起 `internal/rekit/promote` package tests 覆盖 apply what-if no-write、backup/validation rows、blocked deny 与 validation failure restore；Batch 112 起 `promote -Apply` 实际 pack source 写入默认经 façade 委托 Go；
- Go CLI 默认以 JSON 输出 review plan 到 stdout，`isMutation=false`、`reviewRequired=true`；
- G2.1 artifact 写入：`-ReviewOutputDir` / `-PacketPath` / `-DiffPath` 会输出 `packet.json`、`summary.md`、`diffs/combined.diff`、per-item bounded diff，以及 promote tooling candidate 的 `previews/*.sanitized-preview.md`；
- artifact 写入返回 `writesArtifacts=true`，仅代表写 review packet/diff/preview，不代表写 managed docs、pack 或 candidate；
- tests 覆盖 review-only guard、attached-case guard、sync artifact、promote artifact 与 sanitized preview。

当前边界：Batch 106 起 PowerShell façade 默认委托 `/rekit sync` review、`sync -Apply` 实际写入与 `sync -Apply -WhatIf -Format json` 非写入 preview；Batch 107 起默认委托 `/rekit promote` review、review artifact 写入、`promote -CreateCandidates -WhatIf -Format json` 与 `promote -Apply -WhatIf -Format json` 非写入 preview；Batch 108 起默认委托 `promote -CreateCandidates` 实际候选写入；`REKIT_GO_DISABLE=1` 保留 legacy PowerShell fallback。`promote -Apply` 实际 pack source 写入默认经 façade 委托 Go；其 Go helper 的 backup、validation rows、deny 与 restore failure 边界由 `go test ./internal/rekit/promote` 覆盖，`REKIT_GO_DISABLE=1` 保留 legacy PowerShell fallback。

### G2.2：Go gate dry-run（D4）

状态：已完成第一版非写入 dry-run；Batch 114 起 `gate -WhatIf` 默认接入 PowerShell façade。

目标：避免继续扩大 PowerShell runtime，把 heavy-tool gate 的确定性预览逻辑先放入 Go backend。

已实现：

- 新增 `internal/rekit/gate`：生成非写入 JSON gate plan，`isMutation=false`、`reviewRequired=true`、`requiresConfirmation=true`。
- Go CLI 支持 `-Command gate -WhatIf`。
- gate plan 校验 attached case 与 `.rekit/board.json` lane id；未知 lane 直接拒绝。
- event preview 对齐 ledger request 语义：`kind=request`、`status=pending-gate`、可带 `target`/`batchId`，并内嵌 `gate` 详情（action、scope、budget、triedLightSteps、stopConditions、deniedUntilUserConfirmation）。
- D4 dry-run 只预览将登记什么、需要确认什么；不写 JSONL、不执行 full-trace/debug/inject/patch/dump/network、不自动授权。

不迁移：`gate -Apply` ledger 写入、heavy-tool 执行、用户确认后的 heavy-tool 执行闭环。

### G2.3：Go gate pending-gate ledger 写入

状态：已完成第一版显式写入；Batch 115 起 `gate -Apply` 默认接入 PowerShell façade。

目标：在 Go backend 内补上 gate preview → pending-gate request 的最小落账路径，但只写 ledger request，不执行 heavy-tool、不写 confirmed/authority。

已实现：

- Go CLI 支持 `-Command gate -Apply`，将 gate preview append 到 `.rekit/facts/requests.jsonl`。
- `gate -Apply` 要求显式 `-Actor`，记录谁批准写入 pending-gate request；无 `-Actor` 拒绝。
- `gate -WhatIf` 与 `gate -Apply` 互斥；无 `-WhatIf/-Apply` 拒绝，避免误操作。
- 写入事件仍是 `kind=request`、`status=pending-gate`，包含 `gate` 详情；`isMutation=true` 只表示写入 ledger request。
- eventId 由 kind/lane/subject/summary/actor/risk/target/batch/action/scope/budget/triedLightSteps/stopConditions 派生；重复 eventId 返回 `applied=false` 与 `reason=duplicate eventId`，不重复追加。
- 临时 case smoke 写入后已清理测试 request，未留下 D4/G2.3 测试状态。
- Batch 36 已补 PowerShell 读层 parity：`overview`、lane `handoff`、`note -List -Kind request` 会展示 `actor/risk/target/batchId/gate{action,scope,budget,triedLightSteps,stopConditions}` 语义字段；验证脚本见 `rekit/tests/gate-parity-smoke.ps1`。

不迁移：heavy-tool 执行、用户确认后的执行器、confirmed/authority 写入。

### G3：低风险写入命令迁移

顺序：

1. `attach`（G3.1 已完成 Go CLI 手动路径；Batch 105 起公共 façade 默认委托 `-WhatIf` 预览与显式 `-Apply`，Go 写 `.rekit/instance.yml`、legacy `.re-template.yml`、初始 `.rekit/state.json` 与 thin shim，不写 managed docs/board/facts/lanes）；
2. `repair`（G3.2 已完成 Go CLI 手动路径；Batch 105 起公共 façade 默认委托默认/`-WhatIf` 预览与显式 `-Apply`，Go 刷新 `.rekit/instance.yml`、legacy `.re-template.yml`、缺失初始 `.rekit/state.json` 与 thin shim，不写 managed docs/board/facts/lanes）；
3. `sync -Apply`（G3.3 已完成预研与 review parity smoke；G3.4 已完成 Go CLI 手动写入路径；Batch 79 已补 `-Apply -WhatIf` 非写入 JSON preview；Batch 106 起 `/rekit sync` review、`sync -Apply` 实际写入与 JSON preview 默认经 PowerShell façade 委托 Go，文本 dry-run 与 `REKIT_GO_DISABLE=1` 继续 fallback，见 `docs/sync-apply-migration.md`）；
4. `init/bootstrap`（G3.5 已完成 Go CLI 手动路径；Batch 105 起公共 façade 默认委托 `-WhatIf` 预览与显式 `-Apply`，Go 创建/刷新 case-local files 与 managed content；裸 `init/bootstrap` 仍拒绝，要求 `-WhatIf` 或 `-Apply`）；
5. `promote -CreateCandidates`（G3.6 已完成迁移预研与 preflight smoke；G3.7 已完成 Go CLI 写入路径；Batch 108 起实际候选写入默认经 PowerShell façade 委托 Go，见 `docs/promote-candidates-migration.md`）；
6. `promote -Apply`（G3.8 已完成迁移预研与 preflight smoke；G3.9 已完成 Go CLI 手动写入路径；Batch 112 起实际 pack source 写入默认经 PowerShell façade 委托 Go，见 `docs/promote-apply-migration.md`）。

每一步都必须有临时 case smoke、backup 检查和 review-first 边界验证。后续所有实施计划必须写入 `docs/batch-plan.md` 或本文件后再实施，并随代码一起提交推送到远程 `main`，避免只留在聊天上下文中。

### G4：工作线只读/低风险命令迁移

状态：G4.1 已完成 `overview` 只读手动路径；G4.2 已完成 `start -WhatIf/-Apply` 手动路径；G4.3 已完成 `handoff -WhatIf/-Apply` 手动路径；G4.4 已完成 `plan-subagents` review artifact 手动路径；Batch F 已完成 `note` / `continue` 迁移再评估。Batch 72 已允许显式 `REKIT_GO_ENABLE=1` 时将 `start`、`handoff`、`continue` 的 `-WhatIf -Format json` 非写入预览委托 Go；Batch 73/74 曾允许已初始化 case 的 `overview -Format json` 与 `note -List -Format json` 只读查询在显式 Go enable 下委托；Batch 113 已将这两条只读 JSON 查询升级为默认 Go façade 委托；Batch 116 已将 attached case 的 `note` append 与 `note -WhatIf` 升级为默认 Go façade 委托；Batch 117 已将 attached case 的 `start`/`handoff` JSON preview 与显式 apply 升级为默认 Go façade 委托；Batch 118 已将 attached case 的 `continue -WhatIf -Format json` 非写入 preview 升级为默认 Go façade 委托；Batch 119 已将 attached case 缺 board 时的 overview board/facts/policy/default authority lane 初始化迁入 Go，并让 overview 文本/JSON 默认经 façade 委托 Go；Batch 120 已将 `note -List` 文本/table/tsv 查询升级为默认 Go façade；Batch 121 已将 explicit `continue -Apply` case-local facts/routing/run digest/resume/board 写入升级为默认 Go façade；Batch 122/123/124 已用 `_template` pack Go CLI package E2E 分别锁定 workstream 闭环、gate/dispatch 可见性链路与 reviewer/decision 可见性链路，Batch 125 已用 `generic-binary-re` Go CLI package E2E 锁定 non-feature lane 与 pack-specific route 的 pack-neutral 可见性链路，Batch 126 已用 `web-security` Go CLI package E2E 锁定非 RE-only route 与 network gate 的 pack-neutral 可见性链路，Batch 127 已用 `web-security-agent-team-dryrun-smoke.ps1` 在公共 `/rekit` façade 与真实临时 case 中复用该 Web/API dry-run 链路；Batch 75/76 曾允许 `attach`、`repair`、`init/bootstrap` 的 JSON 预览委托；Batch 105 已将这些 case lifecycle 预览/显式 apply 升级为默认 Go façade 委托；Batch 190 已将 `plan-subagents` review artifacts 纳入默认 Go façade 委托；Batch 225 已退休其 PowerShell fallback，使 `REKIT_GO_DISABLE=1` 报 no-fallback error。

顺序：

1. `overview`（G4.1 已完成 Go CLI renderer：读取 board/facts，展示最近 verification/decision，支持 `-Format json` 机器可读 envelope；Batch 119 起缺 `.rekit/board.json` 时由 Go 初始化 case-local board/facts/policy/default authority lane，overview 文本与 JSON 默认经 PowerShell façade 委托 Go；初始化只写 case-local scaffold，不写 facts event、handoff、ledger、authority/confirmed 或 metadata）；
2. `start`（G4.2 已完成 Go CLI 手动路径：`-WhatIf` 非写入预览，`-Apply` 显式创建/进入 feature lane 并初始化 board/facts/policy/default authority lane；Batch 117 起 attached case 的 JSON preview 与显式 apply 默认经 façade 委托 Go，文本 preview 与无 `-Apply` 文本 flow 回退 PowerShell）；
3. `handoff`（G4.3 已完成 Go CLI 手动路径：`-WhatIf` 非写入预览，`-Apply` 显式写项目级/工作线级 handoff、展示最近 verification/decision 并刷新 lane resume/checkpoint；Batch 117 起 attached case 的 JSON preview 与显式 apply 默认经 façade 委托 Go，文本 preview 与无 `-Apply` 文本 flow 回退 PowerShell）；
4. `plan-subagents`（G4.4 已完成 Go CLI review artifact 手动路径：按 manifest `subagentRoutes` 选择 route、拆分 Items/ItemsFile、写 packet/summary review artifact；Batch 58 已补 route/shard/review-loop observability；Batch 190 起默认经 façade 委托 Go；不启动 agent、不写 board/facts/lanes/handoff/authority）。

`continue` 的非写入 JSON preview 与 explicit `-Apply` 已分阶段交给 Go：Batch 118 起 `-WhatIf -Format json` 默认经 Go façade 委托非写入预览；Batch 121 起 explicit `continue -Apply` 默认经 Go façade 委托 case-local facts/routing/run digest/lane resume/checkpoint/board 写入。无 `-Apply` 的文本 workflow、authority/confirmed 写入和实际 heavy-tool 执行仍保留 PowerShell/人工确认边界。

### G4.5：`note` / `continue` 迁移再评估（Batch F）

状态：已完成评估；G4.5a/G4.5b 已完成 Go `note -List` 与 append 手动路径。

结论：

- `note` 已作为 Go 路径落地：`-Command note -List` 读取 `.rekit/facts/*.jsonl`，支持 `-Kind` 与 `-Lane` 过滤，展示 9 类 ledger event 的主要字段，并支持 `-Format json` 输出机器可读 `groups[]`；append 模式支持 9 类 kind、PowerShell 对齐的 enum/schema 校验、lane guard、eventId 去重、`-WhatIf` 预览与只写 facts JSONL。Batch 113 起，公共 façade 默认委托 `note -List -Format json` 只读查询；Batch 116 起，attached case 的 `note` append 与 `note -WhatIf` 也默认委托 Go，输出 JSON envelope；Batch 120 起，`note -List` 文本/table/tsv 查询也默认委托 Go；Batch 124 起，`internal/rekit/cli` package E2E 覆盖 candidate、reviewer verification 与 main decision append 后被 note list、overview 与 handoff 消费；Batch 125 起，`generic-binary-re` package E2E 额外覆盖非 feature lane 的 candidate/verification/decision append 被 overview 与 handoff 消费；Batch 126 起，`web-security` package E2E 额外覆盖非 RE-only Web/API lane 的 candidate/verification/decision append 被 overview 与 handoff 消费；Batch 127 起，`web-security-agent-team-dryrun-smoke.ps1` 通过公共 façade 在真实临时 case 中验证同一 ledger flow、request list、overview 与 handoff 可接手。
- `continue` 已迁移 safe subset：Go 负责 workspace/outbox 收集、request routing、candidate/decision facts、run digest/status、lane resume/checkpoint 与 board refresh；explicit `continue -Apply` 只写 case-local `.rekit/**` 状态。authority append 涉及 backup/diff/CSV schema/conflict/恢复语义，仍不由 Go apply 自动执行；authority/confirmed 写入必须单独 gate 与用户确认。
- Batch E 已把 PowerShell `continue` digest 补齐为 inputs/route/packet refs/outputs/decisions/open risks，降低短期迁移压力；G5 已先落地 Go `continue -WhatIf` 非写入预览，作为后续 apply 迁移前的 preview parity 路径。

`note` 迁移契约：

- 已支持 `-List` 只读模式与 append 手动模式；append 支持 9 种 kind：observation、hypothesis、candidate、verification、decision、intervention、rollback、publication、request。
- 保持 PowerShell schema 校验：confidence/decision/status/verifier/verdict/intervention action 枚举、非空 evidenceRefs、lane 存在性、eventId 去重。
- 只写对应 facts JSONL；不写 board、lane、handoff、authority/confirmed 文件，不执行 full-trace/debug/inject/patch/dump/network。
- tests 覆盖合法写入、非法枚举、未知 lane、duplicate eventId、`note -List -Kind/-Lane`、verification/decision/request/intervention 展示字段、reviewer/decision E2E 展示链路与 PowerShell doctor。

### G5：`continue` 与 policy gate 迁移

状态：G5 preflight baseline、Go `continue -WhatIf` 非写入预览路径与 PowerShell fallback `continue -WhatIf -Format json` 机器可读预览已完成；Batch 118 起 `continue -WhatIf -Format json` 默认经 PowerShell façade 委托 Go；Batch 121 起 explicit `continue -Apply` 默认经 façade 委托 Go，写 case-local facts/routing/run digest/lane resume/checkpoint/board，并将 authority/confirmed 写入 defer 给显式用户确认。

已补齐的 preflight gate tests（见 `rekit/tests/continue-preflight-smoke.ps1`）：

- evidence required；
- accepted verifier verdict；
- confidence threshold；
- CSV schema valid；
- no conflict；
- backup created；
- bounded diff created；
- max rows；
- authorityFiles allowlist；
- append 后 CSV 失败可恢复 backup；
- request routing 写 task/inbox 幂等；
- run digest/status parity（inputs、route、packet refs、outputs、decisions、open risks）；
- `-WhatIf` 不创建 runRoot、不刷新 board/lane resume、不写 facts。

已完成 Go `continue -WhatIf` 非写入预览与 explicit `continue -Apply` safe subset：Go CLI 读取既有 board/lane/outbox/workspace，preview 输出 inputs、packet refs、事件收集、routing/authority 决策预览、wouldWrites 与 blocked actions，并保持不创建 runRoot、不写 facts、不刷新 board/lane resume/checkpoint、不改 authority CSV；apply 写 facts JSONL、request routing、run status/digest、lane resume/checkpoint 与 board refresh，但 authority candidate 会被改为 defer 并写 decision reason，不 append authority/confirmed。任何自动写 authority/confirmed、policy schema 迁移或 heavy-tool 执行仍必须单独停下确认。

### G6：PowerShell 收敛为 wrapper

条件：所有命令 parity 通过，旧 case smoke 通过，case-local shim 和 pack wrapper 不需要改语义。

最终形态：legacy façade 只负责参数兼容、Go binary 查找、fallback 和错误透传，默认 public/runtime path 不依赖该兼容层。

## PowerShell façade 策略

G2.4 增加显式环境变量开关；Batch 104 起，低风险 read-only 命令默认委托 Go；Batch 105 起 case lifecycle 预览/显式 apply 默认委托 Go；Batch 106 起 `/rekit sync` review 与 `sync -Apply` 默认委托 Go；Batch 107 起 `/rekit promote` review 与 JSON what-if preview 默认委托 Go；Batch 108 起 `promote -CreateCandidates` 实际候选写入默认委托 Go；Batch 112 起 `promote -Apply` 实际 pack source 写入默认委托 Go；Batch 113 起 `note -List -Format json` 只读查询默认委托 Go；Batch 114 起 `gate -WhatIf` 非写入 heavy-tool gate preview 默认委托 Go；Batch 115 起 `gate -Apply` pending-gate request ledger 写入默认委托 Go；Batch 116 起 attached case 的 `note` append 与 `note -WhatIf` 默认委托 Go；Batch 117 起 attached case 的 `start`/`handoff` JSON preview 与显式 `-Apply` 默认委托 Go；Batch 118 起 attached case 的 `continue -WhatIf -Format json` 非写入 preview 默认委托 Go；Batch 119 起 attached case 的 `overview` 文本/JSON 与缺 board 初始化默认委托 Go；Batch 120 起 attached case 的 `note -List` 文本/table/tsv 只读查询默认委托 Go；Batch 121 起 attached case 的 explicit `continue -Apply` case-local 写入默认委托 Go：

```text
REKIT_GO_ENABLE=1   # 兼容旧 preview/review 扩展开关；默认集合不再依赖它
REKIT_GO_DISABLE=1  # 强制禁用 Go façade 委托，优先级高于默认委托和 ENABLE
REKIT_GO_EXE=...    # 可选：指定已构建的 rekit-go.exe；未指定时优先 rekit/bin/rekit-go.exe，否则 fallback 到 go run ./cmd/rekit --
```

默认委托 Go 的命令与安全路径：

- `status`（支持默认文本和 `-Format json` 机器可读 envelope）；
- `packs` 只读 pack inventory（支持默认 TSV 和 `-Format json`）；
- kit-root 与 attached case `doctor` / `validate`（支持默认文本和 `-Format json` 验证 rows envelope）；
- attached case 的 `overview` 文本与 `overview -Format json` envelope；缺 board 时可初始化 case-local board/facts/policy/default authority lane，后续读取保持只读；
- attached case 的 `note -List` 文本/table/tsv 与 `note -List -Format json` 只读 ledger event 查询，以及 `note` append / `note -WhatIf` ledger event JSON envelope（只写 facts JSONL 或 preview，不写 authority/confirmed）；
- `gate -WhatIf` 非写入 heavy-tool gate preview 与 `gate -Apply` pending-gate request ledger 写入；
- attached case 的 `start -WhatIf -Format json` 非写入 preview、`start -Apply` lane scaffold 写入、`handoff -WhatIf -Format json` 非写入 preview 与 `handoff -Apply` handoff 文件写入；
- attached case 的 `continue -WhatIf -Format json` 非写入 preview 与 explicit `continue -Apply` case-local facts/routing/run digest/lane resume/checkpoint/board 写入；
- case lifecycle `attach`、`repair`、`init/bootstrap` 预览与显式 `-Apply`；
- `/rekit sync` review、`sync -Apply` 实际写入与 `sync -Apply -WhatIf -Format json` 非写入 preview；
- `/rekit promote` review、review artifact 写入、`promote -CreateCandidates` 实际候选写入、`promote -CreateCandidates -WhatIf -Format json` 非写入 preview、`promote -Apply` 实际 pack source 写入与 `promote -Apply -WhatIf -Format json` 非写入 preview；
- `plan-subagents` review artifact 写入：只生成 packet / summary / combined diff 路径，不自动 spawn reviewer。

`REKIT_GO_ENABLE=1` 保留为兼容开关；当前 documented safe set 已默认开放，后续若新增 preview/review 扩展集合可继续用它灰度。

不委托：实际 heavy-tool 执行、无 `-Apply` 的工作线文本 workflow、authority/confirmed 写入和非 note/gate/continue apply 的其它 ledger 写入命令。文本 `sync -Apply -WhatIf` 与文本 promote what-if 仍作为 PowerShell dry-run fallback，不委托 Go。公共 `/rekit` 入口继续由 PowerShell 处理工作线文本命令、legacy fallback 和未迁移写入路径。

## 验证矩阵

本节记录 Go runtime 迁移相关验证项；跨类别 smoke 的选择指南见 `rekit/tests/README.md`。

| 场景 | 命令 | 预期 |
|---|---|---|
| Go status | `go run ./cmd/rekit -- -Command status` / `-Format json` | 默认输出 runtime root、template root、pack、manifest counts；JSON 输出 `command/schemaVersion/isMutation/runtimeRoot/templateRoot/pack/target/mode/case/manifest` envelope。 |
| Go packs inventory | `go run ./cmd/rekit -- -Command packs` / `-Format json` | 只读列出所有 `packs/*/manifest.yml`，显示成熟度、schema、route、managed/tooling、authority lane、version 与 description；JSON 输出同一 inventory envelope。 |
| Go vmp doctor | `go run ./cmd/rekit -- -Command doctor` / `-Format json` | 默认输出 `pack validation ok`；JSON 输出 `command/schemaVersion/isMutation/pack/target/mode/valid/summary/rows[]` envelope。 |
| Go explicit root doctor | `go run ./cmd/rekit -- -Command doctor -Target .` / `-Format json` | pack validation ok；JSON mode 为 `pack`。 |
| Go case doctor | `go run ./cmd/rekit -- -Command doctor -Target <case> -Pack vmp-re` / `-Format json` | attached case 结构只读验证，默认输出 `instance validation ok`；JSON mode 为 `case`。 |
| Go non-case target doctor | `go run ./cmd/rekit -- -Command doctor -Target .\does-not-exist` | 报错，不得误报 pack validation ok。 |
| Go template doctor | `go run ./cmd/rekit -- -Command doctor -Pack _template` | pack validation ok，模板 pack 自带 pack-neutral subagent routes。 |
| Go sync review artifacts | `go run ./cmd/rekit -- -Command sync -Target <case> -Pack vmp-re -ReviewOutputDir <dir>` | 写 `packet.json`、`summary.md`、`diffs/combined.diff`，返回 `isMutation=false` / `writesArtifacts=true`。 |
| Go sync apply preview | `go run ./cmd/rekit -- -Command sync -Target <case> -Pack vmp-re -Apply -WhatIf` | 输出非写入 JSON preview，返回 `isMutation=false` / `applied=false`；不写 metadata/shim/managed docs/backup/state/review artifact。 |
| Go sync apply | `go test ./internal/rekit/sync` / `go run ./cmd/rekit -- -Command sync -Target <case> -Pack vmp-re -Apply` / `.\rekit\tests\sync-apply-parity-smoke.ps1` | 写入 managed docs / managed block / template create-if-missing / sync state，返回 `isMutation=true`；Batch 106 起公共 façade 默认委托 Go；Batch 110 起 package tests 覆盖 apply preview no-write、backup、managed block、template force、state refresh 与 backup escape guard；parity smoke 对比 PowerShell fallback 与 Go apply/force 后的 managed docs、metadata/shim 和 state。 |
| Go init preview | `go run ./cmd/rekit -- -Command init -Target <newCase> -Pack vmp-re -WhatIf` | 输出非写入 JSON plan，`isMutation=false` / `requiresConfirmation=true`，不创建 target。 |
| Go init/bootstrap apply | `go run ./cmd/rekit -- -Command init -Target <newCase> -Pack vmp-re -Apply` | 创建 metadata / shim / legacy metadata / managed docs / managed block / template create-if-missing / sync state，返回 `isMutation=true`；Batch 105 起公共 façade 对 `init/bootstrap -WhatIf/-Apply` 默认委托 Go，`REKIT_GO_DISABLE=1` 回退。 |
| Go promote review artifacts | `go run ./cmd/rekit -- -Command promote -Target <case> -Pack vmp-re -ReviewOutputDir <dir>` | 写 review packet、bounded diff 和 tooling sanitized preview，不写 pack/candidates。 |
| Promote candidates preflight | `.\rekit\tests\promote-candidates-preflight-smoke.ps1` | 验证 PowerShell `promote -WhatIf -CreateCandidates` baseline、Go promote review artifact/sanitized preview parity、Go `-CreateCandidates -WhatIf` 非写入、默认 façade JSON 预览/默认 review artifact/默认 create-candidates 委托与文本 fallback；不写 pack candidates。 |
| Go promote create candidates | `go test ./internal/rekit/promote` / `.\rekit\tests\promote-candidates-apply-smoke.ps1` | Batch 108 起公共 façade 默认委托 Go 写 candidate/index/tooling candidate；Batch 109 起 package tests 覆盖 what-if no-write、candidate index、sanitization、unique path 与 restore helper，smoke 继续验证 blocked deny、pack-root containment 与 cleanup；不提交临时 candidates。 |
| Promote apply preflight | `.\rekit\tests\promote-apply-preflight-smoke.ps1` | 验证 PowerShell `promote -Apply` disabled fallback baseline、backup、deny、pack-root cleanup、Go apply what-if、默认 façade JSON 预览委托、actual apply 默认委托与文本 fallback。 |
| Go promote apply | `go test ./internal/rekit/promote` / `.\rekit\tests\promote-apply-smoke.ps1` | Batch 112 起公共 façade 默认委托 Go 写 pack managed docs，验证 what-if 非写入、backup、blocked deny、validation rows、validation failure restore、pack-root containment、tooling 不写入、cleanup 与 `REKIT_GO_DISABLE=1` fallback。 |
| Go gate dry-run | `go run ./cmd/rekit -- -Command gate -Target <case> -Pack vmp-re -WhatIf -Action full-trace -Lane <lane>` / retired legacy façade gate what-if compatibility path | 输出非写入 JSON plan，`eventPreview.kind=request`、`status=pending-gate`、`requiresConfirmation=true`；Batch 114 起公共 façade 默认委托 Go，仍不写 ledger、不执行 heavy-tool；Batch 226 起 `REKIT_GO_DISABLE=1` 报 no-fallback error，不再回退 PowerShell。 |
| Go gate no-mode guard | `go run ./cmd/rekit -- -Command gate -Target <case> -Action debug -Lane <lane>` | 报错，必须显式选择 `-WhatIf` 或 `-Apply`。 |
| Go gate apply | `go test ./internal/rekit/gate` / `go run ./cmd/rekit -- -Command gate -Target <case> -Pack vmp-re -Apply -Action debug -Lane <lane> -Actor <user>` / retired legacy façade gate apply compatibility path | 只 append `.rekit/facts/requests.jsonl` pending-gate request，返回 `isMutation=true`，不执行工具；Batch 115 起公共 façade 默认委托 Go，package tests 覆盖 dry-run no-write、actor guard、pending-gate 写入、重复 eventId 幂等和 no authority/confirmed/artifacts；Batch 123 起 CLI package E2E 验证 `_template` pack 的 gate request 可被 overview 与 handoff 消费，Batch 125 起额外验证 `generic-binary-re` non-feature lane pending gate 可被 overview/handoff 消费，Batch 126 起额外验证 `web-security` network pending-gate 可被 overview/handoff 消费，Batch 127 起真实临时 case smoke 额外验证 façade `gate -WhatIf` no-write 与 `gate -Apply -Action network` pending-gate 在 `note -List`、overview 与 handoff 中可见；Batch 226 起 `REKIT_GO_DISABLE=1` 报 no-fallback error，不再回退 PowerShell。 |
| Gate request display parity | `.\rekit\tests\gate-parity-smoke.ps1` | 默认 façade `gate -WhatIf` 不写 ledger，默认 façade `gate -Apply` 写入的 pending-gate request 在 PowerShell `overview`、`note -List`、lane `handoff` 中展示 actor/risk/target/batch/gate 详情，并验证 `REKIT_GO_DISABLE=1` no-fallback 边界不写 ledger。 |
| Go overview initialization / readonly | `.\rekit\tests\overview-readonly-smoke.ps1` | 手动 Go CLI 缺 board 时初始化 case-local board/facts/policy/default authority lane，JSON envelope 标记 `isMutation=true`；后续读取既有 board/facts 输出项目总览，含最近 verification/decision，并保持 snapshot 只读；Batch 119 起 PowerShell façade 默认委托 overview 文本与 JSON，`REKIT_GO_DISABLE=1` 可回退。 |
| Go start apply | `.\rekit\tests\start-apply-smoke.ps1` / `go test ./internal/rekit/cli` | Go CLI 与公共 façade `start -WhatIf -Format json` / `start -Apply` 预览或创建 feature lane，验证 board/facts/policy/lane/workspace scaffold、PowerShell text preview fallback、默认 JSON preview/apply façade 委托、`REKIT_GO_DISABLE=1` fallback 与 doctor；Batch 117 起 attached case 的 start JSON preview 与显式 apply 默认委托 Go；Batch 122 起 CLI package test 覆盖 `_template` pack 的 start → note → continue apply → handoff 最小闭环。 |
| Go handoff apply | `.\rekit\tests\handoff-apply-smoke.ps1` | Go CLI 与公共 façade `handoff -WhatIf -Format json` / `handoff -Apply` 预览或写项目级/工作线级 handoff，验证 lane resume/checkpoint、verification/decision 等 ledger 展示区段、PowerShell text preview fallback、默认 JSON preview/apply façade 委托、`REKIT_GO_DISABLE=1` fallback 与 doctor；Batch 117 起 attached case 的 handoff JSON preview 与显式 apply 默认委托 Go。 |
| Go plan-subagents artifacts | `.\rekit\tests\plan-subagents-smoke.ps1` / `go test ./internal/rekit/cli -run 'TestRunGo(GateDispatchE2EPlanGateOverviewHandoff|GenericBinaryPackNeutralE2EStartPlanGateOverviewHandoff|WebSecurityPackNeutralE2EStartPlanGateOverviewHandoff)'` | Batch 190 起公共 façade 默认委托 Go `plan-subagents` 写 review packet/summary/combined diff，验证 route/taskType 选择、Items/ItemsFile 分片、route/shard/review-loop observability、out-of-case guard、missing routes、doctor、default façade JSON result 与 Batch 225 起 `REKIT_GO_DISABLE=1` no-fallback error；Batch 123 起 CLI package E2E 额外覆盖 `_template` pack 的 plan-subagents → gate request → overview/handoff 可见性链路，Batch 125 起额外覆盖 `generic-binary-re:binary-analysis` pack-specific route 与 non-feature lane handoff，Batch 126 起额外覆盖 `web-security:feature-analysis` Web/API route 与 network gate handoff；不写 board/facts/lanes/handoff/authority，不启动 agent。 |
| Go note list readonly | `go test ./internal/rekit/cli ./internal/rekit/note` / `.\rekit\tests\agent-team-review-loop-smoke.ps1` | Go CLI `note -List` 读取 facts JSONL，验证全量展示、`-Kind`/`-Lane` 过滤、`-Format json` events envelope、invalid kind/format guard、write flag guard、只读 snapshot；Batch 120 起默认 Go façade 覆盖文本/table/tsv 与 JSON 查询，Batch 124 起 CLI package E2E 覆盖 reviewer verification 与 main decision 的 text/JSON 查询链路；`REKIT_GO_DISABLE=1` 回退。 |
| Go note append | `go test ./internal/rekit/cli ./internal/rekit/note` / `.\rekit\tests\agent-team-review-loop-smoke.ps1` | Go CLI `note` append 写 facts JSONL，验证 9 类 kind 基础写入、`-WhatIf` 非写入、enum/schema/lane guard、eventId dedupe 与 unsupported write flags；Batch 116 起公共 façade 默认委托 attached case 的 `note` append 与 `note -WhatIf`，Batch 124 起 CLI package E2E 覆盖 candidate → verification → decision append 后被 overview/handoff 消费；append 输出 JSON envelope，只写 facts JSONL，不写 authority/confirmed、不执行 heavy-tool。 |
| Agent Team D5 dry-run | `.\rekit\tests\agent-team-d5-dryrun-smoke.ps1` | 使用 `_template` 自包含临时 case 跑通 candidate → verification → decision → batch → intervention/rollback → handoff，验证 note list、overview、handoff 与 doctor；只写临时 case 并清理，不写 authority/confirmed、不执行 heavy-tool。 |
| Web-security Agent Team dry-run | `.\rekit\tests\web-security-agent-team-dryrun-smoke.ps1` | 使用 `web-security` 真实临时 case 与公共 `/rekit` façade 跑通 init/start/plan-subagents/candidate/network gate/verification/decision/note-list/overview/handoff/doctor，验证 Web/API route、network pending-gate 与 handoff 可接手；不自动 spawn agent、不联网、不写 authority/confirmed、不执行 heavy-tool。 |
| Shared pack smoke helper | `.\rekit\tests\pack-smoke-lib.ps1` | `web-security`、`malware-analysis`、`vuln-research`、`ctf`、`unpack-pe`、`ollvm`、`android-native` 与 `generic-binary-re` smoke 的共享 helper；统一 Go/PowerShell doctor、Go init/case doctor、`plan-subagents` route packet、promote review managed-doc candidate 与 no-write 边界检查，各 pack wrapper 只保留 route/task/output-contract 配置。 |
| Pack smoke matrix | `.\rekit\tests\pack-smoke-matrix.ps1` / `-Packs web-security,generic-binary-re` / `-Format json` / `-DiscoveryOnly` / `pack-smoke-matrix-selftest.ps1` | 显式运行安全领域 skeleton pack smoke 全量或子集，文本模式逐 pack 输出 running/passed/failed、elapsedMs 和原始 smoke 输出；JSON 模式输出 `pack-smoke-matrix` envelope 与 results[]；discovery 模式对比 `/rekit packs -Format json`、matrix 清单和 wrappers，报告 missing/extra/orphan/missing-script；selftest 锁定 discovery、JSON、去重、文本和 unknown pack guard；只编排或检查现有自包含 smoke，不改变 runtime 或 pack 行为。 |
| Malware-analysis pack smoke | `.\rekit\tests\malware-analysis-pack-smoke.ps1` | 使用 `malware-analysis` 自包含临时 case 验证 Go/PowerShell doctor、Go init/case doctor、`plan-subagents` route packet、promote review 与 no-write 边界；不执行样本、不联网、不写 authority/confirmed。 |
| Vuln-research pack smoke | `.\rekit\tests\vuln-research-pack-smoke.ps1` | 使用 `vuln-research` 自包含临时 case 验证 Go/PowerShell doctor、Go init/case doctor、`plan-subagents` route packet、promote review 与 no-write 边界；不主动扫描、不 fuzz、不 replay exploit、不写 authority/confirmed。 |
| CTF pack smoke | `.\rekit\tests\ctf-pack-smoke.ps1` | 使用 `ctf` 自包含临时 case 验证 Go/PowerShell doctor、Go init/case doctor、`plan-subagents` route packet、promote review 与 no-write 边界；不远程连接、不 brute force、不写 flag/authority/confirmed。 |
| Unpack PE pack smoke | `.\rekit\tests\unpack-pe-pack-smoke.ps1` | 使用 `unpack-pe` 自包含临时 case 验证 Go/PowerShell doctor、Go init/case doctor、`plan-subagents` route packet、promote review 与 no-write 边界；不执行样本、不 debug/dump/patch、不写 unpacked binary/authority/confirmed。 |
| OLLVM pack smoke | `.\rekit\tests\ollvm-pack-smoke.ps1` | 使用 `ollvm` 自包含临时 case 验证 Go/PowerShell doctor、Go init/case doctor、`plan-subagents` route packet、promote review 与 no-write 边界；不执行样本、不 trace/dump/patch、不写 deobfuscated binary/authority/confirmed。 |
| Android native pack smoke | `.\rekit\tests\android-native-pack-smoke.ps1` | 使用 `android-native` 自包含临时 case 验证 Go/PowerShell doctor、Go init/case doctor、`plan-subagents` route packet、promote review 与 no-write 边界；不连接设备、不 attach Frida、不执行 hook、不写 authority/confirmed。 |
| Generic binary RE pack smoke | `.\rekit\tests\generic-binary-re-pack-smoke.ps1` | 使用 `generic-binary-re` 自包含临时 case 验证 Go/PowerShell doctor、Go init/case doctor、`plan-subagents` route packet、promote review 与 no-write 边界；不执行样本、不 debug/trace/dump/patch、不写 authority/confirmed。 |
| Continue preflight baseline | `.\rekit\tests\continue-preflight-smoke.ps1` | 验证 PowerShell `continue` authority append gate matrix、backup/diff、CSV 失败恢复、request routing 幂等、digest/status parity 与 `-WhatIf` no-write，作为 Go `continue` 迁移 baseline。 |
| Go continue preview/apply | `.\rekit\tests\continue-whatif-smoke.ps1` / `go test ./internal/rekit/cli` | 手动 Go CLI `continue -WhatIf` 输出非写入 preview，`continue -Apply` 写 case-local facts/routing/run digest/lane resume/checkpoint/board，并覆盖 PowerShell text preview fallback、`continue -WhatIf -Format json` 与 explicit `continue -Apply` 默认 façade 委托、preview no-write、`REKIT_GO_DISABLE=1` fallback、duplicate skipped 与 authority guard；Batch 122 起 `_template` pack package test 验证 continue apply 输出可被 handoff 消费；不写 authority/confirmed、不执行 heavy-tool。 |
| Go attach preview | `go run ./cmd/rekit -- -Command attach -Target <case> -Pack vmp-re -WhatIf` | 输出非写入 plan，不创建目录或文件。 |
| Go attach apply | `go run ./cmd/rekit -- -Command attach -Target <case> -Pack vmp-re -ProjectName <name> -Apply` | 只写 `.rekit/instance.yml`、legacy `.re-template.yml`、初始 `.rekit/state.json` 与 case-local thin shim，不写 managed docs、board/facts/lanes。 |
| Go repair preview | `go run ./cmd/rekit -- -Command repair -Target <movedCase> -Pack vmp-re -WhatIf` | 输出非写入 repair plan，展示 recorded/new projectRoot 与写入计划。 |
| Go repair apply | `go run ./cmd/rekit -- -Command repair -Target <movedCase> -Pack vmp-re -Apply` | 只刷新 `.rekit/instance.yml`、`.re-template.yml`、缺失的初始 `.rekit/state.json` 与 case-local thin shim，不写 managed docs、board/facts/lanes 或 authority。 |
| Façade Go status | retired legacy façade status compatibility path / JSON format | 默认委托 Go，输出 `rekit go backend`；JSON 输出由 Go-owned status envelope 维护；Batch 224 起 `REKIT_GO_DISABLE=1` 报 no-fallback error，不再回退 PowerShell。 |
| Façade Go packs inventory | retired legacy façade packs inventory compatibility path / JSON format | 默认委托 Go，输出 pack inventory 表或 JSON envelope；Batch 224 起 `REKIT_GO_DISABLE=1` 报 no-fallback error，不再回退 PowerShell。 |
| Façade Go doctor/validate | retired legacy façade doctor/validate compatibility path / JSON format | 默认委托 Go，输出验证 rows envelope；Batch 224 起 `REKIT_GO_DISABLE=1` 报 no-fallback error，不再回退 PowerShell。 |
| Façade Go sync review artifacts | legacy façade sync review artifact compatibility path | Batch 106 起默认委托 Go 写 review artifacts，不写 managed docs；`REKIT_GO_DISABLE=1` 回退 PowerShell。 |
| Façade Go sync apply / preview | legacy façade sync apply and JSON preview compatibility path | Batch 106 起默认委托 Go；actual apply 写 managed docs / managed block / template create-if-missing / sync state；JSON preview 非写入；文本 apply what-if 仍回退 PowerShell dry-run；`REKIT_GO_DISABLE=1` 回退 PowerShell。 |
| Façade Go gate dry-run/apply | retired legacy façade gate dry-run/apply compatibility path | Batch 114 起 what-if 默认委托 Go 输出非写入 gate plan；Batch 115 起 apply 默认委托 Go 只写 pending-gate request；Batch 226 起 PowerShell fallback 已退休，`REKIT_GO_DISABLE=1` 报 no-fallback error；两者都不执行 heavy-tool、不写 confirmed/authority。 |
| Façade Go attach/repair lifecycle | legacy façade attach/repair lifecycle compatibility path | Batch 105 起默认委托 Go；attach/repair 预览非写入，显式 apply 只写 binding metadata、initial state 与 thin shim；`REKIT_GO_DISABLE=1` 回退 PowerShell。 |
| Façade Go init/bootstrap lifecycle | legacy façade init/bootstrap lifecycle compatibility path | Batch 105 起默认委托 Go；预览不创建 target，显式 apply 创建/刷新 case scaffold 与 managed content；裸命令保留 legacy fallback，`REKIT_GO_DISABLE=1` 回退 PowerShell。 |
| Façade Go promote candidate write / JSON preview | legacy façade promote candidate and JSON preview compatibility path | Batch 107 起 JSON 非写入 candidate preview 默认委托 Go；Batch 108 起实际 candidate/index/tooling candidate 写入也默认委托 Go；文本 create-candidates what-if 回退 PowerShell；`REKIT_GO_DISABLE=1` 回退。 |
| Façade Go promote apply / JSON preview | legacy façade promote apply and JSON preview compatibility path | Batch 107 起 JSON 非写入 pack apply preview 默认委托 Go；Batch 112 起实际 pack source 写入也默认委托 Go；文本 apply what-if 回退 PowerShell dry-run；`REKIT_GO_DISABLE=1` 回退。 |
| Façade Go overview text/JSON | legacy façade overview text/JSON compatibility path | Batch 119 起 attached case 的 overview 文本/JSON 默认委托 Go；缺 board 时初始化 case-local board/facts/policy/default authority lane，JSON envelope 标记 `isMutation=true`；后续读取保持只读；`REKIT_GO_DISABLE=1` 回退。 |
| Façade Go note list / append | legacy façade note list/append compatibility path | Batch 120 起 `note -List` 文本/table/tsv 与 JSON 默认委托 Go 输出只读 ledger events；Batch 116 起 attached case 的 append 与 `-WhatIf` 默认委托 Go 输出 JSON envelope，并只写 facts JSONL 或预览；`REKIT_GO_DISABLE=1` 回退。 |
| Façade Go start/handoff preview/apply | legacy façade start/handoff preview/apply compatibility path | Batch 117 起 attached case 的 start/handoff JSON preview 与显式 apply 默认委托 Go；文本 preview、无 `-Apply` 的文本工作线 workflow 与 `REKIT_GO_DISABLE=1` 回退 PowerShell；不写 authority/confirmed、不执行 heavy-tool。 |
| Façade Go continue JSON preview/apply | legacy façade continue JSON preview/apply compatibility path | Batch 118 起 JSON preview 默认委托 Go，输出非写入机器可读预览；Batch 121 起 explicit apply 默认委托 Go，写 case-local facts/routing/run digest/lane resume/checkpoint/board；文本预览、无 `-Apply` 的工作线 flow 与 `REKIT_GO_DISABLE=1` 回退 PowerShell；不写 authority/confirmed、不执行 heavy-tool。 |
| Façade Go plan-subagents artifacts | retired legacy façade plan-subagents artifact compatibility path | Batch 190 起默认委托 Go，返回 JSON result 并写 review packet / summary / combined diff 路径；Batch 225 起 PowerShell fallback 已退休，`REKIT_GO_DISABLE=1` 报 no-fallback error；不自动 spawn agent、不写 board/facts/lanes/handoff/authority。 |
| Façade smoke | `.\rekit\tests\facade-smoke.ps1` / `-CaseRoot <case> -Pack vmp-re` | 默认创建自包含临时 case，也可显式验证长期 case；覆盖 read-only、case lifecycle、sync、promote review/preview、promote candidate 写入与 promote apply 写入默认 Go 委托、continue JSON preview/apply 默认委托、plan-subagents review artifact 默认委托、已退休 fallback no-fallback 边界、overview 文本/JSON、note list 文本/JSON、note append、start/handoff JSON preview/apply 委托，以及剩余文本/write flags fallback；fake `REKIT_GO_EXE` sentinel 证明应委托组合确实经 Go façade。 |
| Legacy façade status | legacy status compatibility command | 仅在改 façade/fallback compatibility 时按需验证旧入口不回归。 |
| Legacy façade doctor | legacy doctor compatibility command | 仅在改 façade/fallback compatibility 时按需验证旧 doctor 不回归。 |
| Wrapper validate | `.\packs\vmp-re\scripts\validate.ps1` | 旧 wrapper 不回归。 |
| 临时 case vmp | init/overview/start/continue/handoff/sync review/doctor | 行为保持兼容。 |
| 临时 case template | init/overview/start/continue/handoff/sync review/doctor | 不泄漏 `vmp-re` / `devirt-main`。 |
| 空白检查 | `git diff --check` | 无 whitespace error。 |

## 新会话接手提示

Batch 101 后的新阶段总导航见 `docs/go-first-convergence-plan.md`。新会话如果要继续几十轮自主推进，应先读该文件顶部执行区和 `docs/batch-plan.md` 最新记录，避免继续沿 PowerShell smoke/catalog 扩张惯性推进。

如果从新会话继续，先确认 `main` 已包含 `4ae3f78 Add Go runtime skeleton`，然后运行：

```text
git pull origin main
git status --short
go run ./cmd/rekit -- -Command release-check -Format json
go run ./cmd/rekit -- -Command doctor
go test ./...
go vet ./...
```

接手时只需记住：

- `/rekit` 是公共 ABI，默认实现继续向 Go-native backend 收敛；Batch 104 起 read-only 默认委托 Go，Batch 105 起 case lifecycle 预览/显式 apply 默认委托 Go，Batch 106 起 `/rekit sync` review 与 `sync -Apply` 默认委托 Go，Batch 107 起 `/rekit promote` review 与 JSON what-if preview 默认委托 Go，Batch 108 起 `promote -CreateCandidates` 实际候选写入默认委托 Go，Batch 113 起 `note -List -Format json` 只读查询默认委托 Go，Batch 114 起 `gate -WhatIf` 非写入 preview 默认委托 Go，Batch 115 起 `gate -Apply` pending-gate request 写入默认委托 Go，Batch 116 起 `note` append / `note -WhatIf` facts JSONL 写入/预览默认委托 Go，Batch 117 起 `start`/`handoff` JSON preview 与显式 `-Apply` 默认委托 Go，Batch 118 起 `continue -WhatIf -Format json` 非写入 preview 默认委托 Go，Batch 119 起 `overview` 文本/JSON 与缺 board 初始化默认委托 Go，Batch 120 起 `note -List` 文本/table/tsv 只读查询默认委托 Go，Batch 121 起 explicit `continue -Apply` case-local 写入默认委托 Go；Batch 122/123/124 已用 `_template` pack Go CLI package E2E 覆盖 workstream、gate/dispatch 与 reviewer/decision 闭环，Batch 125 已用 `generic-binary-re` 覆盖 pack-neutral non-feature lane 闭环，Batch 126 已用 `web-security` 覆盖非 RE-only pack-neutral route/network gate 闭环；`REKIT_GO_DISABLE=1` 仅作为迁移期 fallback/diagnostic 开关。
- G1/G2.5 已完成 read-only doctor skeleton：`status`、pack `doctor/validate`、attached case `doctor/validate`、manifest/instance/runtime/CLI guard tests。
- G2 已完成 review-only JSON plan 与 G2.1 artifact 写入：`sync/promote` 只读 plan、bounded diff、sanitized preview、packet 输出；Batch 106 起 sync review artifacts 由默认 façade 委托 Go，Batch 107 起 promote review/artifacts 也默认委托 Go。
- G2.2 已完成第一版 Go gate dry-run：`gate -WhatIf` 输出非写入 pending-gate JSON plan，拒绝无 `-WhatIf` 调用，不执行 heavy-tool。
- G2.3 已完成 Go gate pending-gate ledger 写入：`gate -Apply` 只 append request JSONL，要求 `-Actor`，不执行 heavy-tool。
- G3.1 已完成 Go attach 手动路径：`attach -WhatIf` 只预览，`attach -Apply` 只写 `.rekit/instance.yml` 与 case-local thin shim；不写 managed docs、legacy metadata、state、board/facts/lanes，也不经 PowerShell façade 委托。
- G3.2 已完成 Go repair 手动路径：默认/`repair -WhatIf` 只预览，`repair -Apply` 只刷新 `.rekit/instance.yml`、`.re-template.yml` 与 case-local thin shim；不写 managed docs、board/facts/lanes 或 authority，也不经 PowerShell façade 委托。
- G3.3/G3.4 已完成 `sync -Apply` 迁移预研、review parity smoke 与 Go CLI 写入路径；Batch 79 已补 `sync -Apply -WhatIf` 非写入 JSON preview；Batch 106 起 `/rekit sync` review、`sync -Apply` 和 `sync -Apply -WhatIf -Format json` 默认委托 Go，文本 dry-run 与 `REKIT_GO_DISABLE=1` 继续 fallback：见 `docs/sync-apply-migration.md`。
- G3.5 已完成 `init/bootstrap -WhatIf/-Apply` Go CLI 手动路径：见 `docs/init-bootstrap-migration.md`。
- G3.6/G3.7 已完成 `promote -CreateCandidates` 迁移预研、preflight smoke 与 Go CLI 写入路径：见 `docs/promote-candidates-migration.md`；Batch 107 起公共 PowerShell façade 默认委托 `-CreateCandidates -WhatIf -Format json` 非写入预览，Batch 108 起实际 candidate/index/tooling candidate 写入也默认委托 Go。
- G3.8/G3.9 已完成 `promote -Apply` 迁移预研、preflight smoke 与 Go CLI 写入路径：见 `docs/promote-apply-migration.md`；Batch 107 起公共 PowerShell façade 默认委托 `-Apply -WhatIf -Format json` 非写入预览，Batch 112 起实际 pack source 写入也默认委托 Go；仍不迁移 authority/confirmed 写入。
- G4.1 已完成 `overview` Go CLI 路径：读取 `.rekit/board.json` 与 9 类 facts JSONL，输出项目总览；Batch 119 起缺 board 时初始化 case-local board/facts/policy/default authority lane，PowerShell façade 默认委托 overview 文本与 `-Format json`；初始化只写 case-local scaffold，不写 facts event、handoff、ledger、authority/confirmed 或 metadata。
- G4.2 已完成 `start` Go 路径：`-WhatIf` 非写入预览，`-Apply` 显式初始化 board/facts/policy/default authority lane 并创建或进入 feature lane；Batch 117 起 attached case 的 `start -WhatIf -Format json` 与 `start -Apply` 默认经 façade 委托 Go，文本 preview 与无 `-Apply` 文本 flow 仍回退 PowerShell。
- G4.3 已完成 `handoff` Go 路径：`-WhatIf` 非写入预览，`-Apply` 显式写项目级/工作线级 handoff 并刷新 lane resume/checkpoint；Batch 117 起 attached case 的 `handoff -WhatIf -Format json` 与 `handoff -Apply` 默认经 façade 委托 Go，文本 preview 与无 `-Apply` 文本 flow 仍回退 PowerShell。
- G4.4 已完成 `plan-subagents` Go CLI review artifact 手动路径：按 manifest `subagentRoutes` 生成分片 packet/summary；Batch 58 已补 route/shard/review-loop observability；Batch 123 已用 `_template` pack Go CLI package E2E 验证 plan-subagents packet 可进入 gate request、overview 与 handoff 可见性链路；Batch 125 已用 `generic-binary-re` 验证 pack-specific route 与 non-feature lane handoff；Batch 126 已用 `web-security` 验证非 RE-only route 与 network gate handoff；公共 PowerShell façade 仍不委托内部命令，不启动 agent、不写 board/facts/lanes/handoff/authority/confirmed。
- Batch 62-64 已新增只读 `packs` inventory，并将 pack maturity 固化为 manifest 显式字段：Go CLI 与 PowerShell fallback 均可列出全部 pack 的 maturity/schema/routes/managed/tooling/authority；`-Format json` 提供机器可读 envelope；Batch 104 起该命令默认经 façade 委托 Go。
- 无 `-Apply` 的工作线文本 workflow、内部命令、authority/confirmed 更新和 schema 迁移仍需单独确认；当前 façade 默认委托 read-only、overview 文本/JSON 与缺 board 初始化、note list 文本/table/tsv/JSON 只读查询、note append/what-if facts JSONL 写入/预览、`gate -WhatIf` 非写入 preview、`gate -Apply` pending-gate request 写入、start/handoff JSON preview 与显式 apply、`continue -WhatIf -Format json` 非写入 preview、explicit `continue -Apply` case-local 写入、case lifecycle、sync review/apply、promote review/artifact、promote candidate 写入、promote apply 写入与 promote JSON 非写入预览路径。

## 风险与止损

- 如果 Go parser 与 PowerShell parser 不一致，停止委托，只保留 Go test fixture，先修 parser。
- 如果 Go doctor 比 PowerShell doctor 更严格，必须先更新文档和 schema，再考虑启用委托。
- 如果 review packet 字段不兼容，Go review 只能作为实验输出，不可默认替代 PowerShell review。
- 如果任何写入命令迁移影响旧 case，立即 fallback 到 PowerShell，并以文档记录差异。
