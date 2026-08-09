# 真实使用加固未来批次 backlog

## 读取指南

本文件只保存 `real-usage-hardening-v1` 尚未成为当前项的未来批次卡。日常接手不要默认读取；只有当前批次完成、需要核对下一张卡，或代码事实要求调整后续顺序时，才从 `docs/real-usage-hardening-roadmap.md` 路由到对应 RH 编号。

## 实施摘要

当前路线已将 Windows soak 的 RH-09 提升为 active card。本文件只保留已明确延期的条件卡；current source、状态和任何重新解锁决定始终由 `docs/real-usage-hardening-roadmap.md` 持有，本文件不是独立选题源。

## 执行清单

### RH-10：跨平台 product path（deferred）

**用户断点**：Windows 是当前支持门槛，macOS/Linux 的真实 Claude product path 尚无等价证据。

**状态**：`deferred`。用户已决定当前只完成 Windows 路线，Linux/macOS 以后再说；不得在 RH-09 完成后自动实施。

**重新解锁条件**：用户以后明确要求 Linux/macOS product path，且进入正式发布、跨平台专项或周期复审窗口；否则保持 `deferred`，不能抢占 Windows 日常可用性。

**范围内**：在可用 runner/host 上执行不含敏感内容的普通 public route gate；修复路径、executable resolution、signals、permissions 和 cleanup 的共享差异。

**范围外**：不把 compile-only 或 workflow inventory 当真实 product-path green，不因无 runner 伪造远程结论。

**真实证据**：各平台实际 job/session conclusion 与 receipt；无法获得 runner 时明确 `blocked`。

**停止/升级条件**：远程成本、登录态或 runner 需外部授权时升级；不能以该阻塞延长本地微批次。

## 验证标准

- 只有 active roadmap 明确解锁的下一批可从本文件提升为当前卡。
- 提升时复制对应卡到 active roadmap，并从本文件移除已激活卡；完成记录最终进入 batch history。
- 所有声称 LLM 成功的证据仍必须来自真实 Claude Code structured-output envelope；deterministic fixture 不得冒充 live provenance。

## 风险与注意事项

- 不把本文件加入默认 `readFirst[]`，也不在新会话接手时预读全文。
- 本文件不拥有 current/state；与 active roadmap 冲突时以 active roadmap 为准并停止自动领取。
- 不因拆文档改变批次顺序、验收或授权边界；拆分只降低上下文负担。
