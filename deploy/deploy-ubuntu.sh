#!/bin/bash
#
# Sub2API Ubuntu Deployment Script
# Sub2API Ubuntu 部署脚本
#
# This script deploys the Sub2API binary and configures:
#   - systemd service
#   - Nginx reverse proxy
#   - config.yaml from saved configuration
#
# Prerequisites / 前置条件:
#   1. Run setup-ubuntu.sh first (installs PostgreSQL, Redis, Nginx)
#   2. Build the binary: ./deploy/build.sh
#   3. Upload binary to server (default: /tmp/sub2api)
#
# Usage / 用法:
#   sudo bash deploy-ubuntu.sh                              # Deploy from /tmp/sub2api
#   sudo bash deploy-ubuntu.sh --binary /path/to/sub2api    # Specify binary path
#   sudo bash deploy-ubuntu.sh --domain example.com         # Set domain for Nginx
#   sudo bash deploy-ubuntu.sh --ssl                        # Enable SSL (requires domain)
#

set -e

# ============================================================
# Configuration
# ============================================================
INSTALL_DIR="/opt/sub2api"
CONFIG_DIR="/etc/sub2api"
SERVICE_USER="sub2api"
SERVICE_NAME="sub2api"
BINARY_PATH="/tmp/sub2api"
DOMAIN=""
ENABLE_SSL=false
NGINX_CONF_DIR="/etc/nginx/sites-available"
NGINX_ENABLE_DIR="/etc/nginx/sites-enabled"
SERVER_PORT=8080

# Load saved configuration
if [ -f "$CONFIG_DIR/.deploy-env" ]; then
    source "$CONFIG_DIR/.deploy-env"
    info_loaded=true
fi

# ============================================================
# Color output
# ============================================================
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

info()    { echo -e "${BLUE}[INFO]${NC} $1"; }
success() { echo -e "${GREEN}[OK]${NC} $1"; }
warn()    { echo -e "${YELLOW}[WARN]${NC} $1"; }
error()   { echo -e "${RED}[ERROR]${NC} $1"; }

# ============================================================
# Parse arguments
# ============================================================
while [[ $# -gt 0 ]]; do
    case "$1" in
        --binary)
            BINARY_PATH="$2"
            shift 2
            ;;
        --domain)
            DOMAIN="$2"
            shift 2
            ;;
        --ssl)
            ENABLE_SSL=true
            shift
            ;;
        --port)
            SERVER_PORT="$2"
            shift 2
            ;;
        --help|-h)
            echo "Sub2API Ubuntu Deployment Script"
            echo ""
            echo "Usage: sudo bash $0 [options]"
            echo ""
            echo "Options:"
            echo "  --binary <path>   Path to the sub2api binary (default: /tmp/sub2api)"
            echo "  --domain <name>   Domain name for Nginx (e.g., api.example.com)"
            echo "  --ssl             Enable SSL with Let's Encrypt (requires --domain)"
            echo "  --port <num>      Backend server port (default: 8080)"
            echo "  --help            Show this help message"
            exit 0
            ;;
        *)
            error "Unknown option: $1"
            exit 1
            ;;
    esac
done

# ============================================================
# Check root
# ============================================================
check_root() {
    if [ "$(id -u)" -ne 0 ]; then
        error "Please run as root (use sudo)"
        exit 1
    fi
}

# ============================================================
# Check prerequisites
# ============================================================
check_prerequisites() {
    info "Checking prerequisites..."

    # Check if setup has been run
    if [ ! -f "$CONFIG_DIR/.deploy-env" ]; then
        error "Server has not been set up. Please run setup-ubuntu.sh first."
        exit 1
    fi

    # Load configuration
    source "$CONFIG_DIR/.deploy-env"

    # Check binary
    if [ ! -f "$BINARY_PATH" ]; then
        error "Binary not found at: $BINARY_PATH"
        echo ""
        echo "  Options:"
        echo "    1. Build locally: ./deploy/build.sh"
        echo "    2. Upload to server: scp bin/sub2api user@server:/tmp/sub2api"
        echo "    3. Specify path: sudo bash $0 --binary /path/to/sub2api"
        exit 1
    fi

    # Check if binary is executable
    if [ ! -x "$BINARY_PATH" ]; then
        chmod +x "$BINARY_PATH"
    fi

    # Verify services are running
    if ! systemctl is-active --quiet postgresql; then
        warn "PostgreSQL is not running. Starting..."
        systemctl start postgresql
    fi

    if ! systemctl is-active --quiet redis-server; then
        warn "Redis is not running. Starting..."
        systemctl start redis-server
    fi

    success "Prerequisites check passed"
}

