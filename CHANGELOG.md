# Changelog

## 0.2.0 - 2026-06-12

### Added

- 新增目录级 canonical `/rekit` skill，clone 后在 kit 仓库内直接可用。
- 新增 `rekit/rekit.ps1` runtime 与 Manifest / Instance / Sync / Promote / Validate 模块。
- 新增 case-local `/rekit` shim 模板与 `.rekit/instance.yml` / `state.json` 实例模型。
- 新增 `promote` 工作流，用于将 case 中可复用 managed docs 生成候选或显式写回 pack。
- 新增 `docs/promote-sync.md`。

### Changed

- `packs/vmp-re/manifest.yml` 升级为 sync/promote/managed block/budget 的单一事实源。
- `bootstrap.ps1`、`update.ps1`、`validate.ps1` 改为兼容 wrapper，转调 `rekit/rekit.ps1`。
- README 与 design 文档改为以 `/rekit`、runtime、pack、instance 四层模型说明。

## 0.1.0 - 2026-06-12

### Added

- 初版 `vmp-re` pack。
- 按需路由、渐进式披露、工具链路由、singleton handler 复核模板。
- `bootstrap.ps1` / `update.ps1` / `validate.ps1`。
- 四层目录模型：`kits/`、`cases/`、`tools/`、`shared-artifacts/`。
