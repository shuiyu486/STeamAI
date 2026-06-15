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
