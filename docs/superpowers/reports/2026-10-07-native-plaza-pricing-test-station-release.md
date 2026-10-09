# 模型广场同源价格：测试站发布

- 目标：独立测试站 `43.133.75.82`，SSH `sub2api-test-station`。沿用用户“部署到测试站”的授权。主站未访问、未同步。
- 改动：价格弹窗移除静态价格表，既有授权模型接口通过 `include_pricing=true` 复用模型广场的同一 `ModelPlazaService`、价格解析和 DTO；模型先经过原生授权、禁用过滤。完整显示原生上下文阶梯与缓存写入5分钟/1小时单价，缺失项为“—”，未知模型为“暂无参考价”。分组、模型、有效倍率、扣费公式保留原生接口及60秒刷新。价格格式也复用模型广场工具；不新增价格源、数据库、依赖或账务规则。公开模型广场仍未启用，无需为弹窗开放它。
- 来源：分支 `codex/native-plaza-pricing` 快进合入根目录 main 并推送。发布源 commit `28219b06140c7dc3fa56c7d16db3306575f37561`，tree `136ec6ba2e1292842c153ff843dec0e84f12064b`；构建前后根目录 main 干净、非 detached，commit/tree 与已获取的 origin/main 一致。功能分支/worktree保留为已合入历史证据，不再作为发布来源。本记录仅文档归档，不重新构建制品。
- 制品：Linux amd64 内嵌 binary SHA256 `8e603d9d071522cb02791786270c4c26f8fe51bfda4485a4414666666882c606`；镜像 ID `sha256:d05789b3f99f2381e43880d56f15dd4c25910d917f93d5729a3e3277993f1ad3`。发布目录 `/opt/sub2api-test-station/releases/28219b06140c7dc3fa56c7d16db3306575f37561`，API容器 `95b63ca84df6`。运行容器commit、镜像和binary校验和与清单一致。
- 路径：API 蓝绿 `test-station-api-green` → `test-station-api-blue`，Caddy平滑reload。本次门禁精确增加4个后端服务/装配文件；依赖、迁移、其他后端变更仍拒绝，monitor仍要求worker更新。worker `a98548b4174b`、探测器 `595856eabbd1`、DB `3cb0cd97a12d`、Redis `17fab82c3e5e`、Caddy `d339db76e851` 保持原实例。
- 验证：46项相关前端测试、typecheck、6文件ESLint、Go端点/服务/装配定向测试与build通过；19项发布控制测试通过，新增范围测试先失败后通过。发布构建同时通过3项locale检查及Vue类型构建。桌面与375px手机真实组件本地mock验收覆盖第三阶梯、零价格、缺价、缓存写入1小时及倍率刷新为0；手机document宽375px，表格局部横滚，无页面溢出，真实刷新SVG与文字可见。本地mock不等同线上认证验收。
- 线上已验证：公网 `/health` 为ok、`/readyz` 为ready，API健康；公网入口 `/assets/index-DK8mKiBB.js` SHA256 `37c4011d6c24e79f613da90f723189d0f36200f0dff84f240080c5e4a45c6ec1`，价格功能包 `/assets/DashboardView-DD1ENDkZ.js` SHA256 `52e568e0186d78326481a4c3b762bfeee30cda2f35659e771de983164dd6bbe2`，均与本地构建一致，价格包包含本轮同源接口与文案。
- 未验证：登录后的真实接口价格与弹窗交互、同组 `/v1/models` 现场列表对照、实际扣费。受保护测试站env内现有 `ADMIN_LAB_ADMIN_EMAIL/PASSWORD` 再次返回 `401 INVALID_CREDENTIALS`；属于既有凭据问题，未改密码、绕过认证或创建临时key。原生价格目录是基础参考价，分组/渠道自定义价格和服务档位仍由原生计费决定。
- 阶段耗时：前端构建16.28秒；宿主镜像准备与新API就绪19.09秒。编译和传输没有独立计时，不推算。连接排空71.37秒，残留0，上限从切流起300秒。
- 结果：部署成功，旧API已排空停止，认证功能验收待补。旧镜像、Compose、env与路由备份保留；本轮未回滚。
- 回滚入口：`sudo -n python3 /var/tmp/sub2api-test-station-api.S1M25H/deploy.py rollback /opt/sub2api-test-station/releases/28219b06140c7dc3fa56c7d16db3306575f37561`。旧版本 `a7e7415928c8debecb72b06128b5e8d764d19f2c`、旧API `2cf7ee16ae82`；无数据库迁移，恢复不覆盖业务数据库。
- 证据：`.release/native-plaza-pricing-preserved/deploy.log`、`public-assets.json`、桌面/手机mock截图，本次staging `manifest.json`和宿主 `deployment.json`。根目录用户两份环境规则修改以stash `9aa34bf0eafc7d5f172575dae9d29b95a0bcebfc` 暂存，归档后恢复并对比原patch。凭据、token及env全文不进入Git或普通日志。
