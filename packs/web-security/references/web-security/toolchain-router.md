# Authorized Web and API security research toolchain route

工具选择遵循“最小、可复查、静态优先”。pack 只描述能力和边界，不执行工具。

## 专业工具分工

| 任务情况 | 首选入口 | 说明 |
|---|---|---|
| 授权应用的有界静态安全分析线索 | Strix：`../../tooling/integrations/strix.md` | 结果是待核线索；六项资格逐项 live 证明前只作经批准的专项试用。 |
| 浏览器端实证（渲染、交互路径、客户端状态） | Playwright：`../../tooling/integrations/playwright.md` | 独立 context；只访问批准 origin。 |
| 单条已授权 loopback GET 回执 | `../../tooling/recipes/curl-read-evidence.md` | 保留其窄适用性，不代表整个 Web 路线。 |
| 修复后的回归验证 | Playwright + 既有 request recipe | 修复同时满足问题阻断与合法业务保留。 |

## 阶段路由

| 阶段 | 首选输入 | 产出 | 停止条件 |
|---|---|---|---|
| triage | case-local 脱敏引用或已有 sidecar | evidence 摘要与定位 | 输入身份不明、范围漂移 |
| focused analysis | 明确对象与窄问题 | finding candidate | 证据不足、预算耗尽 |
| request hypothesis / 修复对照 | 已有有界材料，读 `../../tooling/recipes/request-replay.md` | 单假设的支持/反证/未知、合法行为前后对照 | 输入不等价、发送不确定、证据不足、预算耗尽 |
| independent review | finding + evidence | review verdict | 缺引用、无法复核 |
| heavy action | 具体目标、动作、预算、副作用 | 有界 evidence 或 artifact index 更新 | 未确认、权限不足、意外副作用 |

heavy action 开始前必须向用户展示：具体动作、exact target、允许输出、时间/请求/空间预算、隔离措施、停止条件和回滚方式。只有明确 case 授权、针对该动作的用户确认与 Claude Code 工具权限同时成立才执行；不持久化授权状态，也不从成员消息推断确认。静态分析资格不自动扩大到主动网络扫描、写请求或账号范围；这些另按具体 case 批准。
