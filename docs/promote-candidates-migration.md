# G3.6/G3.7 promote -CreateCandidates 迁移

## 读取指南

- 只接手实现时，先读“实施摘要”“执行清单”“验证标准”。
- 需要判断 PowerShell 与 Go 行为差异时，再读“PowerShell 当前语义基线”和“Go 迁移契约”。
- 本文记录 G3.6 迁移前预研与 G3.7 Go 手动写入路径；公共 `/rekit promote` 仍由 PowerShell façade / fallback 处理写入命令。

## 实施摘要

G3.6 先固化候选生成语义、sanitization 规则、阻断条件和验证资产；G3.7 在 Go backend 中新增显式 `promote -CreateCandidates` 手动路径。`promote -CreateCandidates` 会在 kit pack 内创建 candidate 文件和 `promote-candidates/index.json`，比 review artifact 更接近 pack 写入，因此仍保持手动 Go CLI 使用，不纳入 PowerShell façade 委托安全集合。

当前 Go backend 已具备 promote review-only plan、bounded diff、tooling sanitized preview，以及 `promote.CreateCandidates` helper。裸 `promote` 继续 review-only；`-CreateCandidates -WhatIf` 输出非写入 JSON preview；无 `-WhatIf` 时显式写 candidate/index/tooling candidate，仍不实现 `promote -Apply`。

## 执行清单

- [x] 固化 PowerShell `promote -CreateCandidates` 当前语义基线。
- [x] 固化 Go 迁移契约、边界和不迁移项。
- [x] 固化 managed doc candidate、deny pattern、tooling sanitized candidate、index 写入和 cleanup 的测试矩阵。
- [x] 增加 `rekit/tests/promote-candidates-preflight-smoke.ps1`，验证 PowerShell `-WhatIf -CreateCandidates` baseline 与 Go promote review artifact / sanitized preview。
- [x] 更新 `docs/batch-plan.md`、`docs/go-runtime-migration.md` 与 `CHANGELOG.md`。
- [x] G3.7 实现 Go `promote.CreateCandidates` 手动写入 helper；默认仍不委托 PowerShell façade。
- [x] CLI 支持 `-Command promote -CreateCandidates` 与 `-WhatIf` JSON preview，拒绝 `-Apply` 和 review artifact 混用。
- [x] 增加 `rekit/tests/promote-candidates-apply-smoke.ps1`，验证 candidate 写入、blocked deny、tooling sanitization、pack-root containment、cleanup 与 façade fallback。

## 验证标准

G3.7 完成标准：

```powershell
go test ./...
.\rekit\tests\promote-candidates-preflight-smoke.ps1
.\rekit\tests\promote-candidates-apply-smoke.ps1
.\rekit\tests\facade-smoke.ps1 -CaseRoot 'C:\AI\m_projects\RE\_dryrun_cases\agent-team-dryrun' -Pack vmp-re
.\rekit\rekit.ps1 -Command doctor
.\rekit\rekit.ps1 -Command doctor -Target 'C:\AI\m_projects\RE\_dryrun_cases\agent-team-dryrun'
git diff --check
```

必须额外确认：

- preflight/apply smoke 使用临时 case，验证后删除。
- `promote -CreateCandidates -WhatIf` 不写 `packs/<pack>/promote-candidates/**` 或 `packs/<pack>/tooling/candidates/**`。
- Go `promote -CreateCandidates` 实际写入后，smoke 清理本次新增 candidates 与 `index.json`，不提交临时 candidates。
- Go promote review artifact 仍返回 `isMutation=false` / `writesArtifacts=true`，只写 `.rekit/reviews/**` 或显式 review output。
- tooling sanitized candidate 不残留 case root、绝对路径、trace/dump 文件名、address、ctx/round/task 等 deny 信息。
- PowerShell façade 仍不委托 `promote -CreateCandidates`，即使 `REKIT_GO_ENABLE=1` 也 fallback 到 PowerShell。

## 风险与注意事项

