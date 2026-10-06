# 2026-10-07 模型价格表层级与近24小时排序测试站发布

- 目标：独立测试站 `43.133.75.82/dashboard`；主站未查询、未发布。
- 改动：筛选移到标题右侧，单位移到表格左上，删除发布时间和底部同源说明；模型名与价格使用主文字颜色和650/600字重。保持原生同源价格、上下文阶梯及分组倍率。
- 排序：近24小时全站 requested 模型请求数降序；同次数或无请求按既有已验证发布时间降序，未知日期在后。只展示当前可用模型，类型筛选保留顺序。
- 原生复用：新增已登录 `/api/v1/usage/models/popularity` 汇总，直接调用原生 UsageService.GetModelStatsWithFiltersBySource；固定滚动24小时、requested来源与全站口径，忽略客户端日期/用户/分组参数，只返回模型名、请求数和区间；一分钟共享缓存合并并发，查询超时5秒。前端独立加载，失败保留已有排序，不阻塞价格。没有数据库迁移。
- 分支 `codex/pricing-popularity` 已合入并推送main，保留为历史证据。发布源 commit `092e8a2bca0ddc62bdd0aaf33c3a63f4d4859de0`、tree `aabc3b5ac17d00f7043ca49434e0d4d2e710b878`；发布时根main干净、非detached，与已获取origin/main的commit/tree一致。归档提交只更改文档，不再次构建。
- 运行镜像ID/digest：`sha256:75062a6b9123cd1d7e75156c74b4aff787104b8281693c1291b3f5ced6377558`；二进制SHA256：`6cddd800a768623f8f947f307c83750e4a8b72f3b2e259da631a55fa7c2cf31e`。实际API镜像与记录一致，API healthy。
- 复用/新增验证：70项前端直接相关测试通过（价格16、弹窗23、首页31）；类型检查、5文件ESLint、diff检查通过；5项新热度接口测试及3项已有相关测试在race模式通过；20项发布链测试通过。routes包编译成功，所选范围无测试用例。一次机械检测无发现。
- 真实页面：32模型；筛选与标题中心差0px，单位位于左上，两段说明不存在；主文字 `#f1f9f9`，模型字重650、价格600。旗舰筛选20项。375px页面无横向溢出，局部表格clientWidth263px、scrollWidth1172px，scrollLeft262.5px时模型与容器left均52px。
- 原生数据验证：测试站近24小时usage_logs无请求，真实页面按发布时间兜底；有请求排序由专项测试及本地模拟数据验证，未生成业务请求或插入测试账务。未登录热度入口401。未声称线上验收了非零请求排序。
- 公网health=ok、readyz=ready；公网入口 `/assets/index-BUHMeqda.js` 与价格包 `/assets/DashboardView-CJ19Qzi5.js` 均与本轮构建资源SHA256一致。入口SHA256 `883622a2a54953e8d87d8151f52e5d9f06812108b35a103a32da29e6fd3d1e50`，价格包SHA256 `d34fbbedb92522cae97d851cd97ec9d89ab55acaaea8259c5e9586f9f4b27f6f`。
- 耗时：Vite构建15.80秒，宿主镜像准备及就绪20.25秒；旧实例排空0.52秒，残留0。其余阶段未单独计时。
- 结果：API蓝绿发布成功、未回滚。worker、detector、数据库、Redis保持原实例；Caddy仅平滑reload。用户两份既有规则修改单独暂存并恢复，stash备份保留。
- 回滚入口：服务器 `sudo -n python3 /var/tmp/sub2api-test-station-api.LwBeXV/deploy.py rollback /opt/sub2api-test-station/releases/092e8a2bca0ddc62bdd0aaf33c3a63f4d4859de0`；旧a0a7318e制品和previous-state/previous-Caddyfile保留。
- 限制：扩大Go匹配范围时既有 `TestUserUsageGetByID返回安全详情摘要` 失败，已在本轮修改前的根main复现；本次未改动详情摘要，未声称全量Go测试通过。本轮直接相关专项未发现未解决问题。
- 本地证据：`.release/pricing-popularity-preserved/` 中的deploy.log、deployment.json、public-assets.json、native-requests-24h.json、frontend-tests.log、backend-tests.log、ui-live.json、mobile-live.json及桌面/手机截图；模拟截图不作为线上证据。
