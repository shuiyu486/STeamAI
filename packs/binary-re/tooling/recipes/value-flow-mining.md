# Value-flow and semantics mining

## 目标

从 focused trace 和 dispatch events 中提取 handler 输入输出关系，生成 opcode/source/bitwise/add/bridge 候选，并通过 conservative review 合入。

## Pipeline

1. 提取 bytecode operands、old/new VSP、register writes、memory writes。
2. 聚合 occurrence。
3. 拟合 exact formula / source layout / stack payload。
4. 做 shape clustering 与 recommendation。
5. 对高风险候选做 focused instruction review。
6. 合入 confirmed opcode CSV 或 handler role CSV。
7. 重建 routine IR 和 superinstruction mining 输出。

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
- 自动挖掘只生成候选；低样本/alias-heavy 必须人工复核。
- coverage 不应因新增语义意外下降。
