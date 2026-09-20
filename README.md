# xbn plus ++

基于 [XboardNode-Plus](https://github.com/xboardnext999/XboardNode-Plus) 增加指定用户的代理目标采集，配合 [SubscriptionWatch 3.8.0+](https://github.com/kele812/SubscriptionWatch) 使用。独立版本 **1.0.0**，保留原来的协议、配置、用户同步和流量统计。

采集内容：用户ID、连接时间、来源IP、目标域名或IP、端口和协议。节点直接上报风控机，不把新增采集记录写入Xboard数据库。用户邮箱在风控机匹配；记录默认在风控机保留3天。

## Docker 安装

镜像支持 Linux amd64 / arm64。以下拉取命令需要本仓库已发布对应镜像；尚未发布时可用下面的本地构建方式。

与原版一样，可以用环境变量启动，多填写三项采集配置：

~~~bash
docker run -d --name xbn-plus-plus --restart=always --network=host \
  -v xbn-plus-plus-data:/etc/XboardNode-Plus \
  -e apiHost=https://你的Xboard地址 \
  -e apiKey=原Xboard节点通信密钥 \
  -e nodeID=1 \
  -e kernel=auto \
  -e WATCH_ACCESS_URL=https://你的风控域名 \
  -e WATCH_ACCESS_NODE=风控后台生成的节点标识 \
  -e WATCH_ACCESS_SECRET=风控后台生成的独立密钥 \
  ghcr.io/kele812/xbn-plus-plus:1.0.0
~~~

使用证书文件的节点需保留原有证书挂载及相关配置。采集连接要求有效证书的HTTPS地址。三项采集配置来自风控后台“代理访问记录 → 添加采集节点”，不能使用Xboard通信密钥代替。

## Docker Compose

~~~bash
git clone https://github.com/kele812/xbn-plus-plus.git
cd xbn-plus-plus
cp .env.example .env
chmod 600 .env
nano .env
docker compose up -d
~~~

填写 .env 中的Xboard信息和采集配置后启动。后续升级先将 compose.yml 的镜像版本改为新版本，再执行：

~~~bash
docker compose pull
docker compose up -d
~~~

不使用环境变量时，把原配置目录复制到本项目的 node-config，确认包含 config.yml，使用 compose.config.yml；保留原有证书挂载：

~~~bash
docker compose -f compose.config.yml up -d
~~~

## config.yml 方式

单面板配置中，在最外层添加，和 panel、node 同级：

~~~yaml
watch_access:
  url: "https://你的风控域名"
  node: "风控后台生成的节点标识"
  secret: "风控后台生成的独立密钥"
~~~

其他原配置保持不变。使用 instances 多面板格式时，必须在对应的每一个实例里面填写各自的 watch_access，不能放在最外层，也不能让多个面板共用 WATCH_ACCESS_* 环境变量。单进程同面板的多节点/机器模式会共用采集接入名称；需要区分节点时使用独立实例和独立密钥。

启动后，在风控后台输入用户ID和对应邮箱，开启指定用户采集。没有指定用户时不采集目标地址；只能采集开启后新建立的连接，不能补回过去24小时的历史，也不能看到HTTPS页面内容或路径。

## 从原 XboardNode-Plus 迁移

1. 保存原Compose或启动命令，备份配置并记录旧镜像名称/摘要。
2. 先拉取新镜像，把原部署的 image 改为 ghcr.io/kele812/xbn-plus-plus:1.0.0。
3. 原样保留网络模式、端口、配置和证书挂载，增加 watch_access 或三项采集环境变量。
4. 重建原节点容器，先测试一台。不要让新旧容器同时监听同一个端口。
5. 测试普通代理连接、用户同步和风控记录；异常时切回旧镜像并移除新增采集配置。

容器内程序名仍是 xboard-node，配置仍是 /etc/XboardNode-Plus/config.yml。重建容器会中断现有连接。

## Linux systemd 安装

发布安装包后，使用与上游相同的安装参数：

~~~bash
curl -fL https://github.com/kele812/xbn-plus-plus/releases/latest/download/install.sh -o install.sh
sudo bash install.sh --mode node --panel https://panel.example.com --token TOKEN --node-id 1
~~~

之后编辑 /etc/XboardNode-Plus/config.yml 加入 watch_access，再执行 systemctl restart xboard-node。机器模式、xbctl 和 easy-install.sh 的原有参数继续保留；easy-install.sh 会配置nginx，请按实际需要使用。

## 本地构建

~~~bash
docker build -t ghcr.io/kele812/xbn-plus-plus:1.0.0 .
~~~

首次构建会下载依赖并编译，建议构建机至少有4GB可用内存，不要在繁忙的Xboard面板机上编译。公开镜像发布后，节点只需拉取镜像，不必编译。

## 维护者发布

更新 VERSION 中的版本号，在 GitHub Actions 的 Test and release 工作流中手动运行并勾选 publish，或推送与 VERSION 一致的 v版本号 标签。测试通过后生成两种架构的节点程序、xbctl、SHA256SUMS以及Docker镜像。

GHCR首次创建的软件包可能是私有的；维护者需在包设置中将可见性设为Public，才能匿名拉取。普通push/PR只测试和构建，不发布镜像。

## 采集边界

- 每节点队列1000条、批次100条、约10秒上传一次，失败最多重试3次；故障不阻塞代理连接。过载和重启可能丢记录，风控后台可查看失败/丢弃数量，不是完整审计日志。
- 约30秒同步指定用户，策略超过90秒未更新时暂停采集。停止后风控机立即拒收该用户的新增记录。
- 开启此通道后，原来的访问目标诊断内容不再重复上传到Xboard；正常计费和在线状态上报保留。
- 不保证每条连接都有域名，也不代表网页打开成功；连接复用和后台应用访问会影响记录粒度。

来源、固定上游版本及修改范围见 [UPSTREAM.md](UPSTREAM.md)。许可证：[MPL-2.0](LICENSE)。本项目为独立修改版。
