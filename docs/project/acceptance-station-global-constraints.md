# 验收站与主站发布全局约束

> 生效日期：2026-09-19（发布 SOP 修订）
>
> 适用范围：根线程、所有功能线程、审查线程、运维线程和发布线程。

> **重要纠偏：** 本文件中的“验收站”现在专指新建的独立测试站，不是主站上的历史 `/admin/lab` 路径。旧 `/admin/lab`、`sub2api-acceptance`、`/opt/sub2api/acceptance-live` 和旧验收 env 只保留为历史证据，不得再作为本轮验收入口、宿主或发布目标。

## 当前环境身份

以下身份按用户 2026-10-07 指令更新；本次仅更新规则并核对本地 SSH 路由，不构成重新部署或服务器运行态验证。

- 当前主站/生产服务器：`64.83.10.67`，SSH alias `sub2api-prod`，连接身份 `root@64.83.10.67:22`。
- 独立测试站：`43.133.75.82`，SSH alias `sub2api-test-station`，连接身份 `ubuntu@43.133.75.82:22`。
- `43.133.75.82` 的旧主站／备用服务器身份及 `49.51.203.200` 的旧测试站身份仅属于历史记录，不得作为当前环境身份或操作目标。
- 无额外限定的“服务器”“主站”“生产服务器”均指当前主站 `64.83.10.67`。历史记录中的旧 IP 是当时事实，不据此改变当前操作目标。
- 当前生产域名为 `api.xingqiaolab.top` 和 `codex.xingqiaolab.top`；两者均应解析到当前主站，实际代理模式以 Cloudflare DNS 实时状态为准。

## 1. 验收站固定身份

验收站是一个从 0 开始、功能完整、可以真实商用的独立 Sub2API 实例。它与主站互不可见，所有数据库、Redis、对象存储、账户、会话、账单、支付订单、上游凭据、通知凭据、Docker project/network、运行目录和数据卷均独立。

固定入口与运行身份：

- 公网入口：`http://43.133.75.82/`（当前登记 IPv4 入口；IPv6 状态以宿主实时核验为准）
- 健康入口：`http://43.133.75.82/health`；就绪入口：`http://43.133.75.82/readyz`
- 根路径：独立站根路径 `/`，不经过主站域名、主站 Caddy 或 `/admin/lab` 路径
- 验收 API/登录入口：由独立站自身根路径提供，必须以该站页面和 API 实际响应为准；不得拼接旧 `/admin/lab/api/v1` 前缀
- 主站管理员页面：`https://api.xingqiaolab.top/admin/accounts`；该路径继续走主站，不属于验收站
- 宿主 SSH alias：`sub2api-test-station`（`ubuntu@43.133.75.82:22`）
- 2026-10-09 用户再次确认 `43.133.75.82` 为测试站。本机旧 alias 已纠正；执行前用 `ssh -G sub2api-test-station` 核对，不得按旧 SSH 路由访问 `49.51.203.200`。
- 验收宿主目录：`/opt/sub2api-test-station/`
- 当前活动 release：由宿主 `/opt/sub2api-test-station/release-state.json` 的 `source_commit/source_tree`、发布记录与运行容器 Compose 标签实时解析；不得在规则文档中固定可能过时的 release SHA
- Compose 文件：`<active-release>/infra/independent-test-station/compose.yaml`
- Compose project：`sub2api-test-station`
- Compose network：`sub2api-test-station-network`
- 独立 named volumes：`sub2api-test-station-app-data`、`sub2api-test-station-postgres-data`、`sub2api-test-station-redis-data`
- 验收服务：`test-station-api`、`test-station-worker`、`test-station-detector`、`test-station-postgres`、`test-station-redis`、`test-station-caddy`
- 运行 env：服务器 `/opt/sub2api-test-station/.env`；Compose 发布/核对使用 active release 内的 `.env`

注册默认关闭。验收站只允许独立管理员登录；“测试站可商用”不等于向公网开放注册。

## 2. 凭据与本地读取规则

任何线程需要登录、查看日志、执行验收发布或宿主运维时，使用以下受保护文件；不得把其中的密码、token、私钥、API key、支付密钥、上游 key 或 webhook 写入 Git、规格书、聊天消息、发布证据或普通日志：

- 测试站 SSH 私钥：`/Users/gongtengxinwen/.ssh/tencent_lighthouse_seoul_sub2api`，权限必须为 `0600`
- 测试站 SSH known_hosts：`/Users/gongtengxinwen/.config/sub2api/known_hosts`，权限必须为 `0600`，且必须包含 `43.133.75.82` 的可信 host key
- 测试站运行 env：服务器 `/opt/sub2api-test-station/.env`，权限必须为 `0600`
- 旧验收 env：`/Users/gongtengxinwen/.config/sub2api/acceptance-20260827.env`，仅历史参考，不得用于新独立测试站

