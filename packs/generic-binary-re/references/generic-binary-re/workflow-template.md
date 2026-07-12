# Generic binary RE workflow

## 1. Scope baseline

在开始分析前，main agent 必须把授权边界写入 case-local workspace 或 handoff 摘要：

```text
case: <caseName>
binaries: <case-local aliases only>
functions: <case-local aliases only>
auth_scope: <owner/lab/customer authorization summary>
allowed_actions: <static review | metadata sidecar | disassembly summary review | debug | trace | dump | patch | rename/comment>
disallowed_actions: <unknown samples | uncontrolled execution | network egress | destructive patching | bulk database writes>
outputs: <handoff | function notes | behavior candidate | test plan>
```

## 2. Light-to-heavy route

按成本与外部副作用从低到高推进：

```text
scope / binary and function alias inventory
  -> metadata, import/export, string, section and symbol sidecar review
  -> function / API / format / behavior hypothesis
  -> saved disassembly/decompiler summary review
  -> reviewer verdict
  -> main decision / behavior candidate note
  -> dynamic trace / patch / rename-comment / export only after gate
```

升级到动态或写入动作前，必须记录：

- 静态路径卡在哪里。
- 已尝试的动作。
- 预计 runtime、函数数量、输出大小、隔离方式和网络策略。
- 输出 sidecar 位置。
- stop condition。
- 用户确认与授权范围。

## 3. Candidate and verification

- binary-analysis agent 只提交 function hypothesis、API behavior candidate、format finding、string cluster note 或 stuck note，不直接写 confirmed / authority。
- reviewer 只读复核 sidecar、binary alias、function alias、artifact alias、API alias、tool summary 和风险判断。
- main agent 在 gate 通过后写 decision / publication / handoff。
- rejected / superseded 必须保留原因，避免后续重复误报。

## 4. Agent Team review loop

- 先用 `plan-subagents` 生成 bounded review packet，再由主会话按 route 启动只读或工作区限定 agent。
- reviewer verdict 写入 verification event；main merge decision 写入 decision event。
- confirmed / report / authority 写入必须由 main agent 在 evidence、verifier、scope 和 side-effect gate 通过后执行。
- 子 agent 不负责更新 handoff、authority 或 pack reference。

## 5. Documentation and handoff

- Markdown 只保存摘要、证据定位和下一步。
- 样本、hash、完整二进制、dump、trace、memory snapshot、patch、完整函数体、符号表、IOC 和工具 raw output 保存为 case-local sidecar。
- 每轮结束更新 handoff 或 lane resume，说明 open risks、pending gates 和未验证假设。

## 6. Validation checklist

- 文档没有样本名、hash、IOC、dump、trace、patch、完整函数体、符号表、客户上下文或绝对路径泄漏。
- candidate 能追溯 evidence sidecar 与 verifier verdict。
- 动态执行、调试、trace、dump、patch、批量反编译、自动重命名/注释、外部联网或写回分析数据库有授权、隔离、预算、止损和确认记录。
- confirmed / authority 写入有 reviewer、diff 和回滚线索。
