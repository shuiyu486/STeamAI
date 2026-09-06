# Value-flow and semantics mining

## 目标

从 focused trace 和 dispatch events 中提取 handler 输入输出关系，生成 opcode/source/bitwise/add/bridge 候选，并通过 conservative review 合入。

## Pipeline

1. 提取 bytecode operands、old/new VSP、register writes、memory writes。
2. 聚合 occurrence。
3. 拟合 exact formula / source layout / stack payload。
4. 做 shape clustering 与 recommendation。
5. 对高风险候选做 focused instruction review。
6. 先写 E/F 并完成必要的独立 Reviewer 复核；只有证据支持时，才按 case 现有单写边界更新已有语义/role 表，不自动合入。
7. 若需重建已有派生产物，先核对 case 实际工具、输入及写入范围；本 recipe 不提供 mining/rebuild executor，不要求调用未交付脚本。

## 阻塞标志

- LOW_OCCURRENCE
- ADD_XOR_OR_ALIAS
- MANY_EXACT_FORMULAS
- POINTER_ALIAS
- SOURCE_POINTER_ALIAS
- overwritten staged write
- no stable VSP payload

## 合入边界

- opcode semantics 与 bridge/control/VMExit role 分表。
- 自动挖掘只生成候选；低样本/alias-heavy 按 [Singleton handler 复核流程](../../references/binary-re/singleton-handler-review.md) 检查 final payload、覆盖写与 alias，重要结论由独立 Reviewer 复核。同源不同导出不是独立证据，观察不全时不得仅因无 payload 就归类 bridge；证据不足保持 unknown。
- coverage 不应因新增语义意外下降。
