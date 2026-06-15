# rekit promote / sync

## 方向

| 命令 | 方向 | 目标 |
|---|---|---|
| `sync` | kit -> case | 将 pack 的 managed docs / managed block 下发到已绑定 case。 |
| `promote` | case -> kit | 将已绑定 case 中可复用的 managed doc 改进生成候选或写回 pack，并生成 tooling 候选。 |

两者不对称：`sync` 可自动覆盖 managed files 并备份；`promote` 默认保守，不自动提升 live state。

## sync 规则

- 只处理 `manifest.yml` 的 `managedFiles` 与 `managedBlock`。
- 目标必须是已经 `attach/init` 的 case；拼错路径或普通目录会失败，不会静默创建假 case。
- 覆盖前备份到 `references/vmp-re/.backup/<timestamp>/`。
- `templateFiles` 只在目标缺失时创建。
- 不覆盖：
  - `CLAUDE.local.md` block 外内容
  - `references/vmp-re/task-handoff.md`
  - `tools.local.yml`
  - case-local 文档
  - `captures/**`
  - `artifacts/**`

## promote 规则

- 目标必须是已经 `attach/init` 的 case；普通目录不会参与回流候选。
- 扫描 `manifest.yml` 的 `promoteFiles`，处理 managed docs。
- 同时扫描 `toolingCandidateSources`，将 case 工具链经验脱敏后写入 `packs/<pack>/tooling/candidates/`。
- 默认 `-WhatIf` 用于预览；不带 `-Apply` 时 managed docs 写入 `packs/<pack>/promote-candidates/`。
- `-Apply` 才会写回 pack managed docs。
- tooling 候选不直接覆盖正式 recipe；需要人工审查后合入 `tooling/catalog.yml` 或 `tooling/recipes/*`。
- 命中 `promoteDenyPatterns` 时阻止 managed docs 提升；tooling 候选会先做脱敏再检查残留私有信息。

## 永不提升

- `CLAUDE.local.md` 全文
- `references/vmp-re/task-handoff.md`
- `tools.local.yml`
- `captures/**`
- `artifacts/**`
- dump/trace/binary/log
- 当前 coverage、handler 地址列表、round/task 快照

## 路径安全

manifest 中的文件路径必须是相对路径，并且不能通过 `..` 越出对应根目录：

- pack source 路径不能越出 `packs/<pack>/`
- case target 路径不能越出 `<caseRoot>/`
- `managedBlock.file` / `managedBlock.source` 同样受约束

`validate/doctor` 会尽早检查这些路径，避免等到 `sync/promote` 写文件时才失败。

## 推荐日常流程

在 case 中完成一轮实践后：

```text
/rekit promote
```

如果候选安全，需要写回 pack，直接告诉 Claude：

```text
确认写回这次 promote 候选
```

验证 case：

```text
/rekit doctor
```

最后在 kit 仓库审查并提交：

```text
git diff
git commit
git push
```

> `pwsh <templateRoot>\rekit\rekit.ps1 ...` 只是 backend/自动化入口，不是日常用户入口。
