# Context routing and progressive disclosure

## 读取指南

本文件是新会话和上下文压缩后接手的唯一完整路由入口。新会话先读根 `CLAUDE.md`、本文件、Git 状态，再从下表只选一个场景入口；不要默认串读历史 roadmap、`CHANGELOG.md` 或旧控制面文档。

## 当前指针

- 当前路线：`steamai-windows-native-product-v1`
- 当前路线入口：`docs/windows-native-product-roadmap.md`
- 短投影：`docs/batch-plan.md`
- canonical 产品入口：`cmd/steamai` 与 `.claude/skills/steamai/SKILL.md`
- 薄核心合同：`vnext/**`
- 历史薄核心完成事实：`docs/real-usage-hardening-roadmap.md`

## 按需路由

| 需要判断什么 | 首选入口 | 不要默认读取 |
|---|---|---|
| 当前 Windows 产品路线、状态与剩余 live gate | `docs/windows-native-product-roadmap.md` | 不把旧 thin-core 完成卡当当前边界 |
| 当前路线短投影 | `docs/batch-plan.md` | 不把它当第二份 roadmap |
| 安装、quickstart、更新、卸载与产品定位 | `README.md` | 不读历史 source-clone-only 说明 |
| native shell 机械边界 | `cmd/steamai`、`internal/steamai/app.go`，再按 symbol 路由 | 不把它扩展为团队控制面 |
| canonical `/steamai` 行为 | `.claude/skills/steamai/SKILL.md` | 不调用已删除的 `/rekit` 或旧 runtime |
| case/member/research 模板 | `vnext/README.md`，再只读目标模板 | 不一次加载全部模板 |
| Claude Code 原生能力 | `vnext/capabilities.md` | 不把 session ID 当身份或授权 |
| 自动与 live acceptance | `vnext/acceptance.md` | 不把 synthetic/cross-build 当真实用户体验 |
| learning 批次回流 | `vnext/learning-feedback.md` | 不使用旧单目标 Checkpoint B，不自动写 pack |
| Release 构建与产物 | `.github/workflows/release.yml` | 不把 workflow definition 当 published release |
| pack 编写或内容 | `packs/<pack>/manifest.yml` 与目标 pack 文件 | 不读取真实 case artifact |
| 历史薄核心事实 | `docs/real-usage-hardening-roadmap.md` | 不据此禁止当前已批准 native shell |
| 更旧架构事实 | Git history 或 `CHANGELOG.md` 按关键词查询 | 不把历史命令当当前路径 |

## 执行纪律

- 当前路线拥有当前边界与验收状态；历史路线只保存当时事实。
- 代码、skill、README、acceptance 和当前路线冲突时先收口合同，再宣称完成。
- native shell 只做安装/Fresh/update/uninstall/卸载后窄自清理/exact apply/可见启动/瞬时互斥；不得增加 task、session、message、roster、finding、review 或 learning decision 状态。
- 大文件按 symbol、标题或行号读取；测试失败保留测试名和关键错误。
- source checkout 本身不是 case；不得在本仓库创建 `.steamai-vnext/` case state。

## 验证标准

```text
go test -count=1 -p=2 -timeout=30m ./...
go vet ./...
git diff --check
```

涉及 Claude Code 或 Windows 外部行为时按 `vnext/acceptance.md` 执行对应 opt-in/manual gate。正式 `v1.0.4` Release 与匿名 latest 下载链路、真实 Windows setup/PATH/Fresh/visible member/duplicate Commander/learning/update/uninstall，以及 Claude Code native context/persistent correction/`HOLD_STALE_TASK`/cross-session message 已按各自范围通过；`v1.0.4` 相对 `v1.0.3` 不改变产品 runtime。后续改动不得沿用这些证据冒充新版本验收；workflow definition、fake platform、cross-compile 或 synthetic fixture 不能替代受影响的真实路径。

## 风险与注意事项

- 产品只支持 fresh/current，不提供旧项目导入、迁移、兼容 runtime、双写或 active case跨电脑迁移。
- `CLAUDE.md` 是角色上下文，不授予工具权限或扩展 case 授权。
- actual heavy action 仍依赖明确 case 授权、针对具体动作的用户确认和 Claude Code 权限。
- 不把真实样本、trace/dump/capture、payload、凭据、客户信息、绝对 case 路径或 case 进度写入仓库。
