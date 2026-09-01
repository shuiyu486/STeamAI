<!-- BEGIN template-pack:router -->
# Template pack router

本文件来自 `packs/_template`，用于创建新 pack 时复制和改名；不要直接用于真实 case。

- 当前 pack：`<new-pack-name>`
- 路由入口：`references/<new-pack-name>/README.md`
- 工作流：`references/<new-pack-name>/workflow-template.md`
- 团队协作：`references/<new-pack-name>/agent-team.md`
- 工具路由：`references/<new-pack-name>/toolchain-router.md`

规则：

- 先读 case 的 `.steamai-vnext/CLAUDE.md` 和本 pack 路由，再按当前任务读取一个专项入口。
- Commander 按需创建 durable member；成员身份和任务只由成员目录 `CLAUDE.md` 承载。
- 只把 artifact/evidence/finding/review 与 learning candidate 持久化为团队事实。
- case 私有事实、真实样本、trace、dump、artifact、凭据和绝对路径不得写回 pack。
- heavy action 必须绑定 exact case scope、用户具体确认、Claude Code 工具权限、预算和止损条件。
<!-- END template-pack:router -->