# ============================================================
# Deploy binary
# ============================================================
deploy_binary() {
    info "Deploying binary..."

    # Stop service if running
    if systemctl is-active --quiet "$SERVICE_NAME"; then
        info "Stopping existing service..."
        systemctl stop "$SERVICE_NAME"
    fi

    # Backup current binary if exists
    if [ -f "$INSTALL_DIR/sub2api" ]; then
        local backup_name="sub2api.backup.$(date +%Y%m%d%H%M%S)"
        cp "$INSTALL_DIR/sub2api" "$INSTALL_DIR/$backup_name"
        info "Backed up previous binary: $backup_name"

        # Keep only last 3 backups
        ls -t "$INSTALL_DIR"/sub2api.backup.* 2>/dev/null | tail -n +4 | xargs rm -f 2>/dev/null || true
    fi

    # Copy new binary
    cp "$BINARY_PATH" "$INSTALL_DIR/sub2api"
    chmod +x "$INSTALL_DIR/sub2api"
    chown "$SERVICE_USER:$SERVICE_USER" "$INSTALL_DIR/sub2api"

    success "Binary deployed to $INSTALL_DIR/sub2api"
}

# ============================================================
# Generate config.yaml
# ============================================================
generate_config() {
    info "Generating config.yaml..."

    # Generate JWT secret if not exists
    local JWT_SECRET=""
    if [ -f "$CONFIG_DIR/config.yaml" ]; then
        JWT_SECRET=$(grep -oP 'secret:\s*"\K[^"]+' "$CONFIG_DIR/config.yaml" 2>/dev/null | head -1)
    fi
    if [ -z "$JWT_SECRET" ]; then
        JWT_SECRET=$(openssl rand -hex 32)
        info "Generated new JWT secret"
    fi

    # Generate TOTP encryption key if not exists
    local TOTP_KEY=""
    if [ -f "$CONFIG_DIR/config.yaml" ]; then
        TOTP_KEY=$(grep -A1 'totp:' "$CONFIG_DIR/config.yaml" 2>/dev/null | grep -oP 'encryption_key:\s*"\K[^"]+')
    fi
    if [ -z "$TOTP_KEY" ]; then
        TOTP_KEY=$(openssl rand -hex 32)
        info "Generated new TOTP encryption key"
    fi

    # Write config.yaml
    cat > "$CONFIG_DIR/config.yaml" << EOF
# Sub2API Configuration - Auto-generated by deploy-ubuntu.sh
# Generated at: $(date '+%Y-%m-%d %H:%M:%S')

server:
  host: "127.0.0.1"
  port: ${SERVER_PORT}
  mode: "release"

run_mode: "standard"

database:
  host: "${DB_HOST:-localhost}"
  port: ${DB_PORT:-5432}
  user: "${DB_USER}"
  password: "${DB_PASS}"
  dbname: "${DB_NAME}"
  sslmode: "disable"
  max_open_conns: 100
  max_idle_conns: 50
  conn_max_lifetime_minutes: 30
  conn_max_idle_time_minutes: 5

redis:
  host: "${REDIS_HOST:-localhost}"
  port: ${REDIS_PORT:-6379}
  password: "${REDIS_PASS}"
  db: 0
  pool_size: 256
  min_idle_conns: 32

jwt:
  secret: "${JWT_SECRET}"
  expire_hour: 24

totp:
  encryption_key: "${TOTP_KEY}"

log:
  level: "info"
  format: "json"
  service_name: "sub2api"
  env: "production"
  caller: true
  stacktrace_level: "error"
  output:
    to_stdout: true
    to_file: true
    file_path: "${INSTALL_DIR}/data/logs/sub2api.log"
  rotation:
    max_size_mb: 100
    max_backups: 10
    max_age_days: 7
    compress: true
    local_time: true

default:
  admin_email: "admin@sub2api.local"
  admin_password: ""
  user_concurrency: 5
  user_balance: 0
  api_key_prefix: "sk-"
  rate_multiplier: 1.0

ops:
  enabled: true

dashboard_aggregation:
  enabled: true
  interval_seconds: 60

dashboard_cache:
  enabled: true
  key_prefix: "sub2api:"
EOF

    chmod 640 "$CONFIG_DIR/config.yaml"
    chown "$SERVICE_USER:$SERVICE_USER" "$CONFIG_DIR/config.yaml"

    success "Config generated: $CONFIG_DIR/config.yaml"
}

