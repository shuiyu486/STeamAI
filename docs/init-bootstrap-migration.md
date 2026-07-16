# G3.5 init/bootstrap 迁移记录

## 读取指南

- 只接手实现时，先读“实施摘要”“执行清单”“验证标准”。
- 需要判断与 PowerShell 行为的差异时，再读“PowerShell 当前语义基线”和“Go 迁移契约”。
- 本计划原本记录 G3.5 Go CLI 手动验证路径；Batch 105 已把 `init/bootstrap` 预览与显式 `-Apply` 纳入默认 Go façade 委托，Batch 230 已退休 case lifecycle PowerShell fallback，公共 `/rekit` 入口在 `REKIT_GO_DISABLE=1` 或 Go delegation 不可用时直接 no-fallback 失败。

## 实施摘要

G3.5 已在 Go backend 中补齐 `init/bootstrap` 的手动路径，维护者可以通过 Go CLI 初始化临时 case 并验证 Go 写入语义。该路径本质上是“允许 missing target 的 sync apply”：先创建/刷新 binding metadata 与 thin shim，再按 manifest 落地 managed files、template files、managed block、support file 和 sync state。

实现保持 review-first / 写入边界：Go `init/bootstrap` 只在显式 `-Apply` 时写入；`-WhatIf` 仅输出非写入 JSON plan；Batch 105 起公共 façade 默认委托该预览/显式写入路径，但裸 `init/bootstrap` 仍拒绝，不执行 promote，不写 authority/confirmed，不执行 heavy-tool/debug/inject/patch/dump/network。

## 执行清单

- [x] 固化 PowerShell `init/bootstrap` 当前语义基线。
- [x] 固化 Go G3.5 迁移契约、边界与验证矩阵。
- [x] 在 Go sync/apply 层增加允许 missing target 的 create-local-files 模式，保留 existing attached case 的 moved/different binding guard。
- [x] 在 Go CLI 中新增 `init` / `bootstrap` 手动命令，要求显式 `-WhatIf` 或 `-Apply`。
- [x] 增加 Go tests 覆盖 missing target apply、what-if 不写文件、existing same binding refresh、different binding/moved guard、`-Force` template overwrite。
- [x] 增加 `rekit/tests/init-bootstrap-smoke.ps1`，使用临时 case 验证 Go init/bootstrap apply、backup/state/doctor；Batch 105 后同步验证公共 façade 默认委托，Batch 230 后同步验证 disabled no-fallback。
- [x] 更新 `README.md`、`docs/go-runtime-migration.md`、`docs/batch-plan.md` 与 `CHANGELOG.md`。

## 验证标准

G3.5 完成标准：

```powershell
go test ./...
.\rekit\tests\init-bootstrap-smoke.ps1
.\rekit\tests\facade-smoke.ps1 -CaseRoot 'C:\AI\m_projects\RE\_dryrun_cases\agent-team-dryrun' -Pack vmp-re
.\rekit\rekit.ps1 -Command doctor
.\rekit\rekit.ps1 -Command doctor -Target 'C:\AI\m_projects\RE\_dryrun_cases\agent-team-dryrun'
git diff --check
```

必须额外确认：

- `init -WhatIf` 不创建 target 目录、metadata、managed files 或 review artifacts。
- `init -Apply` 对 missing target 创建完整 case-local runtime 文件，并通过 Go doctor 与 PowerShell doctor。
- `bootstrap` 与 `init` 使用同一语义，只在输出 `command` 字段上反映调用命令。
- `-Force` 只影响 local template files，不改变 managed files overwrite-with-backup 语义。
- Batch 105 起 PowerShell façade 默认委托 `init/bootstrap -WhatIf` 与显式 `-Apply` 到 Go；Batch 230 起 `REKIT_GO_DISABLE=1` 或 Go delegation 不可用时必须报 no-fallback error，不再回退 PowerShell。

## 风险与注意事项

- `init/bootstrap` 会创建新 case 并写 managed docs，比 `attach` 风险更高；Go CLI 与默认 Go façade 委托路径都必须要求显式 `-Apply`，不能把裸 `init` 当作 Go 写入授权；Batch 230 起裸命令与 disabled façade 均直接 no-fallback，文档与新流程应显式写 `-Apply`。
- 必须拒绝把 kit repo root 当作 init target，避免在本仓库内伪造 case state。
- missing target 只允许来自显式 `-Target`；不得从空 target、cwd 或 README 示例中隐式创建 case。
- 已 attached 到同一 templateRoot/templatePack 的 case 可以刷新；moved case、different templateRoot、different templatePack 必须拒绝。
- `overview` / `continue` / `handoff` 仍负责 lazy 初始化 board/facts/lanes；G3.5 不强行扩展 init 的工作线状态写入，避免制造与 PowerShell init 的并行语义。
- 不实现自动 rollback；如果 apply 中途失败，只报告错误和已创建的 backup，人工恢复。

## PowerShell 当前语义基线

