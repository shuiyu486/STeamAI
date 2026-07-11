# Go runtime migration plan

## 目的

本文件记录 `/rekit` runtime 从 PowerShell backend 渐进迁移到 Go backend 的实施方案，避免后续上下文压缩导致路线失真。

核心目标：在不破坏现有 `/rekit`、case-local thin shim、旧 case metadata、pack wrapper 和 review-first 边界的前提下，把越来越复杂的确定性 runtime 逻辑迁移到更易测试、调试和维护的 Go 模块中。

## 结论

采用 **PowerShell façade + Go deterministic backend** 的渐进迁移：

```text
用户 /rekit
  -> rekit/rekit.ps1               # 稳定公共入口，继续兼容旧 case 和 pack wrapper
     -> rekit/bin/rekit-go.exe     # 未来可选 Go backend；缺失或未覆盖命令时 fallback
        -> internal/rekit/*        # Go deterministic runtime modules
```

不要一次性重写全部 PowerShell runtime。首批 Go 只接管只读或 review-only 的确定性能力；涉及 authority 写入、confirmed CSV、外部工具、副作用和复杂工作线自动化的命令继续留在 PowerShell，直到有充分 parity test。

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

PowerShell remains the public ABI. Go modules should not import pack-specific logic; pack behavior must come from `packs/<pack>/manifest.yml` and managed docs.

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

验证：`go test ./...`、`git diff --check`、现有 `rekit.ps1 doctor`。

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
- 保持 `rekit.ps1` 不默认委托 Go，降低回归风险。

验证：

```powershell
go test ./...
go run ./cmd/rekit -- -Command status
go run ./cmd/rekit -- -Command doctor
go run ./cmd/rekit -- -Command doctor -Pack _template
.\rekit\rekit.ps1 status
.\rekit\rekit.ps1 doctor
.\packs\vmp-re\scripts\validate.ps1
git diff --check
```

### G2：Go review-only backend

状态：已完成 review-only plan 与 G2.1 artifact 写入；仍不默认接入 PowerShell façade。

目标：Go 接管 `sync/promote` review-only plan，不写 managed docs、不写 pack、不写 candidates；需要 review packet 时只写 `.rekit/reviews/**` 或显式指定的 artifact 路径。

已实现：

- `internal/rekit/review`：共享 plan/item、hash、deny pattern、managed block helper、bounded diff 与 review artifact writer；
- `internal/rekit/sync`：生成 `sync` review-only plan；G3.4 另支持显式 `sync -Apply` 手动写入路径，G3.5 支持 `init/bootstrap -WhatIf/-Apply` 手动路径；`sync` 仍拒绝 `-WhatIf`；
- `internal/rekit/promote`：生成 `promote` review-only plan，覆盖 deny pattern、tooling source sanitization metadata 与 sanitized preview 内容；G3.7 另支持显式 `promote -CreateCandidates` 手动路径（可配 `-WhatIf` 预览），G3.9 支持显式 `promote -Apply/-Apply -WhatIf` 手动路径；
- Go CLI 默认以 JSON 输出 review plan 到 stdout，`isMutation=false`、`reviewRequired=true`；
- G2.1 artifact 写入：`-ReviewOutputDir` / `-PacketPath` / `-DiffPath` 会输出 `packet.json`、`summary.md`、`diffs/combined.diff`、per-item bounded diff，以及 promote tooling candidate 的 `previews/*.sanitized-preview.md`；
- artifact 写入返回 `writesArtifacts=true`，仅代表写 review packet/diff/preview，不代表写 managed docs、pack 或 candidate；
- tests 覆盖 review-only guard、attached-case guard、sync artifact、promote artifact 与 sanitized preview。

不迁移：PowerShell façade 对 `sync -Apply`、`promote -CreateCandidates`、`promote -Apply` 的委托。

### G2.2：Go gate dry-run（D4）

状态：已完成第一版非写入 dry-run；仍不默认接入 PowerShell façade。

