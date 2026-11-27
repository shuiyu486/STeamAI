# 大漠 C++/MFC 基础工程

这是**脱离按键精灵的可修改工程基础**，不是具体游戏脚本，也不是 STeamAI 的控制面。把本目录复制到独立项目的源码目录使用；构建目录、EXE、SDK 和私有配置不放回 pack。

## 已有什么

- `dm_console`：MFC 对话框界面，开始、暂停/继续、停止、日志、SDK 文件/版本设置、自建窗口测试入口与注册码密码框；默认演示模式，不加载 SDK。界面源码为 `src/mfc_main.cpp` 等 MFC 文件。
- `dm_worker`：标准 C++17 执行程序。控制管道与工作线程分离，暂停/停止不会被工作线程的循环堵住；对象在同一个工作线程初始化 COM、创建、调用和释放。真实绑定/输入链路在 worker 端完成（见文末）。
- `dm_demo_target`：独立 Win32 目标 EXE，只自建自有窗口（上方左半纯红、右半纯蓝色块，底部输入计数），经 stdin/stdout 私有管道向父进程报告 exact HWND/PID 与计数（`TARGET ready` / `stats` / `page`，`close` 或管道 EOF 正常退出）；默认可见供手动观察，维护测试 `--hidden` 永不显示。`--pages N`（2..8）是"打开/切页"的无消耗等价目标：客户区顶部加一条页签，只有页签内的按下切页并换内容配色，其它区域只计数，`TARGET page <k>` 报告当前页；非法值在创建窗口前退出 2。经典模式的尺寸、绘制、计数与协议行不变。不读用户窗口、不做网络。
- 真实接入代码：只接受指定 DLL、SHA-256、CLSID 和预期版本，核对 PE 位数后，通过标准 COM `DllGetClassObject` / `IClassFactory` / `IDispatch::Invoke` 查询 `Ver`；绑定前按文档先完成 `Reg`（用户自己的注册码，只经父进程 private stdin 传入，见文末许可约束）。不依赖按键精灵、`RegDll.dll` 或机器全局注册；不猜 `RegDll` 私有导出签名。
- 测试：Win32 Release 下，worker/合成 COM 回归 29 项、目标协议 7 项、MFC 回归 4 项全部通过（MFC 内含 11 个生命周期检查点及窗口 demo）。**合成测试不代替真实大漠验收；真实结果另列于文末。**

基础交付包含 SDK 初始化/版本探针与执行生命周期。`dm_sdk_smoke` 测试工具仅面向**自建测试窗口**使用固定类型接口：`BindWindow`（gdi/windows/windows/0）、`CmpColor`、`MoveTo`/`LeftClick`/`KeyPress`、`UnBindWindow`；默认目标隐藏，不向其它窗口发送输入。MFC 按钮路径与 worker 对 `dm_demo_target` 的完整链路均已用真实 SDK 通过（见文末）。

不含真实游戏、全局输入、截图、识字、游戏业务或共享字库，不迁移既有游戏功能。付费 `Reg` 仅在用户明确批准并提供自己的注册码时执行（见文末许可约束），工程不自带、不保存注册码。后续按 exact 版本文档逐项加接口，不开放任意方法调用协议。同一窗口由一个执行单元串行操作；多窗口以后按实际需求增加，不预建调度器。当前使用 MFC，不再依赖 Qt。

## 为什么分成三个 EXE

现有 SDK 为 x86，界面与 worker 都按 Win32（x86）构建，与 SDK 同位数。保留 `dm_console`（MFC 界面）＋`dm_worker`（执行）＋`dm_demo_target`（自建测试目标）三个进程：界面不跨线程传 COM 指针，worker 在自己的线程内完成初始化、调用与释放；测试目标是独立 EXE，只经私有管道报告身份与计数，仅在明确选择自建窗口测试时启动。控制命令只有 `pause` / `resume` / `stop`（目标另有 `stats` / `close`），没有网络端口、脚本执行、自动发现或常驻服务。

## 构建

需要 CMake ≥3.20、VS2019（含 C++17 与 MFC 组件）、Windows SDK。三个应用均为 Win32、静态 CRT（`/MT`），界面另静态链接 MFC。

先在独立项目目录执行。以下 `src` 指本目录复制后的源码位置，`build` / `dist` 均为项目内的新目录；CMake 不负责安装缺失组件。

