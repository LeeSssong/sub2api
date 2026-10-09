# 线路详情缓存命中率口径统一

当前会话直接执行，采用 writing-plans、executing-plans 与 TDD。用户已指定统计规则；不提交、推送或部署。

**目标：** 线路详情缓存曲线按后台同线路＋流式的原始用量日志统计，沿用线路的时间窗口。

**设计：** 缓存曲线独立汇总 usage_logs；复用后台 `buildRequestTypeFilterConditionWithAlias`，包含明确的流式与旧版 stream=true 非 WS 行，不筛选完整性、不去重、不套用错误/历史排除。各桶先汇总三个输入类 Token，再计算读取占比。零分母保留 null，现有 UI 显示零且说明无样本。成功率、TTFT、降智与推荐排序的快照字段不在此次曲线改动范围。

- [x] 从最新 origin/main 创建独立分支，执行既有 timeline 定向基线测试。
- [x] 添加 PostgreSQL 临时表回归，调用真实后台趋势查询核对每组、每桶缓存比例，覆盖重复、partial/unknown、legacy、WS、零分母、隔离及时间边界。
- [x] timeline 独立 `stream_cache_usage` 汇总，流式条件复用后台函数；桶起点沿用窗口起点，仅替换返回缓存字段。
- [x] 更新受影响的既有 SLA PostgreSQL fixture 和缓存断言，保留其他指标断言。
- [x] 执行 PostgreSQL 新回归与 SLA 回归、repository 定向测试及服务窗口测试；检查 diff 和根目录 main 不被修改。

验证命令（backend）：

```sh
ROUTE_QUALITY_TEST_DSN='host=127.0.0.1 port=55439 user=route_cache_qa dbname=postgres sslmode=disable' go test -tags route_quality_integration ./internal/repository -run 'TestRouteCacheTimelineMatchesAdminStreamPostgres|TestRouteSLATimelinePostgres|TestMonitorV4TimelineBuckets' -count=1 -v
go test ./internal/repository -run 'TestAccountMonitorRepository.*V4|TestMonitorV4|TestBuildRequestType|TestUsageLogRepository.*Trend' -count=1
go test -tags unit ./internal/service -run 'TestMonitorV4.*Timeline' -count=1
git diff --check
```

## 实际验证

- PostgreSQL 隔离回归 `TestRouteCacheTimelineMatchesAdminStreamPostgres` 五种窗口／粒度全部通过；逐组逐桶与后台 `GetUsageTrendWithFilters` 对账。
- 既有 `TestRouteSLATimelinePostgres` 五种窗口／粒度全部通过；`TestMonitorV4TimelineBuckets`、repository 定向测试和 `go test -tags unit ./internal/service -run 'TestMonitorV4.*Timeline'` 全部通过。
- `git diff --check` 通过；根目录 `main` 仍干净。本地 QA PostgreSQL 已停止，未访问测试站，未提交、推送或部署。
