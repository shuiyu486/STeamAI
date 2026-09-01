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
- 经验由 Commander 从 current accepted evidence chain 自动提炼；Reviewer 先检查 candidate eligibility，再绑定最终 exact patch identity；用户查看完整 patch 并确认 exact tuple 后才回流 pack。
- canonical `/steamai` 使用 skill、模板、目录、Markdown、Git 与 Claude Code 原生能力；仓库没有项目 runtime、旧 Go control plane、PowerShell façade 或 adapter host。

## 初始化边界

- source clone 本身不是 case。首次接入普通外部项目时，从 canonical source clone 的 `/steamai` 生成零写入 exact preview；确认 envelope 绑定 source blob、target action/pre-state、case facts 与全部 planned writes。
- 产品只支持 fresh/current：`.steamai-vnext/CLAUDE.md` 存在才是 completed current case；不存在时必须是普通 fresh 项目。任何已存在但不 exact-current 的 project-local skill 都是冲突；没有旧项目 importer、升级替换、迁移、dual-read、dual-write 或兼容 runtime。
- partial `.steamai-vnext/`、来源不明的 STeamAI 状态或目标冲突均 fail-closed；不自动 repair、rollback、迁移或删除用户文件。
- Apply 在同卷 sibling staging 中完整生成并验证 state tree，先 no-replace 发布并重验 project-local skill，最后才发布包含 marker 的 `.steamai-vnext/`；日常 quickstart 是 `cd <project> → claude → /steamai`，不依赖 source clone、机器 PATH、全局 plugin 或 executable。
- case 初始化把 selected pack 与完整 `common/**` 从同一 exact revision 物化为 case-pinned snapshot；payload digest 覆盖排序后的 path、Git mode/blob、bytes 与 SHA-256。learning feedback 通过 Reviewer exact binding 和用户确认的 exact tuple 从 case 回流 pack；candidate exact SHA 由 review/confirmation 外部绑定，不写入 candidate 自身。用户确认前 canonical pack 零写。
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

默认测试不启动 Claude Code。remote workflow、cross-compile 或 synthetic fixture 不代表真实 Windows live acceptance 或 formal release。涉及 canonical skill、fresh distribution、pack snapshot 或 learning patch 时，追加对应 focused contract tests。
