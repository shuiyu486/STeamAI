# CLAUDE.md

## 项目定位

本仓库是 STeamAI canonical 实现（`https://github.com/shuiyu486/STeamAI`）。v1 正式支持 Windows 10/11 x64；Go module 同时承载窄职责原生 `steamai.exe` 与 acceptance tests，使用同一 canonical identity。

STeamAI 是面向安全研究的、人在环的 Claude Code 多会话团队协作与经验学习层：一个真实项目目录对应一个明确授权的 case；用户主要指挥 Commander，也可观察和纠偏成员；成员身份与当前任务由专属目录的 `CLAUDE.md` 承载；会话、消息、恢复和工具调用复用 Claude Code 原生能力；STeamAI 只补安装/Fresh 的机械边界、Windows 可见进程启动、瞬时单 Commander 互斥、artifact/evidence/finding/review 与经确认的经验批次回流。

本仓库不是安全/RE case、自动分析器、漏洞挖掘或渗透引擎。模板和测试不得包含真实样本、trace/dump/capture、payload、凭据、客户信息、绝对 case 路径或 case 进度。

## 文档不变量 / 上下文路由

本项目文档必须按需路由、渐进式披露。`docs/context-routing.md` 是唯一完整路由表；新会话只读取本文件、router、Git 状态和 router 选中的一个场景入口。不要默认串读历史 roadmap、`CHANGELOG.md` 或旧 release 文档。

当前路线是 `steamai-windows-native-product-v1`，入口为 `docs/windows-native-product-roadmap.md`；`docs/batch-plan.md` 只作短投影。`docs/real-usage-hardening-roadmap.md` 保留 `steamai-vnext-thin-core-v1` 已完成的历史验收事实，不改写为当前产品边界。

## 维护哲学与踩坑护栏

- 先把需求还原成用户实际看到的使用画面、成功信号和授权边界，再决定代码。只有会改变产品行为、外部动作或明显代价的取舍才让用户选择；实现细节自行采用最小可靠默认值，并用通俗语言说明。
- 判断、组队、协作、审查和经验提炼交给 Claude Code/LLM；Go 只补原生能力无法可靠完成的确定性 OS、Git 和文件边界。新增 runtime 状态或抽象前，必须先证明原生 session、消息和简单 case-local Markdown 不足。
- 以“概念数、持久状态和用户步骤不增长”为默认约束。不要因未来可能需要而增加 daemon、Hub、GUI/TUI、数据库、supervisor、兼容层或通用控制面；达到当前可验证目标后停止。
- 正式成员是目录拥有身份的、屏幕上独立可见的普通 Claude Code 会话，不是 subagent、PID 或 session record。一个目录就是一个授权 case；不同 case 不自动发现、迁移、扫描或汇总。
- 经验成熟度来自 accepted evidence chain、Reviewer 检查和用户 exact confirmation，不来自 Git staging。模板不得预填 `pass/accepted`，Apply 不自动 stage/commit/push，未确认内容留在来源 case。
- production 路径只保留一个实现；测试调用 production API/CLI，不维护第二套 oracle、renderer 或兼容实现。完成声明按证据分层：unit/synthetic 只证明机械合同，Windows、Claude Code、跨会话和 Release 行为必须由各自真实路径证明。
- Windows/Release 易错点：正式路径不用 PowerShell/`.cmd`/`.bat`；update 从 canonical checkout 外运行并保留旧 checkout；uninstall 如实报告 helper 最小残留；`SHA256SUMS` 使用下载后的资产 basename；prerelease 不能成为 latest；公开发布前检查 ignored 本机文件、Git 历史和匿名下载链路。
- 文档也保持单一职责：根 `CLAUDE.md` 保存长期目标与护栏，`docs/context-routing.md` 只路由，当前 roadmap 保存当前路线证据，`docs/batch-plan.md` 只投影。完成的 roadmap 不改写；出现新的实质目标时建立新路线并更新指针，不靠追加历史让 active docs 膨胀。

## 薄核心边界

