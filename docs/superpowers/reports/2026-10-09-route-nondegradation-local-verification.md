# 线路不降智率本地验证

状态：本地实现及直接相关验证完成。分支 `codex/route-nondegradation`，基线 `71a85174`，改动未提交、未推送、未合并 main、未部署。未访问主站或测试站，不代表线上验收。

## 计算口径

`不降智率 = (分组用量请求数 - 降智请求数) / 分组用量请求数 * 100%`。

复用原生用量行计数，排除 `usage_completeness=unknown`。账号在该分组最近一次明确检测结论为降智时，该账号后续开始的请求计入降智；明确全部通过后恢复。超时或判定不确定保留原标记；未标记请求按公式计入不降智。多个规则采用账号/分组最新明确结论。

请求尝试开始时间随结果进入用量写入，数据库根据该时间查找账号/分组状态事件，并固化到用量行。恢复账号、重复写入和监控结果清理不会改写已保存快照。已有请求不回填；无用量、旧接口或桶中存在缺失快照时显示“暂无数据”。

## 实际验证

- Go：service、handler、repository 直接相关单元测试通过，覆盖检测、恢复、观察模式、用量时间传递、重试时间更新、原生用量参数形状与线路时间桶。
- PostgreSQL：在本机独立临时集群验证 `TestQualityTrafficSnapshotsPostgres`、`TestQualityTrafficUsageInsertPathsPostgres` 通过，覆盖分组隔离、跨恢复的长请求、旧用量、四种原生写入路径和重复写入幂等。
- 新增“成功但判题结论缺失”初始化回归测试，先复现失败，再修正 SQL NULL 聚合行为，重跑通过。
- 原线路缓存和 SLA PostgreSQL 测试已通过，覆盖 1h/24h/7d 及 hour/day 分桶。
- 前端：三个文件共 26 项 Vitest 测试通过；`vue-tsc --noEmit` 通过；`git diff --check` 通过。
- Chrome/Playwright：1440px、375px 下验证 95%（95／100 次）、无请求显示暂无数据；无页面异常、横向溢出、tooltip/图例文字溢出或 NaN 曲线。已检查截图。

后端单元测试使用仓库现有 `unit` build tag；未运行全量测试。前端输出存在已有的 pnpm 配置、Browserslist 和 localstorage 警告，不影响上述检查退出成功。

## 迁移与限制

- 需要数据库迁移 `266_quality_traffic_snapshots.sql`，新增状态事件表、两个用量字段和插入触发器。未对线上数据库执行迁移。
- 初始标记仅采用保留的、可确认归属分组的监控结果；缺少历史证据不能完整还原迁移前账号状态，也不补造历史用量快照。
- 暂停或删除监控不会自动证明账号恢复，已有明确降智状态持续至新的明确通过结论。
- 应用请求时间与数据库事件时间依赖时钟一致；未验证真实环境的时钟偏差或并发事务提交边界。
- 原生批量图片结算未记录分组，继续遵循原有分组用量范围。人工创建或其他未带开始时间的用量保留缺失快照，不伪装正常。
- 当前仅验证组件示例和后端隔离测试，未运行已登录站点的端到端真实流量验证。

## 本地复核

示例页面：`http://127.0.0.1:5179/src/features/ai-tools/__tests__/fixtures/routeNondegradation.html`，使用固定示例数据，不连接站点 API。

前端：

```sh
pnpm exec vitest run src/features/ai-tools/__tests__/RouteHistoryStrip.spec.ts src/features/ai-tools/__tests__/routeTimeline.spec.ts src/features/ai-tools/__tests__/routeNondegradation.spec.ts
pnpm exec vue-tsc --noEmit
```

后端：

```sh
go test -tags unit ./internal/service ./internal/handler ./internal/repository -run 'TestQualityTraffic|TestQualityObservation|TestQualityOutcome|TestQualityModel|TestOpenAIGatewayServiceRecordUsage|TestGatewayServiceRecordUsage|TestUsageLogRepositoryCreate|TestUsageLogStaticInsertShape|TestPrepareUsageLogInsert|TestUsageLogInsertQueries|TestMonitorV4TimelineBuckets|TestGrokRealtimeBilling' -count=1
```

隔离数据库验证需设置 `ROUTE_QUALITY_TEST_DSN`，使用 `-tags 'unit route_quality_integration'`。用量快照测试只创建临时表，不连接生产数据库。
