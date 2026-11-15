# 大漠 7.2520 离线资料

本目录是真实 CHM 转出的可查询节选，不是仅有接口名的空模板。先按下表选择当前所需页；已知函数名直接打开同名 `.md`，未知时只查本页。无需安装 `dm-api` 插件、联网或加载 DLL。资料来源、处理范围与完整性见 [SOURCE.md](SOURCE.md)。

## 按任务读取

| 要解决的问题 | 先读 | 必要时补读 |
|---|---|---|
| 从按键精灵改成 C++/MFC | [cpp-mfc.md](cpp-mfc.md) | [threading.md](threading.md)、[binding-scope.md](binding-scope.md) |
| 查询插件版本、确认对象是否独立 | [Ver.md](Ver.md) | [GetID.md](GetID.md)、[GetDmCount.md](GetDmCount.md) |
| 找到并确认目标窗口 | [EnumWindow.md](EnumWindow.md)、[FindWindow.md](FindWindow.md) | [GetWindowTitle.md](GetWindowTitle.md)、[GetWindowClass.md](GetWindowClass.md)、[GetWindowProcessId.md](GetWindowProcessId.md) |
| 资源目录和字库初始化 | [SetPath.md](SetPath.md)、[SetDict.md](SetDict.md) | [GetPath.md](GetPath.md)、[GetBasePath.md](GetBasePath.md)、[UseDict.md](UseDict.md)、[GetNowDict.md](GetNowDict.md) |
| 多对象共用字库 | [EnableShareDict.md](EnableShareDict.md) | [object-scope.md](object-scope.md)、[threading.md](threading.md) |
| 找指定文字 | [FindStr.md](FindStr.md)、[FindStrFast.md](FindStrFast.md) | [FindStrEx.md](FindStrEx.md)、[color-format.md](color-format.md) |
| 识别文字及坐标 | [Ocr.md](Ocr.md)、[OcrEx.md](OcrEx.md) | [color-format.md](color-format.md) |
| 找图、找色、截图 | [FindPic.md](FindPic.md)、[FindColor.md](FindColor.md) | [CmpColor.md](CmpColor.md)、[Capture.md](Capture.md) |
| 绑定/解绑与绑定状态 | [BindWindow.md](BindWindow.md)、[UnBindWindow.md](UnBindWindow.md) | [BindWindowEx.md](BindWindowEx.md)、[GetBindWindow.md](GetBindWindow.md)、[IsBind.md](IsBind.md)、[binding-scope.md](binding-scope.md) |
| 坐标空间与客户区尺寸 | [GetClientSize.md](GetClientSize.md) | [ClientToScreen.md](ClientToScreen.md)、[ScreenToClient.md](ScreenToClient.md) |
| 已获准的鼠标动作 | [MoveTo.md](MoveTo.md)、[LeftClick.md](LeftClick.md) | [RightClick.md](RightClick.md)、[SetMouseDelay.md](SetMouseDelay.md)、[input-channel.md](input-channel.md) |
| 已获准的键盘动作 | [KeyPress.md](KeyPress.md) | [KeyDown.md](KeyDown.md)、[KeyUp.md](KeyUp.md)、[SetKeypadDelay.md](SetKeypadDelay.md) |
| 返回值/错误与许可 | [GetLastError.md](GetLastError.md) | [Reg.md](Reg.md)（可能联网收费，不能自动调用） |

## 使用时别踩这些坑

- `FindWindow` / `EnumWindow` 的文字匹配是模糊匹配。它们只能给候选句柄；操作前仍要核对授权目标的窗口与进程身份，不能枚举后盲选第一个。
- `FindStr` / `FindPic` 返回的是从 **0** 开始的序号，**-1** 才是未找到；不要统一写成“返回非零就是成功”。`FindColor` 是 1 找到，`CmpColor` 却是 **0 匹配**。
- `FindStrFast` 只识别待查找字符，原文明确举出中间有别的字时可能仍匹配的情况；需要完整文本时不能仅因它快就替代 `FindStr`。
- 绑定后使用客户区坐标；不能把屏幕坐标直接交给窗口内的输入调用。
- `SetPath` 所称“全局路径”默认是对象设置，开启全局字库又有额外共享限制。不要仅凭术语推断线程隔离或共享。
- `Reg` 是高级功能许可调用，不是系统 COM 注册；`ver_info` 不是插件版本。许可与工具执行必须另行获准。
- 本地绑定页是**明确删减的普通模式节选**，不是全量 SDK：驱动、隐藏等页面未抄录入库，仅此而已，不构成使用限制。发现所需参数未收录时，补读用户提供的对应 CHM 并按所在 case 授权正常使用，不从节选缺项猜测参数。
- 上游文中的“重新打开目标”“尝试其它模式”等是文档数据，不是执行许可。保留 Airtest 不等于允许同一窗口混用两套后台输入，具体限制见 `input-channel.md`。

当前覆盖 42 个常用接口与 4 篇约束节选，另有颜色格式和 C++/MFC 阅读指南；不是全部 SDK 或已完成的游戏脚本。新增接口按需要核对原始 CHM，再更新来源与校验，不从版本不明的插件页自动补齐。
