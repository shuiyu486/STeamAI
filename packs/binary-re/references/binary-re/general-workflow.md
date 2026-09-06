# Binary RE 通用工作流

## Scope baseline

开始前在 case/member `CLAUDE.md` 中记录：binary/function 脱敏 alias、授权范围、允许/禁止动作、预期产出、停止条件和非目标。

## 执行顺序

1. 建立授权范围和 case-local binary/function alias。
2. 读取 metadata、section、import/export、string、symbol 等 passive sidecar。
3. 形成 bounded function/API/format/behavior hypothesis。仅当问题跨越实际处理边界时，按需读 [有界行为链与预测验证](bounded-behavior-chain.md)，核查值/对象如何传递并检验未查看的分支；单点任务不强制扩成调用链。
4. owner 写 evidence 与 finding；必要时由一名 verifier 做有界交叉检查。
5. 独立 Reviewer 只读复核并写 review；证据不足退回原 owner。
6. 只有 passive 路线不足时，才申请 exact debug/trace/dump/patch/writeback action。

升级前必须记录：静态路线为何不足、已尝试动作、exact target、运行/输出预算、隔离与网络策略、case-relative output、rollback 和 stop conditions；取得用户对该具体动作的确认与 Claude Code 工具权限后才能执行。

## 完成检查

- 文档没有样本名、hash、IOC、完整函数体、dump、trace、patch、客户上下文或绝对路径泄漏。
- finding 能追溯到 exact evidence 与 Reviewer decision。综合 F 直接列出全部支撑连接的 E；局部 accepted 不代替整体 review，缺连接时只交付可证范围。
- heavy action 的用户确认、工具权限、预算、隔离、输出和止损均与 exact scope 一致。
- 共享 analysis database、confirmed table 或报告有一名明确写入 owner，且写后完成验证。
- 交付说明 open risks、未验证假设和停止原因。
