# 线路详情成功率统一后台 SLA

用户已在当前会话确认：每条线路以分组 ID 对应后台筛选，线路详情卡片与成功率曲线采用后台 SLA（排除业务限制）统计口径。当前授权为本地实现与直接相关验证，不含提交、推送或部署。

成功数为该分组、该时间段的 usage_logs 行数；失败数为同范围内非 count_tokens、status_code >= 400、非 is_business_limited、error_type 不为 client_canceled 的 ops_error_logs 行数。成功率为成功数 / (成功数 + 失败数)，不按逻辑请求去重，不筛选 usage_completeness，不沿用真实请求查询的历史排除日期、客户端归属或模型不支持文本排除。复用后台原始统计条件，不改变后台行为。

复用现有 monitor-v4/timeline 的权限可见线路、监控快照时间范围、5 分钟/小时/24 小时步长、100 分组/168 桶/10 秒限制。成功率计数独立按原始流水聚合，与缓存、首字和巡检样本不相互 JOIN 放大。响应明确 success_rate_basis=ops_sla；request_count 与 success_count 为 SLA 计数。旧统计响应不得被新客户端误当 SLA。卡片大号数字及提示直接合计同一次时间线响应的计数，避免卡片和曲线读取不同快照。每个桶左闭右开，“天”仍按窗口起点分段。

本次只改用户已确认的线路详情成功率及曲线。近一小时状态、我的线路列表、推荐评分及其他监控页面继续使用原有监控统计；不篡改 real_request_count/real_success_count 含义。缓存率、首字 P50、疑似降智率公式及前端缺样按零绘制规则不变。成功率加载/失败时卡片不回退旧口径；零 SLA 分母卡片显示暂无数据，图表仍按零展示并注明缺样。用提示说明后台 SLA 口径，不增加新布局。

验收：本地 PostgreSQL TEMP 表逐分组、逐桶将时间线与后台 queryUsageCounts/queryErrorCounts 复算比较，覆盖 complete/partial/unknown、同请求多行、业务限制、取消、客户端其他错误、模型不支持、429/529、恢复成功、窗口边界、空桶、跨日；证明其他三指标保持原口径。Vue 测试覆盖卡片和曲线同计数、粗细粒度汇总相同、加载/失败/无数据、拒绝旧合同。运行相关 Go、Vitest、类型检查和前端构建。

基线：本地 origin/main 4ecba1d44e34e8f94e9bc5ef76974737f02beea8。2026-10-09 fetch 连接 GitHub 失败，不能证明远端最新状态。工作区 /Users/awen/.codex/worktrees/route-sla/星桥测试服，分支 codex/route-sla。无数据库迁移。
