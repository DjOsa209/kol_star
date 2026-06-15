# kol-admin backend

Go + MySQL backend for the admin frontend.

## Run

1. Initialize MySQL:

```bash
mysql -uroot -p < migrations/001_init.sql
mysql -uroot -p kol_admin < migrations/002_business.sql
mysql -uroot -p kol_admin < migrations/003_system_admin.sql
mysql -uroot -p kol_admin < migrations/004_default_admin.sql
mysql -uroot -p kol_admin < migrations/005_system_permissions.sql
mysql -uroot -p kol_admin < migrations/006_sync_current_menus.sql
mysql -uroot -p kol_admin < migrations/007_business_import_fields.sql
mysql -uroot -p kol_admin < migrations/008_resource_platform_sync.sql
mysql -uroot -p kol_admin < migrations/009_resource_dynamic_fields.sql
mysql -uroot -p kol_admin < migrations/010_resource_fixed_import_fields.sql
mysql -uroot -p kol_admin < migrations/011_prd_v2_assistant_governance.sql
mysql -uroot -p kol_admin < migrations/012_ai_model_config_menu.sql
mysql -uroot -p kol_admin < migrations/013_platform_sync_engine.sql
mysql -uroot -p kol_admin < migrations/014_platform_sync_control.sql
mysql -uroot -p kol_admin < migrations/015_resource_platform_posts_menu.sql
mysql -uroot -p kol_admin < migrations/016_market_options.sql
mysql -uroot -p kol_admin < migrations/017_campaign_execution_center.sql
mysql -uroot -p kol_admin < migrations/018_dashboard_first_menu.sql
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

Platform-synced resource images are stored under `uploads/resource-images/{resourceId}/`. Each resource avatar and platform-post cover uses a stable filename, so later syncs replace the existing image instead of creating timestamped copies. Image downloads retry temporary network failures and reuse `platform_apis.youtube_proxy_url` when it is configured.

Default seeded login:

- username: `admin`
- password: `admin123`
