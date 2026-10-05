# 线路详情卡片实施计划

在当前会话按 executing-plans 执行。用户已批准设计，无需重复批准。

目标：把 Dashboard 详情表格替换为双列/单列语义卡片，保留全部操作与统计含义。
技术：Vue 3、现有品牌 CSS、Vitest；无后端改动。

- [x] 在 DashboardView.parity.spec.ts 增加每条线路卡片、成功/总请求、缺失与零请求、关联指定线路及关闭返回的回归；先确认新断言失败。现有按 tr 定位的测试改为按 article 定位。
- [x] DashboardView.vue 用 article.route-detail-card 和 dl 替换详情表格，沿用 detailRows、detailMetrics、stateOf、openCreate、checkOf。请求数显示 `real_success_count ?? '—'` / `real_request_count ?? '—'`。
- [x] xingqiao-ai.css 删除仅详情表格使用的样式；添加两列 minmax(0,1fr) 网格、700px 以下单列、品牌令牌、自然换行、触控目标、弹窗内纵向滚动。
- [x] 运行 Dashboard parity 与 model 直接测试、vue-tsc -b、vite build、git diff --check、Impeccable detect。
- [x] 用本地模拟 API 渲染实际构建，检查桌面/手机深浅主题、长名称、四状态、时间切换和关联密钥；保存截图和简要证据。

文件范围：DashboardView.vue、xingqiao-ai.css、DashboardView.parity.spec.ts；本地说明、模拟与截图仅作验证资料。

## 已批准的时间条扩展

- [x] 提取真实请求共用 SQL，新增用户可见线路历史桶接口，添加边界、鉴权与统计口径测试。
- [x] 增加品牌色历史柱条，支持悬停、点击、键盘及加载/失败/零请求状态。
- [x] 完成 41 项前端测试、后端定向测试、23 项 PostgreSQL 实测与生产构建。
- [x] 检查桌面及手机 12 / 48 / 42 桶布局和交互，保存模拟截图与验证记录。

时间条扩展包含后端只读接口变更；无数据库迁移。提交、发布不包含在本次已执行范围内。
