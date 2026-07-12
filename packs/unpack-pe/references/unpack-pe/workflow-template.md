# Unpack PE workflow

## 1. Scope baseline

在开始分析前，main agent 必须把授权边界写入 case-local workspace 或 handoff 摘要：

```text
case: <caseName>
samples: <case-local aliases only>
auth_scope: <owner/lab/customer authorization summary>
allowed_actions: <static review | metadata sidecar | local sandbox summary review | debug | dump | patch | import rebuild>
disallowed_actions: <unknown samples | out-of-scope hosts | uncontrolled execution | network egress | destructive patching>
outputs: <handoff | unpack candidate notes | import recovery candidate | test plan>
```

## 2. Light-to-heavy route

按成本与外部副作用从低到高推进：

```text
scope / sample alias inventory
  -> PE metadata, section, import, string and entropy sidecar review
  -> packer / loader stage / unpack target hypothesis
  -> saved sandbox or debugger summary review
  -> reviewer verdict
  -> main decision / unpack candidate note
  -> dynamic debug / dump / patch / import rebuild only after gate
```

升级到动态或写入动作前，必须记录：

- 静态路径卡在哪里。
- 已尝试的动作。
- 预计 runtime、样本数量、输出大小、隔离方式和网络策略。
- 输出 sidecar 位置。
- stop condition。
- 用户确认与授权范围。

## 3. Candidate and verification

- unpack-analysis agent 只提交 loader hypothesis、unpack candidate、import recovery request 或 stuck note，不直接写 confirmed / authority。
- reviewer 只读复核 sidecar、sample alias、section alias、loader stage、tool summary 和风险判断。
- main agent 在 gate 通过后写 decision / publication / handoff。
- rejected / superseded 必须保留原因，避免后续重复误报。

## 4. Agent Team review loop

- 先用 `plan-subagents` 生成 bounded review packet，再由主会话按 route 启动只读或工作区限定 agent。
- reviewer verdict 写入 verification event；main merge decision 写入 decision event。
- confirmed / report / authority 写入必须由 main agent 在 evidence、verifier、scope 和 side-effect gate 通过后执行。
- 子 agent 不负责更新 handoff、authority 或 pack reference。

## 5. Documentation and handoff

- Markdown 只保存摘要、证据定位和下一步。
- 样本、hash、unpacked binary、dump、trace、memory snapshot、patch、完整 import table、section bytes、IOC 和工具 raw output 保存为 case-local sidecar。
- 每轮结束更新 handoff 或 lane resume，说明 open risks、pending gates 和未验证假设。

## 6. Validation checklist

- 文档没有样本名、hash、IOC、dump、trace、patch、完整 import table、section bytes、客户上下文或绝对路径泄漏。
- candidate 能追溯 evidence sidecar 与 verifier verdict。
- 动态调试、样本执行、dump、patch、外部联网、import rebuild 或写 unpacked 文件有授权、隔离、预算、止损和确认记录。
- confirmed / authority 写入有 reviewer、diff 和回滚线索。