目标：避免继续扩大 PowerShell runtime，把 heavy-tool gate 的确定性预览逻辑先放入 Go backend。

已实现：

- 新增 `internal/rekit/gate`：生成非写入 JSON gate plan，`isMutation=false`、`reviewRequired=true`、`requiresConfirmation=true`。
- Go CLI 支持 `-Command gate -WhatIf`。
- gate plan 校验 attached case 与 `.rekit/board.json` lane id；未知 lane 直接拒绝。
- event preview 对齐 ledger request 语义：`kind=request`、`status=pending-gate`、可带 `target`/`batchId`，并内嵌 `gate` 详情（action、scope、budget、triedLightSteps、stopConditions、deniedUntilUserConfirmation）。
- D4 dry-run 只预览将登记什么、需要确认什么；不写 JSONL、不执行 full-trace/debug/inject/patch/dump/network、不自动授权。

不迁移：PowerShell `/rekit` 默认委托、heavy-tool 执行、用户确认后的 heavy-tool 执行闭环。

### G2.3：Go gate pending-gate ledger 写入

状态：已完成第一版显式写入；仍不默认接入 PowerShell façade。

目标：在 Go backend 内补上 gate preview → pending-gate request 的最小落账路径，但只写 ledger request，不执行 heavy-tool、不写 confirmed/authority。

已实现：

- Go CLI 支持 `-Command gate -Apply`，将 gate preview append 到 `.rekit/facts/requests.jsonl`。
- `gate -Apply` 要求显式 `-Actor`，记录谁批准写入 pending-gate request；无 `-Actor` 拒绝。
- `gate -WhatIf` 与 `gate -Apply` 互斥；无 `-WhatIf/-Apply` 拒绝，避免误操作。
- 写入事件仍是 `kind=request`、`status=pending-gate`，包含 `gate` 详情；`isMutation=true` 只表示写入 ledger request。
- eventId 由 kind/lane/subject/summary/actor/risk/target/batch/action/scope/budget/triedLightSteps/stopConditions 派生；重复 eventId 返回 `applied=false` 与 `reason=duplicate eventId`，不重复追加。
- 临时 case smoke 写入后已清理测试 request，未留下 D4/G2.3 测试状态。
- Batch 36 已补 PowerShell 读层 parity：`overview`、lane `handoff`、`note -List -Kind request` 会展示 `actor/risk/target/batchId/gate{action,scope,budget,triedLightSteps,stopConditions}` 语义字段；验证脚本见 `rekit/tests/gate-parity-smoke.ps1`。

不迁移：heavy-tool 执行、用户确认后的执行器、PowerShell 默认委托、confirmed/authority 写入。

### G3：低风险写入命令迁移

顺序：

1. `attach`（G3.1 已完成 Go CLI 手动路径：`-WhatIf` 预览、`-Apply` 只写 `.rekit/instance.yml` + thin shim；暂不经 PowerShell façade 委托）；
2. `repair`（G3.2 已完成 Go CLI 手动路径：默认/`-WhatIf` 预览，`-Apply` 刷新 `.rekit/instance.yml`、legacy `.re-template.yml` 与 thin shim；暂不经 PowerShell façade 委托）；
3. `sync -Apply`（G3.3 已完成预研与 review parity smoke；G3.4 已完成 Go CLI 手动写入路径，见 `docs/sync-apply-migration.md`；暂不经 PowerShell façade 委托）；
4. `init/bootstrap`（G3.5 已完成 Go CLI 手动路径：`-WhatIf` 非写入预览、`-Apply` 创建/刷新 case-local files 与 managed content；暂不经 PowerShell façade 委托）；
5. `promote -CreateCandidates`（G3.6 已完成迁移预研与 preflight smoke；G3.7 已完成 Go CLI 手动写入路径，见 `docs/promote-candidates-migration.md`）；
6. `promote -Apply`（G3.8 已完成迁移预研与 preflight smoke；G3.9 已完成 Go CLI 手动写入路径，见 `docs/promote-apply-migration.md`）。

