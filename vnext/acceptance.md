# vNext 原生团队验收

本文件只定义 `steamai-vnext-thin-core-v1` 的可重复验收，不是 case 状态、session registry 或消息 ledger。验收使用无真实样本的临时目录，结果不写入模板仓库。

## 证据分层

以下证据互不替代：

1. 默认 `_test.go` reference oracle 证明 fresh exact preview/apply、manifest、模板和 Git patch 的确定性合同；它没有执行 `/steamai`，不是产品实现或产品路径证据。
2. 自动 capability/context/file-access probe 实际调用 Claude Code，但只证明上下文加载和文件访问。
3. 显式 opt-in synthetic product-path gate 必须黑盒执行真实 canonical/project-local `/steamai`，不得调用 test oracle 生成 case；harness 输入的 synthetic confirmation 不冒充真实用户授权。
4. 真实独立 session 与人工 visible/attach acceptance 证明用户可观察、纠偏和成员直连；自动 `--resume` 不能替代用户实际进入可见 session 的体验。

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

### 2. Synthetic fresh product-path gate

该 gate 使用独立环境变量，不与基础 capability probe 混用：

该 gate 当前是 specification-only，尚无可执行 test 名称或命令。必须在干净 canonical revision 和外部无样本临时 case 中真实调用 `/steamai`：确认前 case 零写；harness 在同一 Commander session 中输入明确标为 synthetic 的 `CONFIRM STEAMAI FRESH <preview_identity>`；随后黑盒比较 project-local skill、contracts、selected pack、完整 `common/**`、metadata 和 completed marker。再用第二个临时 case 制造 target drift，旧确认必须失效且 `.steamai-vnext/` 不得发布。测试不得把 `SKILL.md` 复制进 prompt 模拟执行，也不得调用 reference oracle Apply；若当前 Claude Code 不能从 print/resume 模式真实 dispatch skill，直接报告 capability 缺失，不 fallback。

在稳定、可复现地证明 Claude Code CLI 能以无交互 harness 调用 `/steamai` 并实现对应 test 前，不发布 `go test -run` 命令，避免零匹配测试造成假绿。真正发布结论仍需记录一次人工可见 preview/确认体验。

### 3. Persistent multi-session 与人工可见验收

自动 probe 不能证明 session persistence、用户可见性、attach 或成员直连。证据再分三层：

- 自动 persistent opt-in：使用 `STEAMAI_VNEXT_PERSISTENT_MULTISESSION_ACCEPTANCE=1` 启动两个不同 member cwd 的 persistent synthetic sessions，验证上下文隔离及逐一 resume；session ID 只留在 test process memory，resume 重传 `--add-dir <CASE_ROOT>`，不解析 transcript JSONL，也不声称用户看见 terminal/attach。仓库只有在能稳定调用当前 Claude Code persistent CLI 时才实现该 gate；不得用 non-persistent probe 冒充。
- 人工 visible foreground：用户实际从两个 member cwd 启动普通可见 session，观察、暂停和直接输入；普通 foreground 不保证出现在 Agent view。
- 人工 background/attach：明确把 synthetic session 放到后台，按精确 cwd 观察，并由用户实际 `attach` 输入 correction；自动 `--resume` 不能替代这一体验。

完整产品验收还必须在同一临时 case 中完成以下旅程，并保存 case 外的短验收摘要：

1. `claude agents --json --all` 能按精确 member cwd 识别会话；会话 ID 仅用于联系或恢复。
2. Commander 向 owner 发送有界任务；owner 从自身 `CLAUDE.md` 复述当前任务并形成 synthetic evidence/finding。
3. owner 直接请求一名 verifier 做有界复核；不得广播，也不得引入第二名 verifier。
4. 用户通过 `claude attach <id>`，或验收 harness 通过 `claude --resume <session-id>` 向同一 owner session 直接输入一条明确标为 synthetic 的 acceptance correction；owner 更新自己的正式任务并通知受影响成员。跨会话 `SendMessage` 不能冒充 user/direct-session correction，也不授予任务变更权限。
5. 再向同一 session 输入一条携带旧 expected current task 的延迟变更；通过 compare-before-update，owner 必须返回 `HOLD_STALE_TASK`，不能覆盖 correction。
6. Reviewer 只读 artifact/evidence/finding，只写 review；round 1 `needs-evidence` 必须返回原 owner，owner 补证后 Reviewer 只追加 round 2 `accepted`，round 1 bytes 不变。round 2 必须绑定当前 finding/evidence SHA-256；任一文件再变化则 accepted stale。
7. roster 是唯一 durable lifecycle source，只允许 `active`、`completed`、`inactive`；超过 3 名 active 执行成员或 1 名 active Reviewer 的创建请求必须拒绝或暂停等待真实用户明确改变团队模型。session observation 不能改变 roster。
8. `claude logs <id>` 可观察后台验收会话；`claude attach <id>` 是用户直接观察/纠偏路径。自动化环境可使用后台 session，但产品默认仍是可见前台会话。后台验收应使用与 Commander 兼容的 permission mode、预先限定工具，并通过临时 settings 文件显式设置 `crossSessionInbound: "accept"`；否则跨会话消息会停在不可见 permission prompt。该设置只接受本次会话消息，不跳过工具权限，也不授权临时 case 外写入。

## 通过标准

- default oracle、自动 probe、synthetic product-path gate、真实独立 session 与人工 visible/attach acceptance 按本次声明范围分别通过；任一层不能替代另一层，未执行项必须如实记录。
- 每个问题只有一名 owner 和最多一名 verifier；一般发现批量发送，探索过程不广播。
- 未收到可验证结果时不重复投递同一任务；permission-held 消息可能在稍后批准后送达，重复发送会形成协作风暴。
- shared research 文件使用 case-relative path，Reviewer 没有 evidence/finding 写权限。
- 不解析 transcript JSONL，不把消息、PID、session ID 或 endpoint 持久化为产品状态。
- 不调用旧 Go runtime，不因能力缺失回退 legacy `/rekit`、PATH kit 或 PowerShell runtime。

## 清理

验收结束后停止本次创建的后台 session，并删除临时 case；验收摘要只记录通过/失败及必要的能力边界，不保留 session ID、绝对 case 路径、artifact 内容或 case-local hash。停止 session 只清理运行资源，不证明 durable member 完成；研究结论只以存续 case 文件为准。
