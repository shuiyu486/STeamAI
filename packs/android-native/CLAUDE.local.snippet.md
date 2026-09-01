<!-- BEGIN android-native:router -->
# Android native / JNI security research pack router

本 pack 只用于明确授权的安全研究 case，提供 APK native library triage、JNI 边界、hook candidate 与 emulator 侧证据 的声明式方法与审查边界。

- 路由入口：`references/android-native/README.md`
- 团队协作：`references/android-native/agent-team.md`
- 工作流：`references/android-native/workflow-template.md`
- 工具路由：`references/android-native/toolchain-router.md`

规则：

- 只从 case-local pack snapshot 读取本 pack；当前 case 不跟随 source pack 漂移。
- Commander 按需创建 durable member；每个问题一名 owner、最多一名 verifier，重要 finding 由独立 Reviewer 复核。
- app_ref、library_ref、jni_symbol_ref、component_ref 只能是 case-local 脱敏引用，不写入真实对象、原始 artifact、凭据、客户信息或绝对路径。
- heavy action 必须同时满足明确 case 授权、针对具体动作的用户确认与 Claude Code 工具权限；范围、预算或副作用漂移时立即停止。
- 研究结论写入 evidence/finding/review；只有 accepted finding/review 才能生成脱敏 learning candidate。
<!-- END android-native:router -->
