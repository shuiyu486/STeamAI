<!-- BEGIN android-native:router -->
# Android native pack router

本 pack 用于授权 Android native / JNI / NDK 分析、APK native library triage、Frida hook candidate review 与 emulator sidecar review 场景的 Agent Team 工作流。它是多安全领域 pack 的最小骨架，当前重点是 route、ledger、handoff、tooling adapter 和 review-first 契约，不是 APK 自动逆向平台、设备操控器、hook 执行器或移动端动态测试平台。

- 路由入口：`references/android-native/README.md`
- Agent Team route：`references/android-native/agent-team.md`
- 工作流：`references/android-native/workflow-template.md`
- 工具路由：`references/android-native/toolchain-router.md`

规则：

- APK/AAB/DEX/SO、hook script、device/emulator id、traffic/capture、trace、dump、patch、keystore、token、账号凭据、包名、真实端点和绝对路径留在 case-local workspace 或 sidecar，不写回 pack。
- 设备连接、emulator run、Frida attach、hook 执行、网络请求、动态 trace、dump、patch、重签名、安装/卸载应用或外部副作用必须有隔离、预算和 stop condition，并先经 `/rekit gate` preflight；只有本次显式用户确认，或 strict durable autonomy profile + 对应 `authorized-gate`，才允许 executor 执行。
- 子 agent 默认只读或仅写自己的 workspace；main agent 负责 ledger writeback、handoff、review merge 和 authority 确认。
<!-- END android-native:router -->
