# 云猫商品区默认定位：测试站发布

- 目标：独立测试站 `43.133.75.82`，入口 `http://43.133.75.82/redeem`。用户授权本次部署测试站；主站未访问、未发布。
- 改动：兑换页云猫嵌入区默认定位到商品购买卡片，保留完整店铺切换、新窗口入口和窄屏滑动提示。固定偏移只用于已验证店铺；后续整页加载恢复完整模式。
- 来源：`codex/storefront-products` 功能提交 `b3de76e9` 无冲突合入根 main；发布源 commit `38094fcce553ec3878ad23146b1bd00a6d8e9b05`，tree `6206f53a756bcd26a20c955674f73c15b3cf467b`。发布时根 main 干净、非 detached，commit/tree 与已获取 origin/main 一致。该功能分支及 worktree 保留为已合入的历史证据，不重复合并。
- 制品：镜像 ID/digest `sha256:8d71f1cf1c435c7b6b4d81780b3fd37ab24488c19516f2b41d2d7bfb6eb9ca3c`；内嵌二进制 SHA256 `24c8976752e215a1db9300c1a53c30993f316d3c9fc969253e845252f5ec97a7`。发布链 `ops/release-sub2api-test-station-api.sh`；API 蓝绿切换，旧依赖镜像来源核对通过，迁移和依赖文件没有变化。数据库、Redis、worker、detector 保留实例，Caddy 平滑 reload。
- 验证：复用本地37项组件/兑换页/i18n测试、类型检查、四文件ESLint和桌面/390px浏览器验证。21项API发布控制器测试通过；发布构建中3项locale测试、Vue类型检查、前端构建和Go内嵌服务器构建通过；Git差异检查通过。
- 公网功能：现有测试站登录会话刷新 `/redeem`，确认 `data-view=products`、scrollTop=400、iframe仍为原店铺URL。完整店铺切换后view=full、scrollTop=0；返回购买区恢复400。真实商品点击打开订单确认，取消后弹窗消失；未填写联系方式、未提交订单、未付款。截图仅包含云猫区域，不含账户余额或兑换记录。
- 运行结果：发布及最终排空成功，未回滚；最终health=ok、readyz=ready，新API healthy。旧API停止前连接数为0；worker、detector、Caddy、Postgres、Redis实例未重建。
- 耗时：Vite构建16.09秒，新镜像准备及API就绪19.87秒，最终排空0.44秒，残留连接0，上限300秒；其余阶段未单独计时。
- 回滚：旧镜像、容器和配置保留。入口 `sudo -n python3 /var/tmp/sub2api-test-station-api.wZ3K6m/deploy.py rollback /opt/sub2api-test-station/releases/38094fcce553ec3878ad23146b1bd00a6d8e9b05`；旧版本为 `c004475dcf5a46557bc2ffd388bfb07adc94be01`。有后续发布时须先核对当前release，不直接执行历史回滚。
- 本地保留：`.release/test-station-api-38094fcce553ec3878ad23146b1bd00a6d8e9b05/` 制品、manifest和控制器；截图 `docs/superpowers/evidence/2026-10-07-storefront-products/test-station-products.jpg`。两份用户既有环境规则修改单独暂存，归档提交后原样恢复并以SHA256核对，stash恢复点保留。
- 限制：真实付款、发货与兑换到账未验证；云猫未来调整顶部布局时固定400px偏移可能需更新。此归档仅新增记录和截图，不再次部署或构建。
