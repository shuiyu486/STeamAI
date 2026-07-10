# G3.3 sync -Apply 迁移预研

## 读取指南

- 只接手实现时，先读“实施摘要”“执行清单”“验证标准”。
- 需要判断语义差异时，再读“PowerShell 当前语义基线”和“Go 迁移契约”。
- G3.3 时本文是迁移前预研；G3.4 已补 Go `sync -Apply` 手动路径，但当前用户入口仍以 PowerShell façade / fallback 为准，Go 写入命令不经 façade 委托。

## 实施摘要

G3.3 的目标不是直接迁移 `sync -Apply`，而是先把未来迁移必须保持的写入语义、风险点和验证矩阵固化下来。`sync -Apply` 会覆盖 managed docs、写 managed block、创建本地模板文件、刷新 metadata/shim 和更新 state，因此必须比 `attach`/`repair` 更谨慎。

结论：G3.4 已实现 Go `sync -Apply` 内部 helper 与临时 case smoke，仍不纳入 PowerShell façade 委托；后续只有在 Go/PowerShell apply 输出进一步扩展为双临时 case parity，并持续证明 backup/state/doctor 不回归后，才考虑显式委托。

## 执行清单

- [x] 固化 PowerShell `sync -Apply` 当前语义基线。
- [x] 固化未来 Go `sync -Apply` 的迁移契约和不迁移边界。
- [x] 固化 backup、bounded diff、managed block、template local-file skip、失败恢复、旧 case compatibility 的测试矩阵。
- [x] 增加 review-only parity smoke：`rekit/tests/sync-review-parity-smoke.ps1`。
- [x] G3.4 实现 Go `sync -Apply` 内部 helper。
- [x] G3.4 增加临时 case smoke，验证 Go apply 写入、backup、state、doctor；双实现内容 parity 后续继续扩展。
- [ ] 后续批次评估是否把 `sync -Apply` 加入显式 Go façade 安全集合；默认仍不委托。

## 验证标准

当前 G3.3 预研完成标准：

```powershell
.\rekit\tests\sync-review-parity-smoke.ps1
go test ./...
.\rekit\tests\facade-smoke.ps1 -CaseRoot 'C:\AI\m_projects\RE\_dryrun_cases\agent-team-dryrun' -Pack vmp-re
.\rekit\rekit.ps1 -Command doctor
.\rekit\rekit.ps1 -Command doctor -Target 'C:\AI\m_projects\RE\_dryrun_cases\agent-team-dryrun'
git diff --check
```

Go `sync -Apply` 实现完成标准：

- Go CLI `sync -Apply` 只接受显式 `-Target` attached case。
- 默认 `sync` 仍是 review-only；无 `-Apply` 不写 managed docs。
- Go `sync -Apply` 不执行 promote、不写 pack、不写 authority/confirmed、不执行 heavy-tool。
- 临时 case 中 PowerShell apply 与 Go apply 后的 managed files、template targets、managed block host、metadata/shim、state hash 内容一致；允许 backup 目录时间戳不同。
- apply 后 Go doctor 与 PowerShell doctor 均通过。
- 失败注入时不静默吞错；已备份文件可用于人工恢复。

## 风险与注意事项

- `sync -Apply` 是实质 case 写入命令；不能因为 Go review plan 已存在就直接接管。
- PowerShell 当前没有事务性 rollback；未来 Go 不应声称具备自动回滚，除非先实现并验证。
- `managedBlock` 的 PowerShell apply 会在 host 存在时先备份再写入；review 可能显示 `unchanged`，这是一个需要迁移时明确处理的历史语义。
- `templateFiles` 默认只 create-if-missing；只有显式 force 路径才允许覆盖本地模板文件。
- Go `attach -Apply` 当前不写 legacy `.re-template.yml` / `state.json`，但 PowerShell `sync -Apply` 会经 `Invoke-RekitAttach` 刷新 legacy metadata 和 state。Go sync apply 已单独补 legacy metadata 与 sync state 写入，不能退化为 Go attach 的当前最小写入语义。
- 旧 case 可能只有 `.re-template.yml`，也可能存在迁移后的 `.rekit/instance.yml`；迁移必须保持 `AssertAttached` / repair guard 语义。

## PowerShell 当前语义基线

### 入口与 guard

`rekit/lib/Sync.ps1` 的 `Sync-RekitPack` 是当前 canonical 写入实现：

1. 读取 manifest 与 instance。
2. 如果 target 未 attached 且不是 `init/bootstrap` 路径，拒绝。
3. 如果 metadata 指向不同 projectRoot，要求先 `repair -Apply`。
4. `-Review` 时只写 review artifact。
5. 无 `-Apply` 且无 `-WhatIf` 时拒绝写入，提示先 review。
6. 写入前调用 `Invoke-RekitAttach`，刷新 `.rekit/instance.yml`、legacy `.re-template.yml`、`.rekit/state.json`（若缺失）和 case-local thin shim。

### 写入顺序