每一步都必须有临时 case smoke、backup 检查和 review-first 边界验证。后续所有实施计划必须写入 `docs/batch-plan.md` 或本文件后再实施，并随代码一起提交推送到远程 `main`，避免只留在聊天上下文中。

### G4：工作线只读/低风险命令迁移

状态：G4.1 已完成 `overview` 只读手动路径；G4.2 已完成 `start -WhatIf/-Apply` 手动路径；G4.3 已完成 `handoff -WhatIf/-Apply` 手动路径；三者仍不纳入 PowerShell façade 委托，公共 `/rekit overview/start/handoff` 继续走 PowerShell 工作线语义。

顺序：

1. `overview`（G4.1 已完成 Go CLI 只读 renderer：读取既有 board/facts，不创建 board/facts/lanes，不写 handoff/ledger/metadata）；
2. `start`（G4.2 已完成 Go CLI 手动路径：`-WhatIf` 非写入预览，`-Apply` 显式创建/进入 feature lane 并初始化 board/facts/policy/default authority lane）；
3. `handoff`（G4.3 已完成 Go CLI 手动路径：`-WhatIf` 非写入预览，`-Apply` 显式写项目级/工作线级 handoff 并刷新 lane resume/checkpoint）；
4. `plan-subagents`。

`continue` 仍保留 PowerShell，直到 authority gate 测试完善。

### G5：`continue` 与 policy gate 迁移

只有在以下 gate tests 完整后再迁移：

- evidence required；
- accepted verifier verdict；
- confidence threshold；
- CSV schema valid；
- no conflict；
- backup created；
- bounded diff created；
- max rows；
- authorityFiles allowlist；
- append 后 CSV 失败可恢复 backup。

### G6：PowerShell 收敛为 wrapper

条件：所有命令 parity 通过，旧 case smoke 通过，case-local shim 和 pack wrapper 不需要改语义。

最终形态：`rekit.ps1` 只负责参数兼容、Go binary 查找、fallback 和错误透传。

## PowerShell façade 策略

G2.4 已增加显式环境变量开关；默认仍走 PowerShell，只有维护者主动启用时才委托 Go：

```text
REKIT_GO_ENABLE=1   # 允许 rekit.ps1 对已迁移命令委托 Go
REKIT_GO_DISABLE=1  # 强制 fallback PowerShell，优先级高于 ENABLE
REKIT_GO_EXE=...    # 可选：指定已构建的 rekit-go.exe；未指定时优先 rekit/bin/rekit-go.exe，否则 fallback 到 go run ./cmd/rekit --
```

即使启用委托，也只允许命令安全集合：

- `status`；
- kit-root 与 attached case `doctor` / `validate`；
- `sync/promote` review-only（含 `-ReviewOutputDir` / `-PacketPath` / `-DiffPath` artifact 写入）；
- `gate -WhatIf` dry-run（仅输出非写入 plan，不执行 heavy-tool、不写 ledger）。

不委托：`attach`、`repair`、`gate -Apply`、`sync -Apply`、`init/bootstrap -Apply`、`promote -Apply/-CreateCandidates`、工作线命令、ledger `note`。其中 `overview` 可手动运行 Go CLI 只读验证；`start -WhatIf/-Apply`、`handoff -WhatIf/-Apply`、`attach`、`repair`、`sync -Apply`、`init/bootstrap -WhatIf/-Apply`、`promote -CreateCandidates`、`promote -Apply/-Apply -WhatIf` 可手动运行 Go CLI 验证；公共 `/rekit` 入口继续由 PowerShell 处理工作线命令和写入命令。

## 验证矩阵