- 一个 STeamAI 项目对应一个明确授权的安全研究 case；不同 case 只通过审查、脱敏且用户确认的 pack 经验共享。
- Commander 按需组队；正式成员使用专属目录与目录级 `CLAUDE.md`，由原生 launcher 默认打开屏幕上用户可见的独立 Claude Code 窗口。
- 成员可直接定向沟通、请求有界验证和共享关键发现；当前主任务优先。每个问题默认一名 owner、最多一名 verifier；实质改派和 durable member 创建只由 Commander 决定。
- durable member 通常为 1–3 名执行成员加 0–1 名 Reviewer。优先复用已有成员，其次 tactical subagent，只有持续且独立的工作流才新增成员。
- Claude Code 原生 session 是工作记忆，原生消息是协作通道；不自建 session 身份、消息总线、任务数据库、generation/owner ledger 或 supervisor recovery。
- 只持久化团队需要复核的 artifact、evidence、finding、review、learning candidate 与 exact batch patch，不保存全部思考或聊天历史。
- 经验由 Commander 从 current accepted evidence chain 提炼；Reviewer 先逐 candidate 检查 eligibility，再绑定最终 batch exact patch；用户查看完整 patch 并确认 exact tuple 后才回流 pack。
- `steamai.exe` 只负责 setup/update/uninstall、卸载后的窄自清理、Fresh 文件机械操作、exact learning batch apply、Windows 可见进程启动和必要瞬时互斥。不得扩展为 task/session/message/roster/finding/review/learning 判断控制面。
- 正式产品路径不得新增 PowerShell、`.cmd` 或 `.bat` façade；不恢复旧 Go control plane、adapter host 或兼容 runtime。

## 初始化与更新边界

- 日常入口是目标目录中的 `steamai`。Fresh 时 native shell 以 case cwd 启动 `claude "/steamai" --add-dir <CANONICAL_SOURCE>`；current case 只启动 `claude "/steamai"`。
- source checkout 本身不是 case。`.steamai-vnext/` 完全不存在才是 fresh；完整深验通过才是 current；partial、来源不明或冲突均 fail-closed，不 repair/rollback/迁移/删除用户文件。
- Fresh preview 以 canonical working tree 当前实际 bytes 为 authority，stage-0 index 定义 current tracked path/mode，HEAD 只作历史 anchor。确认绑定 source records、target action/pre-state、case facts 与全部 writes。
- Apply 在同卷 sibling staging 中完整生成并验证 state tree，先 no-replace 发布并重验 project-local skill，最后发布包含 marker 的 `.steamai-vnext/`。
- 初始化把 selected pack 与完整 `common/**` 物化为 case-pinned snapshot；payload digest 覆盖排序后的 path、Git mode/blob、bytes 与 SHA-256。current case 不随 canonical checkout 更新。
- setup/update/uninstall 是窄原生产品职责。普通运行不联网；update 必须从 canonical checkout 外运行，并要求 clean canonical checkout，下载与校验全部成功才切换，冲突时停止，不 merge/rebase/stash/reset/clean，source 替换后的旧 checkout 始终保留并输出路径；uninstall 保留 checkout 和所有 case，原生自清理 helper 的已知最小残留会输出精确路径。
- 不提供旧项目 importer、升级替换、迁移、dual-read、dual-write、active case跨电脑迁移或兼容路径。

## 维护入口

- 原生入口与机械实现：`cmd/steamai/**`、`internal/steamai/**`
- canonical 与 project-local skill 唯一 source：`.claude/skills/steamai/SKILL.md`
- 薄核心合同与模板：`vnext/**`
- contract tests：`internal/steamai/vnextcontract/**`
- pack/common：`packs/<pack>/**`、`common/**`
- 当前路线：`docs/windows-native-product-roadmap.md`
- 历史薄核心事实：`docs/real-usage-hardening-roadmap.md`

维护源码、模板和文档时先用 CodeGraph 查看结构、调用链与影响面；其返回源码视为已读。不得用 CodeGraph 处理样本、二进制或 case artifact。

## 验证

```text
go test -count=1 -p=2 -timeout=30m ./...
go vet ./...
git diff --check
```

显式 live gate 按 `vnext/acceptance.md` 分层执行。默认测试不启动 Claude Code。fake process、remote workflow、cross-compile 或 synthetic fixture 不代表真实 Windows setup/PATH、可见窗口、用户纠偏、跨会话协作、learning apply 或 formal release。涉及 canonical skill、Fresh、pack snapshot、learning batch、update/uninstall 或 Release 时追加对应 focused tests 与真实产品路径验收。
