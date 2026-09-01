# STeamAI

STeamAI 是面向安全研究的、人在环的 Claude Code 多会话团队协作与经验学习层。一个真实项目目录对应一个明确授权的安全研究 case：用户主要指挥 Commander，也可以直接进入任意正式成员会话观察、暂停和纠偏。

Canonical GitHub repository：[`shuiyu486/STeamAI`](https://github.com/shuiyu486/STeamAI)。源码使用：

```text
git clone https://github.com/shuiyu486/STeamAI.git
```

## Quickstart

前提只有两个：本机已能正常使用 Claude Code；目标目录是明确授权的安全研究 case。STeamAI 不安装 Claude Code、不管理登录、不要求全局 plugin，也不发布项目内 runtime。

第一次使用一个尚无项目级 `/steamai` 的普通目录时，从 canonical source clone 启动 Claude Code，调用 `/steamai` 并提供外部 case 目录。Commander 先生成零写入 preview；用户确认后才把 canonical skill exact bytes、`.steamai-vnext/contracts/` 声明合同和 selected pack + `common/**` 的同 revision snapshot 发布到目标。source clone 本身不是 case，不能在仓库内创建 case state。

preview 同时包含 project-local skill、`.steamai-vnext/contracts/` 声明式模板/合同包和 exact-revision pack snapshot；每项都绑定 canonical source bytes。确认并分发后，Reviewer、成员模板和 learning 合同都从 case-local `contracts/` 读取。

发布完成后的日常入口只有：

```text
cd <project> → claude → /steamai
```

日常不再依赖 source clone、旧 runtime、机器 PATH 或全局 plugin。首次使用时，Commander 会：

1. 明确研究目标、授权范围、禁止事项和停止条件；
2. 从同一 exact source revision 建立 selected pack 与 `common/**` policy closure 的 case-local 只读 snapshot；
3. 按需创建 1–3 名执行成员和最多 1 名 Reviewer；
4. 展示每名成员的专属目录与前台启动方式；
5. 建立 artifact、evidence、finding、review 和 learning candidate 目录。

正式成员默认运行在用户可见的独立 Claude Code 会话中。用户可以在这些会话里直接观察执行过程和纠偏；原生 session 保存工作上下文，原生 `resume` / `attach` / `respawn` 用于恢复，原生跨会话消息用于定向协作。

## 产品模型

### Commander

Commander 负责理解目标、按需组队、维护授权边界、解决协作冲突、组织独立审查、集成交付和发起经验回流。Commander 不转发所有消息，也不替成员决定每一步工具调用。

### 正式成员

每名正式成员拥有独立目录：

```text
.steamai-vnext/members/<member>/CLAUDE.md
```

该文件承载成员身份、当前正式任务、输入、允许范围、产出、停止/升级条件和退出条件。身份属于目录，不属于 session ID；会话中断后从同一目录恢复或启动新 session 即可继续。

只有 Commander 可以创建 durable member。active team 硬上限为 3 名执行成员和 1 名 Reviewer；新增前优先复用、完成、停用或合并现有成员，其次才考虑短命 tactical subagent。

### 团队协作

- 每名成员的当前主任务优先；普通发现不广播。
- 成员可以直接定向提问、共享关键发现或请求有界验证。
- 每个问题默认一名 owner、最多一名 verifier。
- 会明显打断主任务、改变范围或持续投入的协助由 Commander 决定。
- 用户在成员 session 中的直接输入，或经 `attach` / 同一 session `resume` 的输入，才算用户直接纠偏。
- 跨会话消息不能冒充用户纠偏、改变任务授权或扩大 case 范围。
- 原生消息不是 exactly-once queue；不以新状态机补洞。

### Reviewer

Reviewer 在重要 finding、成员冲突、最终交付或 learning 回流前介入：

- 只读 artifact、evidence 和 finding；
- 只写 review；
- 不执行 heavy action；
- `needs-evidence` 直接返回原 owner 补证；
- 不修改原 evidence/finding，也不进入旧 writeback/reconcile 流程。

## 研究产物

```text
.steamai-vnext/
  CLAUDE.md
  pack-snapshot/
  contracts/
    learning-feedback.md
    legacy-import.md
    templates/**
  members/<member>/CLAUDE.md
  artifacts/index.md
  evidence/E-*.md
  findings/F-*.md
  reviews/R-*.md
  learnings/candidates/L-*.md
```

- `artifacts/index.md` 只索引 case-local 对象，记录相对路径、SHA-256、bytes、来源和授权范围。
- evidence 是可复查观察；finding 必须引用 evidence；review 直接引用 finding/evidence。
- 临时思考、聊天记录和长工具输出留在原生 session，不持久化为团队状态。
- 本仓库的模板和测试不包含真实样本、trace/dump/capture、payload、凭据、客户信息、绝对 case 路径或 case 进度。

## 经验沉淀与回流

Commander 在里程碑或 case 收尾时，只从 accepted finding/review 提炼脱敏 learning candidate。Reviewer 检查证据支持、跨 case 通用性、反例、重复、冲突、目标路径和脱敏。

通过审查后：

1. 在隔离临时 Git clone 中生成完整、标准、可 `git apply --check` 的 exact patch；
2. 向用户展示 candidate、Reviewer decision、base revision/blob、目标 pack 路径和完整 patch；
3. 用户确认前 canonical source pack 零写；
4. 只有用户确认后才写入 pack，且确认只授权这一份 patch；
5. 应用前重验 patch SHA、单目标 scope、filter-aware base currentness 和 `git apply --check`；漂移则停止并重新生成 diff；
6. 不自动 commit 或 push。

运行中的 case 固定读取建立时的 exact-revision snapshot；pack 回流不会隐式改变当前 case，只有后续 case 明确选择新 revision 才消费新经验。

## Legacy 项目导入

旧 `.steamai/` 或 `.rekit/` 项目只通过一次性只读 importer 接入：

- 先生成零写入 import preview；
- 只采用可证明的 case 名称、目标、授权范围、停止条件、selected pack identity 和 case-local 资料相对引用；
- 不导入 session/PID/endpoint、generation、lane owner、receipt、gate、authority/confirmed、消息状态或 runtime health；
- 缺失、冲突、未知 pack、绝对外部路径或授权歧义一律 `needs-user-input`，不猜测；
- 用户确认 exact preview 后，按预览创建 `.steamai-vnext/`、case-local contracts/snapshot，并 create/replace project-local canonical skill；
- 不修改、删除、重命名或续写旧 roots，不 dual-write，不回退旧 runtime。

`.steamai/` 与 `.rekit/` 同时存在时 fail-closed，不拼接两份旧状态。

## 原生能力与验收

当前薄核心复用 Claude Code 原生能力：

```text
claude --add-dir <CASE_ROOT>
claude agents --json --all
claude logs <id>
claude attach <id>
claude respawn <id>
claude --resume <session-id>
```

`--add-dir` 只增加文件访问范围，不会改变成员身份或配置根。自动 capability/context/file-access probe 与真实独立 session live acceptance 是不同门槛，前者不能替代用户可观察、可纠偏和成员直接协作的实测。

维护者入口：

- canonical skill：`.claude/skills/steamai/SKILL.md`
- project-local delivery template：`vnext/project-skill/SKILL.md`
- case/member/research templates：`vnext/templates/**`
- 原生能力合同：`vnext/capabilities.md`
- live acceptance：`vnext/acceptance.md`
- learning feedback：`vnext/learning-feedback.md`
- 已完成路线与验收事实：`docs/real-usage-hardening-roadmap.md`
- 文档路由：`docs/context-routing.md`
- 完成验收与本机验证边界：`docs/real-usage-hardening-roadmap.md` 完成卡与 `vnext/acceptance.md`
- 历史旧架构事实：Git history 或 `CHANGELOG.md` 按需查询

旧 Go control plane、mega CLI、project-local runtime、PowerShell façade、adapter host 与 legacy `/rekit` skill 已在 VNT-05 删除。旧 `.steamai/` / `.rekit/` 只作为一次性 importer 的只读输入；当前产品没有旧 runtime fallback、双写或第二套执行入口。

STeamAI 不是全自动脱壳器、逆向引擎、漏洞挖掘器或通用渗透平台；它提供人在环、多会话协作、证据审查和确认回流层。危险动作仍受 Claude Code 工具权限与 case 授权约束；`CLAUDE.md` 只提供角色上下文，不授予权限。
