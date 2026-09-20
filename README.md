# XBnpp

## 首次安装并绑定机器

先在 Xboard 面板创建机器，取得机器 ID 和 Token；在风控后台对应面板添加采集节点，取得采集标识和密钥。替换下面的示例值后执行。旧版需先备份、卸载，再全新安装。

```bash
curl -fL https://github.com/kele812/XBnpp/releases/latest/download/install.sh -o install.sh
sudo bash install.sh \
  --mode machine \
  --panel 'https://你的Xboard域名' \
  --token '机器Token' \
  --machine-id 3 \
  --kernel xray \
  --watch-url 'https://你的风控域名' \
  --watch-node '采集节点标识' \
  --watch-secret '采集密钥'
```

## 添加另一台机器或另一个面板

在已经安装 XBnpp 的 VPS 上执行，填写对应面板的机器信息和采集信息：

```bash
sudo xbctl bind add-machine \
  --panel-url 'https://另一个Xboard域名' \
  --token '对应机器的Token' \
  --machine-id 5 \
  --kernel xray \
  --watch-url 'https://你的风控域名' \
  --watch-node '对应面板的采集节点标识' \
  --watch-secret '对应面板的采集密钥'
```

`--kernel` 可选 `xray`、`singbox`、`auto`。同一 VPS 上各节点监听端口不能冲突。添加绑定会重启节点；采集用户需在风控后台开启。