| 场景 | 命令 | 预期 |
|---|---|---|
| Go status | `go run ./cmd/rekit -- -Command status` | 输出 runtime root、template root、pack、manifest counts。 |
| Go vmp doctor | `go run ./cmd/rekit -- -Command doctor` | pack validation ok。 |
| Go explicit root doctor | `go run ./cmd/rekit -- -Command doctor -Target .` | pack validation ok。 |
| Go case doctor | `go run ./cmd/rekit -- -Command doctor -Target <case> -Pack vmp-re` | attached case 结构只读验证，输出 `instance validation ok`。 |
| Go non-case target doctor | `go run ./cmd/rekit -- -Command doctor -Target .\does-not-exist` | 报错，不得误报 pack validation ok。 |
| Go template doctor | `go run ./cmd/rekit -- -Command doctor -Pack _template` | pack validation ok，允许 no subagentRoutes warning。 |
| Go sync review artifacts | `go run ./cmd/rekit -- -Command sync -Target <case> -Pack vmp-re -ReviewOutputDir <dir>` | 写 `packet.json`、`summary.md`、`diffs/combined.diff`，返回 `isMutation=false` / `writesArtifacts=true`。 |
| Go sync apply | `go run ./cmd/rekit -- -Command sync -Target <case> -Pack vmp-re -Apply` | 手动写入 managed docs / managed block / template create-if-missing / sync state，返回 `isMutation=true`；不经 PowerShell façade 委托。 |
| Go init preview | `go run ./cmd/rekit -- -Command init -Target <newCase> -Pack vmp-re -WhatIf` | 输出非写入 JSON plan，`isMutation=false` / `requiresConfirmation=true`，不创建 target。 |
| Go init/bootstrap apply | `go run ./cmd/rekit -- -Command init -Target <newCase> -Pack vmp-re -Apply` | 手动创建 metadata / shim / legacy metadata / managed docs / managed block / template create-if-missing / sync state，返回 `isMutation=true`；不经 PowerShell façade 委托。 |
| Go promote review artifacts | `go run ./cmd/rekit -- -Command promote -Target <case> -Pack vmp-re -ReviewOutputDir <dir>` | 写 review packet、bounded diff 和 tooling sanitized preview，不写 pack/candidates。 |
| Promote candidates preflight | `.\rekit\tests\promote-candidates-preflight-smoke.ps1` | 验证 PowerShell `promote -WhatIf -CreateCandidates` baseline、Go promote review artifact/sanitized preview parity、Go `-CreateCandidates -WhatIf` 非写入与 façade fallback；不写 pack candidates。 |
| Go promote create candidates | `.\rekit\tests\promote-candidates-apply-smoke.ps1` | 手动 Go CLI 写 candidate/index/tooling candidate，验证 blocked deny、sanitization、pack-root containment 与 cleanup；不经 PowerShell façade 委托。 |
| Promote apply preflight | `.\rekit\tests\promote-apply-preflight-smoke.ps1` | 验证 PowerShell `promote -Apply` baseline、backup、deny、pack-root cleanup、Go apply what-if 与 façade fallback。 |
| Go promote apply | `.\rekit\tests\promote-apply-smoke.ps1` | 手动 Go CLI 写 pack managed docs，验证 what-if 非写入、backup、blocked deny、validation rows、pack-root containment、tooling 不写入、cleanup 与 façade fallback；不经 PowerShell façade 委托。 |
| Go gate dry-run | `go run ./cmd/rekit -- -Command gate -Target <case> -Pack vmp-re -WhatIf -Action full-trace -Lane <lane>` | 输出非写入 JSON plan，`eventPreview.kind=request`、`status=pending-gate`、`requiresConfirmation=true`。 |
| Go gate no-mode guard | `go run ./cmd/rekit -- -Command gate -Target <case> -Action debug -Lane <lane>` | 报错，必须显式选择 `-WhatIf` 或 `-Apply`。 |
| Go gate apply | `go run ./cmd/rekit -- -Command gate -Target <case> -Pack vmp-re -Apply -Action debug -Lane <lane> -Actor <user>` | 只 append `.rekit/facts/requests.jsonl` pending-gate request，返回 `isMutation=true`，不执行工具。 |
| Gate request display parity | `.\rekit\tests\gate-parity-smoke.ps1` | Go `gate -Apply` 写入的 pending-gate request 在 PowerShell `overview`、`note -List`、lane `handoff` 中展示 actor/risk/target/batch/gate 详情。 |
| Go overview readonly | `.\rekit\tests\overview-readonly-smoke.ps1` | 手动 Go CLI 读取既有 board/facts 输出项目总览；缺 board 时拒绝，不创建 board/facts/lanes；PowerShell façade 即使显式 Go enable 仍不委托 `overview`。 |
| Go start apply | `.\rekit\tests\start-apply-smoke.ps1` | 手动 Go CLI `start -WhatIf/-Apply` 预览或创建 feature lane，验证 board/facts/policy/lane/workspace scaffold、doctor 与 façade fallback；不经 PowerShell façade 委托。 |
| Go handoff apply | `.\rekit\tests\handoff-apply-smoke.ps1` | 手动 Go CLI `handoff -WhatIf/-Apply` 预览或写项目级/工作线级 handoff，验证 lane resume/checkpoint、ledger 展示区段、doctor 与 façade fallback；不经 PowerShell façade 委托。 |
| Go attach preview | `go run ./cmd/rekit -- -Command attach -Target <case> -Pack vmp-re -WhatIf` | 输出非写入 plan，不创建目录或文件。 |
| Go attach apply | `go run ./cmd/rekit -- -Command attach -Target <case> -Pack vmp-re -ProjectName <name> -Apply` | 只写 `.rekit/instance.yml` 与 case-local thin shim，不写 managed docs、board/facts/lanes、legacy metadata 或 state。 |
| Go repair preview | `go run ./cmd/rekit -- -Command repair -Target <movedCase> -Pack vmp-re -WhatIf` | 输出非写入 repair plan，展示 recorded/new projectRoot 与写入计划。 |
| Go repair apply | `go run ./cmd/rekit -- -Command repair -Target <movedCase> -Pack vmp-re -Apply` | 只刷新 `.rekit/instance.yml`、`.re-template.yml` 与 case-local thin shim，不写 managed docs、board/facts/lanes 或 authority。 |
| Façade Go status | `$env:REKIT_GO_ENABLE='1'; .\rekit\rekit.ps1 -Command status` | 通过显式开关委托 Go，输出 `rekit go backend`。 |
| Façade Go sync review artifacts | `$env:REKIT_GO_ENABLE='1'; .\rekit\rekit.ps1 -Command sync -Target <case> -ReviewOutputDir <dir>` | 委托 Go 写 review artifacts，不写 managed docs。 |
| Façade Go gate dry-run | `$env:REKIT_GO_ENABLE='1'; .\rekit\rekit.ps1 -Command gate -Target <case> -WhatIf -Action debug -Lane <lane>` | 委托 Go 输出非写入 gate plan；无 ENABLE 时拒绝并提示手动 Go。 |
| Façade smoke | `.\rekit\tests\facade-smoke.ps1 -CaseRoot <case> -Pack vmp-re` | 覆盖默认不委托、显式安全集合、disable 优先级和写入 flags fallback。 |
| PowerShell status | `.\rekit\rekit.ps1 status` | 现有入口不回归。 |
| PowerShell doctor | `.\rekit\rekit.ps1 doctor` | 现有 doctor 不回归。 |
| Wrapper validate | `.\packs\vmp-re\scripts\validate.ps1` | 旧 wrapper 不回归。 |
| 临时 case vmp | init/overview/start/continue/handoff/sync review/doctor | 行为保持兼容。 |
| 临时 case template | init/overview/start/continue/handoff/sync review/doctor | 不泄漏 `vmp-re` / `devirt-main`。 |
| 空白检查 | `git diff --check` | 无 whitespace error。 |

