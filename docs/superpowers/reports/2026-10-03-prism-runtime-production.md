# 2026-10-03 Prism 适配器主站安装

- 目标：`sub2api-prod` / `64.83.10.67`。用户授权完成 Prism 部署安装，要求不影响 Sub2API 服务。结果：外置适配器和开机自动绑定服务已安装，API 通过蓝绿切换加载 Prism 配置，绿色实例健康接流；旧蓝色实例自然排空 76 秒，未强制终止、未回滚。
- 来源：安装前核实根目录干净且 `main == origin/main`。官方适配器源码取自已推送 `c12511f191fd407ac4cb691494eed5a59ad95e72`，Git archive SHA-256 `52d318a475c93a0e6d24bcd70dba54d646db17d6e01e9653aeed59cc24bcc74e`。网络空间服务及配置工具来自已推送 `bcd8ae8fa69487a765f5c6d4fc6817ed1f06a85a`。
- 应用制品复用上一发布：commit `d80af9a5cb00d304b68a985594c9697f201a0bc3`，tree `7e191e04dc32f2227e3524cbe0608e62b042ae7f`，镜像 ID/digest `sha256:81d729c5c6eb3b6992efe37b21fbdaab9ac7c2abb71ce6dc3b1b988f87064a3d`。归档 SHA-256 `8f178dd1a6ca562d52c9e2b3dec468108bc3e85f3cb8b7869d6e3fb3577ed5da`。应用源码、数据库迁移集合均未变化，未重复构建。
- 安装：Python 3.12 虚拟环境、Playwright 1.63.0、jsonschema 4.26.0、lark 1.3.1 及匹配的 Chromium 153.0.8010.12（1243）。仅安装预构建 wheel / 浏览器及必要系统运行库。镜像外运行目录 `/opt/sub2api-prism`；完整依赖及二进制校验在宿主 `runtime-manifest.json`。首次浏览器检查发现 Ubuntu 沙箱不可用，配置 root:4755 sandbox helper 及邻接 `chrome-sandbox` 链接后通过；未禁用浏览器沙箱。
- 拓扑：`sub2api-prism@green.service` 与 API 共享网络空间，绑定 `127.0.0.1:8319`；worker 有独立适配器绑定，但其应用未启用 Prism 配置。`sub2api-prism-reconcile.timer` 每 10 秒检查容器身份，随后续蓝绿部署更新绑定，只管理 Prism 单元。适配器运行用户为 `sub2api-prism`；所有 Prism 单元共享 1 CPU、MemoryHigh 750 MiB、MemoryMax 900 MiB、禁 swap、256 tasks 限制。检查时约 142 MiB 内存。
- 配置：只对两个 API 槽位增加网关总开关、回环地址和桥接密钥；密钥只保存在受保护环境文件及生产 Compose，均 root:0600。浏览器采用串行模式，单缓存会话，空闲 300 秒回收；6.1 Sol 客户端工具桥开启。没有更改任何账号 Prism 开关或分组，没有自动迁移现有用户流量到实验通道。健康路径为 `/health`，调用路径为 `/v1/responses`。
- 验证：4 项 namespace 选择测试通过；API 配置工具验证保留 worker、检测器及已有环境。适配器 62 项离线测试通过；真实 Chromium 模拟上游验证 3 次文本请求、2 个项目、1 次会话缓存命中；工具桥验证 function/custom 及客户端回灌的 3 次提交。生产容器 network namespace 和服务文件系统限制下重复浏览器模拟通过。回环未认证返回 401；正确桥接密钥但缺账号身份返回 400，证明鉴权链可用且没有发起模型请求。网关与适配器密钥一致，自动绑定 timer enabled/active、最近运行成功；Sub2API `/health` 和 `/readyz` 均成功。
- 应用影响：Go worker、独立重登 worker、检测器、Caddy、PostgreSQL、Redis 的容器 ID 与 StartedAt 均与安装前一致，未重建或重启；API 配置由正常蓝绿流程生效。首次宿主调用缺少 preloaded 归档参数，被预检拒绝，未启动候选；补齐参数后发布成功。没有使用数据库迁移或停服路径。
- 耗时：62 项离线测试 12.38 秒；沙箱浏览器模拟约 6 秒；客户端工具模拟约 4.52 秒；生产 namespace 浏览器模拟约 6 秒。宿主蓝绿发布北京时间 03:15:16 开始，约 03:15:30 切流，排空 76 秒，约 03:16:46 完成。运行依赖和安装日志在 `/opt/sub2api-prism/runtime/`。
- 发布记录：`/var/lib/sub2api/release-records/20261002T191516Z-production-3490972.json`，`result=succeeded`、`downtime_required=false`、`rolled_back=false`。外置适配器不改变应用镜像 commit；本报告提交也不改变应用制品。
- 回滚：保留 `/opt/sub2api/production/compose.yaml.before-prism`。先恢复该受保护 Compose 配置，再使用原宿主执行器及本次成功记录回退 API 槽位；保留数据库、状态及既有镜像。停用外置适配器时先停 reconciliation timer，再停止 Prism 单元；保留 pending 和 tools 数据，不通过删除待决文件解锁。无回滚发生。
- 未验证：真实 Prism OAuth 登录、具体账号模型权益、真实模型响应和计费均未验收；模拟测试不证明上游可用。Prism 仍是 usage=null 的实验通道。管理员需在账号管理编辑选定 OpenAI OAuth 账号后开启 Prism，再做账号测试。测试站未查询、未同步。
