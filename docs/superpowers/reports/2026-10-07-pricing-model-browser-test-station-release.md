# 原生价格表分类、排序与稳定刷新：测试站发布

- 目标：独立测试站 `43.133.75.82`，沿用本任务测试站部署授权；主站未访问、未同步。
- 改动：上方OpenAI价格表按核实的官方发布时间倒序，未知时间置后，仅接受明确模型ID与已核实快照，不猜私有别名日期；加入旗舰、网络安全（Cyber）、图像生成、实时/音频生成、转录、专用及其他类型筛选和数量。类型筛选不影响分组授权列表。分类/时间仅为展示元数据，价格、模型名单、分组和倍率继续读原生接口，未引入静态价格、账务源或新增模型。
- 上下文：每模型一行、名字显示一次；移除独立上下文列，模型名下保留原生阈值，各单价分短/长；三档以上逐档保留，包括缓存写入5分钟/1小时。未知价格仍显示“—”。
- 刷新根因与修复：原60秒轮询和每次恢复visibility均设置loading，v-if将已有表格卸载，形成闪动。现在后续请求保留表格、筛选和滚动容器，仅刷新按钮表示请求状态；短暂窗口切换不重取，超过60秒或错误才同步。失败隐藏过时费用，重试pending也不恢复旧价格，关闭取消和竞态保护保留。
- 来源：`codex/pricing-model-browser`快进合入并推送根目录main；发布源commit `c64f49604a19c9e0fc3bdb97c68de5e077d55f7b`，tree `3453beed0b5911395a5cfd142ac1a0627ae0bb0f`。构建前后main干净、非detached，commit/tree与获取后的origin/main一致。分支/worktree保留为已合入历史证据，本记录仅文档归档，不重新构建。
- 制品：binary SHA256 `716a410c6b39e8b02f603aa0cc35bf0a4d49744d31075c26429cd2c4eaea9fd6`，镜像ID `sha256:5ae4258a36a566e5dc7f10b5ee88e98bda7a1c8d7a4ef0d0ab866324f7998088`。新API `0b987e3bf332`，发布目录 `/opt/sub2api-test-station/releases/c64f49604a19c9e0fc3bdb97c68de5e077d55f7b`；容器commit、binary和镜像均与清单一致。
- 路径：纯前端源码经内嵌构建，API蓝绿blue→green，Caddy平滑reload；无依赖、数据库或worker变更。worker `a98548b4174b`、探测器 `595856eabbd1`、DB `3cb0cd97a12d`、Redis `17fab82c3e5e`、Caddy `d339db76e851` 保持原实例。
- 验证：60项相关前端测试、typecheck、6文件ESLint、19项发布控制测试、shell语法/diff检查通过；排序、单行多档、稳定DOM、窗口切换、失败重试隐藏旧费分别验证。一次只读审查发现两个边界，均通过新增失败测试复现后修复。发布构建通过3项locale检查及类型构建。桌面和375px真实组件mock验收显示过滤后3行，刷新pending表格/分组表仍存在，选择保持旗舰，成功后倍率从0.4同步为0；手机document宽375px、筛选宽260px、表格局部scrollWidth1000px，刷新真实SVG可见。截图明确为本地模拟，不是线上账务数据。
- 公网验证：`/health`为ok、`/readyz`为ready，API healthy。入口 `/assets/index-DP9Y9gKg.js` SHA256 `7c02fe829f36883106c7b9971628e8b60cf65a430c7ff95c10bece69c56d5c4f`，价格包 `/assets/DashboardView-IAnRSKNz.js` SHA256 `ebd07ee6d12b1ee5978efb797ae510947c9c52e7adcc73a73c3f84d8e183b0d0`，均与本地制品一致且包含本轮筛选/排序文案。
- 限制：受保护env现有管理员凭据再次返回401 INVALID_CREDENTIALS，因此登录后的真实筛选、原生接口数据与账务验收未完成；未改密码、绕过认证或创建临时key。官方分类/发布元数据按2026-10-07核实，未知模型仍完整保留且发布时间后置，需要新增官方型号时补充元数据；金额始终原生动态读取。
- 耗时：前端构建16.25秒，宿主镜像准备与新API就绪19.63秒；编译和传输未独立计时，不推算。连接排空77.29秒，残留0，上限从切流起300秒。
- 结果：部署成功，未回滚；旧API已排空停止，旧镜像/Compose/env/路由备份保留。
- 回滚：`sudo -n python3 /var/tmp/sub2api-test-station-api.8YMkh9/deploy.py rollback /opt/sub2api-test-station/releases/c64f49604a19c9e0fc3bdb97c68de5e077d55f7b`，旧版本 `28219b06140c7dc3fa56c7d16db3306575f37561`、旧API `95b63ca84df6`，不覆盖数据库。
- 证据：`.release/pricing-model-browser-preserved/`内deploy.log、public-assets.json和本地mock截图，staging manifest及宿主deployment.json。根目录用户规则改动stash `8abab1774546ce103685e147cf98dc5e592fd080`，归档后原样恢复并对比patch，保留备份。
- 官方依据：[Pricing](https://developers.openai.com/api/docs/pricing)、[Changelog](https://developers.openai.com/api/docs/changelog)及对应型号快照页面。敏感凭据/token/env全文未进入Git或普通日志。
