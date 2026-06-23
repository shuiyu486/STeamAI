# Changelog

## Unreleased

### Added

- 新增仓库根目录 `CLAUDE.md`，提供 Claude Code 维护本 kit/template/runtime 仓库的入口说明、分层改动边界和验证命令。
- 新增 `docs/vision.md`，定义 RE Agent Team 框架定位、模块边界和后续阶段实施方案。
- 新增 `references/vmp-re/agent-driven-re.md`，定义 VMP RE Agent Team 的角色、packet、candidate→confirmed 流程和人工门禁。
- 在 `docs/vision.md` 中新增批次执行协议，明确后续可分批实施、自审调整、需停下询问的边界和计划写回文档要求。
- 新增 `common/policies/agent-team.md` 与 `common/policies/tool-adapters.md`，沉淀跨 pack Agent Team 和外部工具 adapter 通用契约。
- 新增 `docs/agent-team-usage.md`，说明新架构使用方式、旧 case 兼容、主线/功能支线工作流和后续优化空间。
- 新增 `docs/reference-absorption.md`，映射参考文章、`ida-agent-bridge`、`clark-utov` 的吸收点、当前落地能力和后续优化项。
- 新增 `docs/pack-authoring.md`、`docs/evidence-ledger.md`、`docs/orchestration-plan.md` 和 `docs/batch-plan.md`，分别记录 pack 编写、证据账本、半自动编排和后续批次计划。
- 新增 `packs/_template/` pack 作者骨架，用于后续创建 `unpack-pe`、`android-native`、`ollvm` 等新 pack。
- 增强 `doctor` 的 manifest、policy overlay、case thin shim、board/lane 和 JSONL 校验，作为后续 runtime 架构调整的安全网。
- 为 `packs/_template` 补齐最小 policy overlay registry，使模板 pack 可通过 pack validation。
- 将 B3 工作线默认主线、默认 start 类型、长期 handoff 路径、sync backup root、authority files 和 request 默认路由改为 manifest 驱动，减少 `vmp-re` 硬编码。

### Fixed

- 修复 Windows PowerShell 5.1 解析含非 ASCII 的 runtime `.ps1` 文件时可能因 UTF-8 无 BOM 产生 mojibake 并破坏语法的问题。
- 修复裸 `attach` 后执行 `sync` review 时，空 `CLAUDE.local.md` host text 被 PowerShell 参数绑定拒绝的问题，并统一空白 host 下 sync review 与 `sync -Apply` 的 managed block 写入结果。

### Verified

- 使用临时 case 完成 `init/status/doctor/sync/promote` 与 `attach/status/sync/sync -Apply/doctor/promote` smoke test，验证 case-local 边界与 review-first 流程。

### Changed

- 更新 README 顶部定位，将项目说明从单纯 context kit 扩展为面向逆向工程的 Claude Code Agent Team 框架，并区分维护本仓库与接入 RE case 的入口。
- 扩展 `vmp-re` 工作流与工具路由，加入轻到重分析路线、重型工具升级门禁和 `ida-agent-bridge` 候选工具说明。
- 将日常 `/rekit` 工作流收敛为 `overview / continue / start / handoff`，移除公开的 `board / auto / lane / policy` 旧入口。
- 将原集中式 B3 PowerShell runtime 拆分为 `B3.Core/State/Policy/Lane/Auto/Commands` 模块，并新增项目级 handoff 生成。
- 更新 README、skill、case shim、policy/reference/prompt 文档，用户层统一使用“工作线 / 主线 / 功能支线”术语。
- 明确 `/rekit overview` 只是项目总览，`/rekit continue main|<name>` 才选择工作线；`/rekit handoff` 改为项目级接手索引，并新增指定工作线 handoff。

## 0.2.0 - 2026-06-12

### Added

- 新增目录级 canonical `/rekit` skill，clone 后在 kit 仓库内直接可用。
- 新增 `rekit/rekit.ps1` runtime 与 Manifest / Instance / Sync / Promote / Validate 模块。
- 新增 case-local `/rekit` shim 模板与 `.rekit/instance.yml` / `state.json` 实例模型。
- 新增 `promote` 工作流，用于将 case 中可复用 managed docs 生成候选或显式写回 pack。
- 新增 `packs/vmp-re/tooling/`，保存工具 catalog、recipes、脚本模板化清单、补丁/止损经验和 promote 候选。
- 新增 `docs/promote-sync.md`。
- 新增 manifest 路径 root containment 检查，避免 managed/promote/tooling 路径越出 case 或 pack 根目录。

### Changed

- `packs/vmp-re/manifest.yml` 升级为 sync/promote/managed block/tooling/budget 的单一事实源。
- `sync/promote/doctor` 对显式 case target 增加 attached-case guard，避免拼错路径时隐式创建假 case 或从普通目录回流。
- `promoteDenyPatterns` 收紧为覆盖 artifact/capture/trace/dump 路径与更通用 ctx/round 状态。
- `bootstrap.ps1`、`update.ps1`、`validate.ps1` 改为兼容 wrapper，转调 `rekit/rekit.ps1`。
- README 与 design 文档改为以 `/rekit`、runtime、pack、instance 四层模型说明，并去除具体 case 路径示例。

## 0.1.0 - 2026-06-12

### Added

- 初版 `vmp-re` pack。
- 按需路由、渐进式披露、工具链路由、singleton handler 复核模板。
- `bootstrap.ps1` / `update.ps1` / `validate.ps1`。
- 四层目录模型：`kits/`、`cases/`、`tools/`、`shared-artifacts/`。
