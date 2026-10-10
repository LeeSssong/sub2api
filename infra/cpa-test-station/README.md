# CPA 测试站

目标为 `ubuntu@43.133.75.82`。独立 Compose project、network、数据卷和运行目录均以 `cpa-test-station` 命名。运行目录为 `/opt/cpa-test-station`。

复用本地版本的 Linux AMD64 制品：CPA v8.0.20、CPAMP v1.14.4、Ticket Gateway v1.5.1。镜像按 digest 固定；管理页使用 CPAMP v1.14.4 的官方 `management.html`，SHA256 为 `98476e174f8f74001c2d1b576091106a49f00b476843ed2a54d38a9f9f04e4ae`。插件下载地址为 `https://ticket.codingmiao.site/downloads/cliproxy-ticket/1.5.1/cliproxy-ticket-gateway-1.5.1-linux-amd64.tar.gz`，压缩包 SHA256 为 `a40134d7d4e6501f85ea6d9ea08085986e405670b387e147397619a3f1282305`，还须校验包内二进制清单。

CPA 管理入口为 `http://43.133.75.82:8317/management.html`；完整 CPAMP 管理入口为 `http://43.133.75.82:18317/management.html`。分别使用 CPA Management Key 与 CPAMP Admin Key。服务器及云防火墙需要允许这两个 TCP 端口。

域名入口为 `https://cpa-test.xingqiaolab.top/`，使用 CPAMP Admin Key 登录，管理页和插件 API 均由已配置的 Manager Server 提供。独立 `compose.edge.yaml` 使用既有 Caddy v2.10.2 制品、仅发布 443，并通过 CPA 专用网络访问 Manager。DNS A 记录指向 43.133.75.82；首次签发证书时使用 DNS only，让 ACME TLS-ALPN-01 直接验证 443。证书和续期状态保存在独立卷内。将 Caddyfile 放到 `/opt/cpa-test-station/edge/Caddyfile` 后启动 edge project，原有 Sub2API 的 80 端口服务可继续运行。域名入口不依赖新增 8317/18317 公网端口。

## 初始化

插件页面兼容补丁 `ticket-gateway-ui.html` 来自已校验 Ticket Gateway v1.5.1 Linux AMD64 制品的 `/v0/resource/plugins/cliproxy-ticket-gateway/ui`，保留原页面和业务逻辑，仅增加 CPAMP v1.14.4 `enc::v2::` 原生会话格式读取。原始页面 SHA256 见发布记录。将该文件与 Caddyfile 一起安装到 edge 目录，边缘仅替换这一资源路径；插件 API 和后端鉴权继续由 Manager/CPA 提供，不另存管理凭据。插件仍要求原生登录时启用记住凭证。回归命令：`node --test tests/infra/ticket-plugin-session.test.cjs`。

只从已推送、干净且与 `origin/main` commit/tree 一致的根目录 main 推广 Compose 和 `infra/cpa-production/config.example.yaml`。将模板中的管理密钥替换为测试站独立随机密钥。使用全新 auths、插件状态目录、Manager 数据卷和独立密钥，密钥文件权限为 0600，目录权限为 0700。

先拉取固定镜像，准备并校验管理页和插件，再导入 CPA 连接：

```sh
docker compose -f /opt/cpa-test-station/compose.yaml run --rm --no-deps \
  -v /opt/cpa-test-station/secrets/cpa-management-key:/run/cpa-management-key:ro \
  cpa-manager store-cpa-connection \
  --cpa-base-url http://cpa-gateway:8317 \
  --management-key-file /run/cpa-management-key \
  --db-path /data/usage.sqlite --data-key-path /data/data.key
docker compose -f /opt/cpa-test-station/compose.yaml up -d --wait --wait-timeout 90
```

验证两容器健康、管理页校验和、两套密钥鉴权、CPAMP 到 CPA 的连接、插件 v1.5.1 注册及原有 Sub2API 健康。插件初装无默认网关地址；业务账号、网关绑定、CDK 激活和真实模型请求需使用测试站独立凭据另行配置和验收。

首次安装回退时仅停止该 Compose project，保留运行目录和数据卷；升级时保留前一版配置、插件和镜像，切回已验证兼容的制品。不得用删除卷的方式回退。

当前插件目录为只读挂载，页面插件商店安装会返回 `create plugin directory: read-only file system`。升级使用官方校验包，先通过插件 `update/prepare` 排空请求，再停止 CPA gateway、原子替换宿主持久插件文件并启动。保留原配置、安装身份、auth 和数据目录；同步本目录的 CPAMP 会话兼容页面。