线程可以读取非敏感配置名和值（站点、目录、project、network、端口、provider 类型），但不得用 `cat`、`env`、`docker inspect` 或日志命令打印完整 env。需要展示时只展示变量名、是否已设置、文件权限和脱敏摘要。

当前验收管理员账号由上述 env 的 `ACCEPTANCE_ADMIN_EMAIL` 和 `ACCEPTANCE_ADMIN_PASSWORD` 提供；不得在仓库中复制密码。密码轮换后只更新受保护 env，并重新执行登录探针。

## 3. 验收站日常查看与运维

所有线程先确认当前工作区和目标 commit，再执行只读检查。常用命令如下（命令中的 env 文件只能作为 `--env-file` 传给 Compose，不要把内容打印出来）：

```bash
acceptance_ssh='ssh -T -o BatchMode=yes -o StrictHostKeyChecking=yes sub2api-test-station'

# 服务状态
$acceptance_ssh 'sudo -n sh -c '\''config=$(docker ps --filter label=com.docker.compose.project=sub2api-test-station \
  --format "{{.Label \\"com.docker.compose.project.config_files\\"}}" | head -n 1); \
  release=${config%/infra/independent-test-station/compose.yaml}; \
  docker compose --project-name sub2api-test-station \
  --env-file "$release/.env" -f "$config" ps'\''

# 查看单个服务日志（只保留必要窗口，先脱敏再归档）
$acceptance_ssh 'sudo -n sh -c '\''config=$(docker ps --filter label=com.docker.compose.project=sub2api-test-station \
  --format "{{.Label \\"com.docker.compose.project.config_files\\"}}" | head -n 1); \
  release=${config%/infra/independent-test-station/compose.yaml}; \
  docker compose --project-name sub2api-test-station \
  --env-file "$release/.env" -f "$config" logs --tail=200 test-station-api'\''

# 验收入口与健康检查
curl --fail --silent --show-error http://43.133.75.82/health
curl --fail --silent --show-error http://43.133.75.82/readyz
```

允许查看独立测试站容器、Caddy、PostgreSQL、Redis 的运行状态和日志；涉及数据库时只做只读查询。禁止执行 `docker compose down -v`、删除 `sub2api-test-station-*` volume、复制主站数据或用主站 env/旧验收 env 覆盖测试站 env。

## 4. 独立测试站发布入口

独立测试站发布与主站共用同一来源底线：候选先合入并推送根目录 `main`，然后只能从该根目录干净的 `main` 执行发布。发布时必须同时满足：当前分支为 `main`、非 detached HEAD、工作树干净、`HEAD` commit/tree 与本地 `origin/main` 完全一致。

当前仓库的 `ops/release-sub2api-acceptance.sh`、`ops/deploy-sub2api-acceptance-host.sh` 和 `infra/compose.acceptance.yaml` 仍是旧 `/admin/lab` 拓扑的历史发布链，不能发布到 `sub2api-test-station`。

独立站当前 API 发布入口为 `ops/release-sub2api-test-station-api.sh`，宿主执行器为 `ops/deploy-sub2api-test-station-api.py`。它们校验根目录干净 `main == origin/main`、当前 SSH 身份、活动 release、依赖镜像和全部迁移校验和；复用活动配置，仅操作独立站 project。普通发布使用 API 蓝绿，并可串行更新 singleton worker；有待执行的迁移默认拒绝。

2026-10-09 为请求质量快照新增的停机路径只允许 `266_quality_traffic_snapshots.sql`：设置 `TEST_STATION_APPROVED_HOST=43.133.75.82`、`TEST_STATION_UPDATE_WORKER=true` 和 `TEST_STATION_MAINTENANCE_MIGRATION=266_quality_traffic_snapshots.sql`。先构建制品、验证数据库备份的完整恢复，再阻断 API 入口、停止 API/worker、备份停止写入后的数据库，调用原生 `--migrate-only`，更新单套 API/worker，检查就绪后开放流量。临时恢复验证库不接业务流量，验证后删除。首页、detector、PostgreSQL、Redis 不重建。迁移或 API 启动失败且新 worker 尚未启动时恢复数据库备份；worker 启动或开放流量后的失败仅恢复兼容的旧应用，保留新增结构和数据，避免丢弃新写入。新增其他迁移须先适配并验证，不能借此放宽门禁。直接相关发布测试已覆盖门禁、停机顺序、迁移失败恢复及开放流量后保留数据；线上结果由对应发布记录单独确认。