```text
cmake -S src -B build -G "Visual Studio 16 2019" -A Win32
cmake --build build --config Release
ctest --test-dir build -C Release --output-on-failure
cmake --install build --config Release --prefix dist
```

上述路径已在本机 VS2019/Win32 Release 实际构建并运行 CTest，具体覆盖与未验项见文末。缺 MFC 组件时构建会失败，不默默省掉界面后宣称完整交付。关闭测试可加 `-DBUILD_TESTING=OFF`；否则测试需要 Python 3，应用运行本身不依赖 Python。

## 运行与部署

1. 静态 MFC/CRT 使 `dist` 中的 EXE 不依赖 MFC/VC 运行库 DLL，可直接在同类 Windows 机器上运行。这是本基础工程在当前本机/内部交付的取舍，不宣称微软推荐所有对外分发都静态链接；VC 运行库发布安全修复后需重新编译更新。
2. 大漠 DLL、字库等第三方文件不随工程分发：由用户单独提供并放在获准位置，运行时按设置指定 exact 文件；SDK 与运行库版本仍需逐项核对。
3. 双击 `dm_console.exe`。默认演示模式可测开始、暂停/继续、停止；它不操作桌面。可手工指定另一个可信的 `dm_worker.exe`，不会自动扫描磁盘。
4. 实际查询 SDK 前，填入本次 DLL、只读计算其 SHA-256、填写 `Ver` 预期完整字符串与该版本的 CLSID，再勾选本次加载许可。改任一字段即撤销勾选；每次启动重新确认。目录名与 SHA-256 都不能单独证明真实版本。需要绑定链路时，注册码在界面密码框输入、只经父进程 private stdin 传给 worker（见文末许可约束）。
5. 跑完整 demo：把三个 EXE 放在同一目录，取消“演示模式”，填好上述 SDK 字段，勾选“绑定自建测试窗口”，输入自己的注册码及 `ver_info`（可空，不是 SDK 版本号），最后勾选本次许可并点击“开始”。普通运行会出现红蓝测试窗口；程序自动完成取色、一次点击、一次 A 键，随后解绑并关闭该窗口。日志出现 `DEMO pass` 和退出 `code=0` 才算完成；失败则查看 `ERROR` 和业务返回码，不自动重试注册。
6. CLSID 应取自对应 SDK 的类型库/官方声明；已有系统登记可辅助核对，但不能凭登记就证明版本。未知时停止，不调用 `DllRegisterServer` 或猜测私有接口。

EXE 不等于单文件分发；SDK 由用户单独提供，不打包进此仓库。程序不保存凭据；`Reg` 只按文末许可约束经 private stdin 执行，也不安装驱动/修改注册表。加载 DLL 本身会执行其代码；“本程序只调用 Ver”不保证第三方 DLL 内部绝不联网，必须在批准的环境测试。

## 停止与后续扩展

- 暂停和停止均先显示“已请求”，不把发出请求等同于 SDK 已停止。检查点阻止下一步开始，但不能强行打断正在运行的 DLL 调用。
- 停止超过 2 秒，界面显示超时并继续等待，保持可响应；不强杀线程、不自动重启、不伪报释放成功。关闭窗口也会先请求停止，等执行程序退出后再关闭。
- 父进程管道关闭会触发停止，但如果 SDK 卡住，仍需等调用返回；因此当前不承诺阻塞 SDK 的 2 秒停止上界或无人值守。SDK 永久不返回时，界面也会一直等待关闭；当前没有自动强制终止入口，人工终止进程不能当作正常清理成功。
- 最小绑定/输入接口已带显式解绑与失败清理；后续游戏循环仍须补真实目标的停止/暂停回归。若增加 `KeyDown` 等保持按下的动作，还需配对释放，不能用合成测试代替目标实测。
- 版本升级时同时检查 DLL hash、PE 位数、`Ver` 和对应 CHM 差异；仅换文档不会自动换 DLL。资料更新入口为 [`../../dm-docs-update.md`](../../dm-docs-update.md)。

## 当前验证边界

本次在 Windows 10 x64、VS2019/MSVC 19.29、Windows SDK 10.0.20348.0 上完成 Win32 Release 构建：`dm_console.exe`、`dm_worker.exe`、`dm_demo_target.exe`、SDK 冒烟工具与合成 DLL 均已生成。MFC 与 CRT 静态链接；`dumpbin /dependents` 确认应用 EXE 只列系统 DLL，没有 Qt/MFC/VC 动态运行库依赖。该依赖检查不代表所有其它机器均已验收。

