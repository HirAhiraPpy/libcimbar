# Nginx WebSocket 代理配置

## 问题症状

如果你遇到以下问题：
- 手机浏览器显示"连接中..."但一直无法连接
- 服务端没有客户端连接的日志
- 控制台显示 WebSocket 连接失败

这很可能是 Nginx 没有正确配置 WebSocket 代理。

## 完整的 Nginx 配置示例

```nginx
server {
    listen 443 ssl http2;
    server_name your-domain.com;

    # SSL 证书配置
    ssl_certificate /path/to/fullchain.pem;
    ssl_certificate_key /path/to/privkey.pem;
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers HIGH:!aNULL:!MD5;

    # WebSocket 支持
    map $http_upgrade $connection_upgrade {
        default upgrade;
        ''      close;
    }

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;

        # WebSocket 必需的 headers
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection $connection_upgrade;

        # 其他必要的 headers
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # 超时设置（WebSocket 需要长连接）
        proxy_read_timeout 86400s;
        proxy_send_timeout 86400s;
        proxy_connect_timeout 60s;

        # 禁用缓冲
        proxy_buffering off;
        proxy_cache off;
    }
}

# HTTP 重定向到 HTTPS
server {
    listen 80;
    server_name your-domain.com;
    return 301 https://$server_name$request_uri;
}
```

## 关键配置说明

### 1. `map` 指令（必须放在 http 块中）

```nginx
http {
    map $http_upgrade $connection_upgrade {
        default upgrade;
        ''      close;
    }
    ...
}
```

这个 `map` 指令必须放在 `http` 块中，不能在 `server` 块中。如果已经放在 `server` 块中，请移到 `http` 块。

### 2. WebSocket headers

```nginx
proxy_set_header Upgrade $http_upgrade;
proxy_set_header Connection $connection_upgrade;
```

这两个 headers 是 WebSocket 握手的必需项。

### 3. 超时设置

```nginx
proxy_read_timeout 86400s;
proxy_send_timeout 86400s;
```

WebSocket 是长连接，需要设置较长的超时时间。

## 最小可用配置

如果你只需要测试，可以使用以下最小配置：

```nginx
server {
    listen 443 ssl;
    server_name your-domain.com;

    ssl_certificate /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
    }
}
```

## 验证配置

1. **测试 Nginx 配置**:
```bash
sudo nginx -t
```

2. **重新加载 Nginx**:
```bash
sudo nginx -s reload
```

3. **检查 Nginx 错误日志**:
```bash
sudo tail -f /var/log/nginx/error.log
```

4. **检查访问日志**:
```bash
sudo tail -f /var/log/nginx/access.log
```

## 常见问题排查

### 1. 404 错误

确保 `proxy_pass` 地址正确，cimbar 服务器正在运行：
```bash
ps aux | grep cimbar-server
```

### 2. 502 Bad Gateway

cimbar 服务器没有运行或监听端口不对：
```bash
netstat -tlnp | grep 8080
```

### 3. WebSocket 连接立即关闭

检查 `map` 指令是否正确配置，确保 `Connection` header 设置正确。

### 4. SSL 证书问题

使用 Let's Encrypt 免费证书：
```bash
sudo apt install certbot python3-certbot-nginx
sudo certbot --nginx -d your-domain.com
```

## 完整的 /etc/nginx/nginx.conf 示例

```nginx
user www-data;
worker_processes auto;
pid /run/nginx.pid;
include /etc/nginx/modules-enabled/*.conf;

events {
    worker_connections 768;
}

http {
    # Basic Settings
    sendfile on;
    tcp_nopush on;
    types_hash_max_size 2048;
    include /etc/nginx/mime.types;
    default_type application/octet-stream;

    # Logging Settings
    access_log /var/log/nginx/access.log;
    error_log /var/log/nginx/error.log;

    # WebSocket support (必须放在这里！)
    map $http_upgrade $connection_upgrade {
        default upgrade;
        ''      close;
    }

    # Virtual Host Configs
    include /etc/nginx/conf.d/*.conf;
    include /etc/nginx/sites-enabled/*;
}
```

## 快速测试

使用以下命令测试 WebSocket 连接：

```bash
# 在服务器上测试（localhost）
curl -i -N -H "Connection: Upgrade" -H "Upgrade: websocket" \
  -H "Host: localhost:8080" \
  http://localhost:8080/ws

# 通过 Nginx 测试（使用你的域名）
curl -i -N -H "Connection: Upgrade" -H "Upgrade: websocket" \
  -H "Host: your-domain.com" \
  https://your-domain.com/ws
```

## 手机浏览器调试

在 Chrome for Android 上：

1. 访问 `chrome://inspect`（在桌面 Chrome）
2. 选择你的设备
3. 检查 WebSocket 连接状态
4. 查看控制台错误信息

## 总结配置清单

- [ ] `map` 指令在 `http` 块中
- [ ] `proxy_http_version 1.1`
- [ ] `proxy_set_header Upgrade $http_upgrade`
- [ ] `proxy_set_header Connection $connection_upgrade`
- [ ] 足够长的超时设置
- [ ] SSL 证书配置正确
- [ ] cimbar 服务器正在运行
