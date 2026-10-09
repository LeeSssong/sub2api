# 2026-10-07 测试站云猫店铺高度调整

- 目标：独立测试站 `43.133.75.82/redeem`；主站未查询、未发布。
- 改动：`RechargeStorefront.vue` 的 iframe 高度由 `clamp(540px, 68vh, 760px)` 改为 `max(1100px, 100vh)`，保留左侧兑换及最近活动布局。
- 分支 `codex/storefront-full-height` 已合入 main，作为历史证据保留；布局提交 `e8e3f87e`。
- 发布 main commit：`a0a7318eda378a317d595bea5d6d65eb6c588746`，tree：`bf559af2ed1a359b25fcf7befb7a8ece6d991c78`。发布时根目录 main 干净、非 detached HEAD，commit/tree 与获取的 origin/main 一致。
- 二进制来源：`e8e3f87ed51d950a390114415ab4535a5f5567af`，tree：`1c3d37bd8d47c223a702084c15647bf60bad40d6`；SHA-256：`8e6a172749adfcb69abcc99e342c05617006f09c20ad9ad6447475d553f5aba3`。
- 镜像 ID/digest：`sha256:b2bdf7ab2145201c48fc63a6c944b0b423d288de951ed183a8083600f06fe8f7`。
- 验证：兑换及充值导航 20 项测试通过；构建的 i18n 3 项检查、类型检查及 Vite 构建通过。部署后 health=ok、readyz=ready；1512px 桌面 iframe 高 1100px，最后一张商品卡底部 755.78px，完整位于 iframe 内；390px 手机 iframe 高 1100px，无页面横向溢出，兑换及最近活动同宽 278px。
- 复用：初次部署在预检发现其他发布改变活动版本，未写入线上；保留原制品。确认新活动版本已发布完成、后端运行输入未变、Docker 基础层全部保留、二进制 SHA 和来源 commit/tree 相符后，生成独立复用资格证据并复用同一二进制。原始 manifest 未覆盖；证据位于忽略目录 `.release/test-station-api-e8e3f87e-validated-reuse/reuse-validation.json`。
- 耗时：Vite 构建 16.04 秒；宿主镜像构建及就绪 23.22 秒；旧实例排空 0.54 秒，剩余连接 0；并发发布等待及其余阶段未单独计时。
- 结果：API 蓝绿发布成功，未回滚；worker、探测器、数据库、Redis 未重建。原有 AGENTS.md 与环境规则文档改动保留并在发布后恢复。
- 回滚：发布包 deploy.py 的 rollback 子命令，目标 `/opt/sub2api-test-station/releases/a0a7318eda378a317d595bea5d6d65eb6c588746`；发布包位置见本地 `.release/test-station-api-a0a7318eda378a317d595bea5d6d65eb6c588746/remote-staging-path`。旧 `0e8664b1` release、镜像及 previous-state/previous-Caddyfile 保留。
- 限制：当前商品卡完整展示；云猫页面会按 iframe 高度增加自己的页面及页脚空间，仍有约 90px 的页脚滚动。跨域页面未提供自动高度协议，不能保证今后新增内容自动完全展开；支付及真实兑换未执行。
