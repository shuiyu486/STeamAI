# CLAUDE.md

## 项目定位

本仓库是 STeamAI canonical 实现（`https://github.com/shuiyu486/STeamAI`）。Go module 只承载 test-only acceptance，使用同一 canonical identity。

STeamAI 是面向安全研究的、人在环的 Claude Code 多会话团队协作与经验学习层：一个真实项目目录对应一个明确授权的 case；用户主要指挥 Commander，也可观察和纠偏成员；成员身份与当前任务由专属目录的 `CLAUDE.md` 承载；会话、消息、恢复和工具调用复用 Claude Code 原生能力；STeamAI 只补团队协作、artifact/evidence/finding/review 与经确认的经验回流。

本仓库不是安全/RE case、自动分析器、漏洞挖掘或渗透引擎。模板和测试不得包含真实样本、trace/dump/capture、payload、凭据、客户信息、绝对 case 路径或 case 进度。

## 文档不变量 / 上下文路由

本项目文档必须做成按需路由、渐进式披露的样式。`docs/context-routing.md` 是唯一完整路由表；新会话只读取本文件、router、Git 状态和 router 选中的一个场景入口。不要默认串读历史 roadmap、`CHANGELOG.md` 或旧 release 文档。

`steamai-vnext-thin-core-v1` 已完成，验收事实保存在 `docs/real-usage-hardening-roadmap.md`；`docs/batch-plan.md` 只作完成态短投影。当前没有自动推进的后续批次；只有真实使用反馈、明确新目标或授权边界变化时才建立新路线。

## 薄核心边界

- 一个 STeamAI 项目对应一个明确授权的安全研究 case；不同 case 只通过审查、脱敏且用户确认的 pack 经验共享。
- Commander 按需组队；正式成员使用专属目录与目录级 `CLAUDE.md`，默认启动用户可见的独立 Claude Code 会话。
- 成员可直接定向沟通、请求有界验证和共享关键发现；当前主任务默认优先。每个问题默认一名 owner、最多一名 verifier；实质改派和 durable member 创建只由 Commander 决定。
- durable member 通常为 1–3 名执行成员加 0–1 名 Reviewer。优先复用已有成员，其次 tactical subagent，只有持续且独立的工作流才新增成员。
- Claude Code 原生 session 是工作记忆，原生消息是协作通道；不自建 session 身份、消息总线、任务数据库、generation/owner ledger 或 supervisor recovery。
- 只持久化团队需要复核的 artifact、evidence、finding、review 和 learning candidate，不保存全部思考过程或聊天历史。
- 经验由 Commander 自动提炼，经 Reviewer 检查证据、通用性、重复、冲突与脱敏后，向用户展示完整 exact Git patch；只有用户确认才回流 pack。
- canonical `/steamai` 使用 skill、模板、目录、Markdown、Git 与 Claude Code 原生能力；仓库没有项目 runtime、旧 Go control plane、PowerShell façade 或 adapter host。

## 初始化与兼容边界

- source clone 本身不是 case。首次接入外部项目时，从 canonical source clone 的 `/steamai` 生成零写入 preview；确认后只发布 canonical skill exact bytes、`.steamai-vnext/` 声明目录和 exact-revision pack snapshot。
- 日常 quickstart 是 `cd <project> → claude → /steamai`；日常不依赖 source clone、机器 PATH、全局 plugin 或 executable。
- legacy `.steamai` / `.rekit` 只作为一次性只读 importer source；不 dual-write、不运行旧 runtime、不迁移 session/lane/generation/receipt/gate/authority 状态。
- importer 对双根、partial target、未知或 retired pack、绝对/逃逸路径、symlink/reparse、目标 skill 自定义冲突和 source drift 均 fail-closed。
- case 初始化把 selected pack 与 `common/**` 从同一 exact revision 物化为只读 snapshot；learning feedback 通过用户确认的 exact patch 从 case 回流 pack。用户确认前 canonical pack 零写。
- 保持 source-clone-first，不做 installer、GUI/TUI、新 PowerShell runtime 或 production Go helper；只有真实原型证明存在重复且无法原生解决的机械问题，才允许增加窄职责、无状态 helper。

## 维护入口

- canonical skill：`.claude/skills/steamai/SKILL.md`
- project-local delivery template：`vnext/project-skill/SKILL.md`
- 薄核心合同与模板：`vnext/**`
- contract tests：`internal/steamai/vnextcontract/**`
- pack/common：`packs/<pack>/**`、`common/**`
- 已完成路线与验收事实：`docs/real-usage-hardening-roadmap.md`

维护源码、模板和文档时先用 CodeGraph 查看结构、调用链与影响面；其返回源码视为已读。不得用 CodeGraph 处理样本、二进制或 case artifact。

## 验证

```text
go test -count=1 -p=2 -timeout=30m ./...
go vet ./...
git diff --check
```

显式 live gate：

```text
STEAMAI_VNEXT_LIVE_ACCEPTANCE=1 go test -count=1 -run TestLiveNativeContextAndFileAccess ./internal/steamai/vnextcontract
```

默认测试不启动 Claude Code。remote workflow、cross-compile 或 synthetic fixture 不代表真实 Windows live acceptance、remote green 或 formal release。涉及 canonical skill、importer、pack snapshot 或 learning patch 时，追加对应 focused contract tests。
