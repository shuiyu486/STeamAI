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

状态：已完成第一版只读 plan skeleton；仍不默认接入 PowerShell façade。

目标：Go 接管 `sync/promote` review-only plan，不写 managed docs、不写 pack、不写 candidates。

已实现：

- `internal/rekit/review`：共享 plan/item、hash、deny pattern 和 managed block helper；
- `internal/rekit/sync`：生成 `sync` review-only plan，并拒绝 `-Apply` / `-WhatIf` 写入路径；
- `internal/rekit/promote`：生成 `promote` review-only plan，覆盖 deny pattern 和 tooling source sanitization metadata，并拒绝 `-Apply` / `-CreateCandidates` / `-WhatIf`；
- Go CLI 以 JSON 输出 review plan 到 stdout，不写 `.rekit/reviews/**`，因此 `ReviewOutputDir` / `PacketPath` / `DiffPath` 仍留给后续 G2.1；
- 轻量 tests 覆盖 review-only guard 与 attached-case guard。

后续 G2.1：补齐 review artifact 写入、bounded diff 文件、sanitized preview 文件与 PowerShell review packet 字段 parity。

不迁移：`sync -Apply`、`promote -CreateCandidates`、`promote -Apply`。

### G3：低风险写入命令迁移

顺序：

1. `attach`；
2. `repair`；
3. `sync -Apply`；
4. `init/bootstrap`；
5. `promote -CreateCandidates`；
6. 最后评估 `promote -Apply`。

每一步都必须有临时 case smoke、backup 检查和 review-first 边界验证。

### G4：工作线只读/低风险命令迁移

顺序：

1. `overview`；
2. `start`；
3. `handoff`；
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

首批不默认委托 Go，先通过维护者手动运行 Go CLI 验证。后续可以增加环境变量开关：

```text
REKIT_GO_ENABLE=1   # 允许 rekit.ps1 对已迁移命令委托 Go
REKIT_GO_DISABLE=1  # 强制 fallback PowerShell
```

即使启用委托，也只允许命令安全集合：

- `status`；
- `doctor` / `validate`；
- 后续 `sync/promote` review-only。

## 验证矩阵

| 场景 | 命令 | 预期 |
|---|---|---|
| Go status | `go run ./cmd/rekit -- -Command status` | 输出 runtime root、template root、pack、manifest counts。 |
| Go vmp doctor | `go run ./cmd/rekit -- -Command doctor` | pack validation ok。 |
| Go explicit root doctor | `go run ./cmd/rekit -- -Command doctor -Target .` | pack validation ok。 |
| Go non-case target doctor | `go run ./cmd/rekit -- -Command doctor -Target .\does-not-exist` | 报错，不得误报 pack validation ok。 |
| Go template doctor | `go run ./cmd/rekit -- -Command doctor -Pack _template` | pack validation ok，允许 no subagentRoutes warning。 |
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
- G1 已完成 read-only skeleton：`status`、pack `doctor/validate`、manifest/instance/runtime/CLI guard tests。
- G2 已完成第一版 review-only JSON plan skeleton：`sync/promote` 只读 plan，拒绝写入 flags。
- 下一批优先 G2.1：review artifact 写入、bounded diff、sanitized preview、packet 字段 parity 和临时 case golden tests。
- 在 G2.1 前不要启用默认 Go 委托；写入命令、authority/confirmed 更新和 schema 迁移仍需单独确认。

## 风险与止损

- 如果 Go parser 与 PowerShell parser 不一致，停止委托，只保留 Go test fixture，先修 parser。
- 如果 Go doctor 比 PowerShell doctor 更严格，必须先更新文档和 schema，再考虑启用委托。
- 如果 review packet 字段不兼容，Go review 只能作为实验输出，不可默认替代 PowerShell review。
- 如果任何写入命令迁移影响旧 case，立即 fallback 到 PowerShell，并以文档记录差异。
