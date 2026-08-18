# 项目实例目录示例

## 读取指南

本文件只是 `binary-re` workspace layout 示例，不是默认必读清单。只有规划 source clone、project-local runtime、case/tools 目录或迁移 legacy-only `.rekit` 项目时再读取；具体 case 进度、目标、样本和 artifact 仍保存在 project-local handoff 与 sidecar，不写入本示例。

推荐根目录：`C:\AI\m_projects\RE`

```text
C:\AI\m_projects\RE\
  kits\
    STeamAI\                      # canonical repository checkout；旧本地目录名可继续使用
  cases\
    sample-vmp-case\              # 具体 RE 项目实例
      .claude\skills\steamai\     # project-local canonical skill
      .steamai\                   # verified runtime、pack assets、typed state 与 evidence
      CLAUDE.local.md
      references\binary-re\...
      scripts\
      captures\
      artifacts\
  tools\
    x64dbg-mcp\
    ScyllaHide\
    VMPImportFixer\
    vmpfix\
    NoVmp\
    VMPStatic\
  shared-artifacts\
```

## 目录语义

| 目录 | 用途 | 是否建议进 git |
|---|---|---|
| `kits/STeamAI` | canonical source clone；用于创建/维护项目，不作为 current 项目的运行时 fallback | 是 |
| `cases/<case>` | 单个授权目标项目，包含 project-local `/steamai`、verified runtime bundle 与 `.steamai` 状态；legacy-only 项目仍保留 `/rekit` + `.rekit` | 可选，需严格 `.gitignore` |
| `tools` | 第三方/编译工具 | 通常不进 case git |
| `shared-artifacts` | 大文件和临时共享产物 | 不进 git |
