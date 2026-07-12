# Vulnerability research workflow

## 1. Scope baseline

在开始分析前，main agent 必须把授权边界写入 case-local workspace 或 handoff 摘要：

```text
case: <caseName>
targets: <case-local aliases only>
auth_scope: <source and scope>
allowed_actions: <static review | crash triage | patch diff | local repro | fuzz | exploit replay | live target validation>
disallowed_actions: <DoS | destructive writes | credential testing | out-of-scope hosts | unapproved disclosure>
outputs: <handoff | root-cause notes | repro candidates | remediation notes | test plan>
```

## 2. Light-to-heavy route

按成本与外部副作用从低到高推进：

```text
scope / target alias inventory
  -> documentation, code, patch or crash sidecar review
  -> bug class / root-cause hypothesis
  -> small local repro or schema check
  -> reviewer verdict
  -> main decision / report candidate
  -> fuzz / exploit replay / live target validation only after gate
```

升级到主动或高风险动作前，必须记录：

- 轻量路径卡在哪里。
- 已尝试的动作。
- 预计请求量、runtime、样本/输入数量、速率限制和输出大小。
- 输出 sidecar 位置。
- stop condition。
- 用户确认与授权范围。

## 3. Candidate and verification

- vuln-analysis agent 只提交 candidate finding、root-cause hypothesis、repro request 或 patch note，不直接写最终报告。
- reviewer 只读复核 sidecar、目标别名、crash/repro id、patch ref、请求摘要和影响判断。
- main agent 在 gate 通过后写 decision / publication / handoff。
- rejected / superseded 必须保留原因，避免后续重复误报。

## 4. Agent Team review loop

- 先用 `plan-subagents` 生成 bounded review packet，再由主会话按 route 启动只读或工作区限定 agent。
- reviewer verdict 写入 verification event；main merge decision 写入 decision event。
- confirmed / report / authority 写入必须由 main agent 在 evidence、verifier、scope 和 side-effect gate 通过后执行。
- 子 agent 不负责更新 handoff、authority 或 pack reference。

## 5. Documentation and handoff

- Markdown 只保存摘要、证据定位和下一步。
- crash/core/minidump、pcap、trace、request/response、payload、fuzz corpus、scan output、截图和日志保存为 case-local sidecar。
- 每轮结束更新 handoff 或 lane resume，说明 open risks、pending gates 和未验证假设。

## 6. Validation checklist

- 文档没有真实目标、凭据、token、request/response body、PoC payload、crash/core/minidump、pcap、trace、scan output 或漏洞利用细节。
- candidate 能追溯 evidence sidecar 与 verifier verdict。
- 主动扫描、fuzz、exploit replay、真实目标验证、debug、dump、数据导出有授权、预算、速率限制、止损和确认记录。
- confirmed / report 写入有 reviewer、diff 和回滚线索。
