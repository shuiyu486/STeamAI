# VMEnter context probe

## 目标

从静态猜测升级为真实 VMEnter register/stack/memory context，供离线 Unicorn trace 使用。

## 捕获内容

- general registers
- eflags
- stack bytes
- TEB/PEB/GS base
- 关键 heap/DLL memory ranges
- VMEnter call site、return address、encrypted VIP seed

## 推荐流程

1. 静态定位 source stub 到 VMEnter 的 call。
2. 在 VMEnter 入口或 call 前后布置 in-process probe。
3. suspended launcher 或 injector 启动目标并加载 probe。
4. 输出 context JSON。
5. 用 memory augmenter 补必要页。
6. 作为 trace script 输入。

## 注意事项

- 成品 DLL 不进入模板；模板保存接口说明、字段 schema、build/attach recipe。
- 反调试强样本默认避免裸 debug_run。
- 必要交互时使用 ScyllaHide + 管理员 x64dbg attach。
- 输出只记录路径和摘要，不把 context 大 JSON 贴入 Markdown。
