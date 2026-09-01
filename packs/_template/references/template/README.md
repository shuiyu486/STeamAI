# Template pack route

> 这是新 pack 的按需路由入口模板。复制为 `references/<pack>/README.md` 后按领域改写。

## 常驻原则

- 先读 case 的 `.steamai-vnext/CLAUDE.md`，再按本文件只选择一个专项入口。
- pack 只保存跨 case 可复用的流程、证据标准、工具能力和止损条件。
- case 进度、目标和原始研究材料保留在 case-local artifact/evidence/finding/review 中。
- 大 trace、完整反汇编、完整反编译和工具原始输出不进入 pack 或成员上下文。

## 路由表

| 任务 | 读取文档 | 说明 |
|---|---|---|
| 领域工作流 | `workflow-template.md` | 轻到重路线、证据和升级条件。 |
| 组队与复核 | `agent-team.md` | 建议职责、owner/verifier 与 Reviewer 边界。 |
| 工具选择 | `toolchain-router.md` | 工具状态、风险、用户确认和止损条件。 |
| 当前 case 交接 | case-local handoff/finding/review | 不从 pack 推断当前进度。 |

## 维护规则

- 新增流程优先更新本 pack reference；横切规则才进入 common policy candidate。
- 新增工具经验优先更新 `tooling/catalog.yml` 或 `tooling/recipes/*`。
- learning 只从 accepted finding/review 提炼，经 Reviewer 检查并由用户确认 exact patch 后回流。