`rekit/rekit.ps1` 当前把 `init` 与 `bootstrap` 作为同义命令处理：

```powershell
Sync-RekitPack -Target $caseRoot -RepoRoot $RepoRoot -Pack $Pack -ProjectName $ProjectName -WhatIf:$WhatIf -CreateLocalFiles -Apply -ForceLocalTemplates:$Force
```

核心行为：

1. `CreateLocalFiles` 允许 target 没有现有 `.rekit/instance.yml` 或 `.re-template.yml`。
2. 写入前调用 `Invoke-RekitAttach`，进而执行 `Write-RekitInstance` 与 `Write-RekitCaseShim`。
3. `Write-RekitInstance` 创建/刷新 `.rekit/instance.yml`，创建/更新 legacy `.re-template.yml`，并在缺失时创建初始 `.rekit/state.json`。
4. managed files 从 pack 覆盖到 case，目标存在且内容不同则先 backup。
5. template files 默认 create-if-missing；显式 `-Force` 才覆盖本地模板文件并 backup。
6. managed block 写入 manifest 指定 host，host 存在时先 backup。
7. support `.gitignore` 仅在 pack example 存在且 case target 缺失时创建。
8. 最后刷新 `.rekit/state.json`，记录 managed files 的 `sourceHash`、`targetHashAtSync` 与 `lastAction: sync`。

PowerShell init 不主动创建 board/facts/lanes；这些仍由 `overview` / `continue` / `handoff` 等 B3 工作线命令按需创建。

## Go 迁移契约

建议复用 G3.4 `sync.Apply` 的写入实现，但增加受控的 create-local-files 模式：

```text
internal/rekit/sync
  Plan(...)          # existing attached case review-only
  Apply(...)         # existing attached case sync apply
  InitPlan(...)      # new: missing target safe preview, optional
  Apply(...CreateLocalFiles=true, Command="init"|"bootstrap")
```

Go CLI 行为：

- `go run ./cmd/rekit -- -Command init -Target <case> -Pack vmp-re -WhatIf`：输出非写入 JSON plan，`isMutation=false`、`requiresConfirmation=true`。
- `go run ./cmd/rekit -- -Command init -Target <case> -Pack vmp-re -Apply`：创建/刷新 case-local 文件并输出 JSON result，`isMutation=true`、`applied=true`。
- `bootstrap` 与 `init` 同义，`command` 字段保留用户输入命令。
- 裸 `init/bootstrap` 拒绝，提示使用 `-WhatIf` 预览或 `-Apply` 写入。
- `-WhatIf` 与 `-Apply` 互斥。
- review artifact options 不与 `init/bootstrap` 混用。
- Batch 105 起经 PowerShell façade 默认委托；公共 `/rekit init -WhatIf` 与 `/rekit init -Apply` 默认走 Go；Batch 230 起 `REKIT_GO_DISABLE=1` 与 Go delegation 不可用均报 no-fallback error。

路径与 guard：

- 所有 case target path 必须取绝对路径，并通过 safe join 写子路径。
- target 等于 kit repo root 时拒绝。
- target missing 或无 metadata 时允许 `CreateLocalFiles`。
- target 已 attached 到同一 repoRoot/pack 时允许 refresh。
- target moved、different templateRoot、different templatePack 时拒绝。

## 测试矩阵

| 编号 | 场景 | 预期 |
|---|---|---|
| I1 | missing target 执行 `init -WhatIf` | 输出 JSON preview，不创建目录或文件。 |
| I2 | missing target 执行裸 `init` | 拒绝，提示 `-WhatIf` 或 `-Apply`。 |
| I3 | missing target 执行 `init -Apply` | 创建 metadata、shim、managed files、template file、managed block、support file、state。 |
| I4 | `bootstrap -Apply` | 与 `init -Apply` 同语义，输出 command 为 `bootstrap`。 |
| I5 | target 是 kit repo root | 拒绝，不写 `.rekit` case state。 |
| I6 | existing same binding case 执行 `init -Apply` | 等价 sync apply refresh，changed managed files 有 backup。 |
| I7 | moved case 执行 `init -Apply` | 拒绝，提示先 repair。 |
| I8 | different templateRoot/templatePack | 拒绝，不静默重绑。 |
| I9 | local template exists and no force | skip，不覆盖。 |
| I10 | local template exists with `-Force` | backup 后覆盖并替换 placeholders。 |
| I11 | managed block host exists | backup 后 replace/append block，保留 block 外内容。 |
| I12 | `.gitignore` target exists | skip，不覆盖。 |
| I13 | apply 后 Go doctor | 通过。 |
| I14 | apply 后 PowerShell doctor | 通过。 |
| I15 | 默认 PowerShell `/rekit init/bootstrap -WhatIf/-Apply` | 委托 Go；fake backend sentinel 能证明默认委托。 |
| I16 | `REKIT_GO_DISABLE=1` 下 PowerShell `/rekit init/bootstrap` | 不委托 Go，报 no-fallback error，不写 target。 |
