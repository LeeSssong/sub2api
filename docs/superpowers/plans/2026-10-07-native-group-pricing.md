# 扣费标准动态原生接口实现计划

**目标：** 弹窗实时读取原生分组、模型与倍率接口，扣费公式使用同一倍率，支持模型与用户普通模型接口一致。

**架构：** 保留价格弹窗，独立同步原生数据并管理请求取消与定时刷新。后端抽取普通模型目录的候选解析供两个入口复用，固定目录调用现有原生服务，用户禁用模型沿用原生过滤方法。

**技术栈：** Vue 3、TypeScript、Vitest、Go、Gin。

- [x] 基线：运行 `vitest run src/views/user/__tests__/DashboardView.parity.spec.ts` 和 `go test ./internal/handler -run 'TestUserGroupModels|TestGatewayModels_ModelAllowlist' -count=1`。
- [x] 先编写 PricingDialog 组件回归测试：打开后替换旧分组/倍率、手动及定时刷新、零倍率、接口失败、取消与切换工具、空目录。
- [x] 先编写后端端点对比测试：同用户分组普通 `/v1/models` 与 available-models 的 ID 集合相等，覆盖白名单、无账号默认回退、混合平台、用户禁用模型及固定账号目录。
- [x] 运行新增测试确认缺失行为导致失败。
- [x] 修改 `src/api/groups.ts` 支持 AbortSignal；修改 `PricingDialog.vue` 原生工具映射优先、按当前授权分组限定模型查询、三接口同步、同值倍率公式、刷新按钮和生命周期清理。
- [x] 修改 `gateway_user_models.go` 复用原生目录来源与用户禁用规则；从 `gateway_handler.go` 抽取候选解析，保持原有模型元数据输出；固定账号目录复用 `FetchPinnedOpenAIModelsList`。
- [x] 运行受影响组件、首页交互及后端模型列表测试；运行前端类型检查、构建、Go 编译及差异检查。
- [x] 使用本地浏览器 mock 接口验收桌面/手机弹窗、数据更新、刷新/错误状态，截图并运行 UI 检测；如真实服务器未访问，明确标注。

实现由当前 Agent 在独立工作区执行，收尾按 requesting-code-review Skill 使用一名只读审查 Agent；用户未要求提交或发布，保留可审阅的未提交修改。
