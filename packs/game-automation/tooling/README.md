# game-automation tooling

本目录保存 Windows 游戏自动化开发的声明式工具目录与固定专业工具接入说明。它不包含 runtime、自动安装、命令调度器或常驻服务；catalog 始终不可执行。

## 内容

| 路径 | 用途 |
|---|---|
| `catalog.yml` | 工具能力、状态、输入输出、确认要求与止损。 |
| `integrations/airtest.md` | Airtest 固定版本、运行接口、失败/停止与产物回交。 |
| `recipes/*.md` | 完整脚本 workflow 与回归 recipe（后续批次交付）。 |
| `examples/` | 合成窗口示例工程（后续批次交付，只用占位符）。 |

## 边界

- catalog 只描述能力；依赖安装只发生在获准环境，由使用方显式执行，pack 不代装、不升级宿主。
- 录制脚本经检查与去敏后才可作为模板；登录态、真实游戏身份与个人桌面截图不进入 pack。
- 注入输入与非幂等动作是 heavy action：需要 exact case scope 的用户具体确认、Claude Code 工具权限、预算与停止条件。
- learning 只从 accepted 交付/review 提炼，用户确认完整 exact patch 前不写 canonical pack。
