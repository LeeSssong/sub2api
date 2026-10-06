# 上下文价格对照测试站发布记录（2026-10-07）

目标：按用户授权部署独立测试站 `sub2api-test-station`（`43.133.75.82:22`），入口 `http://43.133.75.82/`。主站未访问、未同步。

改动：每个模型一行，短／长上下文左右对照，范围从原生阶梯数据生成；两层表头，中间贯穿 1px 分割线。每侧展示输入、缓存输入、缓存写入、输出；手机局部横向滚动并固定模型列。三档以上及全部上下文价格保留，继续复用现有分组、倍率、模型分类、排序和静默刷新。

缓存标注修订：通用缓存写入字段不能直接推断为五分钟。未提供一小时价格或其与通用价格相同时，只显示一个缓存写入单价；两档不同时保留两档。本次只修订展示，不修改原生价格、计费或接口。官方缓存说明：<https://developers.openai.com/api/docs/guides/prompt-caching>。

## 来源与制品

- 已合入并推送的功能分支：`codex/pricing-context-columns`，保留为历史证据。
- 最终发布源 main commit：`0e8664b1e4243ec0f2d744e0731ee6b056c5691c`。
- 发布源 tree：`c07ee2813d2b7005b8b1119cd96e9c70085ffaa0`。
- 构建发布时根目录 main 干净、非 detached，commit/tree 与已核实 origin/main 一致。
- 运行镜像：`sha256:1cf11255bf985dc4c5f1561f2c815fe8c86cfe10d8c0cd969e4eeca03f4fc158`。
- 内嵌前端的二进制 SHA256：`a8c1f15e21896cf33883b692dfc6ca5ba90f85a535f9ea39ac5767538006b1b3`。
- 发布目录：`/opt/sub2api-test-station/releases/0e8664b1e4243ec0f2d744e0731ee6b056c5691c`。

归档时 main 已含另一任务的后续提交 `e8e3f87ed51d950a390114415ab4535a5f5567af`；该后续代码不属于本轮运行制品。本轮没有替其他任务部署。初版布局制品 `df81a4a24b0aeead107b2280bfad806c64e158fa` 被上述最终版本替代，过程统一记入本记录。

## 验证与耗时

- 复用本轮直接相关验证：64 项测试通过（官方价格 14、弹窗 19、首页 31），类型检查、三文件 ESLint、diff 检查通过；机械检测无发现。
- 构建通过 locale 检查、类型构建和 Vite，前端构建 16.10 秒；宿主镜像准备与就绪 18.9 秒。
- 真实测试站弹窗：32 行对应 32 个模型；短上下文 ≤272K、长上下文 >272K；分割线 1px；GPT 表格没有五分钟／一小时文案。旗舰筛选得到 20 行。
- 手动刷新 pending 期间筛选、价格表与费用表保留。未据此声称验证了刷新成功后的倍率变化。
- 真实 375px 手机布局：页面宽 375px，表格局部 scrollWidth 1172px、clientWidth 263px；横向 scrollLeft 120px 时模型列与容器 left 均为 52px。
- 公网 `/health` 为 `ok`、`/readyz` 为 `ready`，API healthy。公网资源与已核实二进制内嵌资源 SHA256 一致：入口 `index-D0Bwp7V_.js` 为 `8dc618cc3a06a68c293063e8fa954900796a409c466390ca16e095317e8891da`，价格包 `DashboardView-DlQjKNLk.js` 为 `1cd9c5eead2d61561c7e91ea3b72fc7895cc0f7910826576b6c0e4dbf632ef3a`。

结果：API blue → green 平滑切流，公网专项验证成功；旧 API 排空 0.45 秒，残留连接 0 后停止，未回滚。worker、detector、数据库、Redis 保持原实例，Caddy 仅平滑 reload。没有数据库变更。

本地证据：`.release/pricing-context-columns-preserved/` 下的 `deployment.json`、`runtime.json`、`public-assets.json`、`ui-live.json`、`mobile-live.json`、`desktop-live.jpg`、`mobile-live.jpg`、`deploy.log`。临时浏览器、viewport、本地 mock 和依赖软链接已清理。用户原有两份规则修改单独保存并恢复，stash 备份保留。

## 回滚入口

旧镜像、配置和制品保留；本轮未执行回滚。服务器命令：

```sh
sudo -n python3 /var/tmp/sub2api-test-station-api.TjPdZn/deploy.py rollback /opt/sub2api-test-station/releases/0e8664b1e4243ec0f2d744e0731ee6b056c5691c
```

未解决问题：本轮专项验收未发现；主站状态及其他任务后续部署不属于本记录的验证范围。
