# Case 迁移与路径相对化指南

## 读取指南

本文件用于旧 case 移动、路径相对化、attach/repair 接入，以及 attached retired pack 的显式 state-root migration，不是日常维护默认必读清单。先读 `docs/context-routing.md`；只有 status 提示 moved/stale metadata、runtime返回 typed pack migration requirement、需要接入旧 case或修复脚本绝对路径时，再读对应分支。

## 实施摘要

已有安全 case 不需要一次性重建。仍受支持的 legacy pack可按需用 `/rekit attach` / `/rekit status` / `/rekit repair` 处理 metadata 与目录移动；已 attached 的 retired `vmp-re` / `generic-binary-re` 不进入这些 ordinary owners，只走显式 `migrate-state`。具体目标、样本名、真实绝对路径、当前进度和脚本状态应保存在 case-local 文档中，不写入 kit 仓库。

## 执行清单

- 先确认 case root、templateRoot、exact templatePack、state root 和 moved metadata 诊断，再区分目录移动与状态根迁移。
- 仍受支持的 legacy pack做目录移动时，repair 必须 preview-first；确认后才 Apply。
- 已 attached 的 retired `vmp-re` / `generic-binary-re` 只做 `migrate-state` zero-write preview，并原样消费确认后的 exact Apply；不先运行 ordinary status/doctor/sync。
- 大 artifact/capture/dump 不随文档迁移进入模板仓库；脚本路径优先相对化。

## 验证标准

- 目录移动分支：`/rekit status` 不再提示 stale/moved metadata，`/rekit doctor` 通过，case-local shim匹配canonical template。
- retired 状态根迁移分支：最终只存在`.steamai` mutable root，canonical pack为`binary-re`，project-local `/steamai` status与receipt exact replay通过；copy/move后仍可验证。
- 两条分支都不得自动写authority/confirmed、执行heavy tool或触发sync/promote；文档中不出现真实样本、客户信息、payload、flag、绝对case路径或case-specific进度。

## 风险与注意事项

- 不在同一轮直接移动或上传大型 captures/dumps；先规划、复制、验证，再归档旧目录。
- 不把 `bootstrap.ps1` / `update.ps1` 当主流程；底层 runtime 只作为 `/rekit` 内部实现。
- 公共模板仓库只保留通用迁移流程，不记录具体 case 操作日志。

## 推荐 workspace 结构

```text
<workspaceRoot>\
  kits\
    STeamAI\                      # canonical repository checkout；旧本地目录名可继续使用
  cases\
    <caseName>\                   # 具体 case
  tools\                          # 第三方工具
  shared-artifacts\               # 大文件/共享产物
```

## 迁移原则

- 不在同一轮直接移动大型 captures/dump；先规划、相对化路径、验证新目录。
- 先让脚本从 `Path(__file__)` 或配置文件推导项目根目录，再移动目录。
- 大产物留在 `artifacts/` 或 `captures/`，用 `.gitignore` 控制。
- 对仍受支持 legacy pack 的目录移动，用 `/rekit status -> /rekit repair -> /rekit doctor` 确认 metadata；这不是 retired state-root migration，也不把 `bootstrap.ps1` / `update.ps1` 当主流程。
- 旧 PowerShell scripts 只保留为兼容 wrapper。
- 新架构的使用方式、旧 case 接入和主线/功能支线工作流见 `docs/agent-team-usage.md`。

## 旧安全 case 接入 Agent Team 架构

旧 case 不需要一次性重建，但先区分两种状态：

- **尚未 attached 的旧目录**：新接入使用 canonical `binary-re`，不要再创建 `vmp-re` / `generic-binary-re` identity；按 `/steamai` 的 fresh onboarding/attach 路线执行。
- **已经是 legacy-only `.rekit`，且 metadata 的 exact `templatePack` 为 retired `vmp-re` / `generic-binary-re`**：不要运行 ordinary status、doctor、sync或手工改 pack；只进入显式 state-root migration。

retired 项目由 canonical compatibility skill 从可信 `templateRoot` 执行零写 preview：

```text
go run ./cmd/rekit -- -Command migrate-state -Target <caseRoot> -Pack <vmp-re-or-generic-binary-re> -WhatIf -Format json
```

审核 preview 返回的 exact source/target、完整 writes、root/onboarding projection 与 plan SHA。用户确认同一 preview 后，只原样消费返回的 `applyArgs[]`；不要手工拼 `-ExpectedMigrationPlanSha256`，也不要把 source selector替换成`binary-re`。成功后刷新项目内 `/steamai` status；receipt replay、copy/move和canonical health由 current owner验证。

迁移保留既有 lane/facts/evidence/gate/autonomy、references、handoff 与本地模板内容，不写 authority/confirmed，不执行 heavy tool，也不隐式 sync/promote。`.re-template.yml` 作为 receipt-bound source provenance保留，但最终唯一 mutable state root是`.steamai`。

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

## 仍受支持 legacy pack 的目录移动步骤

本节只处理 case 目录移动与 metadata repair，不适用于 retired state-root migration。

1. 关闭正在使用该 case 的 Claude Code、IDA、x64dbg、trace 脚本等进程。
2. 复制 case 到新目录，例如：`robocopy <oldCaseRoot> <newCaseRoot> /E`。
3. 在新目录启动 Claude Code，执行 `/rekit status`。
4. 如果 status 提示 `projectRoot` 与当前目录不一致，先确认这是预期迁移。
5. 执行 `/rekit repair` 预览 metadata 变更；该预览由 Go backend 处理，Go delegation 被禁用或不可用时 fail closed，不回落到 PowerShell 业务实现。
6. 确认无误后，直接告诉 Claude：`确认修复，执行 repair -Apply`。该写入默认由 Go backend 刷新 metadata、legacy metadata、初始 state 与 thin shim。
7. 执行 `/rekit doctor` 验证 case 绑定。
8. 必要时执行 `/rekit sync` 同步最新 managed docs。
9. 搜索并更新只属于旧 case 根目录的绝对路径。
10. 验证关键脚本和分析流程后，再归档旧目录。

## 需要重点检查的文件

```text
CLAUDE.local.md
.rekit/instance.yml
.re-template.yml
references/<legacy-pack>/task-handoff.md
自写脚本中的 PROJECT_ROOT / workdir / output path
tools.local.yml
```

目标/样本路径如果没有变化，不需要因为 case 目录迁移而修改。

## 目录移动的 Skill-first 命令示例

仅对仍受支持的 legacy pack，在移动后的 case 目录启动 Claude Code 后使用：

```text
/rekit status
/rekit repair
确认修复，执行 repair -Apply
/rekit doctor
```

> 底层 runtime 只作为 `/rekit` 的内部实现；日常迁移入口只使用 `/rekit`。