## 新会话接手提示

如果从新会话继续，先确认 `main` 已包含 `4ae3f78 Add Go runtime skeleton`，然后运行：

```powershell
git pull origin main
git status --short
go test ./...
go vet ./...
.\rekit\rekit.ps1 doctor
```

接手时只需记住：

- PowerShell 仍是公共入口，Go backend 还没有被 `rekit.ps1` 默认调用。
- G1/G2.5 已完成 read-only doctor skeleton：`status`、pack `doctor/validate`、attached case `doctor/validate`、manifest/instance/runtime/CLI guard tests。
- G2 已完成 review-only JSON plan 与 G2.1 artifact 写入：`sync/promote` 只读 plan、bounded diff、sanitized preview、packet 输出；promote 写入 flags 仍拒绝，sync 写入仅限后续 G3 手动路径。
- G2.2 已完成第一版 Go gate dry-run：`gate -WhatIf` 输出非写入 pending-gate JSON plan，拒绝无 `-WhatIf` 调用，不执行 heavy-tool。
- G2.3 已完成 Go gate pending-gate ledger 写入：`gate -Apply` 只 append request JSONL，要求 `-Actor`，不执行 heavy-tool。
- G3.1 已完成 Go attach 手动路径：`attach -WhatIf` 只预览，`attach -Apply` 只写 `.rekit/instance.yml` 与 case-local thin shim；不写 managed docs、legacy metadata、state、board/facts/lanes，也不经 PowerShell façade 委托。
- G3.2 已完成 Go repair 手动路径：默认/`repair -WhatIf` 只预览，`repair -Apply` 只刷新 `.rekit/instance.yml`、`.re-template.yml` 与 case-local thin shim；不写 managed docs、board/facts/lanes 或 authority，也不经 PowerShell façade 委托。
- G3.3/G3.4 已完成 `sync -Apply` 迁移预研、review parity smoke 与 Go CLI 手动写入路径：见 `docs/sync-apply-migration.md`。
- G3.5 已完成 `init/bootstrap -WhatIf/-Apply` Go CLI 手动路径：见 `docs/init-bootstrap-migration.md`。
- G3.6/G3.7 已完成 `promote -CreateCandidates` 迁移预研、preflight smoke 与 Go CLI 手动写入路径：见 `docs/promote-candidates-migration.md`；公共 PowerShell façade 仍不委托该写入命令。
- G3.8/G3.9 已完成 `promote -Apply` 迁移预研、preflight smoke 与 Go CLI 手动写入路径：见 `docs/promote-apply-migration.md`；公共 PowerShell façade 仍不委托该写入命令，不迁移 authority/confirmed 写入。
- G4.1 已完成 `overview` Go CLI 只读路径：读取既有 `.rekit/board.json` 与 9 类 facts JSONL，输出项目总览；不创建 board/facts/lanes，不写 handoff/ledger/metadata，公共 PowerShell façade 仍不委托工作线命令。
- G4.2 已完成 `start` Go CLI 手动路径：`-WhatIf` 非写入预览，`-Apply` 显式初始化 board/facts/policy/default authority lane 并创建或进入 feature lane；公共 PowerShell façade 仍不委托工作线命令。
- G4.3 已完成 `handoff` Go CLI 手动路径：`-WhatIf` 非写入预览，`-Apply` 显式写项目级/工作线级 handoff 并刷新 lane resume/checkpoint；公共 PowerShell façade 仍不委托工作线命令。
- 在 PowerShell façade 默认委托前，继续用手动 Go CLI smoke 验证；写入命令、工作线命令、authority/confirmed 更新和 schema 迁移仍需单独确认。

## 风险与止损

- 如果 Go parser 与 PowerShell parser 不一致，停止委托，只保留 Go test fixture，先修 parser。
- 如果 Go doctor 比 PowerShell doctor 更严格，必须先更新文档和 schema，再考虑启用委托。
- 如果 review packet 字段不兼容，Go review 只能作为实验输出，不可默认替代 PowerShell review。
- 如果任何写入命令迁移影响旧 case，立即 fallback 到 PowerShell，并以文档记录差异。
