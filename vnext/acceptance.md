# vNext 原生团队验收

本文件只定义 `steamai-vnext-thin-core-v1` 的可重复验收，不是 case 状态、session registry 或消息 ledger。验收使用无真实样本的临时目录，结果不写入模板仓库。

## 两层门禁

### 1. 自动 capability/context/file-access probe

运行：

```text
STEAMAI_VNEXT_LIVE_ACCEPTANCE=1 go test -count=1 -run TestLiveNativeContextAndFileAccess ./internal/steamai/vnextcontract
```

Windows PowerShell 使用：

```text
$env:STEAMAI_VNEXT_LIVE_ACCEPTANCE = '1'; go test -count=1 -run TestLiveNativeContextAndFileAccess ./internal/steamai/vnextcontract
```

该 probe 在临时 case 中执行两个相互独立的 non-persistent Claude Code 调用：

- context probe 禁用全部工具，working directory 是成员专属目录，只能从自动加载的成员和父级 case `CLAUDE.md` 返回成员身份、正式任务和 case 边界；
- file-access probe 只开放 `Read`，通过 `--add-dir` 读取未出现在 prompt 或 `CLAUDE.md` 中的 case-root canary；
- 两个调用分别通过，避免主动读取 `CLAUDE.md` 掩盖自动上下文加载失效；
- 清除父 Claude Code 的嵌套标记，但不跳过权限、不启用网络或其它工具。

缺少 `claude`、认证或必需 CLI flag 时，probe 必须失败并报告缺失能力；默认 canonical suite 不发起模型调用。

### 2. 真实独立 session live acceptance

自动 probe 不能证明用户能观察、纠偏或让成员直接协作。完整产品验收还必须在同一临时 case 中启动至少两个真实独立 Claude Code session，并保存 case 外的短验收摘要：

1. `claude agents --json --all` 能按精确 member cwd 识别会话；会话 ID 仅用于联系或恢复。
2. Commander 向 owner 发送有界任务；owner 从自身 `CLAUDE.md` 复述当前任务并形成 synthetic evidence/finding。
3. owner 直接请求一名 verifier 做有界复核；不得广播，也不得引入第二名 verifier。
4. 用户通过 `claude attach <id>`，或验收 harness 通过 `claude --resume <session-id>` 向同一 owner session 直接输入一条明确标为 synthetic 的 acceptance correction；owner 更新自己的正式任务并通知受影响成员。跨会话 `SendMessage` 不能冒充 user/direct-session correction，也不授予任务变更权限。
5. 再向同一 session 输入一条携带旧 expected current task 的延迟变更；通过 compare-before-update，owner 必须返回 `HOLD_STALE_TASK`，不能覆盖 correction。
6. Reviewer 只读 artifact/evidence/finding，只写 review；`needs-evidence` 必须返回原 owner，补证后才能 `accepted`。
7. roster 超过 3 名执行成员或 1 名 Reviewer 的创建请求必须拒绝或暂停等待真实用户明确改变团队模型。
8. `claude logs <id>` 可观察后台验收会话；`claude attach <id>` 是用户直接观察/纠偏路径。自动化环境可使用后台 session，但产品默认仍是可见前台会话。后台验收应使用与 Commander 兼容的 permission mode、预先限定工具，并通过临时 settings 文件显式设置 `crossSessionInbound: "accept"`；否则跨会话消息会停在不可见 permission prompt。该设置只接受本次会话消息，不跳过工具权限，也不授权临时 case 外写入。

## 通过标准

- 自动 probe 与 live acceptance 都通过；任一层不能替代另一层。
- 每个问题只有一名 owner 和最多一名 verifier；一般发现批量发送，探索过程不广播。
- 未收到可验证结果时不重复投递同一任务；permission-held 消息可能在稍后批准后送达，重复发送会形成协作风暴。
- shared research 文件使用 case-relative path，Reviewer 没有 evidence/finding 写权限。
- 不解析 transcript JSONL，不把消息、PID、session ID 或 endpoint 持久化为产品状态。
- 不调用旧 Go runtime，不因能力缺失回退 legacy `/rekit`、PATH kit 或 PowerShell runtime。

## 清理

验收结束后停止本次创建的后台 session，并删除临时 case；验收摘要只记录通过/失败及必要的能力边界，不保留 session ID、绝对 case 路径、artifact 内容或 case-local hash。停止 session 只清理运行资源，不证明 durable member 完成；研究结论只以存续 case 文件为准。
