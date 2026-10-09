# 智商监测独立糖果题检测

用户确认范围：使用智商监测自己的题目、参考答案、判题配置，每轮真实调用；默认 gpt-6-astra / gpt-6.1-sol，high；各分组使用五字段 Cron，默认 `*/30 * * * *`。绘图继续每 30 分钟。

## 实现

- 复用分组原生路由和既有计划/结果表。糖果题、绘图在配置 JSON 内分别保存下次执行时间；同一事务领取租约并推进到期题型的时间，CAS 防止陈旧 worker 覆盖新游标。
- 立即检测执行两类题目，不推进定时游标；运行中编辑影响下一轮。无可用账号保留真实失败，答错不通过换账号重试洗成成功；质量策略固定 observe_only。
- 默认糖果题明确允许按形状挑选。自定义题目需配置判题模型；旧规则保留已有独立题目和判题配置，缺失时使用独立默认值，不读取质量运维来源。
- Dashboard GET 只读；展示来源 intelligence_live 的糖果题，旧 quality_ops 快照及历史不删除。每次真实结果计入统计，30 分钟色块可以展开全部检测；运行中仍可查看已完成结果。结果新鲜度按 Cron 下一次检测加执行宽限计算。
- 无 SQL 迁移，无生产配置/数据修改。共享计划租约避免重叠；检测耗时超出 Cron 间隔时不会堆积重叠调用。

## 验证

- Go service：`go test -tags unit ./internal/service -run 'TestIntelligence|TestPelicanGroupTest' -count=1` 通过。覆盖双模型调用、分组 Cron、不同题型独立到期、过期补一次、Cron 无未来日期拒绝、手动运行不推进游标、看板不访问质量来源。
- PostgreSQL：`go test -tags integration ./internal/repository -run 'TestIntelligence(Rule|Single|Live)' -count=1` 在一次性 PostgreSQL/Redis 上通过，包含事务保存、并发编辑/暂停、过期 worker CAS、游标持久化、实时结果可见性。仓库已有 account_ops_batch/once 集成测试构造参数缺失，通过临时 Go overlay 增加 nil AuditLogService 让测试包编译；该补丁未纳入代码改动。Docker 使用本机 colima/build。
- 前端相关测试、vue-tsc、locale 检查与 Vite 生产构建。详细结果见本工作区 output/iq-final-frontend-tests.log、output/iq-frontend-build.log。
- 浏览器使用真实 Vue 配置组件，分组读取/保存为本地 fixture；默认双模型、high、两个分组分别保存 `*/5 * * * *` 与 `15 * * * *` 已核对。1280px/375px 页面与弹窗无横向溢出。截图 output/playwright/iq-cron-desktop.png、iq-cron-mobile.png。此为本地组件交互验证，不是线上或真实上游验收。
- 只读代码审查指出高频运行隐藏既有结果，已以失败回归测试复现并修正，相关测试通过。

## 交付状态

候选实现完成；未合并、推送、部署，未调用真实上游进行付费验收。测试站未查询、未同步。

Agent：主 Codex（运行模型精确 ID 与推理强度未由当前上下文提供）；只读审查 review_live_candy，gpt-6-sol / high。