1. 计算 backup root：`<manifest.workstreamDefaults.backupRoot>/<timestamp>`。
2. `managedFiles`：逐个从 pack copy 到 case。目标存在且内容不同则先 backup。
3. `templateFiles`：`.template.md` 转为 `.md`；目标存在且未 force 时 skip；缺失时替换 `<PROJECT_NAME>` / `<PROJECT_ROOT>` 后创建；force 时先 backup 再覆盖。
4. `managedBlock`：把 pack snippet 写入 `managedBlock.file` 的 block；host 存在时先 backup。
5. `.gitignore` support file：仅当 pack example 存在且 case 缺失时创建。
6. 更新 `.rekit/state.json`，记录每个 managed file 的 sourceHash 和 targetHashAtSync。
7. 运行 `Test-RekitInstance`，输出 `sync ok`。

### Review artifact 语义

PowerShell review 包会提前展示：

- action：`create-managed-file` / `overwrite-with-backup` / `unchanged` / `skip-existing-local-file` / `create-local-template-file` / `replace-managed-block` 等。
- managed file 的 `lastSyncHash` 与 `changedSinceLastSync`。
- backup preview 路径。
- bounded diff 文件。

Go review plan 当前已经覆盖非写入 plan、bounded diff 和主要 action，但未来 apply 迁移前仍要确保 state/backup/force 相关字段不会造成误导。

## Go 迁移契约

下一批实现 Go `sync -Apply` 时建议采用独立 package helper，而不是把写入塞进 CLI：

```text
internal/rekit/sync
  Plan(...)         # 已存在：review-only
  Apply(...)        # 后续新增：显式写入
```

`Apply` 输出应是结构化 JSON：

```json
{
  "schemaVersion": 1,
  "command": "sync",
  "caseRoot": "...",
  "repoRoot": "...",
  "pack": "vmp-re",
  "isMutation": true,
  "applied": true,
  "backupRoot": "...",
  "writes": [
    {"path":"references/...","kind":"managed-file","action":"overwrite-with-backup","backupPath":"..."}
  ],
  "nextSteps": ["run doctor", "review backup if any"]
}
```

必须保持：

- `sync` 默认 review-only。
- `sync -Apply` 显式写入，但暂不经 PowerShell façade 委托。
- `-WhatIf` 不与 `-Apply` 混用；如果实现 Go dry-run，也必须保持不写文件。
- 所有 target path 通过 safe join，禁止越出 case root。
- backup root 来自 manifest，且必须在 case root 下。
- 写入 `.rekit/state.json` 前应基于实际写入后的 target hash 计算。

## 测试矩阵

| 编号 | 场景 | 预期 |
|---|---|---|
| S1 | 普通目录执行 `sync -Apply` | 拒绝，不创建假 case。 |
| S2 | moved case 执行 `sync -Apply` | 拒绝，提示先 repair。 |
| S3 | 不同 `templateRoot` / `templatePack` | 拒绝，不静默重绑。 |
| S4 | managed file 缺失 | 创建文件，state 记录 sourceHash/targetHashAtSync。 |
| S5 | managed file 与 pack 不同 | 写入前 backup，写入后内容等于 pack。 |
| S6 | managed file 自 last sync 后有本地修改 | review 标高风险；apply 仍需显式确认，backup 后覆盖。 |
| S7 | managed file 与 pack 相同 | 不产生 backup；内容保持一致。 |
| S8 | template target 缺失 | 创建 `.md`，完成 project placeholder 替换。 |
| S9 | template target 已存在且未 force | skip，不覆盖本地文件。 |
| S10 | template target 已存在且 force | backup 后覆盖，placeholder 替换正确。 |
| S11 | managed block host 缺失或为空 | 创建 host 与 block。 |
| S12 | managed block host 有旧 block | 替换 block，保留 block 外内容。 |
| S13 | managed block host 无 block | append block，保留原文。 |
| S14 | managed block 已一致 | 内容不变；backup 语义需与 PowerShell 兼容或明确调整。 |
| S15 | `.gitignore` support source 存在且 target 缺失 | 创建 support file；target 已存在则 skip。 |
| S16 | apply 中途源文件缺失/读写失败 | 返回错误；已写入部分不隐瞒；已有 backup 可人工恢复。 |
| S17 | apply 后 doctor | Go doctor 与 PowerShell doctor 均通过。 |
| S18 | PowerShell/Go apply parity | 两个临时 case 对比内容 hash；排除 review artifact 与 backup timestamp 差异。 |

## 当前预研 smoke

`rekit/tests/sync-review-parity-smoke.ps1` 只做 review-only 验证：

1. 创建 `_template` 临时 case。
2. 造出 managed file drift、managed file missing、managed block drift、local template existing 四类状态。
3. 分别生成 PowerShell sync review 与 Go sync review artifacts。
4. 验证两边关键 action 一致、bounded diff 存在、review 不改 case 目标文件。
5. 删除临时 case。

它不执行 `sync -Apply`，也不替代未来 apply parity tests。