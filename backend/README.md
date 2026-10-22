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

Apply `migrations/049_recommendation_advice.sql` before deploying IP recommendation advice. It adds the stored advice, advice origin, suggested scenarios, external references, evidence status, and a resource-content hash to `biz_recommendation_results`. Recommendations still select **only IPs in the internal resource library**. The rule score and rule match reason are separate from the collaboration advice: `source` records whether AI participated in ranking, while `adviceSource` records whether the advice itself came from the configured AI model or the rule fallback. Model output is accepted only for existing candidates; if the model is missing or its response is unusable, the result explicitly uses rule advice. Advice explains suitable collaboration scenarios and why the IP fits the saved brief. Copyright scope, rights, and price remain subject to the IP resource file and rights-owner confirmation.

External context for IP advice defaults to Wikidata's CC0 entity data. The backend searches by the IP's **public name only**, accepts an exact name match after type disambiguation, and stores the source URL, label, and description. It never sends a saved search, product brief, budget, or other internal request data to the external provider. Up to three attributed references may appear with each result. `evidenceStatus` is `available` when a usable source was found, `none` when the search succeeded without a usable match, `unavailable` when an attempted provider request failed, and `not_searched` when this result was not searched (including legacy or stale advice). Only the top three eligible IPs are searched per run, within an eight-second budget. A reference is background context, not proof of licensing availability or an agreed price; the recommendation remains usable from internal resource data when no external source is available.

The IP conversation also supports explicit external discovery with `action: "search"`. Users enter up to 200 characters of public keywords in the web-search mode; only those keywords are sent to a provider. Configure Tavily in the backend's active `config.yaml` (or the file selected by `CONFIG_FILE`):

```yaml
web_search:
  provider: tavily
  tavily_api_key: ""
```

Fill `web_search.tavily_api_key` in the local configuration file. `config.example.yaml` provides an empty template; real `config.yaml` is Git-ignored. Once the updated backend is running, changes use the existing YAML hot-reload mechanism. Search reads the current application config on each request; key changes also change the cache identity. Viper's standard `WEB_SEARCH_PROVIDER` / `WEB_SEARCH_TAVILY_API_KEY` environment overrides are supported.

Alternatively configure **one** provider in the backend process environment and restart:

```bash
# Tavily: provide the key through the deployment secret store.
IP_WEB_SEARCH_PROVIDER=tavily
TAVILY_API_KEY="<server-side-key>"

# Or a self-hosted SearXNG instance with search.formats including json:
IP_WEB_SEARCH_PROVIDER=searxng
SEARXNG_BASE_URL=http://127.0.0.1:8888
```

These legacy environment variables are fallbacks when the corresponding YAML values are empty. For a local shell, export the selected variables before `go run ./cmd/server`; for a container, pass them into its environment. Optional `.env` loading remains compatible, using `ENV_FILE` for another file path; no env file is needed when YAML is configured. Brave discovery uses `IP_WEB_SEARCH_PROVIDER=brave` plus the existing Brave key and storage entitlement flag described below. No provider configured means an explicit error, not simulated results. No subscription, paid plan or external account is created by the app. Tavily uses basic search only; repeated same-user queries are cached for 15 minutes. The user-selected research domains use Tavily's `include_domains_mode: "prefer"`, allowing other relevant public sources in the same single search. Returned sources are labeled and sorted P0 → P1 → P2 → other, preserving provider order within each group. Only returned pages become evidence. Published dates are kept when supplied by the provider; rankings' edition years must not be inferred from those dates. Sources remain external leads until reviewed and entered in the IP resource library; discovery does not modify existing candidate IDs or matching criteria. See `docs/ip-ai-recommendation-workspace.md` for the source catalog, complete interaction and verification scope.

Brave Web Search is optional. Enable it only if the deployment's Brave plan explicitly grants the right to store search-result titles, snippets, and URLs, then set **both** `BRAVE_SEARCH_API_KEY` and `BRAVE_SEARCH_STORAGE_ALLOWED=1` in the backend environment. A key alone does not trigger a Brave request. The backend calls only the fixed Wikidata and Brave HTTPS APIs; it does not fetch linked search-result pages. Do not enable snippet storage without the required entitlement.

Results saved before migration 049 have no collaboration advice until their saved search is run again. **IP 运营 → IP AI 推荐 → 立即刷新** regenerates advice and external references for existing matches in place. It preserves interest feedback and the original new-result flag, and does not create duplicate notifications for those matches.

