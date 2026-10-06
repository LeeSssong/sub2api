# 扣费标准动态更新：本地验证

- 日期：2026-10-07（Asia/Shanghai）。工作分支 `codex/native-group-pricing`，基线 `daf9d1eda246e9ec8e91b161639e6cd156805968`。所有任务修改未提交。
- 分组、模型及倍率分别来自原生 `/api/v1/groups/available`、`/api/v1/groups/available-models`、`/api/v1/groups/rates`。打开及手动刷新读取，打开且页面可见时每 60 秒刷新。关闭/卸载取消，页面隐藏暂停。
- 普通 `/v1/models` 与面板模型接口共享候选来源。定向 Go 测试对比 ID 集合，覆盖 OpenAI 映射与原生通配 ID、Anthropic 映射、分组白名单、用户禁用模型、无账号默认回退、混合平台、CN 映射、固定账号原生清单和原生清单缓存。固定清单空目录返回空数组；读取失败不替换成默认清单，不暴露账号/上游错误。
- 前端 `PricingDialog.spec.ts`（11 项）、`model.spec.ts`（18 项）、`groups.pricing.spec.ts`（3 项）与 `DashboardView.parity.spec.ts`（31 项）共 63 项全部通过，验证三个字段同步、分组新增/删除/改名、用户专属倍率与零倍率、失败重试、定时刷新、关闭取消、切换竞态及页面隐藏恢复。
- `pnpm run typecheck`、`pnpm run build`、相关九个前端文件 ESLint、`git diff --check` 通过。构建已有大分块提示，测试已有 Browserslist/Node localStorage 提示；未改动依赖。
- `go test ./internal/handler -run 'TestAvailableGroupsExposesOnlyAuthorizedNativeToolMappings|TestUserGroupModels|TestGatewayModels|TestOrdinaryPinnedModels|TestPinnedModelsAllowlist|Test.*Denied.*Model|Test.*RetrievedModel' -count=1` 与 `go build -o /tmp/native-group-pricing-server ./cmd/server` 通过。
- UI 机械检测 `impeccable detect --json src/features/ai-tools/PricingDialog.vue` 返回 `[]`。

## 浏览器验证（本地 mock，非线上证据）

本地 Vue/Vite 渲染真实 PricingDialog、BaseDialog 和项目样式，Playwright 仅拦截上述三个 API 使用构造数据。桌面 1440×1000、手机 390×844 均无页面横向溢出，刷新按钮可见；原有表格通过局部横向/纵向滚动查看。

四条 mock 分组（含新增混合组，排除仅映射 Claude 的 OpenAI 组）首次显示后，原生响应改为单条“更新后的原生分组”，模型变为 `gpt-6.1-sol`、用户倍率为 `0`，刷新后整行显示 `0.0x倍率` 和 `模型基础费用 × 0`。模拟 rates 返回 503 时扣费表消失并显示重试，恢复接口后重试显示最新值。整个交互没有 pageerror。

截图：`desktop.png`、`mobile.png`、`mobile-fees.png`（手机局部横滚后倍率和公式两列均完整可见），均为本地 mock 数据，不含生产数据或凭据。

主站与测试站均未访问、未查询、未部署；未验证真实线上接口响应和两站状态。当前未合并、未提交、未推送。

补充验证：`go test -tags unit ./internal/service -run 'TestAPIKeyGetGroupToolMappings|TestNormalizeGroupToolIDs' -count=1` 通过。服务测试需使用项目 Makefile 规定的 unit tag；初次未指定 tag 时被已有 rawChatCompletionsTestAccount 测试辅助函数的构建标签阻断，没有修改无关测试。

收尾只读审查发现并修复两项 P2：保留显式工具映射优先级；模型请求限定到当前工具的授权分组。复核未发现新的相关实质问题。浏览器实际模型查询范围首次为 `1,2,3,5`、更新后为 `2`，未请求无关组 `4`。原生分组接口授权隔离、非法模型查询参数及无关固定目录失败均有新增定向验证。
