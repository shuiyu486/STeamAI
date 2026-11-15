# game-automation tooling

本目录保存 Windows 游戏自动化开发的工具目录、固定资料、显式文档更新工具与参考工程。它不增加 STeamAI runtime、自动安装、通用命令调度器或常驻服务；catalog 始终不可执行，示例在独立项目中构建运行。

## 内容

| 路径 | 用途 |
|---|---|
| `catalog.yml` | 工具能力、状态、输入输出、确认要求与止损。 |
| `integrations/dm.md` | 大漠固定版本、资料查询、既有资产复用与 C++/MFC 接入边界；不捆绑 SDK。 |
| `../upstream/dm-7.2520/README.md` | 离线接口目录、42 个常用接口及线程/共享字库等约束；SOURCE.md 绑定来源与逐页校验。 |
| `integrations/airtest.md` | Airtest 固定版本、运行接口、失败/停止与产物回交；与大漠并存，按任务选用。 |
| `dm-docs-update.md` / `scripts/dm_docs.py` | 对应版本 CHM 的显式比较/导入；只写新目录，变更页待审，不自动升级 SDK。 |
| `examples/dm-cpp-mfc/README.md` | 实际 C++/MFC 基础工程、构建/部署命令及验证边界；不含具体游戏业务。 |
| `recipes/*.md` | 有状态脚本开发与回归参考。 |
| `examples/windows-flow/` | 保留的 Python/Airtest 决策与合成目标示例。 |

## 边界

- catalog 只描述能力；依赖安装只发生在获准环境，由使用方显式执行，pack 不代装、不升级宿主。
- 录制脚本经检查与去敏后才可作为模板；登录态、真实游戏身份与个人桌面截图不进入 pack。
- 注入输入与非幂等动作是 heavy action：需要 exact case scope 的用户具体确认、Claude Code 工具权限、预算与停止条件。
- learning 只从 accepted 交付/review 提炼，用户确认完整 exact patch 前不写 canonical pack。
