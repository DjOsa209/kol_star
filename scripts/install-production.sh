#!/usr/bin/env bash
set -euo pipefail

APP_NAME="${APP_NAME:-kol-admin}"
DOMAIN="${DOMAIN:-xmp.transsion.com}"
PROJECT_ROOT="${PROJECT_ROOT:-/home/trnuser/kol_star}"
RUN_USER="${RUN_USER:-trnuser}"
BACKEND_DIR="$PROJECT_ROOT/backend"
FRONTEND_DIR="$PROJECT_ROOT/vue-pure-admin"
BACKEND_BINARY="$BACKEND_DIR/kol-admin-server"
SERVICE_FILE="/etc/systemd/system/${APP_NAME}.service"
NGINX_SITE="/etc/nginx/sites-available/${APP_NAME}"

require_root() {
  if [[ "$(id -u)" -ne 0 ]]; then
    echo "Please run as root: sudo $0" >&2
    exit 1
  fi
}

require_command() {
  local command_name="$1"
  local install_hint="$2"

  if ! command -v "$command_name" >/dev/null 2>&1; then
    echo "Missing command: $command_name" >&2
    echo "$install_hint" >&2
    exit 1
  fi
}

require_root
require_command go "Install Go 1.23+ first."
require_command pnpm "Install pnpm first: corepack enable && corepack prepare pnpm@latest --activate"
require_command nginx "Install nginx first: apt install -y nginx"
require_command systemctl "systemd is required to install the backend service."

if [[ ! -d "$BACKEND_DIR" || ! -d "$FRONTEND_DIR" ]]; then
  echo "Project directories not found under $PROJECT_ROOT" >&2
  echo "Override with: PROJECT_ROOT=/path/to/kol_star $0" >&2
  exit 1
fi

if ! id "$RUN_USER" >/dev/null 2>&1; then
  echo "Service user does not exist: $RUN_USER" >&2
  echo "Override with: RUN_USER=your-deploy-user $0" >&2
  exit 1
fi

echo "Building backend..."
cd "$BACKEND_DIR"
go build -o "$BACKEND_BINARY" ./cmd/server

echo "Building frontend..."
cd "$FRONTEND_DIR"
pnpm install --frozen-lockfile
pnpm build

echo "Installing systemd service..."
cat > "$SERVICE_FILE" <<EOF_SERVICE
[Unit]
Description=KOL Admin Backend
After=network-online.target mysql.service
Wants=network-online.target

[Service]
Type=simple
User=$RUN_USER
WorkingDirectory=$BACKEND_DIR
Environment=CONFIG_FILE=$BACKEND_DIR/config.yaml
ExecStart=$BACKEND_BINARY
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF_SERVICE

echo "Installing nginx site..."
cat > "$NGINX_SITE" <<EOF_NGINX
server {
    listen 80;
    server_name $DOMAIN;

    root $FRONTEND_DIR/dist;
    index index.html;

    client_max_body_size 100m;

    location / {
        try_files \$uri \$uri/ /index.html;
    }

    location /api/ {
        proxy_pass http://127.0.0.1:8080/api/;
        proxy_http_version 1.1;

        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto https;
    }
}
EOF_NGINX

ln -sf "$NGINX_SITE" "/etc/nginx/sites-enabled/${APP_NAME}"
rm -f /etc/nginx/sites-enabled/default

echo "Reloading services..."
systemctl daemon-reload
systemctl enable --now "$APP_NAME"
nginx -t
systemctl enable --now nginx
systemctl reload nginx

echo "Done."
echo "Backend status: systemctl status $APP_NAME"
echo "Backend logs:   journalctl -u $APP_NAME -f"
echo "Frontend status: systemctl status nginx"
echo "Frontend URL:   https://$DOMAIN"
