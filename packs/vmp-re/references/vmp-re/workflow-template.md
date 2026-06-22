# VMProtect x64 trace-based devirtualization 工作流模板

> 可复用到授权逆向任务。目标是减少公开工具试错、反调试触发和上下文浪费。

## 0. 记录授权与样本基准

为每个新目标建立短基准：

```text
目标路径：<target.exe>
工作目录：<workdir>
保护特征：VMProtect 版本/段名/高熵段/框架特征
基准 dump/rebuilt PE：<rebuilt.exe>
ImageBase：0x...
EntryRVA：0x...
ImportDir/IAT：...
主 VMEnter：RVA/VA 待填
```

规则：授权说明放常驻短文档；样本流水账不要无限追加，长历史归档。

## 1. 工具链 triage 与止损

先读 `toolchain-router.md`，确认当前任务阶段应使用的主线/辅助/止损工具。

快速验证公开工具是否适配：

- import fixer：看是否能命中 protected imports，不要让无效候选无限跑。
- 静态 devirtualizer：检查 VMEnter 模板是否匹配当前版本。
- 若出现大量 unresolved、模板断言失败、候选爆炸，应转 trace-based 路线。

经验：VMProtect 3.7+ on-the-fly/merged handler/rolling key 下，通用 import fixer 与旧静态模型常不适合作为主线。

## 2. 轻到重路线与反调试优先级

默认不要裸调试，也不要一上来捕获 full trace。推荐顺序是先用便宜证据缩小问题，再按证据升级：

```text
static triage
  -> I/O shape / entry / sink
  -> VMEnter source stubs / real context
  -> focused trace
  -> value-flow / producer chain
  -> candidate semantics
  -> verifier / parity / cross-run check
  -> confirmed CSV / routine IR rebuild
  -> heavy trace / dynamic debug only as escalation
```

推荐动作顺序：

1. 正常运行进程 dump / rebuilt PE。
2. 静态定位 VMEnter source stubs、入口、出口、输出 sink。
3. in-process probe 捕获真实 VMEnter context。
4. Unicorn 离线 trace，优先 trace 窄区间。
5. focused instruction trace + value-flow。
6. 只有轻路径无法闭合且说明原因时，才升级 full trace 或动态调试。
7. 只有必要交互时，用 ScyllaHide + 管理员 x64dbg attach。

升级到 heavy trace / dynamic debug 前，至少记录：阻塞在哪个阶段、已尝试的轻量动作、预计 runtime / disk / output size、输出位置和止损条件。

## 3. VMEnter 与真实 context 捕获

关键目标：从“静态猜 seed”升级为“真实 register/stack/memory context”。

模板步骤：

1. 定位 source stub 到 VMEnter 的 call。
2. 记录 call RVA、return address、encrypted VIP seed。
3. 用 probe 在 VMEnter 入口捕获：
   - registers / eflags
   - stack bytes
   - TEB/PEB/GS base
   - 关键 heap/DLL memory ranges
4. 将 context JSON 作为 Unicorn trace 输入。

## 4. 长 trace 与自然 VMExit trace

至少准备两类 trace：

- 长循环代表：用于 opcode semantics 覆盖率。
- 自然 VMExit traces：用于识别 epilogue、bridge、native continuation。

输出建议：

```text
captures/trace_ctx_XXX_*.csv
captures/trace_summary.csv
captures/vm_ir.json
captures/routine_ir.events.csv
captures/routine_ir.summary.csv
```

## 5. Handler semantics lowering 流程

标准 pipeline：

```text
routine_ir.events.csv top unknown
  -> focused instruction trace
  -> occurrence summary
  -> value-flow extraction
  -> formula/source/bridge proposal
  -> manual review for low-sample/alias-heavy
  -> confirmed opcode CSV or handler role CSV
  -> rebuild routine_ir.*
```

普通 opcode 与 bridge role 必须分表：

- `vm_opcode_semantics_confirmed.csv`：有 stable VSP payload / final writes 的 opcode/source/bitwise/add semantics。
- `vm_handler_roles_confirmed.csv`：dispatcher、control-state、readonly key-step、VMExit prep/epilogue。

## 6. 自动化与保守合入原则

可以自动：

- 批量选择 top unknown。
- 生成 focused trace/value-flow。
- formula/source layout 拟合。
- shape clustering / recommendation / motif mining。

不能自动：

- 将 1-count alias-heavy opcode-like candidate 直接写入 confirmed CSV。
- 把 no-stable-VSP-payload handler 当 ordinary opcode。
- 忽略 early write 被 later write 覆盖的问题。

阻塞标志：

```text
LOW_OCCURRENCE
ADD_XOR_OR_ALIAS
MANY_EXACT_FORMULAS
POINTER_ALIAS
SOURCE_POINTER_ALIAS
```

## 7. 文档与接手模板

每个项目至少维护：

- 常驻 router：`CLAUDE.local.md`，小于 8KB。
- 按需入口：`references/vmp-re/README.md`。
- 当前任务接手：`references/vmp-re/task-handoff.md`。
- 工具链路由：`references/vmp-re/toolchain-router.md`。
- 复核专项：`references/vmp-re/singleton-handler-review.md`。
- 上下文管理：`references/vmp-re/progressive-disclosure.md`。

每轮完成后更新 `task-handoff.md`，不要把完整 CSV/JSON 粘到 Markdown。

## 8. 验证 checklist

```powershell
python -m py_compile "<workdir>\auto_mine_handler_semantics.py" "<workdir>\build_routine_ir.py" "<workdir>\mine_routine_superinstructions.py"
python "<workdir>\build_routine_ir.py"
python "<workdir>\mine_routine_superinstructions.py"
```

验证点：

- known events 没有意外下降。
- confirmed CSV schema 未破坏。
- 新增 Markdown 只放摘要和路由。
- 低样本结论有 notes 说明保守性。
