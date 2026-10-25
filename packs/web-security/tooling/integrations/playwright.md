# Playwright 专业能力接入（web-security）

## 目的

浏览器端实证（真实渲染、交互路径、客户端状态）用固定版本的 Playwright 完成；它与 Strix 的静态线索、curl 的单请求回执分工不同，不互相替代。

## 固定版本与来源

- Python 包：`playwright==1.62.0`（PyPI 官方发行）；浏览器二进制用 `playwright install chromium` 安装对应 pin 的 Chromium，不用系统浏览器。
- 依赖只在获准环境安装（属新安装，需用户批准与恢复方式）；pack 不代装、不升级宿主。版本升级即重走本 integration 的实测前提。

## 运行接口（官方形态）

- 独立 `sync_api`/`async_api` 脚本或官方 CLI（`playwright codegen` 录制的脚本须检查、去敏后再作为模板）。
- 不同用户/角色使用**独立 browser context 或 APIRequestContext**：认证状态、cookie、storage 互不共享，也不与成员日常浏览器共享。
- 断言观察真实 DOM/响应/业务状态，不把 HTTP 200/403 或页面一行文本当完整证据；截图/trace/HAR 按需开启。

## 权限与目标边界

- 只访问任务明确批准的 origin/端口；第三方跳转与额外 origin 不自动跟随，出现即报告。
- 认证状态（登录 cookie、token）只存在于授权 case 的私有区域，不进入 prompt、通用报告或 pack。
- 写请求（表单提交、状态修改）属于 heavy action，需要针对该动作的用户确认；数据恢复用 fixture 的显式 reset，不把清库命令套到用户项目。

## 产物与敏感度

- trace/HAR/截图含会话与页面数据：默认存 case 内并登记 artifact index；对外引用前脱敏，token 一律不回显。
- 测试观察（步骤、断言、结果、限制）按 evidence 记录；修复任务代码交付走 code review。

## 停止与清理

- 脚本有总时长/步骤上限；结束后关闭本次创建的 context 与 browser 进程，确认无残留（任务管理器或进程表可核）。
- 中止后重开新 context，不复用可能污染的旧会话状态。

## 不支持什么

- 不做性能压测、大规模并发、验证码绕过或反爬规避；这些不在授权边界内。
- API 服务端事实的证明走 APIRequestContext 或既有 request recipe，与浏览器表现分开记录。
