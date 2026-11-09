# 实用能力与效率结果

本文件收束 `steamai-capability-efficiency-v1`（基线 `60576c08`）及后续交付补缺；原执行计划已删除。**工程实现、合成验证、真实产品验收与方法效果是不同结论，不以批次结束代表全部门槛通过。** 旧工程提交：`9dfb7998`、`dda97bed`、`2afda305`；旧完成声明见 `2db2f7ac`、`c127a707`，以下证据边界取代其中过强表述。

## 当前可交付内容

| 内容 | 仓库交付 | 验证边界 |
|---|---|---|
| Git 内 case 信任与模型选择 | exact 成员目录或 case 根的信任锚校验；维护测试默认 Sonnet 槽位，不继承主模型 | 三种布局机械测试及模型选择测试；未替代可见窗口验收 |
| 工具可用性四态 | `common/policies/tool-adapters.md` 定义四态与 unknown/权限阻塞边界；canonical skill 按需路由 | 静态合同与生产 Fresh 分发测试；不是自动扫描器，也不证明所有宿主工具可用 |
| 自动化执行链 | `workflow.py` 分离 prepare/act、检查停止及暂停、恢复后丢弃旧定位；非幂等动作先 pending，只有明确确认成功才 done | 覆盖末端 pause/resume、输入失败/未确认、停止与重启；真实同步调用的停止延迟仍须实测 |
| 内存定位缓存 | `localize.py` 提供定位回调、模板准入、本地匹配与失配回退；区分目标/动作/状态，拒绝纯色、歧义与非法 bbox | 使用真实 OpenCV 图像匹配的合成测试；首次定位为脚本化回调，不冒充真实视觉模型调用 |
| 可关闭弹窗的无窗口示例 | `synth_target.py`、`synth_device.py` 复用原 Runner；`popup_close_after_ms` 可选正数，None/0 默认持续；关闭后仍需显式 resume | 位移/状态变化在输入前拒绝旧定位，结果未确认保留 pending；目标自报状态与命中回执仅为合成验证 |
| 方法与上下文 | SKILL 去重、证据绑定执行脚本 SHA；逆向/Web 方法补充；纠正“无 xref 即可排除间接目标”的错误推论 | 合同测试与方法回归，不据此宣称研究质量普遍提高 |

新示例与测试均在仓库内，pack manifest 已登记；不再只依赖临时目录里的实现。Fresh 通过生产 `BuildPreview`/`Apply` 逐文件校验 bytes/SHA，已有 case 的 pinned snapshot 不会自动更新。

## 本轮验证

- Python pack tooling：70 项通过，未跳过；另运行 `workflow.py --selftest` 与无窗口 `synth_device.py` 演示，弹窗关闭后由测试控制线程发送 resume，领取一次并到达 RESULT，无系统鼠标/键盘操作。
- Fresh：角色合同、工具四态、游戏示例三个仓库级测试通过；包括 common 完整物化、示例源码原字节分发、旧 case 不随源码修改。
- Sonnet 档：本轮 `ANTHROPIC_DEFAULT_SONNET_MODEL=glm-5.3[1m]`；三个执行子 agent 的 assistant 消息均记录 `glm-5.3`，不是以请求参数或自述代替实际身份。
- 全仓 `go test -count=1 -p=2 -timeout=30m ./...`、`go vet ./...` 与 `git diff --check` 通过；默认回归不调用模型。
- 本轮不修改 Go 产品实现、不重新安装、不新增第三方依赖、不发布 Release；图像测试使用本机已有依赖。

## 新方法对照：blocked / incomplete

新增显式 opt-in 的 `internal/steamai/evaluation/capability_delivery_live_test.go`：6 题、两臂，同 Sonnet 槽位，每臂 240 秒/0.50 美元、整批最多 6 美元；复用生产 evaluator 的进程生命周期、bundle/SHA 与实际模型验证。题目、答案、方法 bytes 和脚本 SHA 在首个调用前冻结，答案不进入 arm，另审计实际工具调用及读取路径。它是维护方编写的 **method-only 合成回归**，不是独立未知留出、生产 Fresh 会话或原 42 次评测的重做。

本次只执行首题两臂，实际 assistant 模型均为 `glm-5.3`；两臂完成并成功读取对应方法，答案字段各 4/4，但**整批未通过**：模型尝试读取未列出的文件和目录；此外测试器错误地把 CLI `--json-schema` 自带的 `StructuredOutput` 当成越界工具。首对检查阻断后，余下 10 个 arm 均为 `not-launched`，无重试，总费用 0.275408 美元。

已为后续运行修正测试器对 `StructuredOutput` 的识别，并给出明确的 `methods/index.md` 入口；只做离线回归，**修正后的测试器尚未重新 live 验证**。本次失败与未启动记录保持不变，不能以修正脚本后重算的分数追认通过，也不能由首题同分推出方法收益或整体 tie。

原始证据位于本机 `%TEMP%\steamai-pd\capability-delivery-3859442810\`，不入仓库；执行时源码保留为 `frozen/runner-executed.go`。执行脚本 SHA-256 为 `3edaead8205dfd3e733c102487c0e127f5a1c91cb6695cf82154ae45fbce3aa0`，原 `summary.json` SHA-256 为 `ad09f0c1d53d4eb52c54d5a0fb3c5a0a1e557012439084d1ee7220b535769884`。路径是本机证据指针，不保证临时目录长期留存。

## 旧 U5 评测的证据边界

历史曾报称“42 run 正确性全过、效果全 tie”。后续会话复核记录指出：快照不等价于完整 Fresh、actual 模型身份缺证、留出隔离不严格、部分题目不可达且 KEY 事后修订、评分行数与运行数不一致、游戏费用/耗时统计有遗漏。2026-11-09 在原 `%TEMP%\steamai-pd\u-cap\` 路径未找到原始评分及运行文件，不能在本轮重新核验那些数字。

因此保留“当时报称 tie”这一历史事实，**不将其作为严格冻结评测通过、稳定平局或方法效果提升的证据**。旧本机弹窗修复、六场景与定位耗时数字也仅属历史报称；本轮新增代码和测试是新的交付，不是恢复了旧材料。多并发失败的资源竞争归因未经证实，超时留下产物也不能改写成正常完成。

## 明确保留的未验范围

- 原效果目标（配对中位耗时 −25%、不必要用户介入 −50%、实际重复读取 −30%）未获充分证据；方法库整体收益仍未知，不能用工程测试替代。
- Git 内 case-at-repo-root 的真实可见成员窗口全链未验；须用户明确同意前台测试时间窗，headless 不能代替。
- 合成定位/输入通路不证明任意真实游戏、GUI 渲染或视觉模型可靠；真实 UI 的最后检查与点击不是原子事务。
- 本次不生成 calibration/promotion attestation，不补判历史 learning 门槛，不发布公开 Release。
