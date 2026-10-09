# CPA 生产网关与独立管理域名规格

**日期：** 2026-10-08

**状态：** 设计已确认，进入实施

**目标环境：** 主站 `64.83.10.67`

**公开入口：** `https://cpa.xingqiaolab.top`

**基线：** `main@0d0bdfb796882b45c31f9c1a60492af649914e70`

## 1. 目标

- 在主站部署互相配套的 CLIProxyAPI（CPA）、CPA Manager Plus（CPAMP）和 `cliproxy-ticket-gateway` 插件。
- 通过独立 Cloudflare 子域名公开 CPA 管理页面，继续使用主站现有 Caddy 统一承载 TLS。
- 保持 Sub2API、Codex2API、数据库、Redis、worker、探测器和现有网络运行态不变。
- 为后续将 CPA 作为 Sub2API 的 OpenAI 兼容上游准备稳定的 Docker 内网地址。
- 向管理员交付在 CPA 中创建专用业务 API Key 的操作步骤。

## 2. 本轮不做

- 不在 CPA 中创建业务 API Key。
- 不修改 Sub2API 的上游账号、分组、调度、倍率、计费或模型映射。
- 不向 CPA 导入 Codex OAuth 账号，不兑换 CDK，不配置未知的票据网关地址。
- 不修改 Codex2API 的容器、网络、端口、数据或配置。
- 不公开 CPA 的宿主端口 `8317` 或 CPAMP 的宿主端口 `18317`。
- 不删除失败部署留下的卷、密钥或回滚制品。

## 3. 方案选择

采用“Cloudflare DNS + 现有 Caddy + 独立 Compose project”。不采用直接开放宿主端口，也不新增 Cloudflare Tunnel 常驻代理。

理由：

- 新域名可以复用现有 80/443 入口和 Cloudflare 代理，不增加公网监听端口。
- Caddy 只需校验配置并平滑 reload，不重启 Sub2API 或 Codex2API。
- CPA/CPAMP 的生命周期、数据和回滚入口独立于两个现有业务栈。
- Sub2API 后续可通过 Docker DNS 直连 CPA，业务请求不经过公网或 Cloudflare。

## 4. 拓扑与数据流

```text
管理员浏览器
  -> Cloudflare: cpa.xingqiaolab.top
  -> 生产 Caddy
     -> CPA 管理页面、插件页面和管理 API
     -> CPAMP 用量/状态接口（按路径转发）

Sub2API（后续单独接入）
  -> http://cpa-gateway:8317/v1
  -> CPA 账号池
  -> Codex/OpenAI 上游

Codex2API
  -> 保持原拓扑，不连接 CPA project
```

## 5. 运行资源

### 5.1 宿主布局

- 安装根：`/opt/cpa-manager-plus`
- Compose project：`xingqiao-cpa`
- 服务：`cpa-gateway`、`cpa-manager`
- 专用网络：`xingqiao-cpa-internal`
- 共享网络：现有外部网络 `sub2api_default`
- CPA 在共享网络中的别名：`cpa-gateway`
- CPAMP 在共享网络中的别名：`cpa-manager`

两个服务都不配置 `ports`。Caddy 和未来的 Sub2API 上游通过 `sub2api_default` 访问；CPA 与 CPAMP 之间通过专用网络通信。

### 5.2 持久化

- CPA 配置：`/opt/cpa-manager-plus/cliproxyapi/config.yaml`
- CPA OAuth 文件：`/opt/cpa-manager-plus/cliproxyapi/auths/`
- CPA 日志：`/opt/cpa-manager-plus/cliproxyapi/logs/`
- 插件二进制：`/opt/cpa-manager-plus/cliproxyapi/plugins/`
- 插件状态：`/opt/cpa-manager-plus/cliproxyapi/ticket-gateway-data/`
- CPAMP 数据：独立 named volume `xingqiao-cpa-manager-data`
- 密钥目录：`/opt/cpa-manager-plus/secrets/`，目录与文件权限分别限制为 `0700` 和 `0600`

## 6. 制品来源与架构

主站为 `linux/amd64`。所有制品固定版本和 digest，不使用运行时漂移的 `latest`：

| 制品 | 固定来源 | AMD64 digest / checksum |
|---|---|---|
| CLIProxyAPI | `eceasy/cli-proxy-api:v8.0.20` | manifest `sha256:0c59d29e962089bec30e5cdf174ef29024693bb3af7fb2a7c111dfab664b84c5` |
| CPA Manager Plus | `seakee/cpa-manager-plus:v1.14.4` | manifest `sha256:37933b2b64dd60c7a1696096d738ad7cf9f0a7475fd7bc12c2eb7a0124b2f7f9` |
| Ticket Gateway | `cliproxy-ticket-gateway-1.4.0-linux-amd64.tar.gz` | archive SHA-256 `3d3c6126e965258a5a56c7d29bc28594785382f69eecaa17e83e0b639921a88c` |

插件包从文档指定的 `ticket.codingmiao.site` 下载，必须同时通过发布清单和包内校验。ARM64 本地插件不得上传主站。

## 7. 密钥边界

部署时只生成管理栈运行所需的密钥：

- CPA Management Key：登录 CPA 管理面和供 CPAMP 管理 CPA。
- CPAMP Admin Key：管理 CPAMP 自身数据与设置。
- CPAMP data key：加密 CPAMP 保存的 CPA 连接信息。

