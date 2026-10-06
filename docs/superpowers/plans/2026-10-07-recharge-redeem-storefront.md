# 充值与兑换合并页 Implementation Plan

> 执行方式：当前会话单写入者，使用 executing-plans；用户已确认设计，直接完成本地开发与验证。

**Goal:** 将原生兑换和已配置云猫购买统一在一个响应式页面。
**Architecture:** 从公开 custom_menu_items 解析用户可见安全店铺 URL。新增轻量购买面板，兑换行为复用 RedeemView；底部导航沿用原生支付开关，旧店铺链接指向 /redeem。
**Tech Stack:** Vue 3、TypeScript、Tailwind、Vitest。

- [x] 基线：既有 RedeemView、AppSidebar.storefront、UserRechargeNav 共 18 测试通过。
- [x] 先扩展既有测试验证配置店铺显示、无配置/管理员配置隐藏、外链不泄露登录凭据、底部入口归一，以及支付关闭时仍可购买。
- [x] 创建 src/utils/rechargeStorefront.ts，调用 sanitizeUrl，仅解析用户可见 xingqiao-storefront；src/components/payment/RechargeStorefront.vue 展示标题、购买说明、iframe、独立外链及错误恢复。中英文文案同步。
- [x] 修改 RedeemView：移动表单/规则至左栏，购买面板右栏，桌面 2:3、窄屏单栏；保留最近活动和原生兑换刷新逻辑。UserRechargeNav 新增 storefrontAvailable 参数，保留充值/兑换码页签；支付关闭且有店铺时仅隐藏不可用提示。
- [x] AppSidebar 去掉中间店铺入口，底部余额入口保留原生支付开关路由，旧 /custom/xingqiao-storefront 路由重定向；合并页 title 使用中英文 rechargeTitle。
- [x] 运行直接相关 Vitest、vue-tsc、eslint 检查、构建与 Impeccable 检测。浏览器使用本地演示数据核对桌面/375px 的位置、无横向溢出、规则展开、兑换反馈与外链；明确第三方支付/真实入账未验证。
- [x] 复核 git diff，保留独立分支及工作区，交付截图与验证结果。

用户已确认并授权提交本次实现及验收材料、合并根 main；合入结果以 Git 合并提交为准。复用现有有效测试，核对合入内容和已有未提交修改。
