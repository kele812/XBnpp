# 来源与修改

项目显示名称：xbn plus ++；独立版本：1.0.0。

基于 [xboardnext999/XboardNode-Plus](https://github.com/xboardnext999/XboardNode-Plus)，固定上游提交 `f2aca7940600207cc75ab4fe0c91003b6bd6dfeb`。该项目源于 [cedar2025/Xboard-Node](https://github.com/cedar2025/Xboard-Node)，上游 README 声明 MPL-2.0；本项目保留此许可证和来源说明。

新增 `internal/watchaccess`：只采集风控后台选定的用户，使用独立签名密钥直接向 SubscriptionWatch 上报连接目标；队列、重试和策略有效期均有限制。接入 sing-box 与 Xray 的连接观察点；开启该通道后，不重复向 Xboard 上传原来的访问诊断信息。用户同步、计费和在线状态仍沿用上游实现。

新增 `watch_access` 配置以及 Docker 的 `WATCH_ACCESS_URL`、`WATCH_ACCESS_NODE`、`WATCH_ACCESS_SECRET` 环境变量。多面板实例必须逐实例配置采集密钥，避免复用凭据导致归属错误。

安装器、xbctl 更新地址、Docker 镜像和发布工作流改为本仓库。为方便现有部署迁移，保留程序名 `xboard-node`、服务名 `xboard-node.service` 和 `/etc/XboardNode-Plus` 配置目录；它是原节点的替换版本，不应与同一节点的旧程序并行运行。

本项目不是上游官方发布，也不保证与未来上游版本自动兼容。后续合并需重新测试。
