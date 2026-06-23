# rekit promote / sync

## 方向

| 命令 | 方向 | 目标 |
|---|---|---|
| `sync` | kit -> case | 将 pack 的 managed docs / managed block 下发到已绑定 case。 |
| `promote` | case -> kit | 将已绑定 case 中可复用的 managed doc 改进生成候选或写回 pack，并生成 tooling 候选。 |

两者不对称：`sync` 是模板升级，`promote` 是经验蒸馏。日常 `/rekit sync` / `/rekit promote` 默认采用 LLM-first review：先生成审查包，Claude 比较优劣/冲突并让用户确认，确认后才写入。

## review-first 流程

默认日常流程：

```text
/rekit sync
/rekit promote
```

Claude 会先使用内部 runtime 生成只读 review 包。用户不需要手动执行底层脚本。

review 包写入 case-local 目录：

```text
<caseRoot>/.rekit/reviews/<timestamp>-sync/
<caseRoot>/.rekit/reviews/<timestamp>-promote/
```

每个 review 至少包含：

- `packet.json`：结构化事实包，供 Claude 判断。
- `summary.md`：简短机械摘要。
- `diffs/combined.diff` 和逐文件 bounded diff。
- `previews/*.md`：promote tooling 的脱敏预览。

内部 review 模式是强只读；旧式 dry run 不等同于 LLM review。

## sync 规则

- 只处理 `manifest.yml` 的 `managedFiles`、`templateFiles`、`managedBlock` 和少量 support files。
- 目标必须是已经 `attach/init` 的 case；拼错路径或普通目录会失败，不会静默创建假 case。
- review 报告应说明每项：是否会 create / overwrite / backup / skip，case 是否相对 last sync hash 有本地修改，风险与推荐动作。
- 用户确认后，写入型 sync 才覆盖 managed files；覆盖前备份到 manifest `workstreamDefaults.backupRoot` 下的时间戳目录。
- `templateFiles` 只在目标缺失时创建。
- 不覆盖：
  - `CLAUDE.local.md` block 外内容
  - pack 的长期 handoff local file，例如 `references/vmp-re/task-handoff.md`
  - `tools.local.yml`
  - case-local 文档
  - `captures/**`
  - `artifacts/**`

## promote 规则

- 目标必须是已经 `attach/init` 的 case；普通目录不会参与回流候选。
- review 扫描 `manifest.yml` 的 `promoteFiles`，处理 managed docs 的 case -> pack 差异。
- review 同时扫描 `toolingCandidateSources`，生成脱敏 preview 供 Claude 判断是否值得吸收。
- 命中 `promoteDenyPatterns` 的 managed docs 只在 packet 中记录 metadata 和 deny pattern；不要输出 raw diff，避免把 case-specific 信息带回模板审查材料。
- 用户确认后，才生成 `packs/<pack>/promote-candidates/` 或 `packs/<pack>/tooling/candidates/`，或由 Claude 改写正式 pack 文档。
- 直接整文件写回 pack managed docs 不作为默认推荐路径；优先让 Claude 提炼经验片段。
- tooling 候选不直接覆盖正式 recipe；需要人工审查后合入 `tooling/catalog.yml` 或 `tooling/recipes/*`。

## 永不提升

- `CLAUDE.local.md` 全文
- pack 的长期 handoff local file，例如 `references/vmp-re/task-handoff.md`
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

`doctor` 会尽早检查这些路径，避免等到 `sync/promote` 写文件时才失败。

## 推荐日常流程

在 case 中完成一轮实践后：

```text
/rekit promote
```

Claude 读取 review 包后输出：

- 每个大项即将同步/回流什么。
- 新旧经验是否冲突。
- 新旧经验各自优劣。
- 推荐同步、候选、改写、跳过或取消。

用户确认具体动作后再执行写入。常见确认语义：

```text
只生成 tooling candidate
生成 promote 候选，不写回 pack
同步全部推荐项
只同步 references/vmp-re/README.md
取消
```

最后验证：

```text
/rekit doctor
```

> 底层 runtime 只是 `/rekit` 的内部实现；日常不要手动执行。
