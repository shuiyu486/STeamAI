# 定点工具与脚本 backlog

## 已交付工具

本目录仅交付一个显式调用的窄工具：[export_function_evidence.py](export_function_evidence.py)。它在已有授权 IDA/IDAPython 会话中读取一个已分析的 Windows x64 函数及最多两个明确调用点，把有界观察写入指定 case artifact；不自动启动/安装 IDA、不执行目标、不修改数据库、不默认调用反编译器。

使用、限制和真实验收边界见 [IDA 定点函数取证](../recipes/ida-function-evidence.md)；输出形状见 [schema](../schemas/ida-function-evidence-v1.schema.json)。脚本随 case-pinned snapshot 固定，但不会因被复制、import 或 catalog 引用而自动执行。工具与测试只有一份生产实现；fake IDA 不替代真实 IDA 验收，也不授予独立证据成熟度。

以下仍是已在 case 中验证、值得模板化的脚本类型 backlog，**不是已交付文件或可执行命令**。这里只记录接口和参数化要求，不搬运含 case 私有路径的源码；不因新增上述窄工具而恢复通用 runtime/adapter host。

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
