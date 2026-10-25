# Context routing and progressive disclosure

## 读取指南

本文件是新会话和上下文压缩后接手的唯一完整路由入口。先读根 `CLAUDE.md`、本文件和 Git 状态，再按当前任务只选一个入口；不默认串读历史 roadmap、`CHANGELOG.md` 或所有 pack。

当前没有 active 实施路线。专业交付执行计划和无待办短投影已删除，结果与未解决问题见 `docs/professional-delivery-results.md`。日常使用直接读 README/skill；核查历史证据时才读验收记录，旧结果不授予新动作权限。

## 当前工作入口

| 需要判断什么 | 首选入口 | 读取边界 |
|---|---|---|
| 产品目标、自主协作与管理边界 | 根 `CLAUDE.md` 的“智能成员与自主协作基准” | 不把方法参考改成逐步强制流程，不以加规则代替能力 |
| 安装、quickstart、更新、卸载 | `README.md` | 区分当前源码与已发布版本；已有 case 不自动更新 |
| 三线实际可用范围、效果证据与已知缺口 | `docs/professional-delivery-results.md` | 批次收束不等于所有 gate 通过；关注阻塞期间停止、模板伪匹配与 learning 待验项 |
| 实用能力与效率路线（U0–U6）的验收结论 | `docs/capability-efficiency-roadmap.md` | 42 run 冻结评测全对全 tie：只交付可靠性改善，方法库增益未被证明也不被否定；评测材料在 `%TEMP%\steamai-pd\u-cap\` |
| native shell 机械边界 | `cmd/steamai`、`internal/steamai/app.go`，再按 symbol 路由 | 不扩为团队控制面 |
| canonical `/steamai` 行为 | `.claude/skills/steamai/SKILL.md` | 不调用已删除的 `/rekit` 或旧 runtime |
| case/member/research 模板 | `vnext/README.md`，再选目标模板 | 不一次加载全部模板 |
| 原生 session、消息与恢复 | `vnext/capabilities.md` | session ID 不代表身份或授权 |
| 自动与 live acceptance | `vnext/acceptance.md` | 静态/fake/headless 不能代替受影响的真实路径 |
| learning 批次回流 | `vnext/learning-feedback.md` | exact confirmation 不可省，不自动写 pack |
| replay、V0–V4、calibration、promotion、field outcome | `vnext/verified-learning.md` | 缺证不晋级；盲审读取 manifest 绑定的 immutable packet |
| Release 构建与产物 | `.github/workflows/release.yml` | workflow 存在不等于已发布 |
| pack 编写或内容 | `docs/pack-authoring.md` 或 `packs/<pack>/manifest.yml` 与目标文件 | 不读取真实 case artifact，不自动执行工具 |

## 历史证据（只按具体问题读取）

以下旧路线保留证据，不是当前实施任务；不续写新实验、不自动重跑旧题、不把未验项改成完成。

| 需要追溯的事实 | 入口 | 不能继承的结论 |
|---|---|---|
| 旧专业负责人 REA/IDA/Strix 接入 | `docs/professional-delegation-roadmap.md` | 旧 GUI open/close 不证明分析期间保留；旧 PD-04C 只有有限答案盲评 none，适用范围见专业交付验收记录与 Git 历史 |
| Fresh 机器确认、工作区信任、七字段纠偏 | `docs/low-friction-collaboration-roadmap.md` | 有限 A/B tie 不证明普遍增益，自动信任不是通用配置管理 |
| 自主协作合同与可见任务 | `docs/autonomous-collaboration-roadmap.md` | 保留 inconclusive，不把合同落地当效果提升 |
| 已知报告校准与新9题研究对照 | `docs/research-validity-roadmap.md` | 保留 pass-limited/tie，不作为新行为或 learning 晋级证据 |
| 专业取证、跨步骤预测、客户端/API 联合 | `docs/research-execution-roadmap.md` | 保留原对照 incomplete/inconclusive，不续写新日志 |
| 双领域方法、分岔点与主辅 pack | `docs/research-capability-roadmap.md` | 四组32轮平局不证明新能力更强 |
| verified-learning Gate 1–4 既有证据与未验门槛 | `docs/verified-learning-roadmap.md` | 无 current go/eligible comparative journey；多 case field evidence 仍 pending |
| Windows 产品基线 | `docs/windows-native-product-roadmap.md` | 已发布 v1.0.4 历史验收不证明后续源码行为 |
| 薄核心与更旧架构 | `docs/real-usage-hardening-roadmap.md`；必要时 Git history/`CHANGELOG.md` | 历史产品命令不是当前路径，不恢复旧控制面 |

## 执行与验证纪律

- 源码、skill、README 与 acceptance 冲突时先修合同；已完成工作的执行步骤删除，最终结论与失败/未知保留在对应验收记录。没有 active 路线时不另造短投影。
- native shell 只做已有安装/Fresh/update/uninstall/卸载后自清理/evaluation/exact apply/可见启动/瞬时互斥，不增加团队判断状态。
- 源仓库不是 case，不在此创建 `.steamai-vnext/`；不保存真实样本、流量、凭据、客户信息或绝对 case 路径。大文件按 symbol、标题或行号读取。
- 默认验证不调用模型、不占前台。先跑与修改相关的 focused tests；全量机械回归命令如下，受影响的真实行为另按 acceptance 独立验收，不能继承旧版本结果：

```text
python -m unittest discover -s tests/pack_tooling -p "test_*.py"
go test -count=1 -p=2 -timeout=30m ./...
go vet ./...
git diff --check
```

`CLAUDE.md` 只提供角色上下文，不授予工具权限；具体动作确认、Fresh/learning exact confirmation 与用户纠偏边界不因文档精简而变化。
