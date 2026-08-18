# Binary RE 通用工作流

## Scope baseline

开始前由 main agent 在 case-local handoff 或 workspace 记录：

```text
binaries/functions: <case-local aliases only>
auth_scope: <authorization summary>
allowed_actions: <static review | sidecar | saved disassembly review | gated actions>
disallowed_actions: <out-of-scope targets | uncontrolled execution | network egress | destructive writes>
outputs: <handoff | function note | behavior candidate | verification plan>
```

## 执行顺序

1. 建立授权范围和脱敏 binary/function alias。
2. 读取 metadata、section、import/export、string、symbol 等 passive sidecar。
3. 形成 bounded function/API/format/behavior hypothesis。
4. 由独立 reviewer 只读复核 evidence packet。
5. main agent 记录 verification、decision、open risk 和下一步。
6. 只有 passive 路线不足时，才提交 exact debug/trace/dump/patch/writeback gate request。

升级前必须记录：静态路线为何不足、已尝试动作、exact target、runtime/输出预算、隔离与网络策略、sidecar 位置、stop conditions 和回滚线索。

## Candidate 与证据

- `binary-analysis` lane 不直接写 confirmed/authority。
- reviewer verdict 只验证 candidate，不替代 main decision。
- raw binary/tool output 保留在 case-local artifact/sidecar；Markdown 只保存摘要和相对定位。
- rejected/superseded candidate 保留理由，避免再次进入同一错误路线。

## 完成检查

- 文档没有样本名、hash、IOC、完整函数体、dump、trace、patch、客户上下文或绝对路径泄漏。
- candidate 能追溯到 exact evidence packet 与 reviewer verdict。
- heavy action 有 strict profile、fresh authorized gate、预算、隔离、止损和 execution observation。
- authority 写入有 main decision、diff、验证和回滚线索。
- handoff 说明 open risks、pending gates 与未验证假设。
