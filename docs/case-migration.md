# Case 迁移与路径相对化指南

本文件描述把已有 RE case 移到新目录时的通用流程。具体样本名、真实绝对路径、当前进度和脚本状态应保存在 case-local 文档中，不写入 kit 仓库。

## 推荐 workspace 结构

```text
<workspaceRoot>\
  kits\
    re-context-kits\              # 模板仓库
  cases\
    <caseName>\                   # 具体 case
  tools\                          # 第三方工具
  shared-artifacts\               # 大文件/共享产物
```

## 迁移原则

- 不在同一轮直接移动大型 captures/dump；先规划、相对化路径、验证新目录。
- 先让脚本从 `Path(__file__)` 或配置文件推导项目根目录，再移动目录。
- 大产物留在 `artifacts/` 或 `captures/`，用 `.gitignore` 控制。
- 迁移后用 `/rekit status -> /rekit repair -> /rekit doctor` 确认 metadata，不再把 `bootstrap.ps1` / `update.ps1` 当主流程。
- 旧 PowerShell scripts 只保留为兼容 wrapper。
- 新架构的使用方式、旧 case 接入和主线/功能支线工作流见 `docs/agent-team-usage.md`。

## 旧 case 接入 Agent Team 架构

旧 case 不需要一次性重建。推荐分两步接入：

1. **绑定 metadata 和 thin shim**：

```text
/rekit attach -Target <caseRoot> -Pack vmp-re
```

PowerShell `/rekit attach` 只写 `.rekit/instance.yml`、`.rekit/state.json`、`.re-template.yml` 和 case-local `/rekit` shim，不覆盖已有 reference、handoff 或工具链文档。维护者手动验证用的 Go `attach -Apply` 当前只写 `.rekit/instance.yml` 与 thin shim；不替代日常 `/rekit attach`。

2. **同步 managed docs 前先 review**：

```text
/rekit sync
```

默认只生成 `.rekit/reviews/<timestamp>-sync/packet.json`、`summary.md` 和 bounded diff。确认具体范围后，才执行写入型 `sync -Apply`。

接入后仍然可以继续使用主线/功能支线：

```text
/rekit overview
/rekit continue main
/rekit start <feature>
/rekit continue <feature>
/rekit handoff
```

`.re-template.yml` 仍作为旧 case 兼容入口保留；新 runtime 优先读取 `.rekit/instance.yml`，缺失时回退读取 `.re-template.yml`。

## Python 路径相对化建议

若脚本在 case 根目录：

```python
PROJECT_ROOT = Path(__file__).resolve().parent
DEFAULT_CAPTURES = PROJECT_ROOT / 'captures'
```

若脚本在 `scripts/`：

```python
PROJECT_ROOT = Path(__file__).resolve().parents[1]
DEFAULT_CAPTURES = PROJECT_ROOT / 'captures'
```

避免写死：

```python
Path(r'<oldCaseRoot>')
```

## 建议迁移步骤

1. 关闭正在使用该 case 的 Claude Code、IDA、x64dbg、trace 脚本等进程。
2. 复制 case 到新目录，例如：`robocopy <oldCaseRoot> <newCaseRoot> /E`。
3. 在新目录启动 Claude Code，执行 `/rekit status`。
4. 如果 status 提示 `projectRoot` 与当前目录不一致，先确认这是预期迁移。
5. 执行 `/rekit repair` 预览 metadata 变更。维护者也可手动用 Go `repair -WhatIf` 对比 JSON preview，但日常入口仍是 `/rekit`。
6. 确认无误后，直接告诉 Claude：`确认修复，执行 repair -Apply`。Go `repair -Apply` 目前只作为维护者验证路径，不经 PowerShell façade 委托。
7. 执行 `/rekit doctor` 验证 case 绑定。
8. 必要时执行 `/rekit sync` 同步最新 managed docs。
9. 搜索并更新只属于旧 case 根目录的绝对路径。
10. 验证关键脚本和分析流程后，再归档旧目录。

## 需要重点检查的文件

```text
CLAUDE.local.md
.rekit/instance.yml
.re-template.yml
references/vmp-re/task-handoff.md
自写脚本中的 PROJECT_ROOT / workdir / output path
tools.local.yml
```

目标样本路径如果没有变化，不需要因为 case 目录迁移而修改。

## Skill-first 命令示例

在新 case 目录启动 Claude Code 后使用：

```text
/rekit status
/rekit repair
确认修复，执行 repair -Apply
/rekit doctor
```

> 底层 runtime 只作为 `/rekit` 的内部实现；日常迁移入口只使用 `/rekit`。
