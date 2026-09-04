# 管理员代充值订单与额度展示设计

## 已确认事实

- 测试服 `payment_orders` 已存在 `paid_quota_usd`、`gift_quota_usd`、`total_quota_usd`。
- Ent `PaymentOrder` 已生成对应字段，并已有 `operator_user_id`、`operator_note`、`operator_recharged_at`。
- 用户管理充值当前主要调用 quota ledger 接口，订单管理尚未完整展示额度字段。
- 测试服当前版本为 `924d5eb1d3e19b73823d6755bed213598f9694fb`。

## 目标

1. 订单管理列表在“实付”之后展示获得总额度、充值额度、赠送额度。
2. 全部支付方式筛选增加“管理员代充值”。
3. 用户管理充值创建 `payment_type=admin_recharge` 的已完成订单，并与额度钱包变更保持同一业务事务语义。
4. 管理员代充值必须填写交易单号；前后端均执行 trim 后非空及字符集校验。
5. 订单详情只读展示交易单号、三类额度、操作管理员和备注。

## 非目标

- 不新增或删除 `payment_orders` 的额度字段。
- 不改变支付宝、微信、Stripe、Airwallex 的支付回调和真实支付统计。
- 不把测试服数据、凭据或容器修改复制到 Git。
- 不部署主站。

## 方案

复用现有 `payment_orders`、quota wallet 和管理员订单 API：管理员充值成功后写入管理员订单事实及 quota ledger，订单使用 `admin_recharge` 支付方式、`COMPLETED` 状态，`total_quota_usd = paid_quota_usd + gift_quota_usd`，`pay_amount` 保存人民币充值金额，`payment_trade_no` 保存人工交易单号。管理员代充值不进入外部支付渠道退款按钮和外部支付统计。

## 接口与校验

- 管理员充值请求新增 `payment_trade_no` 字段。
- 服务器端拒绝空白、超长或包含非允许字符的交易单号；允许字母、数字、空格内 trim 后的短横线、下划线、点、斜线和冒号。
- `payment_type` 固定为 `admin_recharge`，客户端不可覆盖为外部支付方式。
- 后端创建订单时重新计算 paid/gift/total，不信任客户端 total。
- 重复交易单号在 `admin_recharge` 范围内返回冲突，不重复发放额度。

## 前端

- `PaymentOrder` 类型补齐额度、交易单号和管理员字段。
- 共用订单表和管理员订单表增加三列；管理员订单筛选增加管理员代充值。
- 用户余额充值弹窗增加交易单号输入，充值操作必填；赠送额度和充值金额保持现有输入。
- 订单详情显示交易单号与额度字段，管理员字段只读。

## 测试与验收

- Go：校验交易单号、管理员充值创建/幂等/重复交易号、DTO 映射和订单筛选。
- Vitest：充值弹窗提交字段与必填校验、订单表三列和支付方式筛选、详情只读字段。
- 运行直接相关 Go 测试、前端 Vitest、`go build ./cmd/server`、前端 typecheck/build 和 `git diff --check`。
- 测试服部署后只读核对 source commit/tree/image digest、健康状态和管理员页面；人工创建一笔测试充值验证订单字段与余额变化，随后按测试站数据治理要求记录或清理。

## 发布与回滚

- 必须从远端 `main` 可审计基线创建独立分支并提交；当前本地无 Git 历史且 tree 不一致，需先恢复基线。
- 候选分支推送后，由发布总控合入根 `main`，再从干净 `main` 发布测试站。
- 测试站发布失败保留候选分支和证据，使用上一 release 回滚；不触碰主站。

## 待验证项

- 当前远端 `main` 是否允许本机推送，以及本地无历史问题如何恢复。
- 现有管理员充值服务是否已提供订单创建事务入口，还是需要在 service 层新增协调器。