- `promote -CreateCandidates` 写入 kit pack root 下的 `promote-candidates/**`；测试必须 cleanup，不能把临时 candidates 提交进仓库。
- candidate 内容来自 case managed docs，必须先过 deny pattern；不能把 case-specific 进度、绝对路径、trace、dump、capture、artifact 或真实样本信息写入 pack。
- tooling candidate 需要 sanitization 后再次过 deny pattern；sanitization 失败时必须 blocked，不得生成 candidate。
- G3.7 仍不实现 `promote -Apply`，不覆盖 pack managed docs，不写 authority/confirmed，不执行 heavy-tool。
- PowerShell façade 仍不委托 `promote -CreateCandidates`；公共 `/rekit` 入口继续走 PowerShell。

## PowerShell 当前语义基线

`rekit/lib/Promote.ps1` 当前由 `Promote-RekitChanges` 处理 promote：

1. `-Review`：只写 promote review artifacts。
2. 无 `-Apply`、`-CreateCandidates`、`-WhatIf` 时拒绝写入，提示先 review 或显式写入。
3. `-CreateCandidates`：在 `packs/<pack>/promote-candidates/` 下创建 candidate 文件。
4. managed doc candidate：
   - 只处理 manifest `promoteFiles` 中同时属于 `managedFiles` 的路径；
   - case 文件缺失则 skip；
   - case 与 pack 内容相同则 unchanged；
   - case 文本命中 manifest deny patterns 或 case-specific patterns 则 blocked；
   - 通过后写 `<timestamp>_<safeName>.candidate.md`；
   - 写入 `promote-candidates/index.json`。
5. tooling candidate：
   - 读取 manifest `toolingCandidateSources`；
   - 先执行 `Convert-RekitToolingCandidateText`，替换 case root、绝对路径、case terms、artifacts/captures path、trace/dump 文件名、address、ctx/round/task；
   - sanitization 后再跑 deny patterns；
   - 通过后写 `tooling/candidates/<timestamp>_tooling_<safeName>.candidate.md`。
6. `-WhatIf` 可与 `-CreateCandidates` 组合，只打印 would-write / blocked / summary，不写 candidates。

## Go 迁移契约

G3.7 已新增独立 Go helper，不把写入逻辑塞进 CLI：

```text
internal/rekit/promote
  Plan(...)              # 已存在：review-only plan + sanitized preview
  CreateCandidates(...)  # 显式写 candidate files / index；-WhatIf 时只返回 preview
```

必须保持：

- 裸 `promote` 默认 review-only。
- `promote -CreateCandidates` 只能作为显式手动 Go CLI 写入路径，暂不经 PowerShell façade 委托。
- `-CreateCandidates` 不得与 `-Apply` 混用。
- `-WhatIf` 只能输出非写入 JSON preview。
- candidate root 必须固定在 pack root 下的 `promote-candidates/**` 或 manifest 认可位置；禁止越出 pack root。
- tooling sanitization 与 deny pattern 必须与 PowerShell baseline 对齐。
- 结果必须结构化输出 `isMutation=true`、`createdCandidates`、`blocked`、`skipped`、`indexPath`、`toolingCandidates`。

## 测试矩阵

| 编号 | 场景 | 预期 |
|---|---|---|
| P1 | managed promote file unchanged | skip / unchanged，不生成 candidate。 |
| P2 | managed promote file changed 且无 deny | PowerShell what-if 显示 would candidate；Go review action `candidate-after-llm-review`。 |
| P3 | managed promote file 含绝对路径 | blocked；Go review action `blocked-deny-pattern`。 |
| P4 | promote file 不属于 managedFiles | skip-non-managed-promote-file。 |
| P5 | case promote file 缺失 | skip-missing-case-file。 |
| P6 | tooling source 含 case root / absolute path / trace / address / ctx / task | sanitized preview 替换敏感项。 |
| P7 | tooling sanitization 后仍命中 deny | blocked-after-sanitization。 |
| P8 | PowerShell `-WhatIf -CreateCandidates` | 不写 pack candidates。 |
| P9 | Go promote review artifacts | 只写 review artifacts，不写 candidates，`isMutation=false`。 |
| P10 | façade explicit Go enable + `promote -CreateCandidates` | 不委托 Go，继续走 PowerShell fallback。 |
| P11 | Go `promote -CreateCandidates` apply | 写 managed candidate、`index.json` 与 sanitized tooling candidate；blocked deny 不写 candidate；smoke 清理新增文件。 |
