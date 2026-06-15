# Script template backlog

本目录记录已在 case 中验证、值得模板化的脚本类型。第一版先记录接口和参数化要求，不直接搬运含 case 私有路径的源码。

## 值得模板化的脚本

| 模板 | 来源脚本形态 | 需要参数化的内容 | 输出 |
|---|---|---|---|
| `trace_vmenter_seed.template.py` | `trace_vmenter_seed.py` | target/rebuilt PE、VMEnter RVA、context JSON、dispatch limit、focus handler | trace CSV / focused trace / summary |
| `launch_suspended_with_probe.template.py` | `launch_suspended_with_probe.py` | target path、probe DLL/source、output context、timeout | VMEnter context JSON |
| `augment_context_memory.template.py` | `augment_context_memory.py` | context JSON、memory map、required ranges | augmented context JSON |
| `extract_handler_value_flow.template.py` | `extract_handler_value_flow.py` | trace CSV、handler RVA、output path | value-flow CSV/JSON |
| `auto_mine_handler_semantics.template.py` | `auto_mine_handler_semantics.py` | routine IR events、confirmed CSV、recommendation output | candidate semantics |
| `build_routine_ir.template.py` | `build_routine_ir.py` | trace events、opcode CSV、role CSV | routine IR events/summary |
| `mine_routine_superinstructions.template.py` | `mine_routine_superinstructions.py` | routine IR events、min support、n-gram bounds | superinstruction report |

## 模板化规则

- 不写死 case 路径、样本名、RVA、seed、ctx id。
- 所有大输出写机器文件，只在终端返回摘要。
- 脚本要支持 `--help`、明确输入输出、可单独 `py_compile`。
- 与 confirmed CSV 相关的脚本必须保守，不自动合入低样本候选。
