# 2026-09-29 智能运维官方化及重登部署

- 目标：64.83.10.67 / api.xingqiaolab.top；授权保留5xx、移除智能运维其余本地定制，沿用官方重登并不停机发布。费用及API/后台角色分工保留。
- 来源：已推送干净根main `671bfbd059647eb475e76703dada901dbfc330d0`；tree `1ee5f3457beb3b3399e920e8c0e98de2d9ebd5b8`；官方基线e0b227cc / 2.9.1。本记录随后追加，不改变部署来源。
- 应用digest：`sha256:cf441d30d9cf37c9ef1665ffedd56f4a0b060181cb3bf46795d6a8e6850cadbe`；镜像 `ghcr.io/leesssong/xingqiao-sub2api:release-671bfbd059647eb475e76703dada901dbfc330d0-cf441d30d9cf37c9ef1665ffedd56f4a0b060181cb3bf46795d6a8e6850cadbe`。
- 专用Worker镜像ID/digest：`sha256:906095cc20f11c106087c57e48b10e7b525aba77a1d2b7d9913465fb6b1dd4ca`。原样官方脚本/Dockerfile；Turb d32e49e623dddf71b5fa6f0f5b0250bef963bdbd、toSub2 8548397e89bf80e508eda64a87e0d556d43abc84，来源在infra/reauth-worker/dependencies.json。最终制品从上述main构建一次、同digest使用。
- 改动：移除quality remove_models/探测映射/本地判题提示、Pelican本地报告统计及附加入口；恢复官方。补回5xx开关/回显/保存，保留后台5xx链路和费用probe。历史迁移、旧报告表不删，线上removed_models状态为0。
- 部署：专用token仅存受保护配置；Worker共享普通Go Worker网络，确保Mihomo回环/租约同实例。supervisor串行调用官方--once，发布/回滚前停止领取并排空登录，之后接入新网络；不强杀在途登录。
- 本地验证：后端Quality/Pelican/ProbeCost/ProcessRole/OAuthAutoConfig四包通过，routes编译及go build通过；前端105项和typecheck通过；官方Python24项、生命周期7项通过；蓝绿preserve-detector/post-success-rollback/september29-online模拟通过。目标镜像断网imports、空mock claim、curl、Node20/jsdom及TLS桥通过。
- 替代验证：用户说明原账号失效后停止重试，使用合成账号和独立临时Postgres/Redis运行已提交源码的Linux测试二进制。TestAccountTokenGuardV2RepositoryLeasesAndReauth验证互斥领取、租约、旧快照拒绝、原子写回succeeded；两项Quality5xx验证cron/合并/去重；三项全部PASS。服务层OpenAIOAuthReauth/ParseReauth定向亦PASS，含错误身份保护。不使用业务数据，临时testcontainers已清理。
- 线上：blue API、普通Worker、专用Worker healthy；公网版本2.9.1，质量/凭证运营/鹈鹕/自动配置200，本地报告入口404；新前端资源含5xx开关。5条规则中2条5xx开启状态保留，账号158的openai_apikey_cache_creation_as_input仍true。
- 原真实任务：task1已被领取，登录后因等待期间凭据变化被CAS拒绝；新快照task18被身份不匹配保护拒绝。用户确认原账号失效后停止该账号验证，未绕过保护或重新登记其已移除监控项。有效真实账号的成功登录尚未验证，隔离PASS不等同于外部真实登录成功。
- 时间（北京时间）：应用18:25:01开始、18:32:12切流、18:32:43结束，共462秒；构建/传输约408秒，宿主就绪/切流/排空约54秒。旧green排空29秒、正常退出0，无强制停止。专用Worker18:33:11启动；隔离验证18:54:42通过。
- 采样：18:25:35至18:37:42，114次健康检查全部200且status=ok；不代表未采样窗口绝对无波动。
- 结果：succeeded/promoted/blue，无维护停服、无回滚。宿主记录 `/var/lib/sub2api/release-records/20260929T103149Z-production-2232192.json`；迁移hash保持a3be3a718ef8c6a3980b71d4c3367c20776cebed128d206fdbf54cb35dff1477。
- 回滚：保留上版green/普通Worker镜像release-3dfddccd0d73e188c572b3f54448532ceb7845e5-01e5b3fa8734f789e24e1fbe41fbbcd8e5aafadb3ceea4b7e5ee0deab14ecff8；通过现有宿主脚本--rollback --record及原受保护环境恢复。专用Worker用/usr/local/libexec/sub2api-reauth-worker.sh pause排空；配置备份在/var/lib/sub2api/release-staging/smartops-671bfbd059647eb475e76703dada901dbfc330d0/config-backup。恢复旧配置前先停用专用Worker。
- 测试站未查询、未同步；生产数据库、Redis、探测器及其他无关服务未重建。历史表惰性保留。