# ============================================================
# Install systemd service
# ============================================================
install_service() {
    info "Installing systemd service..."

    # Copy config.yaml to install dir as well (for config path resolution)
    cp "$CONFIG_DIR/config.yaml" "$INSTALL_DIR/config.yaml"
    chown "$SERVICE_USER:$SERVICE_USER" "$INSTALL_DIR/config.yaml"

    cat > "/etc/systemd/system/${SERVICE_NAME}.service" << EOF
[Unit]
Description=Sub2API - AI API Gateway Platform
Documentation=https://github.com/Wei-Shaw/sub2api
After=network.target postgresql.service redis-server.service
Wants=postgresql.service redis-server.service

[Service]
Type=simple
User=${SERVICE_USER}
Group=${SERVICE_USER}
WorkingDirectory=${INSTALL_DIR}
ExecStart=${INSTALL_DIR}/sub2api
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal
SyslogIdentifier=${SERVICE_NAME}

# Security hardening
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
PrivateTmp=true
ReadWritePaths=${INSTALL_DIR}

# Environment
Environment=GIN_MODE=release
Environment=SERVER_HOST=127.0.0.1
Environment=SERVER_PORT=${SERVER_PORT}
Environment=TZ=Asia/Shanghai

# Resource limits
LimitNOFILE=65536
LimitNPROC=65536

[Install]
WantedBy=multi-user.target
EOF

    systemctl daemon-reload
    success "systemd service installed"
}

