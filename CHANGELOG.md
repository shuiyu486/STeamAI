# Changelog

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
