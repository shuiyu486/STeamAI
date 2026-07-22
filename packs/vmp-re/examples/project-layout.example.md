# 项目实例目录示例

## 读取指南

本文件只是 `vmp-re` workspace layout 示例，不是默认必读清单。只有规划 kit/case/tools 目录、迁移旧 case 或解释 `.rekit` / case-local shim 位置时再读取；具体 case 进度、目标、样本和 artifact 仍保存在 case-local handoff 与 sidecar，不写入本示例。

推荐根目录：`C:\AI\m_projects\RE`

```text
C:\AI\m_projects\RE\
  kits\
    re-context-kits\              # 模板仓库
  cases\
    sample-vmp-case\              # 具体 RE 项目实例
      .claude\skills\rekit\       # case-local 薄 shim
      .rekit\                     # instance.yml / state.json
      CLAUDE.local.md
      .re-template.yml             # 兼容旧入口，逐步迁移到 .rekit
      references\vmp-re\...
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
| `kits/re-context-kits` | 通用上下文模板仓库，包含 canonical `/rekit` 与 packs | 是 |
| `cases/<case>` | 单个授权目标项目，包含 case-local `/rekit` shim 与 `.rekit` 状态 | 可选，需严格 `.gitignore` |
| `tools` | 第三方/编译工具 | 通常不进 case git |
| `shared-artifacts` | 大文件和临时共享产物 | 不进 git |
