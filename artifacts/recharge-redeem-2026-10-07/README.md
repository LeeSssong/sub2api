# 充值与兑换合并页验证记录

日期：2026-10-07（Asia/Shanghai）。分支：codex/recharge-redeem-storefront。开发基线：origin/main aed5de647dfae7b7410f657a0a1a42c1c2ab1162。用户已授权本地提交及合入 main；本次不包含推送或部署。合并准备时根 main 为 d10f7fd9，本次涉及的代码与开发基线一致，复用上述验收证据；其他窗口未提交修改保留。

## 实现

账户权益通栏；桌面左侧原生兑换/规则、右侧云猫店铺，比例 2:3；窄屏先兑换再店铺，并有购买外链。最近活动通栏。用户可见的原配置店铺地址复用，HTTP(S) 校验，不追加站点登录 token。最新用户修正：去掉侧栏中间的充值与兑换菜单，仅保留底部余额入口；底部按原生支付开关进入充值或兑换页，保留充值/兑换码标签。兑换页签显示原生表单和真实云猫。旧店铺链接仍进入 /redeem。有店铺时隐藏误导性的支付不可用提示，无店铺保持原生导航。中英文标题/新文案保持对应。

## 验证证据

- 修改前直接相关基线 18 项通过。新增验收断言在实现前失败，缺失店铺、导航、提示行为为预期失败；第一版 39 项测试通过（含中英文键完整性）。入口修正后新增期望先出现 5 项预期失败，调整实现后直接相关的 RedeemView、AppSidebar.storefront、AppSidebar、UserRechargeNav 共 36 项通过；未改动语言包，复用键完整性证据。
- vue-tsc --noEmit、针对修改文件的 eslint、git diff --check 通过；Vite production build 成功，输出到 /tmp/xq-recharge-redeem-build。已有 Browserslist、Node localstorage、chunk size 和动态/静态 import 警告保留，未扩大修改。
- Impeccable detect 返回 []。
- 本地 Chrome 实际页面：1600×1000 桌面双栏，375×900 手机单栏，均无页面或内容区横向溢出；从密钥页点击底部余额入口，实际进入兑换页签；重复侧栏菜单不存在；旧 URL 重定向复用首次证据；规则能展开；兑换失败保留输入、成功清空并刷新余额，均为模拟 API，未真实入账。
- 真实云猫公开店铺在 iframe 加载成功，未购买或进入支付。verification.json 保留实际测量。
- 滚动到底部，卡片与侧栏账号区底边：桌面均 976px、手机均 882px；底部留白分别 24px、18px。
- 与密钥页比较：首块顶边桌面均 84px、手机均 68px；固定标题顶边桌面均 16.5px、手机均 15.5px。alignment.json 保留测量。
- 深浅主题截图经实际渲染核对。窗口及主题变化等待已有 CSS 过渡结束，避免采集中间状态。
- 第一版一次专项只读子 Agent 审查未发现阻塞问题；本次仅入口和标签的小范围修正，不重复扩大审查。主 Agent 与审查 Agent 均为 Codex，具体模型 ID/推理档位未从运行时获得，未作猜测。

## 范围限制

真实付款、实际兑换到账、公网资源版本和服务器运行态均未验证；未访问主站或测试站。云猫公告属于第三方，本次未调整。演示账户和余额仅用于本地浏览器验证。

## 复核

截图：desktop.png、mobile.png、desktop-light.png、mobile-light.png、mobile-bottom.png。本目录两个 mjs 脚本使用本机 Chrome、Playwright 与 127.0.0.1:43127 的 Vite browser-test 模式运行；站点 API 全部模拟，云猫仅公开读取。开发临时 Vite 进程在验证后停止。
