# 线路图指标排序与蓝色首字 P50：测试站发布

- 目标：用户本次指定的独立测试站 `43.133.75.82`，SSH `sub2api-test-station`。主站未查询、未同步。
- 改动：图例和悬浮提示统一为请求成功率、首字 P50、缓存命中率、疑似降智率；首字 P50 的折线、图例线段和提示文字统一改为蓝色，深色 `#82b7f3`、浅色 `#326ea9`。统计、开关、提示和时间轴逻辑保持原样。
- 来源：`codex/route-chart-order` 快进合入并推送根目录 `main`；发布源 commit `32b125ab35864602459033c1c68b4d62370ca290`，tree `a65f1d4f4815b59960411db048f28fb7cf15e160`。发布脚本验证根目录干净、非 detached main 与获取后的 origin/main commit/tree 一致。
- 制品：image ID `sha256:5c9d5e914e4ea0e165c27af1b18867b2070372d0758d79f9136dd82ef0a96cae`；binary SHA256 `4625fd3aa70331d5b32436b395ceb8cb411c5a13098d698ec191a9ed0597243c`。发布目录 `/opt/sub2api-test-station/releases/32b125ab35864602459033c1c68b4d62370ca290`。
- 预检纠偏：发布状态中的旧 API/worker ID 已失效。按唯一运行服务、健康状态、原镜像 digest、原 Compose 路径及 Caddy 实际运行配置核验后，仅原子校正状态文件中的两个容器 ID；备份 `/opt/sub2api-test-station/release-state-before-route-chart-order.json`。未更换凭据、镜像或预检时的路由。
- 本地验证：线路图和时间线 12 项既有测试、类型检查、locale 3 项、正式前端构建通过；发布控制器既有 22 项测试通过。桌面 1440px、手机 375px 深浅主题四组组件渲染验证排序、三处颜色一致、无页面横向溢出、图例无重叠、开关可用、无 pageerror。渲染使用明确的本地样例数据，不属于真实站点数据验收。
- 公网验证：`/health` 返回 ok，`/readyz` 返回 ready。实际提供的 `DashboardView-BEBlkBV1.js` 与 `DashboardView-6YuEoMFD.css` 逐字节匹配本次构建，JS SHA256 `1fd0ed330ed8cf71fd0f3db8f9d6806a3e218959cc0c5dc307a718c4a0344154`，CSS SHA256 `fe1ab6892df25044961342475ad473c7925882b439fc0aac437642dcfa342a71`；资源中核对新顺序与两套蓝色色值。
- 路径与结果：API green → blue 蓝绿成功，新 API `a5eafa0951a7` 健康；Caddy 平滑 reload。worker `351e15732a54`、detector `595856eabbd1`、Caddy `d339db76e851`、Postgres `3cb0cd97a12d`、Redis `17fab82c3e5e` 保持原 ID。未更新数据库、worker 或探测器，未回滚。
- 阶段耗时：前端 Vite 构建 16.32 秒，宿主制品准备和就绪 20.89 秒；完成页面相关验证后检查旧实例，残留连接为 0，排空结束及停止 0.89 秒。编译、传输和人工诊断阶段未独立计时，不推算。
- 回滚入口：`sudo -n python3 /var/tmp/sub2api-test-station-api.fugqkJ/deploy.py rollback /opt/sub2api-test-station/releases/32b125ab35864602459033c1c68b4d62370ca290`。旧 API 容器、镜像、路由和配置已保留；不涉及数据库回滚。
- 未验证项：受保护根 env 及活动 release env 的管理员登录均返回 HTTP 401，真实登录后的线路详情页面未验收；已请求用户在现有测试站浏览器登录。未重置账号或密码，不将本地样例截图混称线上截图。
- 证据：`.release/route-chart-order-local-verification.json`、`.release/route-chart-order-{dark,light}-{1440,375}.png`。功能分支与 worktree 已合入 main，仅作历史证据，不用于后续发布；本记录单独归档，不改变已部署制品。
