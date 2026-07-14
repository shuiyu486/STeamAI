# VMP 工具链路由与状态表

> 目的：记录“查找到 / 已使用 / 可使用 / 应止损”的工具链，按任务阶段路由，避免未来新样本重新摸索。状态会随任务推进更新。

## 状态标记

| 状态 | 含义 |
|---|---|
| 主线 | 当前项目已验证有效，后续优先使用。 |
| 辅助 | 可用，但只在特定场景使用。 |
| 慎用 | 容易触发反调试、卡死或产生超大输出，必须有止损条件。 |
| 止损 | 已验证不适配当前样本；新样本可短测，但不要作为主线。 |
| 候选 | 尚未充分集成；未来可评估。 |

## 按任务路由

| 任务 | 首选工具 | 备用/辅助 | 注意事项 |
|---|---|---|---|
| 判断是否适合公开 unpack/import 工具 | `VMP-Imports-Deobfuscator`、`VMPImportFixer`、`vmpfix` 短测 | `VMPStatic`、`minivmpfix`、`NoVmp` | 设置候选数、timeout、日志静音；无命中就止损。 |
| 定位 VMEnter / source stubs | 项目 VMEnter trace 脚本静态扫描模式 | IDA Pro / idalib MCP / x64dbg 静态辅助 | 不要大范围反汇编输出进主会话。 |
| 捕获真实 VMEnter context | in-process VMEnter context probe | DLL injector、ScyllaHide | 优先 in-process probe，避免裸调试。 |
| 离线 dispatch trace | Unicorn trace 脚本 | memory augmenter 补 ranges | 先补 TEB/PEB/GS、必要 DLL/heap 页。 |
| 聚合 trace / VMExit / motif | trace analyzer、routine IR builder、superinstruction miner | handler effect derivation | 只回传摘要，不粘完整 trace。 |
| focused handler 复核 | focused trace + occurrence analyzer | IDA 静态精读 | 用下一 dispatch event 做 exit state。 |
| value-flow / formula fitting | value-flow extractor、batch semantics miner | 专项 `fit_*` 脚本 | 低样本/alias-heavy 必须人工复核。 |
| 动态调试交互 | ScyllaHide + 管理员 x64dbg + x64dbg MCP | x64dbg-automate | 反调试强的样本避免裸 `debug_run`。 |
| 开源工具发现 | Exa / WebSearch / GitHub README review | 手工短测 | 搜索结果只作为候选，不直接替代验证。 |
| 上下文管理 | `references/vmp-re/*` | `captures/doc_archive/**` | 归档只按需片段读取。 |

## 重型工具升级门禁

full trace、动态调试、注入、patch、dump、长时间符号执行等都属于重型动作。执行前先记录：

```yaml
heavy_action: full-trace | debug | inject | patch | dump | symex
decision_reason: <为什么轻路径无法闭合>
tried_light_steps:
  - <已完成的静态/窄 trace/value-flow 动作>
budget:
  runtime_s: <估计>
  disk_mb: <估计>
outputs:
  - <sidecar 输出位置>
stop_conditions:
  - <何时停止>
authorization: manual-gate | preauthorized
```

规则：

- 没有明确阻塞原因时，不把 full trace / dynamic debug 当默认开局。
- 工具大输出只保存到 sidecar，Markdown 只写摘要和路径。
- 会修改 IDB、patch 字节、注入进程、产生 dump 或外部副作用的动作必须有授权来源；若当前 lane packet / autonomy profile 已预授权，可在 scope、预算、止损和记录要求内自主执行，否则先确认。
- 多 agent 不并发写同一个 IDB、debug session 或 confirmed 文件。

## 候选工具进入流程

```text
candidate
  -> short-test with timeout / output cap / stop condition
  -> auxiliary when useful but not主线
  -> mainline-template only after repeated case validation
  -> stoploss when noisy, unstable, or mismatched
```

新增或重评工具时，只记录工具能力、输入输出、适用阶段、风险和止损条件；不要把完整 README 或长 build log 粘入模板。

## 工具状态模板

| 工具/脚本 | 状态 | 路径/入口 | 当前结论 |
|---|---|---|---|
| VMEnter trace 脚本 | 主线 | `<case>/trace_vmenter_seed.py` 或项目等价脚本 | VMEnter seed、dispatch trace、focused trace 的核心脚本。 |
| VMEnter context probe | 主线 | `<case>/vmenter_context_probe*.dll` | 捕获真实 VMEnter context。 |
| suspended launcher / injector | 主线/辅助 | `<case>/launch_*`、`inject_*` | 捕获启动期 VMEnter 或注入 probe。 |
| memory augmenter | 主线 | `<case>/augment_context_memory.py` | 为 Unicorn trace 补 PEB/DLL/heap 页。 |
| value-flow extractor | 主线 | `<case>/extract_handler_value_flow.py` | 聚合 bytecode / old-new VSP / writes。 |
| batch semantics miner | 主线 | `<case>/auto_mine_handler_semantics.py` | 批量挖掘、shape clustering、recommendation。 |
| routine IR builder | 主线 | `<case>/build_routine_ir.py` | 合并 opcode/role confirmed CSV，输出 coverage。 |
| superinstruction miner | 主线 | `<case>/mine_routine_superinstructions.py` | 挖 repeated n-gram / shape motif。 |
| IDA Pro / idalib MCP | 辅助 | 本机 IDA MCP | 适合静态 xref/函数精读；大范围查询交给子 agent 或片段化。 |
| x64dbg MCP | 慎用 | 本机 x64dbg MCP | 可自动化调试；反调试强样本避免裸 `debug_run`。 |
| ScyllaHide | 辅助 | 本机 ScyllaHide | 必要动态交互时配合管理员 x64dbg attach。 |
| VMP import fixers | 止损/短测 | 工具目录 | 新样本可短测；无 protected import 命中就转 trace-based。 |
| 静态 devirtualizer | 止损/短测 | 工具目录 | 若 VMEnter 模板不匹配，快速止损。 |

## 新样本 toolchain triage

1. 静态识别：PE sections、entropy、imports、VMEnter candidates。
2. 公开工具短测：每个工具都设 timeout / 候选上限 / 日志静音；只看是否有明确命中。
3. 若公开工具无命中：立即转真实 context + Unicorn trace，不继续消耗在 import fixer。
4. 动态交互前：确认反调试风险；优先 ScyllaHide + 管理员 attach，避免裸调试。
5. 主线推进：focused trace + value-flow + conservative lowering。
6. 每轮沉淀：把工具适配性、止损原因、可复用参数写回本文件。

## 止损条件

- 日志快速膨胀到 MB 级且无有效命中。
- attach/debug_run 触发反调试弹窗或目标退出。
- import fixer 只产生 unresolved/missing export/emulation failed。
- 静态 devirtualizer 在 VMEnter 模板处断言失败。
- 工具需要手动盯进度且无法稳定复现。

## 维护规则

- 新增或重新评估工具后更新本文件。
- 不贴完整 README / build log；只记录路径、状态、适用场景和止损原因。
- 若工具进入主线，补充到 `workflow-template.md` 的对应阶段。
- 若工具只对当前样本有用，放 `task-handoff.md`，不要污染通用模板。
