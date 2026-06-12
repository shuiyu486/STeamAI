# re-context-kits design

## 三层模型

1. **模板仓库**：`kits/re-context-kits`，保存可复用 pack、脚本和脱敏示例。
2. **项目实例**：`cases/<case-name>`，保存当前目标的活文档、small state 和必要脚本。
3. **本地覆盖**：`tools.local.yml`、真实样本路径、授权材料、dump/trace 等不进模板。

## managed vs local

- Managed files：由模板仓库更新，例如 `workflow-template.md`、`toolchain-router.md`。
- Local files：项目自己的状态，例如 `task-handoff.md`、coverage、handler 列表。
- `CLAUDE.local.md` 通过 managed block 更新，避免覆盖本地内容。

## Bootstrap 不是安装

`bootstrap.ps1` 的含义是“把模板应用到目标项目”，不是安装软件。它复制 reference、插入路由 block、生成 `.re-template.yml` 与项目 `task-handoff.md`。