Apply `migrations/050_project_upload_audit.sql` before deploying project uploader auditing. It adds `biz_projects.created_by` for both individual project creation and spreadsheet imports. The API derives the ID from the authenticated session, never from an editable request field, and returns read-only `uploadedBy`/`uploaderName` in project lists. Earlier projects retain `created_by=0` because their original uploader cannot be recovered. IP resources and IP document files already store the authenticated user in `biz_ip_resources.created_by` and `biz_ip_files.uploaded_by`; their list/detail responses now expose the same read-only audit metadata.

Apply `migrations/051_monitor_live_sessions.sql` before deploying live online users and operation auditing. Every local or SSO login now creates a random server-side session; access and refresh tokens are accepted only while that session is active, and **在线用户** shows sessions with activity in the last 15 minutes. The forced-offline action and ordinary logout revoke the selected session immediately. The old deterministic token format is rejected on deployment, so existing users must sign in again. Operation logs include only authenticated write attempts, record success or failure, and support module, status, date, and page filters. Read-only POSTs continue to appear in the system request log but not in operation logs. Earlier rows made by the former all-POST logger remain stored but are excluded by `audit_version=1`; the old demo monitor rows are removed by exact-match cleanup. New installations do not seed monitor logs. Credential-bearing request bodies are omitted from system logs.

Apply `migrations/052_log_list_performance.sql` before deploying the log list performance update. System logs now query one page at a time instead of loading the full table, and the new `(audit_version, id)` index supports the operation log's newest-first list. Existing log rows remain intact.

Apply `migrations/053_ip_market_penetrations.sql` before deploying IP market penetration entry. It adds optional per-market percentages (0–100) to IP resources while retaining the existing market list for filters and recommendations. Earlier IP records remain editable with no percentages.

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

The IP contact picker searches UAC through the server-side `uac-user/user/pages` API using the application's client credentials. The pagination query is sent as `param_json` in a signed open-platform request (`app_id`, `sign_method`, `timestamp`, `nonce`, and `sign`), alongside the application Bearer token. Keep the server clock synchronized because UAC rejects timestamps outside its allowed window. Grant this app access to that API and the `uid`, `realName`, `employeeNo`, `email`, `status`, and department fields. The API returns only active colleagues to the browser and never exposes their email there. If `uac_app_secret` is missing or UAC search is unavailable, the picker falls back to SSO users who have already signed in to XMP and labels the limited result. IP stage notifications recheck the selected employee number and UID in UAC before sending a Feishu application message. Delivery runs in the background after each committed stage change and is best effort; failures are logged without rolling back the submitted workflow.

Platform-synced resource images are stored under `uploads/resource-images/{resourceId}/`. Each resource avatar and platform-post cover uses a stable filename, so later syncs replace the existing image instead of creating timestamped copies. Post covers keep their original remote URL in `cover_remote_url` and their local cache path in `cover_url`; APIs prefer the remote URL and expose the local path as a fallback. Image downloads retry temporary network failures and reuse `platform_apis.youtube_proxy_url` when it is configured.

TikHub requests use `https://api.tikhub.dev` by default for mainland-China connectivity. Set `TIKHUB_API_BASE_URL=https://api.tikhub.io` for deployments in other regions.

Website traffic sync reads `https://traffic.cv/{domain}` and parses the rendered HTML. It first uses HTTP and then falls back to the installed Chrome/Chromium when the site returns a JavaScript challenge. If the deployment IP is challenged persistently, set `TRAFFIC_CV_PROXY_URL` to an HTTP/SOCKS proxy whose egress can open Traffic.cv. Administrators can also use **资源管理 → Website → 导入 Traffic.cv HTML** to upload or paste a page saved after manually completing the browser verification; the same parser then fills the Website metrics without copying browser credentials into the backend.

Website and X cooperation content use a real 1280×720 browser-rendered page screenshot as the thumbnail. Project imports automatically start a background capture for newly imported Website/X content that does not already have a cover. Before capture, the browser dismisses native dialogs, prefers minimal-consent actions such as "Only necessary" or "Reject", and removes large residual cookie/privacy/modal overlays while preserving ordinary fixed page content. The server discovers Chrome or Chromium from `PATH`; set `KOL_CHROME_BIN=/absolute/path/to/chrome` when the browser binary is installed elsewhere. TikTok, Instagram, and YouTube continue to use cover images returned by their platform APIs.

For production, run the backend as a dedicated non-root service account whenever possible so Chrome keeps its sandbox enabled. If the process must run as root (for example, in an isolated container), the backend automatically adds Chrome's `--no-sandbox` argument; non-root processes never receive that argument. Restart the backend after installing Chrome or changing `KOL_CHROME_BIN`.

