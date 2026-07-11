# Template workflow

> 这是领域 workflow 模板。复制到新 pack 后，替换为该领域的轻到重路线、证据要求和验证标准。

## 1. Scope baseline

记录目标、授权、输入、输出、工具边界和非目标。

```text
case: <caseName>
target: <target placeholder>
entry: <known entry or unknown>
outputs: <expected outputs>
non-goals: <what not to do>
```

## 2. Light-to-heavy route

按成本从低到高推进：

```text
static triage
  -> narrow observation
  -> focused evidence collection
  -> candidate hypothesis
  -> verifier / cross-check
  -> confirmed output
  -> heavy tool only as escalation
```

升级到 heavy tool 前，必须记录：

- 轻量路径卡在哪里。
- 已尝试的动作。
- 预计 runtime / disk / output size。
- 输出 sidecar 位置。
- stop condition。
- 是否需要用户确认。

## 3. Candidate and verification

- worker / feature agent 只提交 candidate。
- reviewer 只读复核。
- main agent 在 gate 通过后写 confirmed / authority。
- rejected / superseded 保留原因。

## 4. Agent Team review loop

- 先用 `plan-subagents` 生成 bounded review packet，再由主会话按 route 启动只读或工作区限定 agent。
- reviewer verdict 写入 verification event；main merge decision 写入 decision event。
- 任何 confirmed / authority 写入都必须由 main agent 在 gate 通过后执行。
- 子 agent 不负责更新 handoff、authority 或 pack reference。

## 5. Documentation and handoff

- Markdown 只保存摘要、证据定位和下一步。
- 大输出保存为 sidecar。
- 每轮结束更新 handoff 或 lane resume。

## 6. Validation checklist

- 文档没有真实样本、绝对路径或大输出。
- candidate 能追溯 evidence。
- confirmed 写入有 verifier 和 diff。
- heavy tool 有预算、止损和确认记录。