新站当前已部署的镜像/数据属于独立服务器上的既有运行态，不构成当前代码发布控制器已适配的证据。不得把旧 `/admin/lab` 健康结果、主站 Caddy 路由或旧验收脚本输出写成新独立站发布成功。

独立测试站发布成功只表示服务部署和基础健康检查完成，不代表真实充值、消费、支付、上游、通知或目标功能已经验收通过。真实功能必须由管理员在独立数据和独立凭据上人工验证。

旧 `/admin/lab` 历史入口不得作为新站别名；任何仍依赖它的任务必须先改写为新站根入口并重新记录目标 commit/tree。

## 5. 主站发布 SOP 与授权

以根目录 `AGENTS.md` 的“发布 SOP（2026-09-19 生效）”为唯一发布策略入口。本节替代旧 A/B/C/D 四条授权路径，不再要求固定口令、测试站前置验收或发布后强制同步。

- 默认本地开发和直接相关验收通过后发布主站。“部署主站”“部署生产”及目标、范围明确的等价指令授权本次正常发布链；已有授权无需重复确认。
- 正常授权包含平滑 reload、切流短暂波动、旧实例最长 300 秒排空及到期终止残留连接。提前排空则提前停止，旧版本制品及恢复入口至少保留至下一次发布成功。
- 优先直接更新实际生效的运行文件；无法直接更新则蓝绿发布。直接更新须有来源、生效方式、备份、原子替换、验证与恢复；配置允许经验证的平滑热加载，敏感值仅保存在受保护位置。不能编译生效的源码不得直接覆盖冒充发布。
- 数据库结构或数据迁移默认停机、单套发布；只有用户针对当次明确要求时才设计其他方式，例外不得持续生效。已明确包含数据库变更的本次发布授权涵盖该停机路径，执行前告知预计中断、迁移范围和恢复方案。超出授权范围的新发现先说明并取得授权。
- 停机前准备好制品和可用恢复方案；停止业务写入与任务，完成对应备份，再迁移、启动和验证，成功后开放流量。应用回滚不能代替数据库恢复；恢复流量后不得盲目还原备份丢弃新数据。
- 普通蓝绿发布保留旧服务直到新实例就绪，先内部冒烟再切流、公网验证、排空；失败恢复旧路由。无关服务不重建，worker 更新须保证任务不重复、不丢失。
- 复用有效测试和可追溯制品，不因推送/部署重复测试、构建或验收。同一发布使用同一 digest；来源仍须满足根目录干净且已推送的 main 约束。新增冲突或变化只补相关验证。
- 脚本未支持 SOP 的能力，须先适配并验证；不得通过跳过门禁或伪造证据来加速。原蓝绿脚本维护分支的停 API/worker 行为只可能用于符合上述规则的数据库停机发布，不能用于普通应用发布。

## 6. 测试站按需使用

- 测试站不作为主站发布的强制中转，不强制同步；不同步或同步失败不自动回滚主站，也不阻塞下一次主站发布。
- 用户明确要求测试站验收或双站同步时，按当次要求执行。已有同一合规制品可复用，不重复构建。
- 只根据实际证据报告两站版本；测试站未查询标注“未查询”，未同步标注“未同步”，不得声称一致。不为填报版本额外访问测试站。
- 运行文件直接更新可能造成同一 commit 下的运行内容差异，记录应用过的文件版本/校验和，不仅凭 commit 判断一致。
- 同步只推广代码和合规制品，绝不复制主站业务数据、凭据或覆盖独立卷。

## 7. 最小发布检查与记录

发布前一次性核对目标与授权、代码/制品来源、有效测试证据、环境兼容性、采用的发布路径和恢复入口。发布后完成版本、就绪及本次改动的最小线上验证。

只保留一份简短记录：目标、commit/tree、制品 digest 或文件校验和、验证、阶段耗时、结果、回滚入口/是否回滚和未解决问题。记录由脚本尽量自动生成；不要求多份重复报告，不临时扩大测试。异常才针对具体原因进一步诊断。

## 8. 脚本适配状态

本次修订是规则更新，不代表部署脚本已完成对应改造或线上验证。待核验/适配项包括：已有制品复用、无关 worker/探测器不重建、300 秒连接排空与超时处理、直接更新/热加载路径，以及数据库单套停机发布的备份和恢复流程。发布前仅核验本次所需能力；不足时先适配并做直接相关验证，不得宣称已有完整支持。
