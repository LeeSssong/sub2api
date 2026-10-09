# CPA 发布回滚记录

- 目标：64.83.10.67；本次 CPA 发布已回滚，未交付可用状态。
- 来源 main commit：515410e3184bad388a81988f275f638b7afb4a68；tree：5fc38c3060ba354c69d08ded1ad8e25cb11d03d6。
- CPA 镜像 digest：sha256:0c59d29e962089bec30e5cdf174ef29024693bb3af7fb2a7c111dfab664b84c5；Manager：sha256:37933b2b64dd60c7a1696096d738ad7cf9f0a7475fd7bc12c2eb7a0124b2f7f9。
- 故障：Caddy reload 继承容器残留的 SUB2API_ACTIVE_UPSTREAM=sub2api-green:8080，实际服务为 blue，产生 502。只还原 Caddyfile 不足以恢复。
- 恢复：使用发布前备份，显式传入 SUB2API_ACTIVE_UPSTREAM=sub2api-blue:8080 平滑 reload；还原宿主 Caddyfile；停止新增 CPA 两个容器，数据保留。原有应用及 Caddy 未重启。
- 时间：北京时间 22:38 首次回退；22:39 恢复源站 200；22:40 完成公网验证和停止 CPA。恢复阶段约 2 分钟；此前发布各阶段耗时未完整记录。
- 验证：公网 Sub2API 首页、health 均 200；源站 readyz 200；公网 Codex2API health 200。原有三个容器 ID、启动时间不变，重启计数为 0。未执行付费模型请求。
- 回滚入口：/opt/cpa-manager-plus/backups/caddy-20261008-142859/Caddyfile；还原文件 SHA256：5371f1c9be0bace617fd65db9ebfeb3d13fad043df73eef5937ed0c6065f3ebd。
- 未解决：CPA 公网发布停止；DNS 记录保留。Caddy 单文件 bind mount 与宿主路径存在 inode/内容差异，后续发布必须核对，不能直接复用容器旧环境 reload。release.env 当前已记录 blue。业务数据和凭据未修改。测试站未查询、未同步。
