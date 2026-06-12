# 项目实例目录示例

推荐根目录：`C:\AI\m_projects\RE`

```text
C:\AI\m_projects\RE\
  kits\
    re-context-kits\              # 模板仓库
  cases\
    sample-vmp-case\              # 具体 RE 项目实例
      CLAUDE.local.md
      .re-template.yml
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
| `kits/re-context-kits` | 通用上下文模板仓库 | 是 |
| `cases/<case>` | 单个授权目标项目 | 可选，需严格 `.gitignore` |
| `tools` | 第三方/编译工具 | 通常不进 case git |
| `shared-artifacts` | 大文件和临时共享产物 | 不进 git |
