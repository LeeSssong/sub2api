# 扣费标准文字对比度测试站发布

- 目标：首尔独立测试站 `43.133.75.82:22`，alias `sub2api-test-station`。按当前用户指令与实际 SSH 配置操作；磁盘环境文档中的旧地址不作为操作目标。主站未查询、未部署。
- 改动：下方分组扣费表采用与官方模型/价格相同的主文字颜色 `rgb(241, 249, 249)`、14px 正文字号；分组名字重650，其余正文600。表头采用主文字颜色、12px字号。仅修改三条局部CSS规则，数据、倍率与扣费逻辑保持原有实现。
- 分支：`codex/fee-table-contrast` 已快进合入根目录main，属于已合并历史证据。
- 发布源码 commit：`c004475dcf5a46557bc2ffd388bfb07adc94be01`；tree：`ed430a2ddd64cca70ddf1b7865a3ea3d40158f71`。构建前根目录main工作树干净、非detached，commit/tree与已获取的origin/main一致。
- 制品：`sha256:ce74caa8b7bc63f0629c2637ac18f0a732470995af68e639b7c7cbb212af5346`；binary SHA256：`19513e0feb7176e96d22b361173a7dbdd8619ee204c08b1cf66637a1505d0caa`。本轮构建一次，API蓝绿发布；worker、detector、Caddy、PostgreSQL、Redis容器身份保持一致。
- 验证：现有PricingDialog测试23/23，相关Vue ESLint、git diff检查通过；impeccable detect无发现。发布构建包含i18n测试3/3、vue-tsc与Vite成功。公网 `/health` 为ok、`/readyz` 为ready；运行commit/tree与制品一致。
- 线上界面：真实登录会话下桌面1512px与375px手机视口验收。各正文列颜色与官方价格一致，长模型列表正常换行；手机表格clientWidth289、scrollWidth720，横向滚动可到431，文档宽度375无页面溢出。截图位于本地 `.release/fee-table-contrast-preserved/desktop.png`、`mobile.png`、`mobile-rates.png`。
- 阶段耗时：本地前端与embed构建约42.95秒（文件时间测量）；上传至完成切流约31.04秒（其中宿主制品构建和就绪20.47秒）；线上界面验收未单独计时；旧实例排空23.33秒，结束时剩余连接0，最大排空时间300秒。
- 结果：API green `2490c7576440` 生效，旧blue实例已停止，未回滚。未解决问题：无。
- 回滚入口：`sudo -n python3 /var/tmp/sub2api-test-station-api.xJ95AE/deploy.py rollback /opt/sub2api-test-station/releases/c004475dcf5a46557bc2ffd388bfb07adc94be01`。旧镜像、发布目录和Caddy备份保留。

本记录的归档提交只增加文档，无需重复构建或部署。
