# IDA 定点函数取证

## 适用窗口与执行边界

在**已有合法 IDA/IDAPython 会话中打开的、已稳定分析的授权 Windows x64 PE 数据库副本**上，显式调用 [唯一导出器](../scripts/export_function_evidence.py)，取得一个函数及最多两个明确外部调用点的新静态观察。本路径不同于 [已有 sidecar 的只读复核](ida-agent-bridge-readonly.md)：后者不产生新 export。

这是一个定点工具，不是自动分析器、后台 runtime、adapter host 或安装器。它不启动 IDA、不打开数据库、不调用反编译器、不执行目标或调试器、不自动分析，不做 rename/comment/type/patch/save IDB。未识别函数、未知分析状态、非 x64 PE、范围中含未覆盖字节/数据 item 或 patched bytes 都停止，不切换后端猜测结果。脚本 import 不访问 IDA、不产生输出。

执行前仍须明确 case 授权、这次动作的具体确认、工具权限和输出范围；pack 内存在脚本不是执行许可。仅使用指定的 case-pinned 脚本，不临时下载或执行 mutable source 的另一份代码。

## 准备输入

由 owner 与 Commander 确认：

- 已索引输入和数据库的 case-local alias；原始输入身份/来源由相应 E 独立绑定。
- 已加载数据库记录的 input SHA-256、image base、函数**入口** EA，以及 1～8 个不重叠的允许 `[start, end)` 区间。EA 是当前 DB 地址，不把 RVA/文件偏移混用。
- 可选最多两个不同的外部调用点；须位于允许范围且有指向目标入口的已识别 call xref，不接受“看起来像调用”的普通引用。
- 目标 `CASE_ROOT/.steamai-vnext/artifacts/` 已存在且为普通目录。输出使用唯一小写 JSON basename，不覆盖、不沿 symlink/reparse/UNC/设备路径或 ADS 写入。
- 先使用可丢弃 DB 副本；确认没有后台分析/调试、没有会改变本次选定范围的并发用户操作。宿主 autosave 不受脚本控制，不能承诺整个 IDA 进程零写。

脚本核对 IDA/IDAPython 版本、PE/x64/metapc、analysis-ready、debugger-inactive、DB input 记录及 image base；采集前后核对 DB change count、来源和同一有界观察。**磁盘 IDB SHA 不代表内存 DB；数据库记录的 input SHA 不代表本轮重新 hash 原 binary。** 输出将这些未证明项明确标为 false/限制，不复制 input 绝对路径。

## 显式调用

在已授权 IDAPython 会话中加载任务所列的 pinned 脚本，然后调用其 `main(argv)`。以下仅是参数示意，`PINNED_SCRIPT`、`CASE_ROOT`、`INPUT_SHA256` 和各 EA/range 必须替换为本次已确认值，不可原样执行：

```python
import runpy

exporter = runpy.run_path(PINNED_SCRIPT, run_name="steamai_ida_evidence")
status = exporter["main"]([
    "--case-root", CASE_ROOT,
    "--output-name", "function-check.json",
    "--input-alias", "input-a",
    "--database-alias", "database-a",
    "--expected-input-sha256", INPUT_SHA256,
    "--expected-image-base", IMAGE_BASE,
    "--function-ea", FUNCTION_EA,
    "--range", FUNCTION_RANGE_START, FUNCTION_RANGE_END,
])
```

- `runpy.run_path` 直接运行任务指定的 exact `.py` 文件，使用非 `__main__` 名称避免隐式执行 CLI；它不为这个源文件生成 `__pycache__`。不改全局 `sys.dont_write_bytecode`，也不使用会向 pinned 目录写 bytecode 的 `SourceFileLoader.exec_module`。
- 有函数 tail/chunk 时必须逐个包含在明确允许范围；`--range START END` 可重复。外部调用点另加 `--callsite CALLSITE_EA` 及覆盖其完整指令的 range。
- `--help` 可在普通 Python 中查看参数，不加载 IDA。生产导出必须在实际 IDAPython 会话运行，不用 fake module 充当实机。
- 默认/硬上限为 `--max-entries 1000`、`--max-output-bytes 262144`、`--timeout-seconds 30`，只允许降低。条目总数包含函数记录、chunks、指令和 xrefs；使用有界迭代，不先全量枚举再截断。
- 截止在迭代和发布前合作式检查，不能中断一个已阻塞的 IDA API；不是 OS 级硬超时。路径重验也不是对抗恶意并发替换的隔离机制。

成功返回状态 0，并输出 artifact exact path、raw SHA-256、bytes。JSON 形状见 [输出 schema](../schemas/ida-function-evidence-v1.schema.json)。该 schema 描述已有生产者，不授予执行权限，也不替代脚本的范围和计数校验。

## 失败与材料使用

- 超时、预算超限、范围越界、函数/来源缺失、patched bytes、当前观察/DB 状态漂移、未知 API 或输出冲突，均非零失败，不登记为成功 artifact；不自动重试、reanalyze 或扩大范围。
- 写盘中止可能留下本次不完整输出，错误会给出 exact 路径。不得将残留登记为完整观察，不自动删除用户文件或覆盖重跑。
- 输出范围中的指令和 xrefs 来自同一 DB。采集前后的重复读取只检测 currentness，**不是独立证据或 V2**；不可将未取数的 xref 目标当已分析函数。
- artifact 登记后，E 引用输入 alias、range、工具版本和观察；F 分清局部行为、跨步解释及尚未证明。按 [跨步骤行为链](../../references/binary-re/bounded-behavior-chain.md) 在查看另一分支前提出区分性预测，必要的新导出另作具体有界动作。
- 综合 F 直接引用所有支撑连接的 E，由 Reviewer 整体复核。客户端静态构造不等于实际发送，静态分支可达不等于运行发生；实际客户端/API观察走另外明确授权的路径。

fake IDA/unit tests 只证明脚本的机械边界。只有真实 IDA 副本执行、结果可核对且来源/范围明确，才可记录该工具路径实机可用；不外推去混淆、脱壳、未识别函数或其他架构。
