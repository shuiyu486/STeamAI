# REA 专业能力接入（binary-re）

## 目的

程序分析任务在派单时即选择最合适的固定专业入口：系统性分析优先使用固定版本的 REA，已有高价值 IDB 或 REA 不支持的操作直接使用现有 IDA MCP；不先执行较弱的默认流程，卡住后再"升级"。

## 固定版本与材料

- 方法材料：`../../upstream/rea/`，固定为 `rea-agents@6.7.0`（commit `27dc42d1`）；来源、拼接 SHA-256 与纪律见其 `SOURCE.md`，按任务需要读取切片，不默认串读全部。
- 本 pack 固定的是方法材料，不是运行环境。负责人用已批准的 CLI `--version` 或 MCP 初始化 `serverInfo` 核对实际 REA 版本，并记录 Node、provider 身份与目标绑定；区分发行版、协议版本与引擎版本，不能互相代替。不把 `binary_session` 类大响应探测作为必经前置（本机曾见 67–145KB 超限）；没有可核验身份时报告缺口，不把跳过探测当作核验成功。实际版本或目标身份不符时按停止条件处理。

## 工具暴露边界

- 只为确需 REA 的成员在其 exact 成员目录注册 local-scope MCP：同一 Claude 配置域、固定版本、经用户批准；不自动安装或升级 REA、不做全局注册、不给 Commander 或其他成员默认加载。
- `steamai.exe` 不因此获得 MCP 配置管理职责；Windows 正式路径不新增 PowerShell、`.cmd` 或 `.bat` 包装。
- local scope 只决定工具在该目录的会话中加载，不是文件或网络隔离；具体动作仍受工具权限、针对该动作的用户确认与 case 授权约束。
- 成员配置变更后须重启会话并核实工具实际可见；`doctor` 类自检通过不能代替真实连接成功。

## 路线选择

| 任务情况 | 首选入口 |
|---|---|
| 新目标、系统性探索、多格式（native/JS/托管/包）分析、证据整理与比较 | REA |
| 已有积累类型/命名/注释的高价值 IDB、定点函数操作 | 现有 IDA MCP |
| REA 该版本不支持的操作（如部分 IDA dossier 面） | 直接专业工具，不强行绕 REA |

已知限制（随上游版本更新重核）：

- 复用当前 GUI 数据库的 attached 模式只认 legacy 工具目录；现代目录的 attached 生命周期未适配，缺工具即拒绝。
- headless 模式复制输入新建私有库，不继承已有 IDB 的分析积累；关闭按 `save:false` 释放，不保存也不关闭用户 GUI 数据库。headless decompile 需分页/分片读取，防止单响应超限。
- 关闭回执的 `cleanup_incomplete` 不证明 worker 进程已退出；需要确认释放时以进程观察为准，连接关闭不等于目标已清理。
- IDA 结果为 live 观察，不套用其他后端的快照缓存收益。

## 证据交接

- 会话内 `evidence_id` 只在单一 MCP 连接内有效；跨成员或跨会话只交接完整导出的 bundle 文件，不转发裸 ID 让对方"自行取回"。
- 导出文件按现有 artifact index 登记（case-relative path、SHA-256、bytes、来源、授权范围）；E 引用实际观察与 limitations，F 表达结论与未证明范围。
- REA 的 `pass`、完整性或 coverage 校验只是输入，不自动等于 STeamAI 的 review `accepted`；重要结论仍走现有 Reviewer 复核。
- 关闭会话前导出必须保留的材料；按所选后端合同核对清理状态，不把"连接关闭"当作"目标已清理"。

## 停止条件与输出超限处置

范围漂移、预算耗尽、REA/provider 版本或目标身份不符、工具执行终态不明时停止并报告具体缺口；不自动重试、不扩大目标、不以较弱替代流程冒充同等交付。

输出超限按严重度分级处置（实测 `search_strings`/`search_procedures` 无 `limit` 参数、结果无界，`inspect_analysis_view` 不支持 search 父证据分页）：

- 可选清单类探测大响应：可跳过；版本与目标身份等必需核验改用已批准的低输出接口，无法核验仍须停止。
- 任务必需结果超限：不重试大响应；在已有读写授权内把 evidence bundle 导出到 case 内 artifact，按字段或小范围读取。导出成功不等于内容已读；导出或有界读取仍不足时停止，不读取未授权的 case 外缓存。
