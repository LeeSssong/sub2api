# 线路详情层级与整点统计：测试站发布

- 目标：独立测试站 `43.133.75.82`，SSH `sub2api-test-station`。用户明确授权本版部署测试站。主站未访问、未同步。
- 改动：删除顶部说明、强化倍率、加强卡片背景与柔和阴影，卡片间距 20px；保留“近 24 小时”，24h/7d 完整滚动窗口按当前整点结束，近 1h 按分钟实时滚动。统计边界与快照刷新时间分开，汇总和曲线复用同一持久化窗口。
- 来源：`codex/route-card-refinements` 已快进合入并推送根目录 main。发布源 commit `612b67cd93286563737fb72bc5a4d6c4481f2b30`，tree `04e1adfce6483d3c177e20654e31911861118e66`。构建前后核对 main 非 detached、工作树干净、commit/tree 与获取后的 origin/main 一致。本记录单独归档，不改变运行制品。
- 制品：API 与 worker 使用同一 image ID `sha256:ab167cb8995efb8fbe85d143a3655bf7d581fd6b6d2ef5acd3f608747f881373`。发布目录 `/opt/sub2api-test-station/releases/612b67cd93286563737fb72bc5a4d6c4481f2b30`。合规 main 构建内嵌服务器，沿用校验过 digest 的旧镜像运行依赖；未改依赖或 migrations。
- 路径：API 蓝绿（`test-station-api-green` → `test-station-api-blue`）；worker TERM 后等待正常退出，单实例替换；探测器、DB、Redis、Caddy 容器保持原 ID，仅 Caddy 平滑 reload 路由。worker 活动容器 `a98548b4174b`，API `bfda2271c71c`，均健康且 commit/image 一致。
- 验证复用：43 项直接相关前端测试、3 项 locale、Vue 类型构建与完整应用构建、MonitorV4 service/repository/handler 三包测试、桌面和375px浅深主题预览。新增：15项发布控制测试（单worker切换、就绪失败、回滚探测失败仍恢复worker等）、runner生命周期测试、发布逻辑独立审查；审查发现的回滚问题已复现并修复。
- 线上验证：公网 `/health` ok、`/readyz` ready；实际页面说明消失、倍率14px、卡片背景/阴影/间距符合批准版，24个整点桶；375px无横向溢出，关联按钮44px高。API验证24h汇总与24个曲线桶计数一致，1h汇总与12个5分钟桶一致。DB只读验证24h/7d分别为24/168小时整点窗口，1h滚动60分钟，刷新时间保持分钟新鲜度。
- 阶段耗时：前端构建16.37秒；宿主制品准备/新API就绪18.95秒；worker替换7.15秒；旧API连接排空0.65秒、残留0、上限300秒。其余本地编译和传输阶段没有独立计时，不推算。
- 结果：本版成功发布、未回滚，旧API已排空停止、旧镜像和配置保留。根目录原有两份环境身份文档修改在发布阶段临时stash，结束后原样恢复；stash保留为恢复证据。功能分支/worktree已合入main，作为历史证据保留，不再用于发布。
- 回滚入口：`sudo -n python3 /var/tmp/sub2api-test-station-api.5gG71N/deploy.py rollback /opt/sub2api-test-station/releases/612b67cd93286563737fb72bc5a4d6c4481f2b30`。旧API `1817bdf7d313`、旧路由、worker配置/env与旧worker镜像均保留。此版没有数据库结构迁移，回滚不恢复/覆盖业务数据库。
- 已有未解决项：近7天的小时曲线查询达到服务10秒 deadline，HTTP 500。用保留旧API在内部地址对照同站数据，旧版10.04秒、新版10.03秒均500；旧API对照后再次停止，未向旧API切流。七天汇总与DB整点范围已验证，但七天小时曲线未通过线上验收。管理员公告读取亦有既有错误日志。以上不混称本次改动验证通过。
- 截图：`.release/route-card-refinements-test-station-desktop.jpg`、`.release/route-card-refinements-test-station-mobile.jpg`；统计API对照证据 `.release/route-refinements-api-verification.json` 与 `.release/route-refinements-timeline-version-comparison.jsonl`。凭据均通过受保护来源使用，未入记录/仓库。
