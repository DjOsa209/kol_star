# kol-admin backend

Go + MySQL backend for the admin frontend.

## Run

1. Initialize MySQL:

```bash
mysql -uroot -p < migrations/all.sql
```

2. Start the API:

```bash
go run ./cmd/server
```

Runtime settings live in `config.yaml`. Update the MySQL username, password, and database before starting if needed.

```yaml
mysql:
  username: root
  password: your_password
  database: kol_admin
```

The default address is `:8080`. The frontend development proxy points `/api` to `http://localhost:8080`.

For an existing database, apply `migrations/042_ip_operations.sql` and then `migrations/043_ip_assessment_details.sql` before using **IP运营**. They add the IP resource library, request workflow, detailed assessment fields, and the new role menus. IP PDFs and visual assets are stored privately under `uploads/ip-documents/` and served only through authenticated endpoints.

To enable product cooperation discovery and the two recommendation modules on an existing database, apply `migrations/044_product_discovery.sql`, `migrations/045_recommendations.sql`, and `migrations/046_recommendation_menus.sql` in that order, then restart the API. Product names extracted from published internal cooperations remain pending until an operator confirms their brand and product; only confirmed records appear in brand/product search and recommendation evidence. Saved searches produce an immediate result set and are checked each Monday after 09:00 Asia/Shanghai; each saved search creates at most five new in-app notifications per week. Recommendation requests and responses are omitted from system request logs.

For the IP feedback priority update, apply `migrations/047_ip_priority_tiers.sql` once before deploying the updated API. It converts existing numbered recommendation ranks to T0/T1/T2; multiple IPs may share a tier.

Apply `migrations/048_ip_recommendation_access.sql` to grant the IP recommendation menu to roles that already have IP demand, feedback, or marketing access. This also repairs environments where the recommendation menu was missing.

You can also load another config file:

```bash
CONFIG_FILE=/absolute/path/config.yaml go run ./cmd/server
```

Viper watches the config file and hot-reloads runtime settings. MySQL account/password/database changes reconnect the database; CORS changes take effect on the next request. `server.addr` is read at startup and requires a restart.

Project imports run platform synchronization in the background and send the final result through Feishu instead of exposing a progress dialog. Administrators can configure an application bot, a group webhook, or both under **系统管理 → 抓取控制 → 飞书通知配置**. Application-bot delivery uses the importing user's email from `sys_users`; the corresponding Feishu application must have message-sending permission. The same settings can be supplied in `config.yaml`:

```yaml
feishu:
  application_enabled: false
  app_id: ""
  app_secret: ""
  api_base_url: https://open.feishu.cn
  webhook_enabled: false
  webhook_url: ""
  frontend_url: ""
```

Enterprise UAC SSO can be enabled alongside the local administrator login. When enabled, regular users must use SSO; password login remains available only to accounts with the `admin` role. The callback validates a short-lived state cookie, creates a `common` user on first login, and binds an existing account when its email matches.

```yaml
sso:
  enabled: true
  provider: uac
  frontend_url: https://xmp.example.com/
  redirect_uri: https://xmp.example.com/sso/callback
  uac_gateway: https://uac.example.com
  uac_app_id: your_app_id
  uac_app_secret: your_app_secret
  uac_lang: zh_CN
  uac_source: ""
```

Register the exact path-based `redirect_uri` in UAC; URL fragments such as `#/sso/callback` must not be used because UAC redirects do not preserve them. Apply `migrations/032_sso_users.sql` before enabling SSO. UAC callback tokens are never stored in request logs.

The IP contact picker searches UAC through the server-side `uac-user/user/pages` API using the application's client credentials. Grant this app access to that API and the `uid`, `realName`, `employeeNo`, `email`, `status`, and department fields. The API returns only active colleagues to the browser and never exposes their email there. If `uac_app_secret` is missing or UAC search is unavailable, the picker falls back to SSO users who have already signed in to XMP and labels the limited result. IP stage notifications recheck the selected employee number and UID in UAC before sending a Feishu application message. Delivery runs in the background after each committed stage change and is best effort; failures are logged without rolling back the submitted workflow.

Platform-synced resource images are stored under `uploads/resource-images/{resourceId}/`. Each resource avatar and platform-post cover uses a stable filename, so later syncs replace the existing image instead of creating timestamped copies. Post covers keep their original remote URL in `cover_remote_url` and their local cache path in `cover_url`; APIs prefer the remote URL and expose the local path as a fallback. Image downloads retry temporary network failures and reuse `platform_apis.youtube_proxy_url` when it is configured.

TikHub requests use `https://api.tikhub.dev` by default for mainland-China connectivity. Set `TIKHUB_API_BASE_URL=https://api.tikhub.io` for deployments in other regions.

Website traffic sync reads `https://traffic.cv/{domain}` and parses the rendered HTML. It first uses HTTP and then falls back to the installed Chrome/Chromium when the site returns a JavaScript challenge. If the deployment IP is challenged persistently, set `TRAFFIC_CV_PROXY_URL` to an HTTP/SOCKS proxy whose egress can open Traffic.cv. Administrators can also use **资源管理 → Website → 导入 Traffic.cv HTML** to upload or paste a page saved after manually completing the browser verification; the same parser then fills the Website metrics without copying browser credentials into the backend.

Website and X cooperation content use a real 1280×720 browser-rendered page screenshot as the thumbnail. Project imports automatically start a background capture for newly imported Website/X content that does not already have a cover. Before capture, the browser dismisses native dialogs, prefers minimal-consent actions such as "Only necessary" or "Reject", and removes large residual cookie/privacy/modal overlays while preserving ordinary fixed page content. The server discovers Chrome or Chromium from `PATH`; set `KOL_CHROME_BIN=/absolute/path/to/chrome` when the browser binary is installed elsewhere. TikTok, Instagram, and YouTube continue to use cover images returned by their platform APIs.

For production, run the backend as a dedicated non-root service account whenever possible so Chrome keeps its sandbox enabled. If the process must run as root (for example, in an isolated container), the backend automatically adds Chrome's `--no-sandbox` argument; non-root processes never receive that argument. Restart the backend after installing Chrome or changing `KOL_CHROME_BIN`.

Default seeded login:

- username: `admin`
- password: `admin123`
