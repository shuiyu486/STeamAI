# Strix 专业能力接入（web-security）

## 目的

有界授权 Web 应用的静态安全分析委托给固定版本的 Strix（开源 AI hacker agent）；Strix 产出**待核线索**，不自动成为 accepted finding，不替代成员的实证责任。

## 固定身份

- `strix-agent==1.7.0`，安装身份钉 `git+https://github.com/usestrix/strix @ v1.7.0`，commit `55bc07991aacb1c26a81b43e7ca53148730d2019`（tag 不代替 commit，安装后以包元数据核对）。
- 沙箱镜像 `ghcr.io/usestrix/strix-sandbox:1.3.0`；Docker engine ≥29.x。执行前核对镜像 digest 与本地一致。
- Python 侧依赖版本随安装固定；升级须重走资格核查。

## 前置资格（六项，逐项 live 证明后才有自主委托资格）

范围、可确认停止（含内部 agent）、含子代理的费用核算、纠偏生效方式、产物完整取回、实际模型核对。既有实测结论（随版本重核）：

- **范围**：指令约束，非机械隔离——scope 写进 instruction file，不能防止模型越界；所以 instruction 必须由成员显式写文件并核对。
- **停止**：容器停止可确认，但内部 agent 子任务的边界以指令为准；`--max-budget` 是响应后检查，不是硬上限。
- **费用**：模型单价经 litellm 成本表注册后每事件 `cost_usd` 非空；值未知时保持 unknown，不显示免费。
- **纠偏**：Windows 下无热消息接口；停止后在新目录重开（不带 `--resume`，`--resume` 可能先恢复旧 child）。
- **产物**：结果报告与 SARIF 取回后完整存 case artifact；SARIF 的合成位置不等于实际源码位置，不得直接当源码定位。
- **模型**：请求日志的 model 字段不证明响应身份；以响应侧证据核对并记录，缺证保持 unknown。

## 官方调用形态（占位符）

- instruction 写成独立文件（`SCOPE_FILE`），命令单层调用（不嵌套 shell 包装）：`strix-agent <TARGET_INPUT> --instruction-file <SCOPE_FILE> --max-turns <N> --max-budget <USD>`。
- Windows 经 SDK/包装调用时必须 `tty=true`（`tty=false` 曾实测 exit 0 空输出）；直接 `docker exec`/CLI 调用不受此限。
- 每次任务用全新空目录；容器结束并确认清理，不把 transport 断开当完成。

## 任务边界

- 只分析任务明确授权的目标与版本；静态资格不授予主动网络扫描、写请求或账号操作资格。
- 空回执、失败 scan、不认识的字段一律保留 unknown，不报"零漏洞"。
- 对不认识输出的复核走独立 Reviewer，不因工具自信改判。

## 产物回交

- 报告/SARIF/日志按 artifact index 登记（path、SHA-256、bytes、来源、授权范围）；发现进入 finding 候选并走 evidence/review 链。
- Strix 的 severity/claim 只是输入；STeamAI 结论以成员复核证据为准。
