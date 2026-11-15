# Game Automation 按需路由入口

> `game-automation` 覆盖已授权 Windows 游戏的自动化脚本开发：从识别状态到有界恢复/停止的可维护脚本交付。这是**开发**任务——交付可运行代码与使用说明，不是漏洞研究；不为开发任务伪造安全 finding。

## 常驻原则

- 先读 case 的 `.steamai-vnext/CLAUDE.md` 与本文件，再只选择一个任务入口。
- 一个 case 只绑定明确批准的目标（窗口标题/类名/进程 identity 由任务列出）；不按全桌面或模糊进程名盲点。
- 首版交付平台只有 Windows（Windows 10/11 x64）；Android/iOS 与多引擎适配不在本 pack 范围内。大漠自带的防护/驱动参数是插件文档化能力，按所在 case 授权正常使用。
- 开发交付：项目代码/补丁 + 测试观察（按 evidence 记录）+ 使用说明 → code review；代码写在任务分配的 case-relative 项目源码路径内，同一源码文件不并发写。

## 路由表

| 任务 | 读取文档 | 说明 |
|---|---|---|
| 框架选择（大漠/Airtest/Poco/Playwright） | `toolchain-router.md` | 大漠与 Airtest 并存，按用户选择、已有资产和目标能力选型；不默认同时控制同一窗口。 |
| 大漠接入、旧资产复用、C++/MFC 开发 | `../../tooling/integrations/dm.md` | 固定用户选定版本；先核对位数与资源，不自动迁移或执行旧脚本。 |
| 大漠接口参数、返回值、字库与线程资料 | `../../upstream/dm-7.2520/README.md` | 仅对应所标注归档；按函数名打开单页，不要求插件或联网。 |
| 大漠新版文档比较/导入 | `../../tooling/dm-docs-update.md` | 变更页待审，新旧目录分开，不升级 DLL、不覆盖旧 case。 |
| 脱离按键精灵的 C++/MFC 基础工程 | `../../tooling/examples/dm-cpp-mfc/README.md` | 实际源码与三 EXE（console/worker/demo_target）；MFC 按钮到真实大漠的窗口 demo 已通过，实测边界见工程 README，具体游戏流程后续开发。 |
| Airtest 接入、版本、运行接口 | `../../tooling/integrations/airtest.md` | 保留 Python 图像自动化路径；固定版本、官方调用、失败/停止与产物回交。 |
| 工具能力、输入输出与止损 | `../../tooling/catalog.yml` | 声明式目录，不自动执行。 |
| 完整 workflow、停止语义与交付 | `workflow-template.md` | 任务输入、阶段、决策核心结构、停止验收判据。 |
| 有状态 Windows 自动化实现方法 | `../../tooling/recipes/windows-stateful-automation.md` | 状态枚举、识别通道、等待与非幂等保护。 |
| 六场景验收与回归 | `../../tooling/recipes/automation-regression.md` | 正常/延迟/错误/暂停恢复/停止/重复启动矩阵。 |
| 参考实现骨架 | `../../tooling/examples/windows-flow/` | 决策核心与执行解耦、fake adapter、控制文件与 ledger。 |
| 成员间任务交接 | `task-handoff.template.md` | 状态/ledger/产物/边界交接字段。 |

## 脚本行为硬边界（自首版生效）

- 识别状态 → 校验前置 → 执行一项动作 → 等待预期状态 → 校验结果；不堆叠固定 sleep 代替状态等待，每次等待/重试有上限。
- 只对安全、幂等或已确认未发生的动作做恢复；提交/支付/领取等非幂等操作不得盲重试。
- 前台输入模式窗口失焦、目标消失、绑定失效、意外弹窗、识别低置信度时暂停或安全停止；后台模式只有在 exact 目标绑定与后台识别/输入均获准并实测有效时才允许非前台，不自动回退前台输入。恢复前重新观察实际状态，不从旧计数器继续点击。
- 支持暂停、继续、正常停止和独立紧急停止入口；结束释放本次连接与自有资源，不关闭用户游戏或清除用户存档。
- 日志说明步骤、观测、决定与失败原因；失败截图只裁目标区域并存在 case 内。
- 脚本自身状态机是应用逻辑，不扩展成 STeamAI 通用工作流引擎，不新增常驻自动化服务。

## 写入与执行边界

- 不把真实游戏身份、账号凭据、未脱敏截图、绝对路径写入 pack；示例配置只用合成窗口与占位符。
- 向目标窗口注入输入属 heavy action：需要明确 case 授权、针对该动作的用户确认与 Claude Code 工具权限；范围或目标变化即停止。
- 可复用经验只从 accepted 交付/review 提炼，用户确认 exact patch 前不写 canonical pack。
