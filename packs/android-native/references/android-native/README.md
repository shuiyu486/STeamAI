# Android native / JNI security research route

`android-native` 是已授权 Android native、JNI、NDK 与本地库分析的声明式研究 pack。它只提供 APK native library triage、JNI 边界、hook candidate 与 emulator 侧证据 的按需 reference、tooling recipe 与安全边界，不执行研究动作。

## 读取顺序

| 场景 | 读取 |
|---|---|
| 规划领域流程 | `workflow-template.md` |
| 组建成员或请求复核 | `agent-team.md` |
| 选择工具或判断 heavy action | `toolchain-router.md` |
| 交接当前研究 | `task-handoff.template.md` |

## 常驻边界

- 先读 case `.steamai-vnext/CLAUDE.md`，再从 case-local pack snapshot 按需读取本文件。
- 真实对象、原始 artifact、敏感日志、凭据、客户信息、绝对路径与 case 进度只留在 case 内。
- app_ref、library_ref、jni_symbol_ref、component_ref 使用 case-local 脱敏引用；finding 必须引用 evidence，review 必须引用 finding/evidence。
- heavy action 必须有明确 case 授权、针对该具体动作的用户确认与 Claude Code 工具权限；任何范围或预算漂移都停止。
- 可复用经验只从 accepted finding/review 提炼，经 Reviewer 检查并由用户确认 exact patch 后才回流。
