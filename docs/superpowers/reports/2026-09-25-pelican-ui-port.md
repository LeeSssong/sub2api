# Sub 鹈鹕测智前端移植

日期：2026-09-25。分支：`codex/pelican-ui-port`。状态：本地候选，待主任务整合；未推送、未部署、未连接真实上游。

## 来源与范围

来源为 `/tmp/sub2api-2.8.11-design-review/sub2api-2.8.11/frontend/src`。沿用档案实现与文案，包括糖果逻辑题、鹈鹕 HTML 题、模型/思考强度、最多 8 路并行输出、Cron 计划、结果记录和用户分组作品展示。没有加入已取消的自定义评分、关键词、额外浏览器渲染判定或新控制面。

以下 13 个文件与档案逐字一致：

- `api/pelicanShowcase.ts`、`api/admin/scheduledTests.ts`
- `components/admin/account/{IQTestModal,PelicanTestFields,PelicanRecordsDashboard,AccountActionMenu}.vue`
- `components/user/pelican/PelicanShowcaseCard.vue`、`components/user/pelican/pelicanShowcaseFormat.ts`
- `utils/{intelligenceTest,pelicanHtml}.ts`
- `views/user/PelicanShowcaseView.vue`
- `views/admin/settings/PelicanShowcaseSettings.vue`、`views/admin/settings/pelicanShowcase.ts`

档案 `LICENSE` 与已保留的 `upstream/sub2api/LICENSE` 内容相同（LGPL-3.0）。原有版权与许可证文件未修改。

## 必要接入差异

- `ScheduledTestsPanel` 接入档案内嵌鹈鹕模式、配置快照、记录详情和轮询。普通连接测试保留本地空 Cron 默认值；鹈鹕使用档案 `*/30 * * * *` 默认值。移除已分配给 Codex2API 的质量策略筛选依赖，Sub 类型不包含 `quality` 或质量判定字段。
- 账号页新增档案 `IQTestModal` 入口；两个现有 `AccountTestModal` 文件未修改。`accounts.ts` 的档案差异均为无关 harvest 功能，未移入。
- 设置页仅加入作品展示配置与保存逻辑。全平台活动分组供展示选择；原有订阅与调度选择器继续限定 OpenAI，保留本地成本、调度和监控配置。
- 新增用户路由、功能开关、中英文词条和管理员个人导航入口。本地普通用户有独立定制导航，额外接入受同一开关控制的鹈鹕入口，保留原链接与商店。
- `BaseDialog` 只增加档案预览所需的 `fullscreen` 支持和对应 CSS，未移入无关抽屉/遮罩行为改动。公共设置 store 已完整缓存配置，源码与档案一致，无需另改 store。

## 验证

先移入档案测试，确认缺少模块/计划字段导致失败，再移入实现。

直接相关的 14 个 Vitest 文件共 **104 项通过，11 项既有跳过**（分批运行、复用未受后续改动影响的证据）：

- 档案专属测试：IQTestModal 6、ScheduledTestsPanel 4、PelicanRecordsDashboard 6、PelicanShowcaseView 5、pelicanHtml 3。
- 账号菜单 11；两个普通连接测试组件合计 5；设置页 41 通过 / 11 既有跳过（新增档案展示保存用例）。
- BaseDialog 1、既有 AppSidebar 11、featureFlags 5、i18n 完整性 3；定制用户导航 3（新增开关启用/关闭红绿回归）。

`vue-tsc --noEmit`、`vue-tsc -b`、Vite 生产构建均通过；最终构建输出到 `/tmp/pelican-ui-final-build`。`git diff --check` 通过。依赖清单与 lockfile 未改动。

浏览器冒烟使用临时本地 fixture harness，全量拦截 Axios 与手动 SSE，未发真实 API 请求。检查 1440×1000 与 390×844 下的深色用户作品页、作品大图及浅色管理员双路输出：入口可见、结果完成、窄屏无水平溢出、iframe 为 `sandbox="allow-scripts"` 且包含档案 CSP，无浏览器 error 日志。该验证没有为产品添加任何额外渲染判定。

截图保留在该 worktree 的 `output/playwright/pelican-{gallery,admin}-{desktop,mobile}.png`；临时 harness 已移到 `/tmp/pelican-smoke.{html,ts}`，本地预览进程与浏览器标签已关闭。截图、构建产物和临时 harness 不入提交。

## 尚未验证

真实后端联调、上游生成、数据库和服务器行为由主任务负责；此分支仅包含前端与本报告。没有生产/测试站操作，也不宣称线上完成。
