# Claude Code 原生能力契约

## 必需能力

vNext 薄核心只假定：

- 用户能从指定成员目录启动一个普通、可见的 Claude Code 会话；
- 该会话自动加载启动目录及父目录的 `CLAUDE.md`；
- 用户能直接观察、输入、暂停和恢复该会话；
- 成员能读取被明确加入访问范围的 case 根目录。

成员启动目录是 `.steamai-vnext/members/<member>/`。启动时应把 case 根作为额外可访问目录，例如在成员目录执行：

```text
claude --add-dir <CASE_ROOT>
```

`--add-dir` 只表达文件访问范围，不用于切换成员身份或加载兄弟目录规则。项目共享规则已位于成员启动目录的父层级。

## 可选能力

- `ListAgents` / `SendMessage`：发现和联系可达的独立 Claude Code 会话；不是 durable、exactly-once 消息队列。后台自动验收若需要无人值守接收跨会话消息，必须在临时 session settings 中显式设置 `crossSessionInbound: "accept"`；产品默认不全局修改用户设置。
- Agent view / `claude agents --json --all`：查看当前可观察的交互式或后台 session；可用 `--cwd` 限定目录。普通 foreground session 不保证一定出现在 Agent view；未观察到只能标记 `unknown`，不能推断 offline/completed。
- `claude logs <id>`：查看后台成员最近终端输出；`claude attach <id>`：进入后台 session 并直接纠偏；`claude respawn <id>`、`claude --resume <session-id>`：恢复已有独立 session。resume 时必须重新传入当前 case 的 `--add-dir <CASE_ROOT>`，不能假定 launch-only access flag 自动恢复。只有用户直接输入或同一 session 的明确 resume/attach input 才验证 direct correction；跨会话 `SendMessage` 不能冒充用户纠偏或授予任务变更权限。同一 member cwd 若观察到两个可写 session，agent 任务改写必须 hold，等待用户直接选择。
- `claude --bg`：用户不需持续观察时的可选后台模式，不是默认。
- tactical subagent：正式成员内部的窄任务，不成为 durable member。

## 降级

- 无跨会话消息：Commander 给出定向消息，用户在目标成员终端输入；成员目录与研究文件仍是共享面。
- 无后台 session：始终使用可见前台会话。
- 原会话不可恢复：从同一成员目录启动新会话，读取最新成员 `CLAUDE.md` 与共享研究产物。
- 无 Agent Team experimental 功能：不受影响；vNext 不依赖 teammate task list。
- 原生能力缺失时不得回退旧 Go runtime、PATH executable、外部 kit 或 PowerShell runtime。

## 验收入口

自动 capability/context/file-access probe 和真实独立 session live acceptance 的命令、场景与通过标准见 `vnext/acceptance.md`。自动 probe 不能证明用户直接观察/纠偏或成员直连，live acceptance 也不能替代可重复的 context/file-access 合同。

## 非承诺

- 不保证消息顺序、永久离线投递或 exactly-once；
- 不把 session ID、session name、PID 或 endpoint 当成员身份或授权；
- 不保证长期无人值守运行；
- 不解析 Claude Code transcript 内部 JSONL 作为产品数据库。
