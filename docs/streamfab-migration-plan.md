# StreamFab 项目迁移与路径相对化计划

> 目标根目录：`C:\AI\m_projects\RE`。

## 目标四层结构

```text
C:\AI\m_projects\RE\
  kits\
    re-context-kits\              # 模板仓库
  cases\
    streamfab-vmp\                # 当前 StreamFab 项目实例
    cdm-widevine\                 # 后续其它项目
  tools\
    x64dbg-mcp\
    ScyllaHide\
    VMPImportFixer\
    vmpfix\
    NoVmp\
    VMPStatic\
  shared-artifacts\
```

## 迁移原则

- 不在同一轮直接移动大型 captures/dump，先规划和路径相对化。
- 先让 Python 脚本从 `Path(__file__)` 推导项目根目录，再移动目录。
- 大产物留在 `artifacts/` 或 `captures/`，用 `.gitignore` 控制。
- 迁移后用 `bootstrap.ps1` / `update.ps1` 维护上下文模板。

## Python 路径相对化建议

当前若脚本在项目根目录：

```python
PROJECT_ROOT = Path(__file__).resolve().parent
DEFAULT_CAPTURES = PROJECT_ROOT / 'captures'
```

若后续脚本迁到 `scripts/`：

```python
PROJECT_ROOT = Path(__file__).resolve().parents[1]
DEFAULT_CAPTURES = PROJECT_ROOT / 'captures'
```

避免写死：

```python
Path(r'C:\AI\m_projects\RE\streamfab')
```

## 建议迁移步骤

1. 在 `C:\AI\m_projects\RE` 下创建 `kits/ cases/ shared-artifacts/`。
2. 克隆/维护模板仓库到 `kits/re-context-kits`。
3. 对当前 `streamfab` 脚本做路径相对化，先验证现目录可运行。
4. 创建 `cases/streamfab-vmp`。
5. 复制小型上下文文档、脚本、confirmed CSV 到新目录。
6. 大型 dump/trace/captures 分批迁移或用 junction/归档保留。
7. 在新目录运行 `bootstrap.ps1` 或 `update.ps1`，生成 `.re-template.yml`。
8. 运行关键验证：`py_compile`、`build_routine_ir.py`、文档预算检查。
9. 确认无误后，再停止使用旧 `streamfab` 路径。

## 当前迁移准备状态

- 四层目录骨架已创建：`kits/`、`cases/`、`shared-artifacts/`。
- 模板仓库已位于：`C:\AI\m_projects\RE\kits\re-context-kits`。
- 当前项目已先行相对化 4 个主线脚本默认路径：`auto_mine_handler_semantics.py`、`build_routine_ir.py`、`mine_routine_superinstructions.py`、`trace_vmenter_seed.py`。
- 尚未移动 `streamfab` 项目目录。
- 下一步继续相对化 `extract_handler_value_flow.py`、`analyze_focused_handler_occurrences.py`、`augment_context_memory.py`、`launch_suspended_with_probe.py` 等辅助主线脚本。

## 待人工确认

- `captures/` 中哪些大型 trace 需要迁移，哪些只归档。
- 当前 `streamfab` 是否需要独立 git 仓库。
- `tools/` 是否保持当前已有位置，还是逐步清理重命名。
