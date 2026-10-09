# CPAMP 账号预计额度自动刷新

基于 CPA-Manager-Plus v1.14.4 的前端，保留既有插件和管理 API。账号页立即异步查询当前页 Codex 凭证，之后每五分钟更新一次；页面隐藏、断开或离开后停止。单次管理 HTTP 查询超时 15 秒，同账号请求合并，后台刷新串行执行。失败保留旧额度并显示更新时间/失败提示。自动查询只更新显示，不调用凭证状态失效或变更操作。

`upstream.patch` 包含源码及直接相关测试，`source.json` 保存上游下载地址、源码/补丁/构建页面 SHA256。`management.html` 是一次构建的完整单文件产物，供边缘代理通过 `/management.html` 和 `/index.html` 读取。

复现：下载 `source.json` 中的归档并校验 SHA256，解压后执行 `git apply upstream.patch`，然后 `npm ci --ignore-scripts --no-audit --no-fund` 和 `VERSION=v1.14.4-xingqiao-quota npm run build`。验证命令：`npm --workspace apps/web exec -- vitest run src/features/accounts/model/quotaAutoRefresh.test.ts src/features/accounts/hooks/useAccountQuotaAutoRefresh.test.tsx src/components/quota/quotaRefresh.test.ts`；`npm run type-check`。

部署使用根目录干净且已推送的 main。备份边缘 Caddyfile 与旧管理页面，将管理页面原子安装到 `/opt/cpa-test-station/edge/management.html`，验证并平滑 reload Caddyfile 后核对公网文件 SHA256及账号页自动查询结果。不重启 CPA/CPAMP，不发送模型业务请求。失败恢复旧页面和 Caddyfile并平滑 reload。
