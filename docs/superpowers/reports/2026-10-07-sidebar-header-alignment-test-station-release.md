# 2026-10-07 侧栏品牌区与顶栏对齐：测试站发布

- 目标：独立测试站 `43.133.75.82`，SSH `sub2api-test-station`。用户授权本版部署测试站并覆盖对应 UI 规则；主站未访问、未发布。
- 改动：侧栏品牌区从壳层顶边开始，与顶栏共用 `--xq-header-height`，桌面 64px、窄屏 56px；取消叠加的侧栏顶部留白和品牌区纵向内边距，保留水平及底部留白。覆盖后台手机展开菜单。`upstream/sub2api/frontend/DESIGN.md` 为本项目唯一对齐规则来源，明确分隔线实际测量差值不超过 1px。
- 发布链适配：只允许随前端源码发布该 `DESIGN.md`，不扩大依赖、构建配置、迁移或其他后端的允许范围。新增门禁测试先复现拒绝，再修正通过。
- 来源：`codex/sidebar-header-alignment` 已合入并推送根目录 main。发布源 commit `31ea61de6dad440929866e42a566b6fe8ed1582f`，tree `bddf861f763bb8f173e8983eb8bf68845ed60680`。构建时根 main 干净、非 detached，与获取后的 origin/main commit/tree 一致。本记录仅归档，不再次构建；该功能分支/worktree保留为已合入的历史证据。
- 制品：镜像 ID/digest `sha256:5a8e7049a0beafbd30a354f62dc2648516621c889923b5767997f7e46624f932`；内嵌二进制 SHA256 `a35cec928711fb16bf189ec64746e6542ef1242d717010738f929e06c2822ec8`。同一合规制品 API 蓝绿发布；无数据库迁移，worker、detector、数据库、Redis 保留原实例，Caddy仅平滑 reload。
- 验证：复用15项侧栏/顶栏测试及本地用户/管理员、版本号有/无、桌面/中屏/手机的浏览器测量，分隔线差值均0px。新增21项发布链测试、shell语法与Git差异检查通过。发布构建中3项locale测试、Vue类型检查、前端构建及Go内嵌服务器构建通过。
- 公网：health=ok、readyz=ready。入口 `/assets/index-IVOEFQXl.js` SHA256 `65c32cbbc4dbc57f0ec98c5c731c75c3ade1d0e96bbabba40701d2133326bfb5`，实际侧栏样式 `/assets/AppLayout-B-gfcH7p.css` SHA256 `d1a51ba9e5bf2060cc40e774df1b2a685b85d199dc25c0852b834d29698910b1`，均与本轮构建一致。
- 真实页面：复用Chrome已有登录会话刷新 `/usage`，1512px、900px、390px以及手机后台菜单展开时实测分隔线差值均0px；桌面两边bottom=64px、手机两边bottom=56px。验证结束恢复窗口尺寸和菜单状态，截图仅截顶部，避免账号和业务数据。此时另一任务已发布后续 commit `c004475dcf5a46557bc2ffd388bfb07adc94be01`；该提交包含本次修正，侧栏代码及对齐规则与本版完全一致，页面入口为 `/assets/index-p-XxAkXx.js`。未把后续任务制品混称为本次制品。
- 耗时：Vite构建15.79秒；宿主镜像准备及新API就绪19.60秒；旧实例最终排空0.34秒，残留连接0，上限300秒。其余阶段未独立计时。
- 结果：本次发布成功、未回滚，旧版本镜像与配置保留。两份既有环境规则修改单独暂存，归档后原样恢复并核对文件及patch校验和；stash恢复点保留。
- 回滚入口：服务器 `sudo -n python3 /var/tmp/sub2api-test-station-api.Iyy6WF/deploy.py rollback /opt/sub2api-test-station/releases/31ea61de6dad440929866e42a566b6fe8ed1582f`。当前已有后续发布，执行历史回滚前须先核对当前release及后续改动，不能直接覆盖后续任务。
- 未解决项：受保护运行env中的管理员凭据在旧版及本版均返回401；未修改凭据或账号。真实页面验收已通过现有浏览器会话完成，不再受此问题阻塞。后续需由凭据维护者核对env与实际账号的一致性。
- 本地证据：`.release/sidebar-header-alignment-preserved/` 下的deploy.log、deployment.json、public-assets.json、ui-live.json与local-browser-verification.json；早期登录探针失败日志不作为验收通过证据。
