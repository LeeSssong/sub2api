# 线路详情布局：main 合并与独立测试站发布

目标：用户明确授权合并 main 并部署测试站。实际目标为 `ubuntu@43.133.75.82:22`（`sub2api-test-station`），公网 `http://43.133.75.82/`，仅操作 project `sub2api-test-station`。未访问或部署主站。

## 来源与制品

- 功能提交 `ae248476`（`codex/route-card-layout`）已合入根目录 main；分支/worktree 保留为历史证据。本次包含 main 已有的关联密钥与充值兑换界面。
- 正式部署来源为干净、非 detached 且已推送的 main：commit `85bc13514a402cb2978e64fa84303b67fe6cbf45`，tree `a7011044858c5cfe9adbd4537cfaa9da98aebbf0`，构建／部署前与获取的 origin/main 完全一致。后续本报告归档提交只增加文档，不改变运行制品。
- 镜像 `sub2api-test-station-runtime:85bc13514a402cb2978e64fa84303b67fe6cbf45`，Image ID `sha256:f188521b904117b2c9ddafa79baf1e424b4f208946e62cf88ce786b9fc60dc76`。
- Binary SHA256 `08a2b3cdec0ef19ee63564853b5548b53b4dfae37a42e5b821be5f3508d960c2`；上传包 SHA256 `6f3a022e59f67f0b31cdc8988dfa38ba42c7f456de9e7ee7df0c541f2b77e3e4`。
- 编译输入来自先前干净且已推送的 main `eb023b0d72a505d20b147cd377ba88f30b9b6660` / tree `34d297516863c89519a8fabfd7b748e7b9c7adbb`。重试只修正发布控制器，整个 `upstream/sub2api` 输入与此版本无差异；脚本核对 Git tree、二进制 checksum、依赖底座 ID 后复用既有构建，未重复编译。服务器内二进制 checksum 与输入一致，嵌入的编译 commit 仍为 eb023b0d，发布配置/source_commit 为 85bc1351。

## 改动、验证与耗时

- 恢复指标选择框；请求计算默认隐藏，成功率区域悬停／聚焦／点击显示，Escape 或离开隐藏；最佳线路为卡片内部左上角斜丝带，名称保持对齐；关联密钥按钮位于右侧并增强对比。
- 合并后 37 项直接相关前端测试通过；完整前端 build（locale 3 项、vue-tsc、Vite）通过。发布控制器 8 项测试通过，包括复现后修复的捕获目录隔离测试；shell 语法和 git diff --check 通过。
- 北京时间 2026-10-07 02:20 开始构建。locale 测试阶段 3.61s，Vite 构建 16.60s。首次候选在 02:21:52 启动失败：继承 standalone 请求捕获槽位，与旧 API 争抢目录锁；未成功切流，执行器回滚并确认旧版 health/readyz 正常。
- 控制器修正为独立 `SUB2API_CONTAINER_SLOT`，采用现有后端槽位能力，不修改后端、不停旧服务。02:25:54 完成切流；新镜像准备与就绪阶段 9.22s。
- 公网 health/readyz 返回 200 和正确 JSON；两个 Dashboard JS 与全局 CSS 均与本地 SHA256 一致。受保护凭据登录后，真实页面显示 4 张 Codex 卡片，角标完全位于卡片内、首行名称高度一致、16 个选择框可见；曲线开关 false/true 和 Space 恢复均有效；请求样本默认 none，点击可见、Escape 隐藏。
- 关联按钮打开 GPT-Pro20x 的创建入口，取消返回；未创建密钥。375px 视口 scrollWidth=375，弹窗内容 clientWidth=scrollWidth=333，角标不越界。截图保留于本地 `.release/route-card-layout-test-station-desktop.jpg` 与 `...-mobile.jpg`，不含密码/token。
- 页面验证后旧实例已无建立的请求连接，finalize 检查／停止耗时 0.47s，残留连接 0（上限 300s）。最终 API healthy；worker、detector、PostgreSQL、Redis、Caddy 的容器 ID 与发布前一致，均继续运行。未执行数据库迁移或重建无关服务。

## 结果与回滚

最终 `deployment.json` 为 `result=succeeded, rolled_back=false`。首次失败记录保留于 eb023b0d release，最终成功记录位于 `/opt/sub2api-test-station/releases/85bc13514a402cb2978e64fa84303b67fe6cbf45/`。

旧 API 容器 `f9de9f43fc59` 已停止但保留，旧镜像 `sha256:27a0d9546954b46e0f4ebbbb9ce0924bebe77b1b82f282967217775b1b560232` 与旧 release `42788429bd86f5a701aa35c9e143cd62dbd926cd` 均保留。回滚入口：测试站宿主 `sudo -n python3 /var/tmp/sub2api-test-station-api.0d7g9e/deploy.py rollback /opt/sub2api-test-station/releases/85bc13514a402cb2978e64fa84303b67fe6cbf45`（自动启动旧 API、恢复路由与状态；不涉及数据库恢复）。

发布前既有 AGENTS.md 与环境约束文件的未提交修改已单独暂存，归档完成后原样恢复；未混入功能提交。未验证真实支付、真实上游或密钥创建；本次只验收布局及入口交互。没有待解决发布故障。