# ============================================================
# Configure Nginx
# ============================================================
setup_nginx() {
    info "Configuring Nginx reverse proxy..."

    local server_name="_"
    if [ -n "$DOMAIN" ]; then
        server_name="$DOMAIN"
    fi

    cat > "${NGINX_CONF_DIR}/sub2api" << 'NGINX_EOF'
# Sub2API Nginx Configuration
# This file is auto-generated by deploy-ubuntu.sh

# Upstream definition
upstream sub2api_backend {
    server 127.0.0.1:SERVER_PORT_PLACEHOLDER;
    keepalive 32;
}

# WebSocket upgrade mapping: only forward "upgrade" when the client asked for it.
# An empty value keeps the upstream connection on HTTP/1.1 keepalive, so ordinary
# API requests are no longer forced into upgrade mode.
map $http_upgrade $connection_upgrade {
    default upgrade;
    ''      '';
}

# Rate limiting
limit_req_zone $binary_remote_addr zone=sub2api_api:10m rate=100r/s;
limit_req_zone $binary_remote_addr zone=sub2api_static:10m rate=200r/s;

# Connection limiting
limit_conn_zone $binary_remote_addr zone=sub2api_conn:10m;

server {
    listen 80;
    SERVER_NAME_PLACEHOLDER

    client_max_body_size 256m;

    # Security headers
    # nosniff / X-Frame-Options / Referrer-Policy / CSP 均由后端 SecurityHeaders
    # 中间件输出；此处不再重复，避免同一响应出现冲突的双值头。
    # 这里只补后端不输出的 HSTS（需配合 HTTPS 生效）。
    add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;

    # Gzip compression
    gzip on;
    gzip_vary on;
    gzip_min_length 1024;
    gzip_comp_level 6;
    gzip_types
        text/plain
        text/css
        text/xml
        text/javascript
        application/json
        application/javascript
        application/xml
        application/xml+rss
        application/x-font-ttf
        font/opentype
        image/svg+xml
        image/x-icon;

    # API routes - proxy to backend
    location ~ ^/(api|v1|v1beta|backend-api|antigravity|setup|health|responses|images)/ {
        limit_req zone=sub2api_api burst=200 nodelay;
        limit_conn sub2api_conn 20;

        proxy_pass http://sub2api_backend;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        # WebSocket support: Connection 取自上面的 map，普通请求保持 keepalive
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection $connection_upgrade;

        # Timeouts for long-running AI requests
        proxy_connect_timeout 60s;
        proxy_send_timeout 600s;
        proxy_read_timeout 600s;

        # Buffer settings
        proxy_buffering off;
        proxy_cache off;
    }

    # Sora media proxy
    location /sora/media/ {
        limit_req zone=sub2api_api burst=50 nodelay;

        proxy_pass http://sub2api_backend;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        proxy_connect_timeout 60s;
        proxy_send_timeout 300s;
        proxy_read_timeout 300s;
    }

    # Static assets (embedded in backend, proxied)
    location ~* \.(js|css|png|jpg|jpeg|gif|ico|svg|woff|woff2|ttf|eot|map)$ {
        limit_req zone=sub2api_static burst=500 nodelay;

        proxy_pass http://sub2api_backend;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # Cache static assets
        proxy_cache_valid 200 1h;
        expires 1h;
        add_header Cache-Control "public, immutable";
        # 本 location 自带 add_header，会屏蔽 server 级头，HSTS 需在此补齐
        add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;
    }

    # Main application (SPA fallback)
    location / {
        limit_req zone=sub2api_api burst=100 nodelay;
        limit_conn sub2api_conn 20;

        proxy_pass http://sub2api_backend;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header Connection "";

        # Timeouts
        proxy_connect_timeout 60s;
        proxy_send_timeout 600s;
        proxy_read_timeout 600s;
    }

    # Health check endpoint
    location = /health {
        proxy_pass http://sub2api_backend;
        access_log off;
    }

    # Block sensitive paths
    location ~ /\.(git|env|config) {
        deny all;
        access_log off;
        log_not_found off;
    }
}
NGINX_EOF

    # Replace placeholders
    sed -i "s/SERVER_PORT_PLACEHOLDER/${SERVER_PORT}/g" "${NGINX_CONF_DIR}/sub2api"

    if [ -n "$DOMAIN" ]; then
        sed -i "s/SERVER_NAME_PLACEHOLDER/server_name ${DOMAIN};/" "${NGINX_CONF_DIR}/sub2api"
    else
        sed -i "s/SERVER_NAME_PLACEHOLDER/server_name _;/" "${NGINX_CONF_DIR}/sub2api"
    fi

    # Enable site
    ln -sf "${NGINX_CONF_DIR}/sub2api" "${NGINX_ENABLE_DIR}/sub2api"

    # Remove default site if it conflicts
    if [ -f "${NGINX_ENABLE_DIR}/default" ]; then
        warn "Removing default Nginx site to avoid conflicts"
        rm -f "${NGINX_ENABLE_DIR}/default"
    fi

    # Test Nginx config
    if nginx -t 2>/dev/null; then
        systemctl reload nginx
        success "Nginx configured and reloaded"
    else
        error "Nginx configuration test failed!"
        nginx -t
        exit 1
    fi
}

