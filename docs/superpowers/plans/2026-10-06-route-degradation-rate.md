# 线路疑似降智率实施计划

用户已选择口径：答题完成但评分未达标的巡检轮次 / 获得有效评分的巡检轮次。

实现范围：AI 工具线路详情图例与悬浮明细，复用 monitor-v4/timeline 和糖果题巡检记录。分组规则对应的账号各自一轮计一次；多模型、多样本全部完成有效评分才进入分母，任一 incorrect 则计一次疑似降智。超时、传输错误、判题 unknown、跳过、部分保存均排除。按用户后续要求，无有效轮次按 0% 展示并连续连线，悬浮提示注明“无有效评分，按 0% 展示”；仅展示回填，不增加统计轮次。

数据来源：scheduled_test_results 的 quality_round_id、结果配置快照和 quality_judgment；通过原有分组规则关联上的 tested_group_id 不可变快照确定线路。只统计明确指定线路的糖果题分组规则，不能把判题模型所在组当作被测线路；独立规则、全部分组规则、无法证明原分组的旧关联暂不纳入。现有历史保留策略仍限制可统计范围，不补造历史。

- [x] 写轮次判定、时间桶归属及图表无样本/分母测试，确认失败。
- [x] 扩展时间线 DTO；追加独立只读查询，不让巡检样本与真实请求 JOIN 扩大计数；按完成时刻归桶。
- [x] 增加图例开关、百分比曲线、计数提示和零分母展示，验证 API 输入。
- [x] 跑直接相关 Go/Vitest 测试、前端类型检查和 UI 检测，复查差异。

工作区：codex/route-degradation-rate，基线本地 origin/main e4bfb6c4；远端 fetch 因连接失败未确认最新。根 main 和已有工作区保持不写入。未获得部署授权，本轮只交付本地候选。

## 本地验证记录

2026-10-06：独立 PostgreSQL 16 测试库的 TEMP 表验证通过，覆盖窗口边界、部分保存、失败排除、真实请求计数不变及线路快照迁移。新增 264_quality_rule_tested_group.sql 仅对关联创建后未编辑过的模板回填分组；其余历史保持 NULL。创建计划时从事务内锁定的模板读取分组，编辑模板后旧计划保持旧分组、新计划保存新分组；迁移重复运行不覆盖已有快照。

已执行并通过：

- `go test -tags route_quality_integration ./internal/repository -run 'TestGradedCandyTimelinePostgres|TestQualityRouteSnapshotPostgres|TestMonitorV4TimelineBuckets|TestGradedCandyRound|TestQualityTimelineRoundAssigned' -count=1 -v`，DSN 仅指向自行创建的本地 QA 容器。
- `vitest run src/features/ai-tools/__tests__/RouteHistoryStrip.spec.ts src/features/ai-tools/__tests__/routeTimeline.spec.ts`：12 项通过。
- `vue-tsc --noEmit`：通过。
- impeccable UI 检测：无发现；桌面 520px／窄屏 320px 本地模拟预览：图例、20%（2／10 轮）、零分母提示与断线正确，窄屏提示未溢出。
- 两轮只读代码审查，发现的历史归属和旧 API 未知计数问题已修复并复审通过。

截图是本地模拟数据预览，非线上验收：`/Users/awen/.codex/visualizations/2026/10/06/01a11073-be89-7150-950d-5b9d179f6a9d/route-degradation-preview.png`。

未提交、推送、合并或部署；未启用测试站暂停的定时巡检。本地迁移验证不代表线上迁移容量/停机预检已通过，后续发布仍须遵循项目门禁。

## 2026-10-06 展示口径调整

用户明确要求无有效评分时按 0% 显示、保持曲线连续。已更新绘制值、悬浮提示和无障碍说明；轮次统计与后端口径不变。原断线截图与初次验证记录仅保留为历史，不代表当前候选展示。

调整后验证：前端两个测试文件共 12 项通过，vue-tsc --noEmit 通过，git diff --check 通过。未提交或部署。
