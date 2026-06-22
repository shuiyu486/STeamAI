# IDA / x64dbg / MCP usage

## IDA / idalib MCP

适用：

- 静态 xref
- 函数边界与 basic blocks
- 局部反编译精读
- 类型/注释辅助

规则：

- 大范围搜索交给子任务或脚本摘要。
- 不把完整 decompile/disasm 输出粘进主会话。
- 只读取目标函数、目标 handler 或必要行范围。

## ida-agent-bridge 候选用法

`ida-agent-bridge` 可作为 headless IDA / 文件系统索引候选工具，当前状态是 `candidate`，不是本 pack 的硬依赖。

适用：

- 让 Agent 先读 `function_index.tsv`、`strings.tsv`、`imports.tsv` 等索引，再窄范围查询函数。
- 用短连接命令获取局部 pseudocode、xref、hexdump 或函数详情。
- 对已导出的函数 sidecar 做 grep / diff / 增量复查。

建议流程：

1. 先确认是否已有 bridge/export 目录；没有则评估是否需要启动。
2. 默认优先 `--skip-export` 或窄范围查询；只有批量分析需要 `function_index.tsv` 时才确认全量导出。
3. Agent 只读索引和 sidecar；rename/comment/patch 属于共享 IDB 写入，必须避免多 Agent 并发写。
4. 大输出保存在 bridge export 目录或 case sidecar，Markdown 只写摘要、命令和路径。

止损条件：

- 首次全量导出耗时或输出规模超过当前任务预算。
- 目标函数已可通过现有 IDA MCP / sidecar 精读，不需要额外 bridge。
- bridge 查询开始替代 focused trace / value-flow 验证，导致结论只依赖 F5。

## x64dbg MCP

适用：

- 必要动态交互
- breakpoint / memory / register spot check
- 与 ScyllaHide 组合验证少量假设

风险：

- 反调试强样本可能在裸 debug_run 下弹窗或退出。
- attach 前确认权限、ScyllaHide、目标状态。

推荐：

1. 默认优先离线 dump、probe、Unicorn trace。
2. 只有必要人工交互时才使用 x64dbg。
3. 使用管理员 x64dbg + ScyllaHide attach，而非裸启动。
4. 记录观察摘要，不把调试大输出写入 Markdown。