本轮不生成供 Sub2API 调用的 `sk-...` 业务 API Key。本地开发机现有密钥不复制到主站。所有密钥只保存在主站受保护目录或 CPAMP 数据卷中，不进入 Git、聊天、发布记录或普通日志。

## 8. 公网入口

### 8.1 DNS

在 Cloudflare `xingqiaolab.top` zone 新建：

- 类型：`A`
- 名称：`cpa`
- 内容：`64.83.10.67`
- Proxy status：Proxied
- TTL：Auto

当前权威 DNS 查询为 NXDOMAIN，因此不存在需要覆盖的同名记录。

### 8.2 Caddy

在仓库 `infra/Caddyfile` 和生产 `/opt/sub2api/production/Caddyfile` 增加独立站点块：

- Host：`cpa.xingqiaolab.top`
- 管理页面、插件资源和 CPA 管理 API 转发到 `cpa-gateway:8317`
- CPAMP 用量接口按路径转发到 `cpa-manager:18317`
- 公网 `/v1/*`、`/v1beta/*` 和其他模型推理入口显式拒绝
- 不改变 `api.xingqiaolab.top` 和 `codex.xingqiaolab.top` 的已有站点块

Caddyfile 必须先在候选文件上通过 `caddy validate`，然后原子替换并执行平滑 reload。若证书、路由或健康验证失败，立即恢复备份并 reload。

生产实时核对显示当前由 `sub2api-caddy-1` 直接发布宿主 80/443，未运行仓库中的 `nginx-tls-front`。本轮只适配这一实际活动入口，不重建边缘容器。仓库保留的 Nginx TLS front 目前只装载 `SITE_ADDRESS` 单域名证书；未来若恢复该拓扑，必须先为 `cpa.xingqiaolab.top` 增加独立证书和 SNI vhost，否则不得执行相应 Sub2API Compose 发布。

## 9. 部署顺序

1. 从已提交并推送的干净 `main` 准备 Compose、Caddy 和发布记录输入。
2. 在主站创建独立目录、权限、密钥和持久化路径。
3. 下载并核验镜像 digest 与 AMD64 插件归档。
4. 启动 `xingqiao-cpa`，等待 CPAMP 健康、CPA 管理页可用和插件 v1.4.0 注册。
5. 将两个新服务接入 `sub2api_default`，验证 Caddy 容器能通过 Docker DNS 访问它们。
6. 创建 Cloudflare DNS 记录。
7. 校验并 reload Caddy，验证新域名 TLS、管理页和管理鉴权。
8. 复核 Sub2API、Codex2API 的公网健康、容器 ID、启动时间和 restart count 未变化。
9. 生成一份简短发布记录，不写入任何密钥或生产账号数据。

## 10. 验证门槛

部署成功必须同时满足：

- `xingqiao-cpa` 两个容器运行，CPAMP 健康检查通过。
- CPA 日志确认 `cliproxy-ticket-gateway` v1.4.0 注册，状态接口可访问。
- CPA Management Key 和 CPAMP Admin Key 鉴权分别通过。
- `https://cpa.xingqiaolab.top/management.html` 返回 HTTP 200，页面资源完整。
- 公网模型推理入口被拒绝；Docker 内网 CPA `/v1/models` 在无 Key 时返回预期鉴权错误。
- `https://api.xingqiaolab.top/healthz`、`/readyz`、`/health` 保持成功。
- `https://codex.xingqiaolab.top` 的既有健康入口保持成功。
- Sub2API、Codex2API 及其数据库、Redis、worker、探测器的容器 ID、启动时间和 restart count 没有因本次部署变化。
- 测试站未访问、未同步，并在发布记录中明确注明。

不把“CPA 空号池”视为部署失败；本轮没有账号导入授权，也不进行真实模型消费测试。

## 11. 回滚

切流前失败：停止新 project，保留目录、密钥和卷，现有站点不受影响。

切流后失败：

1. 恢复生产 Caddyfile 备份并 reload。
2. 删除或关闭 Cloudflare `cpa` 记录的代理入口。
3. 停止 `xingqiao-cpa` 容器，但保留数据卷、插件包和配置。
4. 验证 Sub2API 与 Codex2API 入口和容器运行态。

不得使用 `docker compose down -v`，不得删除新栈数据卷，也不得重启无关服务。

## 12. 管理员创建 Sub2API 专用 Key

部署后由管理员执行：

1. 打开 `https://cpa.xingqiaolab.top/management.html`，使用 CPA Management Key 登录并选择“记住凭据”。
2. 在“OAuth 登录”中导入并确认至少一个可用的 Codex OAuth 账号。
3. 打开“配置”中的 API Keys 区域，新增一把随机、唯一、只供 Sub2API 使用的 `sk-...` Key。
4. 保存配置后，在另一个会话中使用该 Key 请求 CPA `/v1/models`，确认鉴权成功。
5. 在 Sub2API 新建 OpenAI API Key 类型上游时填写：
   - Base URL：`http://cpa-gateway:8317/v1`
   - API Key：刚创建的专用 Key
6. 先加入隔离测试分组并保持不可调度，完成模型列表和最小真实请求验证后再决定生产分组和调度状态。

CPA Management Key、CPAMP Admin Key 与业务 `sk-...` Key 互不通用。