Default seeded login:

- username: `admin`
- password: `admin123`

Apply `migrations/054_ip_workflow_deadlines.sql` before deploying IP request deadlines and Feishu completion reminders. IP feedback has 10 working days after demand submission; marketing feedback has 5 working days after IP review. China mainland holidays **and makeup workdays** are counted from the following day, through 23:59:59 Asia/Shanghai. Verified 2025/2026 calendars are embedded; maintain complete later annual schedules in the hot-reloaded `work_calendar` list in `config.yaml`. Unknown years are explicitly pending rather than assumed weekdays. Old requests retain unknown stage timestamps. See [IP deadline rules, annual calendar maintenance and QA](../docs/ip-request-deadlines.md).

Marketing feedback is now submitted independently by IMC, brand, user, media and PR partners. Each request patches only its selected function under a row lock, uses a per-function version to reject stale writes, and completes only when all five functions submit. The shared 5-working-day deadline does not reset on individual submissions. Completion uses the existing Feishu application channel to notify the authenticated demand creator; the earlier in-app popup and workflow notice polling were removed. This change reuses `marketing_profile` JSON and needs no migration beyond 054. Existing completed requests and historical shared opinions remain readable.

## AI creator discovery and recommendation

For an existing database, apply [migration 055](migrations/055_creator_discovery.sql) before deploying the updated API and frontend, then restart the API:

```bash
mysql -uroot -p < migrations/055_creator_discovery.sql
```

New installations already receive these tables from `migrations/all.sql`. The migration adds discovery jobs, candidate snapshots and a stable platform identity mapping for duplicate-safe resource imports. It does not seed creator data.

Configure the existing integrations before use:

- In **系统管理 → 抓取控制 → 平台 API 配置**, supply the YouTube API Key and, when needed, the YouTube proxy. Discovery uses the same effective configuration as platform synchronization, including the `platform_apis.youtube_api_key` and `platform_apis.youtube_proxy_url` configuration fallback.
- In **AI 模型配置**, enable a model, provide its existing server-side endpoint and credentials, and enable the recommendation feature. The same model configuration serves both search planning and candidate recommendation. Keep real credentials in the deployment configuration or configuration UI; do not include them in README examples, screenshots or logs.

The unified **AI 推荐** entry accepts a natural-language requirement. Users do not need to choose a platform or prepare search terms. The backend create endpoint, `POST /business/creator-discovery/jobs/create`, requires only `query` (up to 2,000 characters), for example:

```json
{"query":"推荐适合在日本推广新款手机、近期发布过手机评测的达人"}
```

The configured model plans one to three content searches from that requirement. The current discovery provider is **YouTube Data API v3**: it searches public videos, identifies their authors, and collects channel details, available metrics, recent works and source URLs for up to 12 distinct channels per job. These snapshots are stored as `pending` candidates. The model then evaluates the collected candidates and automatically produces recommendations; prior approval or entry in the resource library is not required. This implementation covers YouTube discovery, not all websites or social platforms. Missing configuration, failed provider calls and unusable model responses produce explicit errors or partial results rather than mock creators or rule-generated recommendations.

Human review follows the recommendation. Before importing a candidate, the operator confirms the market, content language and topic, supplies a review note, and selects evidence from that candidate's collected profile or works. Approval creates a resource with status `观察中`, score `0`, level `D` and risk `待核实`; pricing remains unconfirmed. A stable YouTube channel identity prevents duplicate imports across jobs, and an existing resource's business fields are preserved. Approval retains the current AI assessment and links the candidate to its resource; rejection invalidates the saved recommendation so it can be regenerated without that candidate. Discovery and recommendation alone never write a formal resource.

All job operations and candidate reviews check the authenticated owner's user ID and require access to the creator recommendation menu (`/business/assistant`). Import approval additionally requires resource-library menu access (`/business/resources`). Candidate IDs, source evidence and resource permissions are checked on the server. Discovery request and response bodies are omitted from system request logs, as with existing recommendation endpoints.

Execution currently uses workers inside a **single API process**: at most two jobs execute concurrently, each user can have one active job, and the eight-minute timeout includes waiting for a worker slot. Job records, collected snapshots and completed recommendations are persisted in MySQL, while running workers and cancellation handles remain in memory. On API restart, unfinished jobs become `partial`; saved candidates remain available, but work does not automatically resume. Users can regenerate a recommendation from saved candidates or retry the job, which starts a new planning and collection run. This is not a durable queue or a coordinated multi-instance worker service; deploy one discovery worker process until shared job claiming and recovery are implemented.
