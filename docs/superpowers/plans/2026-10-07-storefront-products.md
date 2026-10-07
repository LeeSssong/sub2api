# 云猫商品默认定位实施计划

> 本会话单写入者内联执行，按 executing-plans、test-driven-development 和 verification-before-completion 验证。用户仅授权本地开发，保留分支，不提交、推送、合并或部署。

**Goal:** 兑换页云猫嵌入窗口默认显示商品区，订单确认弹窗仍可使用。

**Architecture:** RechargeStorefront 管理完整/商品两种显示模式；仅滚动自有容器，不读取跨域文档。对已核实店铺使用 400px 初始滚动和对称增加 iframe 高度，后续加载恢复完整显示；所有其他 URL 保持原行为。

**Tech Stack:** Vue 3、TypeScript、现有 i18n、Vitest、浏览器实际验证。

## 步骤

- [x] 从已获取 origin/main 创建独立 codex/storefront-products 分支及 worktree；复用本地依赖；原兑换页 16 项基线测试通过。
- [x] 在 src/components/payment/__tests__/RechargeStorefront.spec.ts 编写入口限制、切换、首次/后续加载和 URL 变化的行为测试，运行 `node_modules/.bin/vitest run src/components/payment/__tests__/RechargeStorefront.spec.ts`，确认新增功能缺失导致失败。
- [x] 修改 src/components/payment/RechargeStorefront.vue：已核实 URL 默认商品模式；自有 viewport 初始 scrollTop=400；iframe 高度为 viewport+800；完整店铺按钮切换同一 iframe；后续 load 恢复完整模式，URL watcher 重置；窄屏横向滚动及可见提示。中英文 dashboard.ts 增加三个对应文案键。
- [x] 运行组件、兑换页和现有 i18n 检查，执行 `node_modules/.bin/vue-tsc --noEmit` 与针对修改文件的 ESLint、Impeccable detector。
- [x] 在仅本机可访问的预览页挂载真实组件和真实云猫 iframe；桌面/390px 截图，验证商品首屏、完整模式恢复、订单确认/取消与新窗口链接。不提交订单、不真实付款。
- [x] 检查 git diff/status，记录实际通过的验证、截图及未验证付款/线上状态，保留 worktree 供后续发布。
