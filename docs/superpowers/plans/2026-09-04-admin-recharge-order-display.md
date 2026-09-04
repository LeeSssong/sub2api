# 管理员代充值订单与额度展示实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 在测试服完成管理员代充值订单化、额度展示和交易单号审计。

**Architecture:** 复用现有 `payment_orders`、quota wallet 和管理员 API；只补齐订单创建协调、DTO、前端表格/详情/弹窗，不新增平行账务事实源。

**Tech Stack:** Go、Ent、PostgreSQL migration、Vue 3、TypeScript、Vitest、pnpm。

---

### Task 1: 恢复可审计 Git 基线

**Files:**
- Modify: `.git/*` 仅通过 Git 命令生成

- [ ] 核对远端 `main` commit/tree 与本地关键源码 blob。
- [ ] 将当前快照形成明确基线提交并标注来源 commit/tree；不得把凭据或运行数据加入提交。
- [ ] 创建 `codex/admin-recharge-order-display` 分支并确认工作树干净。

### Task 2: 管理员充值后端协调器

**Files:**
- Modify: `upstream/sub2api/backend/internal/handler/admin/user_handler.go`
- Modify: `upstream/sub2api/backend/internal/service/admin_user.go` 或对应订单协调服务
- Modify: `upstream/sub2api/backend/internal/service/admin_recharge_validation.go`
- Test: 对应 Go handler/service 测试

- [ ] 先写失败测试：交易单号 trim、非法字符、重复交易号和 `total=paid+gift`。
- [ ] 实现固定 `admin_recharge`、后端重算额度、操作人和备注写入。
- [ ] 将额度钱包发放与订单创建通过同一协调器保证幂等。
- [ ] 运行定向 Go 测试并提交。

### Task 3: 管理员订单 DTO 与查询

**Files:**
- Modify: `upstream/sub2api/backend/internal/handler/admin/payment_handler.go`
- Modify: `upstream/sub2api/frontend/src/types/payment.ts`
- Test: `upstream/sub2api/backend/internal/handler/admin/payment_handler_test.go`

- [ ] 先写失败 DTO 映射和 `admin_recharge` 筛选测试。
- [ ] 返回三类额度、交易单号、管理员字段。
- [ ] 确保旧订单缺省值可显示且不改变现有支付订单语义。
- [ ] 运行测试并提交。

### Task 4: 订单列表、筛选和详情 UI

**Files:**
- Modify: `upstream/sub2api/frontend/src/components/payment/OrderTable.vue`
- Modify: `upstream/sub2api/frontend/src/components/admin/payment/AdminOrderTable.vue`
- Modify: `upstream/sub2api/frontend/src/components/admin/payment/AdminOrderDetail.vue`
- Modify: `upstream/sub2api/frontend/src/views/admin/orders/AdminOrdersView.vue`
- Modify: `upstream/sub2api/frontend/src/i18n/locales/zh/misc.ts`
- Modify: `upstream/sub2api/frontend/src/i18n/locales/en/misc.ts`
- Test: related Vitest specs

- [ ] 先写失败组件测试，断言三列顺序、支付方式选项和详情只读字段。
- [ ] 实现额度格式化、管理员代充值标签和详情展示。
- [ ] 管理员代充值不显示外部渠道退款动作。
- [ ] 运行前端定向测试、typecheck/build 并提交。

### Task 5: 用户充值弹窗交易单号

**Files:**
- Modify: `upstream/sub2api/frontend/src/components/admin/user/UserBalanceModal.vue`
- Modify: `upstream/sub2api/frontend/src/api/admin/users.ts`
- Test: `upstream/sub2api/frontend/src/components/admin/user/UserBalanceModal.spec.ts`

- [ ] 先写失败测试，断言交易单号必填、trim 后提交和空白拒绝。
- [ ] 增加交易单号输入并提交到管理员充值 API。
- [ ] 保持退款操作不要求交易单号。
- [ ] 运行组件测试并提交。

### Task 6: 汇总验证与测试站发布准备

- [ ] 运行 Go 定向测试、前端定向测试、构建/typecheck、diff-check。
- [ ] 生成交接报告：基线、分支、提交、文件、测试、迁移、配置、凭据/数据边界、回滚和未验证项。
- [ ] 推送候选分支；未经发布总控授权不合入 `main` 或部署。
