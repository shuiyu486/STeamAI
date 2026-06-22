# CLAUDE.md

## 项目定位

`re-context-kits` 是面向逆向工程的 Claude Code Agent Team 框架，用于组织多 Agent 协作、RE 工具链、证据账本、工作线管理、验证门禁和可复用 pack。

当前阶段，本仓库主要是 Agent Team 的 context / workflow / tooling / runtime 底座，不是具体 RE case，也不是已经全自动化的脱壳器或逆向引擎。

维护本仓库时，不要因为 README 的 case 初始化示例而创建无关 case。只有在需要验证 `init`、`attach`、`sync`、`promote` 行为时，才创建临时 case。

## 常用维护入口

- `/rekit` skill 说明：`.claude/skills/rekit/SKILL.md`
- runtime 入口：`rekit/rekit.ps1`
- runtime 模块：`rekit/lib/*.ps1`
- vmp-re pack：`packs/vmp-re/**`
- pack manifest：`packs/vmp-re/manifest.yml`
- 通用策略：`common/policies/**`
- 通用 prompts：`common/prompts/**`
- 新架构使用与旧 case 兼容：`docs/agent-team-usage.md`
- 参考资料吸收映射：`docs/reference-absorption.md`
- 长期愿景与阶段路线：`docs/vision.md`
- 后续批次计划：`docs/batch-plan.md`
- 设计文档：`docs/design.md`
- pack 编写指南：`docs/pack-authoring.md`
- evidence / intervention 账本草案：`docs/evidence-ledger.md`
- orchestration 计划：`docs/orchestration-plan.md`
- sync/promote 说明：`docs/promote-sync.md`
- 变更记录：`CHANGELOG.md`

## 维护者工作流

改动前先判断属于哪一层：

1. Skill UI：改 `.claude/skills/rekit/SKILL.md`
2. Runtime：改 `rekit/rekit.ps1` 或 `rekit/lib/*.ps1`
3. Pack：改 `packs/vmp-re/**`
4. Common policies/prompts：改 `common/**`
5. 用户文档：改 `README.md` 或 `docs/**`

后续路线可以按 `docs/vision.md` 分批实施。用户已授权：每批完成后自行 review/评估并做低风险调整；只有遇到产品方向变化、破坏性动作、外部副作用、动态调试/注入/patch/dump、confirmed/authority 写入、runtime schema 迁移或难以判断的架构取舍时，再停下来询问用户。为避免上下文压缩导致偏差，后续批次计划必须持续写回文档，不能只留在聊天上下文中。

不要复制 runtime 逻辑到 case shim；case-local `/rekit` 应保持 thin shim，并回到 kit 仓库中的 canonical runtime。

## 验证命令

仓库级只读检查：

```powershell
.\rekit\rekit.ps1 status
.\rekit\rekit.ps1 doctor
```

改动 pack wrapper 时可额外检查：

```powershell
.\packs\vmp-re\scripts\validate.ps1
```

涉及 `init`、`attach`、`sync`、`promote` 的改动，应使用临时 case 验证，不要在 kit 仓库内伪造 case state。

## 关键边界

- README 中的 `/rekit init -Target ...` 是给外部 case 用的，不是维护本仓库的必经步骤。
- `packs/vmp-re/manifest.yml` 是 managed files、template files、promote files、tooling files、budgets 的单一事实源。
- `sync` 是 kit -> case；`promote` 是 case -> kit。
- `sync` / `promote` 默认 review-first，写入前需要用户确认具体范围。
- 不要把真实样本、trace、dump、capture、artifact、绝对路径或 case-specific 进度写入本仓库模板。
- `.gitignore` 已排除常见 RE artifacts；新增产物类型时先确认是否应继续排除。
