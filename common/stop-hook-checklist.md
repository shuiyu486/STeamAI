# Stop hook checklist

每轮修改项目文件后：

1. 更新相关 README / CLAUDE / reference / task handoff。
2. 若产生可复用经验，运行 `/rekit promote` 的 review-first 流程，避免把 case 私有状态回流模板。
3. 若代码变更，运行语法检查或测试。
4. 若 confirmed CSV 变更，重建 routine IR / superinstruction。
5. 运行 `/rekit doctor`，验证 Markdown 大小不超预算。
6. 如果当前目录不是 git 仓库，在最终回复说明无法 commit/push。
7. 普通 batch 在 Windows 本机验证完成后只做一次 implementation commit/push，并立即继续下一批；不轮询或等待远程 workflow，不创建 release inspection commit。只有正式发布、跨平台专项或每 3–5 批周期复审才等待并记录异步远程结果。
