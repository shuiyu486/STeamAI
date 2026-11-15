# Windows 桌面 MCP

## 用途与选择

这是三个 pack 共用的桌面基础入口：截图、查看 UI 元素、鼠标移动/点击、键盘输入和滚动。需要操作普通 Windows 软件时优先复用它，不为每个软件临时编写点击脚本；浏览器专用操作仍用 Playwright，游戏后台绑定和既有字库仍用大漠，Python 图像流程仍可用 Airtest。一次只选择一个操作者和输入通道，避免多个成员争抢同一桌面。

选用开源 [CursorTouch/Windows-MCP](https://github.com/CursorTouch/Windows-MCP)，固定 [windows-mcp 0.8.9](https://pypi.org/project/windows-mcp/0.8.9/)，MIT，Windows 10/11 x64。本版本要求 Python >=3.14；uv 可单独管理解释器，不替换系统 Python。STeamAI 只提供原生 MCP 接入合同，不自建 runner、adapter host、代理 LLM 或后台服务，不把第三方源码/运行环境打包进 snapshot。

固定包的上游 SHA-256（仅主包，不能当作全部传递依赖锁定）：

| 文件 | SHA-256 |
|---|---|
| `windows_mcp-0.8.9-py3-none-any.whl` | `e90e5b02b3e545991f203ce93cefb436a326861657968591653fa254105dc85d` |
| `windows_mcp-0.8.9.tar.gz` | `712a681f3783132e4e1cfada9ee49341082a4ba3ebc3fa48e36cb6e1b3f1a247` |

## 一次性启用

先说明并确认：下载上述固定版本及 Python 3.14/依赖，写当前用户的 uv 缓存/工具目录；通过 Claude Code 原生命令向当前配置域注册 `windows-desktop`。`--scope user` 会让该配置域所有项目可见，不只是一个 case；不同 `CLAUDE_CONFIG_DIR` 需要分别接入。已有同名配置先查看，不覆盖或删除。Fresh、普通 `steamai` 启动及读取本文都不自动安装或注册。

批准后，在已安装原生 Claude Code 和 uv 的环境执行：

```text
uv tool install --python 3.14 windows-mcp==0.8.9
claude mcp add windows-desktop --scope user --env ANONYMIZED_TELEMETRY=false WINDOWS_MCP_WATCHDOG=false -- uv tool run --offline windows-mcp@0.8.9 serve --transport stdio --control --tools Screenshot,Snapshot,Click,Type,Scroll,Move,Shortcut,Wait
```

安装只做一次；正常 MCP 启动使用离线工具环境，不因启动成员而联网更新。若离线命令找不到该环境，停止并检查安装状态，不自动去掉 `--offline`。机器上 uv 不在 WezTerm server 的 PATH 时，在注册命令中用已核实的 `uv.exe` 绝对路径；路径仅存本机 Claude 配置，不写入 pack/case 模板。

仅某个项目需要时可选择 `--scope project`，写入该项目 `.mcp.json` 并走原生信任确认；不假定 `--add-dir` 会加载该文件，也不假定所有成员 cwd 能继承它。跨项目基础能力推荐明确 opt-in 的 user scope，而不是为每名成员复制配置。卸载配置用原生 `claude mcp remove windows-desktop --scope user`；删除工具环境是另一项有明确目标的动作，不自动删除 uv 或共享 Python。

旧交互会话不保证热发现外部新增的 MCP。先正常退出，再从原 cwd 用 `claude --resume <原会话ID或名称> --add-dir <CASE_ROOT>` 恢复同一会话；需要维持所选档位时另带 `--model opus|sonnet`。不要在旧会话仍写入时另开同目录副本。新进程中用 `/mcp` 查看工具；原生 `claude mcp get windows-desktop` 可单独核对该 server，避免为检查它而启动全部其它 MCP。本机 2.1.273 不支持较新版的 `/mcp reconnect all`，也不能把 Reconnect 当成热发现保证。

## 固定启动边界

- 显式 `ANONYMIZED_TELEMETRY=false`。该版本遥测默认开启；静态源码显示关闭后不构造 PostHog analytics、不生成其用户 ID。静态审读不是运行期网络验收，不能据此承诺所有依赖零出站。
- 白名单为 `Screenshot,Snapshot,Click,Type,Scroll,Move,Shortcut,Wait`；上游强制保留 `ControlStatus`，预期共 **9** 项。大小写必须精确；本版拼错会静默移除，所以必须核对实际 `tools/list`。
- 不暴露 `PowerShell`、`FileSystem`、`Registry`、`Process`、`Scrape`、`Clipboard` 或自动启动应用工具。已有 shell 能力仍受 Claude Code 自身权限约束，不通过桌面 MCP 绕过。
- stdio 生命周期由 Claude Code 管理，不安装系统服务、不自动提权。显式 `--control` 保留观察真人接管的低级键鼠钩子和提示层；启动本身不代表发生点击。`ControlStatus(release=true)` 用于操作完成后归还桌面，不能长期占用控制权。安装前须披露这些副作用；不能悄悄关闭接管保护。UIA watchdog 与接管保护不同，前者显式关闭，不影响按需取树。
- 截图/UI 树会进入当前 Claude 会话，可能发给用户配置的模型服务；关闭包遥测不等于截图不上云。只观察获准窗口，避免暴露其它应用、凭据和隐私；不擅自截全桌面，不把原图复制进 canonical。

## 实际能力与限制

输入走前台 `SendInput` / `SetCursorPos`，不是大漠式后台绑定。需要已解锁的交互桌面；不能保证提权窗口、UAC 安全桌面、锁屏、独占全屏或拒绝注入输入的游戏可用。UIA 不一定看得到游戏控件，此时可用截图坐标，但应先观察并核对窗口、DPI、缩放和坐标范围，不能宣称“任意游戏都能操作”。DXGI 截图能力也须对实际目标验证。

没有合适 UIA 元素时不自动升级权限、安装驱动或绕过目标保护。目标窗口丢失/变更、坐标不确定、真人接管、输入未生效、超时或停止要求到来时停止，不盲点、不循环重试、不扩大目标。

## 分层验收

1. **安装与配置**：核对固定版本、解释器、原生 MCP 注册位置；命令成功只证明注册，不证明界面可控。
2. **协议**：仅对该 MCP 做 initialize、`tools/list`、`ControlStatus`，确认上述 9 项工具且无多余 shell/文件工具；不顺带启动所有其它 MCP，不移动鼠标、不截图。
3. **前台实操**：用户明确同意时间窗后，只对自己新建的无害测试窗口做一次截图、一次点击、输入固定测试文本，再读取结果核对。限制截图范围，记录实际窗口/DPI和计数；结束关闭全部自开窗口。未取得时间窗时不执行此层，也不能用协议通过替代它。
4. **真实目标**：在具体 case 授权下另验软件或游戏；自建测试窗口成功不等于所有软件可用。多会话串行操作、随时接受用户接管，不增加桌面锁服务或任务数据库。

按 `../policies/tool-adapters.md` 报告当前状态。本文与配置示例本身不宣称已安装、已在当前会话暴露或已实测；实际观察留在执行环境，已有 case 的 pinned snapshot 不自动更新。
