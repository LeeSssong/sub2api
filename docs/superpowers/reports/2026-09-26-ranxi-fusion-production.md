# v2.8.13 + PR84 主站发布

目标：sub2api-prod / 64.83.10.67。北京时间 2026-09-26 完成蓝绿发布，未执行停服流程。用户明确接受短暂 reload/切流波动并免除本次远端来源核验；来源为本地干净 main，未推送。

- 运行 commit：`d8859f6949b3cee22fd5a455d16ab845bacfeaf8`
- tree：`e694214b7cd8afd149ff61922fc368340edb35f9`
- 主机核验镜像 digest/ID：`sha256:cf5affdce7ead279982efba85e6a0414c86d5360219672788cd1c95e78eb82ee`
- 版本：`0.2.8+xingqiao.ranxi.2.8.13`；活动槽 green，worker 同步更新。
- 迁移收据哈希：`5b011a1ade72118f5a69c1afaa728f5aa5060b27444b199362483a27a03a08df`。
- 权威主机发布记录：`/var/lib/sub2api/release-records/20260925T162700Z-production-807360.json`。

复用原融合和 PR84 定向测试。新增本地来源校验验证（精确 commit 通过，错误 commit/rehearsal/脏树拒绝）。上线前发现重复鹈鹕路由，复现启动 panic 后删除重复注册，相关路由测试通过；首次构建未部署，修复后重建。

Caddy 配置先备份、验证再平滑 reload；已加载 24h 保留和固定槽图片回调。worker 配置增加 60s graceful stop。配置准备未停止 API。首次 reload 的现有 WebSocket 可能重连，未测量所有用户的连接波动。

线上验证：发布链候选 health/版本/公共设置/models 及公网基础探针通过；追加 health、readyz、管理版本、凭证守护、质量计划、鹈鹕历史接口通过。凭证守护 enabled=false，PR84 自动关闭启用账号数=0。未发真实模型请求、通知或凭据守护动作。

阶段时间：修复后镜像使用前端缓存重建后端；上传与构建独立耗时未由控制器持久记录，不补造。主机发布始于 00:27:00，green 于 00:27:25 启动，worker 于 00:27:36 启动（北京时间，秒数取容器 StartedAt）；排空循环记录 11 秒、forced=false。retain initially 保留等待退出标记，后续在发布锁下验证旧 blue 镜像匹配且正常退出码 0，于约 00:31:36 清除标记并补记 drain completed。

结果：发布成功，未回滚。旧 blue 镜像和容器保留；恢复使用既有 host 发布链回滚入口及上述记录，不将旧镜像恢复等同于数据库回滚。配置备份：主站 production 下 `Caddyfile.before-fusion-20260926` 和 `compose.yaml.before-fusion-20260926`。

偏差/未解决：发布脚本既有 detector_enabled 分支重建了 model-detector，已健康；不符合无关服务不重建约束，后续普通发布需修正此行为，不能声称本次没有重建。数据库、Redis、Caddy 容器身份保持。未验证所有真实上游功能或全用户无波动。测试站未查询、未同步。远端没有本次提交，按用户本次授权以本地来源发布。
