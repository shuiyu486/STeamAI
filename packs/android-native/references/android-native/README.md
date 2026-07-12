# Android native pack

`android-native` 是面向授权 Android native / JNI / NDK 分析、APK native library triage、Frida hook candidate review 与 emulator sidecar review 的最小 Agent Team pack 骨架。它用于验证 `re-context-kits` 在移动端 native 安全分析子领域的 pack-neutral 边界：工作线、packet、ledger、review gate、handoff、sync/promote 和 tooling adapter 契约应保持可审计、可交接、review-first。

当前不是 APK 自动逆向平台、设备操控器、hook 执行器、移动端动态测试平台或真实设备取证系统；它只提供 case workspace 组织规则与工具经验入口。

## 路由表

| 任务 | 读取文档 | 说明 |
|---|---|---|
| 接手本 pack | `workflow-template.md` | scope、轻到重路线、candidate/review/confirmed 流程。 |
| 规划多 Agent 分片 | `agent-team.md` | subagent routes、packet 输出契约、review-first 合并边界。 |
| 选择工具或 adapter | `toolchain-router.md` | APK/native triage、emulator/hook review、dynamic/gated action 边界。 |
| 查看通用规则 | `<templateRoot>/common/policies/agent-team.md` | 角色、packet、状态流和人工确认边界。 |
| 查看工具 adapter 规则 | `<templateRoot>/common/policies/tool-adapters.md` | capability card、sidecar 输出、heavy-tool gate。 |

## 常驻边界

- 不把 APK/AAB/DEX/SO、hook script、device/emulator id、traffic/capture、trace、dump、patch、keystore、token、账号凭据、包名、真实端点、客户上下文或绝对路径写入 pack。
- 设备连接、emulator run、Frida attach、hook 执行、网络请求、动态 trace、dump、patch、重签名、安装/卸载应用或外部副作用必须在授权范围内，并记录隔离、预算、止损和确认。
- 子 agent 输出 candidate / verification；main agent 负责 ledger、handoff、review merge 和 authority 确认。
- 大输出必须保存为 case-local sidecar，只在聊天和 Markdown 中引用摘要与路径。

## 维护规则

- 新工具先进入 `tooling/catalog.yml` 的 `candidate`、`auxiliary` 或 `cautious` 状态。
- 至少两个授权 case 重复出现的通用 Android native 工作流规则，才考虑提升到 common policy 或 runtime。
- 本 pack 的目的首先是验证多安全领域 pack 架构，不要复制 `vmp-re` 的领域细节或把具体 APK、hash、package name、hook、device id、traffic、trace、dump 和动态结论写入模板。
