# Android native workflow

## 1. Scope baseline

在开始分析前，main agent 必须把授权边界写入 case-local workspace 或 handoff 摘要：

```text
case: <caseName>
apps: <case-local aliases only>
libraries: <case-local aliases only>
auth_scope: <owner/lab/customer authorization summary>
allowed_actions: <static review | APK metadata sidecar | JNI mapping review | emulator summary review | hook review | frida attach | network capture>
disallowed_actions: <unknown apps | real user devices | uncontrolled execution | credential use | out-of-scope endpoints | destructive patching>
outputs: <handoff | JNI hypothesis notes | hook candidate | test plan>
```

## 2. Light-to-heavy route

按成本与外部副作用从低到高推进：

```text
scope / app and library alias inventory
  -> APK manifest, DEX/native library, symbol and ABI sidecar review
  -> JNI bridge / component / hook point hypothesis
  -> saved emulator, logcat or Frida summary review
  -> reviewer verdict
  -> main decision / hook candidate note
  -> device/emulator attach / hook execution / network capture only after gate
```

升级到动态或外部副作用动作前，必须记录：

- 静态路径卡在哪里。
- 已尝试的动作。
- 预计 runtime、app/library/symbol 数量、请求量、输出大小、隔离方式和网络策略。
- 输出 sidecar 位置。
- stop condition。
- 用户确认与授权范围。

## 3. Candidate and verification

- native-analysis agent 只提交 JNI hypothesis、library summary、hook candidate、emulator review request 或 stuck note，不直接写 confirmed / authority。
- reviewer 只读复核 sidecar、app alias、library alias、JNI symbol alias、component alias、tool summary 和风险判断。
- main agent 在 gate 通过后写 decision / publication / handoff。
- rejected / superseded 必须保留原因，避免后续重复误报。

## 4. Agent Team review loop

- 先用 `plan-subagents` 生成 bounded review packet，再由主会话按 route 启动只读或工作区限定 agent。
- reviewer verdict 写入 verification event；main merge decision 写入 decision event。
- confirmed / report / authority 写入必须由 main agent 在 evidence、verifier、scope 和 side-effect gate 通过后执行。
- 子 agent 不负责更新 handoff、authority 或 pack reference。

## 5. Documentation and handoff

- Markdown 只保存摘要、证据定位和下一步。
- APK/AAB/DEX/SO、hash、包名、endpoint、device/emulator id、hook script、traffic/capture、trace、dump、patch、keystore、token、账号凭据和工具 raw output 保存为 case-local sidecar。
- 每轮结束更新 handoff 或 lane resume，说明 open risks、pending gates 和未验证假设。

## 6. Validation checklist

- 文档没有 APK/package/hash、真实 endpoint、device/emulator id、hook script、traffic/capture、dump、trace、patch、token、账号凭据、客户上下文或绝对路径泄漏。
- candidate 能追溯 evidence sidecar 与 verifier verdict。
- 设备连接、emulator run、Frida attach、hook 执行、网络请求、动态 trace、dump、patch、重签名、安装/卸载应用或外部副作用有授权、隔离、预算、止损和确认记录。
- confirmed / authority 写入有 reviewer、diff 和回滚线索。
