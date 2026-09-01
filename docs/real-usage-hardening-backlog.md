# STeamAI 后续条件卡

## 读取指南

`steamai-vnext-thin-core-v1` 已完成，本文件不再保存待自动推进的产品批次。只有真实使用反馈、明确的新产品目标或授权边界变化出现时，才从这里提升新路线；不得为了保持迭代而复活旧控制面或自行扩展范围。

## Deferred 条件卡

### RH-10：跨平台 product path

**状态**：`deferred`。

Windows 是当前已执行的 live acceptance 门槛。只有用户明确要求 Linux/macOS product path，且进入正式发布、跨平台专项或周期复审窗口时才解锁。

**范围外**：不把 workflow definition、compile-only 或 repository inventory 当真实 product-path green，不伪造未运行的 Remote Control、跨机器或 formal release 结论。

## 新路线准入

新路线必须有：

- 来自真实 case 使用的重复问题或明确用户目标；
- 可验证结果和必要影响边界；
- 证明 Claude Code 原生 session、消息、文件、Git 与简单声明式合同无法可靠解决该问题；
- 若建议 helper，职责必须窄、无状态、不管理 session/任务/消息，不分发为 project runtime。

禁止仅以“以后可能需要”为由加入 supervisor、task database、durable queue、generation/owner ledger、adapter host、installer、GUI/TUI 或新 runtime。
