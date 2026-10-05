### Task 1: 合并后端与前端契约并保留测试站页面

**Files:**
- Resolve: `git diff --name-only --diff-filter=U` 的 37 个冲突文件，当前 merge 已在独立 worktree 开始，双方 SHA 见规格。
- Inspect merged: upstream/sub2api/backend/internal/{repository,service,server,web}/ 与 frontend/src/{router,components,views,features,api,types}/ 中双方修改文件。
- Test: 对应已有 *_test.go 和 *.spec.ts；新增仅针对实际发现的回归。

**Interfaces:** 保留主站全部 API 与鉴权，中间兼容层保留测试站 groups/available-models、monitor-v4/timeline、monitor-v4/check 和 group tool mappings。页面模板/样式以测试站为基准，适配主站数据类型与逻辑。

- [ ] 阅读 frontend/AGENTS.md；记录每个冲突的取舍及自动合并中的语义风险。
- [ ] 在修改业务逻辑前先运行直接相关已有测试，记录失败；补充揭示真实问题的最小回归测试。
- [ ] 解决后端 5 个文本冲突；核查 current_operational 与主站查询/快照服务兼容、P50、分组检查范围、权限和支付方式逻辑。
- [ ] 解决前端文本冲突；核查测试站样式与组件逻辑，保留主站新增页面/路由，暂不增加未经视觉确认的导航入口。
- [ ] 对旧调度日志给出真实可行的兼容处理；若不可等价保留，保留证据并明确上报，不能用空壳通过验收。
- [ ] 运行 pnpm typecheck、直接相关 Vitest、pnpm build；Go 对受影响服务/路由/仓库/迁移运行相关测试并编译入口。环境依赖失败先诊断，不降低门禁。
- [ ] 记录未验证项、实际命令和结果；无冲突标记且候选可编译后提交合并候选，不推送共享 main。
