# 项目实例目录示例

本文件只展示声明式 source clone、授权 case 与外部工具的分离；不保存具体 case 进度、目标、样本或 artifact。

```text
<workspace>/
  STeamAI/                         # canonical source clone
  cases/
    <authorized-case>/
      .claude/skills/steamai/      # project-local canonical skill
      .steamai-vnext/
        CLAUDE.md
        contracts/
        pack-snapshot/
        members/
        artifacts/
        evidence/
        findings/
        reviews/
        learnings/
      <case-local-artifacts>/
  tools/                           # 可选第三方工具，不进入 pack snapshot
```

| 目录 | 用途 | 边界 |
|---|---|---|
| `STeamAI` | canonical source、声明式 pack 与 contract tests | 不是 case，不创建 case state。 |
| `cases/<authorized-case>` | 一个明确授权的安全研究 case | pack snapshot 固定 exact revision；真实 artifact 留在 case。 |
| `tools` | 用户自行管理的第三方工具 | 不由 pack 自动安装或执行。 |
