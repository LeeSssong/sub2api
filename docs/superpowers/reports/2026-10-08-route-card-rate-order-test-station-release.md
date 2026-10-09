# 线路详情卡片按倍率升序：测试站发布

- 目标：独立测试站 `43.133.75.82`，SSH `sub2api-test-station`，延续用户本次测试站界面修改要求。主站未查询、未同步。
- 改动：仅线路详情卡片按实际显示的有效倍率升序，优先使用用户专属倍率、否则使用分组倍率；同倍率保留原相对顺序，未知倍率置后。最佳线路判定、工具卡片及我的线路列表不改变。
- 来源：`codex/route-card-rate-order` 快进合入并推送根目录 main。发布源 commit `92c375891b55de00e1856a3e7d52ed3e8ce92c5d`，tree `fe046a958e5e600f3b6cd3840fd224499d2eb1d0`。发布脚本核验根目录干净、非 detached main 与获取后的 origin/main commit/tree 一致。
- 制品：image ID `sha256:8fa77206f8b2e1580c8311e1b70f0b009f3c2196ed93dece855a9cb3d9974d96`；binary SHA256 `1ff0a2b2de5afecb03fc140474e9a35c56e0dc21da830894259b5e1fa078ba83`；release `/opt/sub2api-test-station/releases/92c375891b55de00e1856a3e7d52ed3e8ce92c5d`。
- 本地验证：先新增页面交互回归测试，确认旧质量排序导致失败；替换排序后 Dashboard 32 项和线路图 6 项合计 38 项通过，类型检查、locale 3 项和正式前端构建通过。覆盖专属倍率、零倍率、相同倍率、未知倍率、1h/24h/7d 和最佳线路不变；Impeccable 扫描无发现。复用上次发布的同一控制器 22 项测试证据，本次未修改发布脚本。
- 线上验证：独立 Chrome 验收标签页使用现有测试站登录态，真实 Codex 卡片依次为科研分组 0.001x、GPT-Pro5x 0.2x、GPT-Pro20x 0.3x、生图 1.0x；最佳标识仍在 GPT-Pro20x。桌面和 375px 窄屏均确认此顺序，窄屏无页面横向溢出，截图在本次浏览器工具输出中可复核。图例沿用已发布顺序，请求成功率、首字 P50、缓存命中率、疑似降智率，首字蓝色曲线正常。公网 health/readyz 均 200 且分别 ok/ready。
- 资源核对：公开 `DashboardView-DYsxWDZH.js` 和 `index-TTgb8d9b.js` 与本地构建逐字节一致；SHA256 分别为 `bbcda9d7cffeef386ab9cce394949eee20603fe53c8254640a4023a88c216379` 和 `a1de8f9430eb591f666ca82d9e9874bb51a0a4d15dfcd1468acfb99825f80e1a`。
- 发布结果：API blue → green 蓝绿成功，新 API `f6abcd30a387` 健康，未回滚。worker `351e15732a54`、detector `595856eabbd1`、Caddy `d339db76e851`、Postgres `3cb0cd97a12d`、Redis `17fab82c3e5e` 保持原 ID，仅 Caddy 平滑 reload 路由；未进行数据库变更。
- 阶段耗时：Vite 构建 16.62 秒，宿主制品准备与就绪 19.69 秒；验收后旧 API 检查残留连接为 0，停止及排空完成 0.39 秒。其他阶段未独立计时，不推算。
- 回滚入口：`sudo -n python3 /var/tmp/sub2api-test-station-api.qzhpa3/deploy.py rollback /opt/sub2api-test-station/releases/92c375891b55de00e1856a3e7d52ed3e8ce92c5d`。旧容器、镜像和路由配置保留，不涉及业务数据库回滚。
- 验收限制：原有浏览器标签页导航后出现空白，独立同浏览器验收标签页实际页面正常且完成上述验证；未确认原标签页空白原因。本次未再尝试配置文件中的失效登录凭据，也未更改账号或密码。线上仅验收默认 24h 卡片；1h/7d 排序由本地交互测试覆盖，不扩大到统计 API 专项。
- 功能分支与 worktree 已合入 main，保留为历史证据，不作为后续发布来源。本记录单独归档，不改变已部署制品。
