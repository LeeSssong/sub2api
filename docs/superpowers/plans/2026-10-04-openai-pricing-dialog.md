# AI 工具价格弹窗实施计划

> 在本窗口按步骤执行；用户已确认原型并授权修改代码、部署独立测试站。

**目标：** 将确认的价格弹窗落实到正式用户端，显示当前工具分组实际配置模型、五档官方参考价格与分组扣费标准。

**架构：** Dashboard 新增价格入口，独立 PricingDialog 复用 BaseDialog。分组模型通过 JWT 用户接口返回，复用原生 GatewayService 的分组模型枚举及模型白名单过滤，不复制原型快照，不改计费逻辑。

**范围：** 前端 DashboardView、PricingDialog、officialPricing 数据、ai-tools API、相关 CSS；后端 gateway_user_models handler 及用户路由。无 migration、依赖或运行配置变化。

- [x] 增加行为测试，验证工具模型范围、档位切换、未知价格、失败重试以及按用户可见分组限制模型数据。
- [x] 实现 `/groups/available-models`：认证、原生分组授权、账号模型枚举、白名单过滤；不返回账号或凭据。
- [x] 实现固定尺寸弹窗、五档选择、中文双层表头与上下文竖线、独立模型滚动区、四列扣费表；移植已确认价格，禁止计算未提供档位。
- [x] Dashboard 新增价格入口及焦点恢复，不阻塞工作区或线路统计加载。
- [ ] 执行前端定向 Vitest、typecheck、build，后端定向 handler 测试和编译、发布脚本契约测试；检查 UI 深浅色及窄屏。
- [ ] 提交候选；远端可达后核对最新主线并整合。发布前 stash 保护根工作区已有内容，保证干净 main 与 origin/main commit/tree 一致。
- [ ] 从根 main 执行 `ops/release-sub2api-test-station.sh`，核对 migration 集未变化、备份与回滚目标、健康/就绪及线上功能；恢复用户本地修改。
- [ ] 记录发布证据与未验证项。没有主站授权，不执行主站写入。

## 本地验证（2026-10-04）

- AI 工具、创建密钥与价格弹窗定向 Vitest：39 项通过；locale key：3 项通过。
- 前端 typecheck、vue-tsc -b、Vite 正式 build、定向 ESLint、git diff --check：通过。
- 后端 UserGroupModels 与原生 GatewayModels 定向测试及 go build：通过。
- 独立测试站 release contract、host executor、migration gate：通过。
- Impeccable detector：无发现。Chrome 实际渲染检查桌面和 375px 窄屏通过；深色截图 `/tmp/xq-pricing-code-desktop.png`，浅色和线上真实分组仍待验证。
- 本地 pnpm 版本与既有 lockfile overrides 不匹配，因此复用已有依赖；没有改依赖和 lockfile。ESLint parser 从既有 pnpm 包目录加载。
- GitHub fetch 已恢复成功，origin/main 仍为 `2e43f835`；当前根 main 为 `28b7644e`，其领先提交仅为既有原型/文档，不改变正式应用目录。
- 测试站 SSH 连接持续超时，尚未部署；主站未操作。后续发布必须先读取线上 release-state 并核对 migration 集、回滚目标，再从干净且已推送的根 main 执行发布脚本。
- 原型未跟踪源码、mock、测试 harness、依赖链接和已有本地修改不属于本次推广内容。
