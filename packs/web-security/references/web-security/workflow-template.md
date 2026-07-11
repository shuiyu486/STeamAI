# Web/API security workflow

## 1. Scope baseline

在开始分析前，main agent 必须把授权边界写入 case-local workspace 或 handoff 摘要：

```text
case: <caseName>
targets: <domains / apps / APIs, redacted as needed>
auth_scope: <source and scope>
allowed_actions: <passive | authenticated browsing | replay | scan | fuzz | exploit replay>
disallowed_actions: <DoS | destructive writes | credential stuffing | out-of-scope hosts>
outputs: <handoff | findings | remediation notes | test plan>
```

## 2. Light-to-heavy route

按成本与外部副作用从低到高推进：

```text
scope / asset inventory
  -> passive documentation and route map
  -> small request/response sidecar review
  -> endpoint / flow hypothesis
  -> focused replay or schema check
  -> reviewer verdict
  -> main decision / report candidate
  -> active scan / fuzz / exploit replay only after gate
```

升级到主动或高风险动作前，必须记录：

- 轻量路径卡在哪里。
- 已尝试的动作。
- 预计请求量、runtime、速率限制和输出大小。
- 输出 sidecar 位置。
- stop condition。
- 用户确认与授权范围。

## 3. Candidate and verification

- feature agent 只提交 candidate finding 或 request，不直接写最终报告。
- reviewer 只读复核 sidecar、schema、请求 ID、响应摘要和影响判断。
- main agent 在 gate 通过后写 decision / publication / handoff。
- rejected / superseded 必须保留原因，避免后续重复误报。

## 4. Agent Team review loop

- 先用 `plan-subagents` 生成 bounded review packet，再由主会话按 route 启动只读或工作区限定 agent。
- reviewer verdict 写入 verification event；main merge decision 写入 decision event。
- confirmed / report / authority 写入必须由 main agent 在 evidence、verifier、scope 和 side-effect gate 通过后执行。
- 子 agent 不负责更新 handoff、authority 或 pack reference。

## 5. Documentation and handoff

- Markdown 只保存摘要、证据定位和下一步。
- 请求/响应、HAR、pcap、scan output、截图和日志保存为 case-local sidecar。
- 每轮结束更新 handoff 或 lane resume，说明 open risks、pending gates 和未验证假设。

## 6. Validation checklist

- 文档没有真实目标、凭据、token、cookie、请求/响应正文、漏洞利用 payload 或 scan output。
- candidate 能追溯 evidence sidecar 与 verifier verdict。
- 外部请求或主动扫描有授权、预算、速率限制、止损和确认记录。
- confirmed / report 写入有 reviewer、diff 和回滚线索。
