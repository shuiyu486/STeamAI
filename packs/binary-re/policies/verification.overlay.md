# VMP routine IR verification overlay

## Confirmed table 变更后

1. 重建相关 routine IR 与 superinstruction 派生产物。
2. 检查 coverage、unknown 数量和新增 lowering 是否符合预期。
3. 更新 case-local handoff/finding/review 的状态与下一步。
4. 运行与变更直接相关的最短测试和 `git diff --check`。

## Handler lowering 标准

- 低样本、1-count、alias-heavy 或 pointer-heavy handler 不可只凭 formula 自动写 confirmed table。
- no-stable-VSP-payload 只保守归类为 bridge/control，除非 focused instruction review 支持 opcode semantics。
- 成员建议必须由明确 owner 合并，并由 Reviewer 或 verifier 独立检查。

## Heavy action

full trace、动态调试、注入、patch、dump 或 network 触发前：

1. 记录轻路径为何不足和已尝试动作。
2. 向用户展示 exact target、隔离、预算、case-relative output、rollback 与 stop conditions。
3. 取得用户对该具体动作的确认，并满足 Claude Code 工具权限。
4. scope、预算、输出或风险变化时停止并重新确认。
5. 执行后写 case-local evidence，记录方法、结果、限制、预算消耗与 boundary hit。
