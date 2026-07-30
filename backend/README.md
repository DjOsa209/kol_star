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

You can also load another config file:

```bash
CONFIG_FILE=/absolute/path/config.yaml go run ./cmd/server
```

Viper watches the config file and hot-reloads runtime settings. MySQL account/password/database changes reconnect the database; CORS changes take effect on the next request. `server.addr` is read at startup and requires a restart.

Platform-synced resource images are stored under `uploads/resource-images/{resourceId}/`. Each resource avatar and platform-post cover uses a stable filename, so later syncs replace the existing image instead of creating timestamped copies. Post covers keep their original remote URL in `cover_remote_url` and their local cache path in `cover_url`; APIs prefer the remote URL and expose the local path as a fallback. Image downloads retry temporary network failures and reuse `platform_apis.youtube_proxy_url` when it is configured.

Website and X cooperation content use a real 1280×720 browser-rendered page screenshot as the thumbnail. The server discovers Chrome or Chromium from `PATH`; set `KOL_CHROME_BIN=/absolute/path/to/chrome` when the browser binary is installed elsewhere. TikTok, Instagram, and YouTube continue to use cover images returned by their platform APIs.

For production, run the backend as a dedicated non-root service account whenever possible so Chrome keeps its sandbox enabled. If the process must run as root (for example, in an isolated container), the backend automatically adds Chrome's `--no-sandbox` argument; non-root processes never receive that argument. Restart the backend after installing Chrome or changing `KOL_CHROME_BIN`.

Default seeded login:

- username: `admin`
- password: `admin123`