# ============================================================
# Setup SSL with Let's Encrypt
# ============================================================
setup_ssl() {
    if [ "$ENABLE_SSL" = false ]; then
        return 0
    fi

    if [ -z "$DOMAIN" ]; then
        error "SSL requires a domain name. Use --domain <your-domain>"
        exit 1
    fi

    info "Setting up SSL with Let's Encrypt for $DOMAIN..."

    # Install certbot
    if ! command -v certbot &>/dev/null; then
        info "Installing certbot..."
        apt-get install -y -qq certbot python3-certbot-nginx >/dev/null 2>&1
    fi

    # Obtain SSL certificate
    certbot --nginx -d "$DOMAIN" \
        --non-interactive \
        --agree-tos \
        --register-unsafely-without-email \
        --redirect \
        2>/dev/null || {
        warn "Automatic SSL setup failed. You can set it up manually:"
        echo "  sudo certbot --nginx -d $DOMAIN"
    }

    success "SSL configured for $DOMAIN"
}

# ============================================================
# Start service
# ============================================================
start_service() {
    info "Starting Sub2API service..."

    systemctl start "$SERVICE_NAME"
    systemctl enable "$SERVICE_NAME" >/dev/null 2>&1

    # Wait for service to start
    sleep 2

    if systemctl is-active --quiet "$SERVICE_NAME"; then
        success "Sub2API service is running"
    else
        error "Service failed to start!"
        echo ""
        echo "  Check logs with:"
        echo "    sudo journalctl -u $SERVICE_NAME -n 50"
        exit 1
    fi
}

# ============================================================
# Verify deployment
# ============================================================
verify_deployment() {
    info "Verifying deployment..."

    sleep 3

    # Check health endpoint
    local health_response
    health_response=$(curl -s -o /dev/null -w "%{http_code}" "http://127.0.0.1:${SERVER_PORT}/health" 2>/dev/null || echo "000")

    if [ "$health_response" = "200" ]; then
        success "Health check passed (HTTP 200)"
    else
        warn "Health check returned: $health_response (service may still be starting up)"
        info "  Check logs: sudo journalctl -u $SERVICE_NAME -f"
    fi
}

# ============================================================
# Print completion
# ============================================================
print_completion() {
    local access_url="http://$(hostname -I 2>/dev/null | awk '{print $1}' || echo 'SERVER_IP')"

    if [ -n "$DOMAIN" ]; then
        if [ "$ENABLE_SSL" = true ]; then
            access_url="https://$DOMAIN"
        else
            access_url="http://$DOMAIN"
        fi
    fi

    echo ""
    echo "=============================================="
    success "Deployment Complete!"
    echo "=============================================="
    echo ""
    echo "  Access URL:  $access_url"
    echo "  Backend:     127.0.0.1:${SERVER_PORT}"
    echo "  Config:      $CONFIG_DIR/config.yaml"
    echo "  Binary:      $INSTALL_DIR/sub2api"
    echo ""
    echo "=============================================="
    echo "  Service Commands"
    echo "=============================================="
    echo ""
    echo "  Status:    sudo systemctl status $SERVICE_NAME"
    echo "  Start:     sudo systemctl start $SERVICE_NAME"
    echo "  Stop:      sudo systemctl stop $SERVICE_NAME"
    echo "  Restart:   sudo systemctl restart $SERVICE_NAME"
    echo "  Logs:      sudo journalctl -u $SERVICE_NAME -f"
    echo ""
    echo "=============================================="
    echo "  Nginx Commands"
    echo "=============================================="
    echo ""
    echo "  Test:      sudo nginx -t"
    echo "  Reload:    sudo systemctl reload nginx"
    echo "  Status:    sudo systemctl status nginx"
    echo ""
    echo "=============================================="
    echo "  Database"
    echo "=============================================="
    echo ""
    echo "  Connect:   sudo -u postgres psql -d $DB_NAME"
    echo "  User:      $DB_USER"
    echo "  Database:  $DB_NAME"
    echo ""
    echo "=============================================="
}

# ============================================================
# Main
# ============================================================
main() {
    echo ""
    echo "=============================================="
    echo "  Sub2API Ubuntu Deployment"
    echo "=============================================="
    echo ""

    check_root
    check_prerequisites
    deploy_binary
    generate_config
    install_service
    setup_nginx
    setup_ssl
    start_service
    verify_deployment
    print_completion
}

main "$@"
