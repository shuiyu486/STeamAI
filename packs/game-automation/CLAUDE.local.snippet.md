<!-- BEGIN game-automation-template:router v0.1.0 -->
## Game Automation 按需路由

`game-automation` 面向已授权的 Windows 游戏自动化脚本开发：识别状态 → 校验前置 → 执行一项动作 → 等待预期状态 → 校验结果，附有界恢复与安全停止。先读 `references/game-automation/README.md`，只按当前任务选择一个入口。

| 任务 | 读取 |
|---|---|
| 选择自动化框架/入口 | `references/game-automation/toolchain-router.md` |
| 大漠接入、既有资产与 C++/MFC | `tooling/integrations/dm.md` |
| 大漠接口与线程/字库离线资料 | `upstream/dm-7.2520/README.md`（其它版本先核对对应 CHM） |
| 更新大漠文档、比较新版变化 | `tooling/dm-docs-update.md` |
| 不依赖按键精灵的 C++/MFC 基础工程 | `tooling/examples/dm-cpp-mfc/README.md` |
| Airtest 接入、安装与运行接口 | `tooling/integrations/airtest.md` |
| 工具能力与止损 | `tooling/catalog.yml` |

规则：只绑定明确批准的目标窗口/设备，不按全桌面或模糊进程名盲点；非幂等操作不得盲重试；暂停/停止入口在主循环与等待中都必须及时响应。脚本、配置、依赖锁与使用说明交付在明确分配的项目路径内；测试观察按 evidence 记录。首版交付平台只有 Windows。
<!-- END game-automation-template:router -->
