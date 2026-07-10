# G3.8 promote -Apply 迁移预研

## 读取指南

- 只接手下一批实现时，先读“实施摘要”“执行清单”“验证标准”。
- 需要判断 PowerShell 与 Go 行为差异时，再读“PowerShell 当前语义基线”和“Go 迁移契约”。
- 本文只记录 G3.8 预研与验证资产；Go backend 仍不实现 `promote -Apply`，公共 `/rekit promote` 仍由 PowerShell façade / fallback 处理写入命令。

## 实施摘要

G3.8 不迁移 `promote -Apply` 写入路径，只固化当前 PowerShell baseline、Go 迁移契约和 preflight smoke。`promote -Apply` 会把 case managed docs 写回 pack source，风险高于 candidate 写入；因此下一步实现前必须先证明 backup、deny pattern、pack-root containment、validation 与 cleanup 行为可复现。

当前 Go backend 已支持 `promote` review-only artifact 和 G3.7 `promote -CreateCandidates` 手动路径；`promote -Apply` 仍应继续拒绝。后续如实现 Go helper，应作为显式手动 CLI 写入路径，不纳入 PowerShell façade 委托安全集合，不写 authority/confirmed，不执行 heavy-tool。

## 执行清单

- [x] 固化 PowerShell `promote -Apply` 与 `-Apply -WhatIf` 当前语义基线。
- [x] 固化 Go `promote -Apply` 迁移契约、边界和不迁移项。
- [x] 增加 `rekit/tests/promote-apply-preflight-smoke.ps1`，验证 PowerShell apply baseline、Go apply guard、façade fallback 与 cleanup。
- [x] 更新 `docs/batch-plan.md`、`docs/go-runtime-migration.md` 与 `CHANGELOG.md`。

## 验证标准

G3.8 预研完成标准：

```powershell
go test ./...
.\rekit\tests\promote-apply-preflight-smoke.ps1
.\rekit\tests\facade-smoke.ps1 -CaseRoot 'C:\AI\m_projects\RE\_dryrun_cases\agent-team-dryrun' -Pack vmp-re
.\rekit\rekit.ps1 -Command doctor
.\rekit\rekit.ps1 -Command doctor -Target 'C:\AI\m_projects\RE\_dryrun_cases\agent-team-dryrun'
git diff --check
```

必须额外确认：

- preflight smoke 使用临时 case，验证后删除。
- PowerShell `promote -Apply -WhatIf` 不写 pack source、不写 backup、不写 candidate。
- PowerShell `promote -Apply` baseline 若写入 pack source，smoke 必须在 `finally` 中恢复原文并清理新增 backup/candidate 文件。
- Go backend 仍拒绝 `promote -Apply`，直到后续批次明确实现写入 helper。
- façade 即使 `REKIT_GO_ENABLE=1` 也不得委托 `promote -Apply` 到 Go。

## 风险与注意事项

- `promote -Apply` 直接覆盖 pack managed docs；测试必须只使用 `_template` pack 的可恢复安全文本，并在 `finally` 中恢复。
- backup root 当前位于 `packs/<pack>/promote-candidates/.backup/**`；测试必须清理本次新增 backup，不能提交临时 backup/candidate。
- candidate 内容来自 case managed docs，必须先过 deny pattern；不能把 case-specific 进度、绝对路径、trace、dump、capture、artifact 或真实样本信息写入 pack。
- G3.8 不实现 Go `promote -Apply`，不覆盖 pack managed docs，不写 authority/confirmed，不执行 heavy-tool/debug/inject/patch/dump/network。
- PowerShell façade 仍不委托 `promote -Apply`；公共 `/rekit` 入口继续走 PowerShell。

## PowerShell 当前语义基线

`rekit/lib/Promote.ps1` 当前由 `Promote-RekitChanges` 处理 promote apply：

1. 无 `-Apply`、`-CreateCandidates`、`-WhatIf` 时拒绝写入，提示先 review 或显式写入。
2. `-Apply -WhatIf`：遍历 `promoteFiles`，对 changed + safe managed docs 输出 `would promote candidate: <rel>`；对 denied 内容输出 `blocked promote: <rel>`；不写 pack source、不写 backup。
3. `-Apply`：
   - 只处理 manifest `promoteFiles` 中同时属于 `managedFiles` 的路径；
   - case 文件缺失则 skip；case 与 pack 内容相同则 unchanged；pack 文件缺失则 error；
   - case 文本命中 manifest deny patterns 或 case-specific patterns 则 blocked；
   - 通过后先调用 `Backup-RekitPackFile` 备份原 pack 文件到 `promote-candidates/.backup/<timestamp>/<rel>`；
   - 再把 case 文本写入 pack source；
   - 写入后运行 `Test-RekitPack` 并打印预算结果；
   - 不写 candidate index，不写 tooling candidate。
4. tooling candidate 只在 `-CreateCandidates` 或任意 `-WhatIf` 时走 `Write-RekitToolingCandidates`；实际 `-Apply` 不写 tooling candidate。
5. summary 输出 `changed=<n> blocked=<n> toolingCandidates=<n> apply=<bool> createCandidates=<bool> whatIf=<bool>`。

## Go 迁移契约

后续如实现 Go `promote -Apply`，建议新增独立 helper，不把写入逻辑塞进 CLI：

```text
internal/rekit/promote
  Plan(...)              # 已存在：review-only plan + sanitized preview
  CreateCandidates(...)  # 已存在：显式写 candidate files / index
  Apply(...)             # 后续可选：显式写 pack managed docs + backup
```

必须保持：

- 裸 `promote` 默认 review-only。
- `promote -Apply` 只能作为显式手动 Go CLI 写入路径，暂不经 PowerShell façade 委托。
- `-Apply` 不得与 `-CreateCandidates`、review artifact options 混用。
- `-Apply -WhatIf` 如实现，只能输出非写入 JSON preview。
- 写入前必须复用 `Plan` 的 action、deny pattern 与 case-specific pattern 结果；只有 `candidate-after-llm-review` 可进入 apply 写入。
- pack source path 与 backup path 必须证明在 pack root 内；禁止越出 pack root。
- 写入前必须备份 pack source；写入后必须运行 pack validation；validation 失败时后续实现应优先设计自动恢复或显式失败恢复提示。
- 结果必须结构化输出 `isMutation`、`applied`、`changed`、`blocked`、`skipped`、`backupPath`、`writes`、`requiresReview`、`requiresCleanup`、`deniedWriteAction`。

## 测试矩阵

| 编号 | 场景 | 预期 |
|---|---|---|
| A1 | managed promote file unchanged | skip / unchanged，不写 pack。 |
| A2 | managed promote file changed 且无 deny | `-Apply -WhatIf` 显示 would promote；`-Apply` 备份后写 pack source。 |
| A3 | managed promote file 含绝对路径 / trace / artifact | blocked，不写 pack source。 |
| A4 | promote file 不属于 managedFiles | skip-non-managed-promote-file。 |
| A5 | case promote file 缺失 | skip-missing-case-file。 |
| A6 | pack source 缺失 | blocked/error，不静默创建 pack source。 |
| A7 | PowerShell `-Apply` backup | 新增 `.backup/<timestamp>/<rel>`，内容等于写入前 pack source。 |
| A8 | PowerShell `-Apply` validation | 写入后运行 pack validation；smoke 最终恢复原文并清理 backup。 |
| A9 | Go `promote -Apply` guard | 当前仍拒绝 `promote -Apply`。 |
| A10 | façade explicit Go enable + `promote -Apply` | 不委托 Go，继续走 PowerShell fallback。 |
