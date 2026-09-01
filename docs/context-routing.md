# Context routing and progressive disclosure

## 读取指南

本文件是新会话和上下文压缩后接手的唯一完整路由入口。本项目文档必须做成按需路由、渐进式披露的样式。新会话先读根 `CLAUDE.md`、本文件、Git 状态，再从下表只选择一个场景入口；不要默认串读历史 roadmap、`CHANGELOG.md` 或旧控制面文档。

## 当前指针

- 路线：`steamai-vnext-thin-core-v1`
- 已完成路线事实源：`docs/real-usage-hardening-roadmap.md`
- 短投影：`docs/batch-plan.md`
- canonical 产品入口：`.claude/skills/steamai/SKILL.md`
- 薄核心合同：`vnext/**`

## 按需路由

| 需要判断什么 | 首选入口 | 不要默认读取 |
|---|---|---|
| 已完成路线、验收与后续立项边界 | `docs/real-usage-hardening-roadmap.md` 顶部与完成卡 | 不读旧 release handoff |
| 已完成路线短投影 | `docs/batch-plan.md` | 不把它当第二份 roadmap |
| 产品定位与 quickstart | `README.md` | 不读历史自包含 runtime 说明 |
| canonical `/steamai` 行为 | `.claude/skills/steamai/SKILL.md` | 不调用已删除的 `/rekit`、Go CLI 或 runtime bundle |
| case/member/research 模板 | `vnext/README.md`，再只读目标模板 | 不一次加载全部模板 |
| Claude Code 原生能力 | `vnext/capabilities.md` | 不把 session ID 当身份或授权 |
| 自动与 live acceptance | `vnext/acceptance.md` | 不把 synthetic probe 当真实用户确认 |
| legacy `.steamai` / `.rekit` 导入 | `vnext/legacy-import.md` | 不运行旧 runtime，不迁移 lane/session/receipt/gate |
| learning 回流 | `vnext/learning-feedback.md` | 不使用旧 promote/writeback，不自动写 pack |
| pack 编写或内容 | `packs/<pack>/manifest.yml` 与目标 pack 文件 | 不读取真实 case artifact |
| 历史旧架构事实 | Git history 或 `CHANGELOG.md` 按关键词/ID 查询 | 不把历史命令当当前产品路径 |
| 文档减压或路由审计 | 本文件 + 目标文档顶部 | 不批量重写历史以改变事实 |

## 执行纪律

- 已完成路线文件拥有验收事实；当前无自动推进的候选批次。
- 代码事实与完成态合同冲突时，先更新路线边界与验收，再立项实现。
- 大文件只按 symbol、标题或行号读取小片段；测试失败先保留测试名和关键错误。
- durable docs 只保留当前事实与 canonical 指针；历史细节留在 Git history/archive。
- source clone 本身不是 case；不得在本仓库创建 `.steamai-vnext/` case state。

## 验证标准

```text
go test -count=1 -p=2 -timeout=30m ./...
go vet ./...
git diff --check
```

涉及 Claude Code 原生行为时按需运行 `vnext/acceptance.md` 中的 opt-in live gate。未执行的 Windows live acceptance、remote CI 或 formal release 必须如实标注，不由 workflow definition、cross-compile 或 synthetic fixture代替。

## 风险与注意事项

- legacy `.steamai` / `.rekit` 是只读 importer source，不是 current runtime。
- `CLAUDE.md` 是角色上下文，不授予工具权限或扩展 case 授权。
- actual heavy action 仍依赖明确 case 授权和 Claude Code 权限确认。
- 不把真实样本、trace/dump/capture、payload、凭据、客户信息、绝对 case 路径或 case 进度写入仓库。
