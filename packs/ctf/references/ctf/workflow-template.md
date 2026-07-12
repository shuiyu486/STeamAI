# CTF workflow

## 1. Scope baseline

在开始分析前，main agent 必须把授权边界写入 case-local workspace 或 handoff 摘要：

```text
case: <caseName>
challenges: <case-local aliases only>
auth_scope: <competition/lab/course scope>
allowed_actions: <static review | local run | local repro | remote connect | fuzz | exploit replay | writeup draft>
disallowed_actions: <real-world targets | DoS | credential testing | out-of-scope hosts | flag sharing outside case>
outputs: <handoff | solver notes | writeup candidates | test plan>
```

## 2. Light-to-heavy route

按成本与外部副作用从低到高推进：

```text
scope / challenge alias inventory
  -> static artifact and hint sidecar review
  -> category / primitive / constraint hypothesis
  -> local repro or small solver check
  -> reviewer verdict
  -> main decision / writeup candidate
  -> remote connection / fuzz / exploit replay only after gate
```

升级到远程或高风险动作前，必须记录：

- 轻量路径卡在哪里。
- 已尝试的动作。
- 预计请求量、runtime、输入数量、速率限制和输出大小。
- 输出 sidecar 位置。
- stop condition。
- 用户确认与授权范围。

## 3. Candidate and verification

- challenge-analysis agent 只提交 solver candidate、writeup candidate、repro request 或 stuck note，不直接写最终 writeup / authority。
- reviewer 只读复核 sidecar、challenge alias、artifact ref、solver ref、flag 状态摘要和风险判断。
- main agent 在 gate 通过后写 decision / publication / handoff。
- rejected / superseded 必须保留原因，避免后续重复误报。

## 4. Agent Team review loop

- 先用 `plan-subagents` 生成 bounded review packet，再由主会话按 route 启动只读或工作区限定 agent。
- reviewer verdict 写入 verification event；main merge decision 写入 decision event。
- confirmed / report / authority 写入必须由 main agent 在 evidence、verifier、scope 和 side-effect gate 通过后执行。
- 子 agent 不负责更新 handoff、authority 或 pack reference。

## 5. Documentation and handoff

- Markdown 只保存摘要、证据定位和下一步。
- flag、payload、solver 私有脚本、challenge 原始文件、pcap、dump、trace、远程响应和日志保存为 case-local sidecar。
- 每轮结束更新 handoff 或 lane resume，说明 open risks、pending gates 和未验证假设。

## 6. Validation checklist

- 文档没有 flag、远程靶场地址、账号凭据、payload、challenge 原始文件、pcap、dump、trace 或具体解法泄漏。
- candidate 能追溯 evidence sidecar 与 verifier verdict。
- 远程连接、bruteforce、fuzz、exploit replay、高流量动作、debug、dump 有授权、预算、速率限制、止损和确认记录。
- confirmed / writeup 写入有 reviewer、diff 和回滚线索。