CTest 三组全过，共 43 个 Python 用例：worker/合成 COM 29 项（版本、Reg 前置与许可输入拒绝、绑定/取色/输入失败清理、暂停/恢复/停止/父管道关闭），目标协议 10 项（身份、计数、close/EOF、非法命令、`--pages` 切页/非法参数、经典模式恒为 1），MFC 4 项（11 个生命周期检查点、窗口 demo、9 个参数拒绝子场景、3 个许可输入拒绝子场景）。测试走实际控件消息和进程管道，不使用真实鼠标键盘。合成 DLL 的跨进程取色模拟固定红蓝测试图，只验证调用与流程；真实取色证据来自下述真实 SDK 测试。此结果**不等于可见界面外观/DPI、多窗口游戏或长时间无人值守验收**。

`--pages` 模式另由 `../windows-flow/workbench.py observe/matrix` 在屏外窗口上实测（显示到所有显示器之外、不占前台；画面经 `PrintWindow(PW_CLIENTONLY)` 自绘与窗口 DC 两路一致才算有证据；点击坐标必须在目标 DPI 上下文里发送，否则被系统按 DPI 放大而切错页签），结果与边界记在 [windows-flow README](../windows-flow/README.md)。隐藏窗口读不到像素，只报 unknown。

真实大漠 **`Ver` 探针已验证通过**：经用户批准、指定 7.2520 DLL 并核对文件 hash 与 CLSID 来源后，worker 完成版本比较并正常 stop/释放/exit 0，无强杀，DLL 文件 hash 未变（本机路径、CLSID 与 hash 不写入仓库）。

**worker 端真实链路已通过**：对 `dm_demo_target` 自建目标按顺序完成 `Ver` → `Reg`（成功）→ `BindWindow`(gdi/windows/windows/0) → `CmpColor` 正反探针 → click 计数 exact 1 → `'A'` 键计数 exact 1 → `UnBindWindow` → 目标正常关闭，全链 exit 0。目标运行在 `CreateDesktop` 新建的非活动桌面：默认显示、可绘制、可绑定，但从不 `SwitchDesktop`，用户输入桌面与前台不变，SDK 文件不变。此前记录的绑定阶段 `0xC0000005` 崩溃根因已定位：漏掉文档要求的 `Reg` 前置步骤；补 `Reg` 后同一 MTA、mode0 与直接 COM 工厂路径即可绑定，未改加载方式。另一实测边界：从未显示的隐藏目标 `BindWindow` 成功但取色失败（无画面），因此隐藏验收使用非活动桌面显示的自建目标。

**MFC 端真实整体验收也已通过**：隐藏的 MFC 界面预填同一组真实控件，经 `BM_CLICK` 触发实际开始按钮，再启动上述 worker；日志依次出现 `SDK license-registered`、全部 `DEMO` 阶段、`DEMO pass`、`STATE stopped` 和 worker `code=0`，界面进程也 exit 0。未强杀，无新增测试桌面残留，用户输入桌面与 DLL hash 均未改变。普通双击路径仍显示界面和自建目标，未做前台外观/DPI 验收。

**许可约束（`Reg`）**：按文档 7.x 除 `Ver` 版本探针外的所有功能都需先完成 `Reg`。注册码是用户自己的，只在 MFC 密码框输入并经父进程 private stdin 传给 worker，不进命令行参数、日志或任何持久文件；每次确认都可能联网/计费；`ver_info` 是注册时传入的自定义版本信息，可空，不是 SDK 版本；版本核对仍用 `Ver`。缺少 `Reg` 批准、`Reg` 返回失败码或许可输入时立即停止，绝不调用 `BindWindow`。

没有真实游戏、全局输入或游戏迁移验收；Airtest 并存、版本更新流程、不安装驱动、不做系统 COM 注册的边界不变。源码、EXE 构建、MFC 联动、真实 SDK 的各阶段分别记录，不能互相替代。

实现参考：[Microsoft 标准 COM 导出](https://learn.microsoft.com/en-us/windows/win32/api/combaseapi/nf-combaseapi-dllgetclassobject)、[DLL 搜索路径](https://learn.microsoft.com/en-us/windows/win32/api/libloaderapi/nf-libloaderapi-loadlibraryexw)。本示例为显式文件选择采用直接取得标准 class factory 的窄加载路径；Microsoft 的一般调用路径是 COM 激活。它是否适用于某版大漠仍以该版真实测试为准，不宣称厂商支持了此加载方式。
