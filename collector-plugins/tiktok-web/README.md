# TikTok Web Collector Plugin

这个插件作为独立采集 Agent 运行：它调用你部署的 `Douyin_TikTok_Download_API`，采集 TikTok 公开博主数据后，通过 `kol_admin` 的回传接口写入资源库。

## 后端配置

在 `backend/config.yaml` 中设置一个强随机 token：

```yaml
collector:
  agent_token: "replace-with-a-long-random-token"
```

插件回传时会把这个 token 放在 `X-Collector-Token` 请求头里。

## 运行前提

1. 启动 `kol_admin` 后端，例如 `http://127.0.0.1:8080`。
2. 启动 `Douyin_TikTok_Download_API`，例如 `http://127.0.0.1:8001/api/tiktok/web`。
3. 在采集服务自己的 `crawlers/tiktok/web/config.yaml` 里维护 Cookie、User-Agent、代理。

## 手动采集

```bash
python3 collector-plugins/tiktok-web/tiktok_collector.py \
  --creator-url "https://www.tiktok.com/@tiktok" \
  --collector-base-url "http://127.0.0.1:8001/api/tiktok/web" \
  --server-url "http://127.0.0.1:8080" \
  --token "replace-with-a-long-random-token"
```

也可以直接传 username：

```bash
python3 collector-plugins/tiktok-web/tiktok_collector.py \
  --username "tiktok" \
  --collector-base-url "http://127.0.0.1:8001/api/tiktok/web" \
  --server-url "http://127.0.0.1:8080" \
  --token "replace-with-a-long-random-token"
```

## 回传协议

插件会 POST 到：

```text
POST /collector/tiktok/callback
```

核心 payload：

```json
{
  "collector": "tiktok-web",
  "creator": {
    "username": "tiktok",
    "secUid": "...",
    "name": "TikTok",
    "profileUrl": "https://www.tiktok.com/@tiktok",
    "followerCount": 1000,
    "followingCount": 10,
    "likesCount": 10000,
    "videoCount": 20
  },
  "posts": []
}
```

后端会按 `resourceId`、`secUid`、`username` 匹配资源；找不到则自动创建 TikTok 资源。
