# HTTPS 配置指南

## 为什么需要 HTTPS

现代浏览器（特别是 Chrome 和 Safari）要求摄像头访问必须在**安全上下文**（Secure Context）中进行：

- **HTTPS** - 安全的 HTTPS 连接
- **localhost** - 本地回环地址（包括 127.0.0.1）
- **file://** - 本地文件（不适用于本服务器）

如果你在手机上通过非 HTTPS 的 IP 地址（如 `http://192.168.1.100:8080`）访问服务器，摄像头将无法工作。

## 解决方案

### 方案 1: 使用 ngrok（最简单，推荐）

[ngrok](https://ngrok.com) 可以提供公共 HTTPS 隧道到你的本地服务器：

```bash
# 1. 安装 ngrok
# 下载：https://ngrok.com/download

# 2. 启动 cimbar 服务器
./build-cgo/bin/cimbar-server --addr :8080 --output-dir /tmp/cimbar

# 3. 在另一个终端启动 ngrok
ngrok http 8080

# 4. ngrok 会显示一个 HTTPS 地址
# 用手机访问该 HTTPS 地址即可
```

### 方案 2: 使用 Caddy（自动 HTTPS）

[Caddy](https://caddyserver.com) 是一个自动启用 HTTPS 的 Web 服务器：

```bash
# 1. 安装 Caddy
# https://caddyserver.com/docs/install

# 2. 创建 Caddyfile
cat > Caddyfile << EOF
your-domain.com {
    reverse_proxy localhost:8080
}
EOF

# 3. 启动 Caddy
caddy run

# 4. 确保你的域名 DNS 指向你的服务器 IP
```

### 方案 3: 使用 nginx（需要自己的域名和证书）

```nginx
server {
    listen 443 ssl;
    server_name your-domain.com;

    ssl_certificate /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;

    location / {
        proxy_pass http://localhost:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_read_timeout 86400;
    }
}
```

### 方案 4: 自签名证书（仅用于测试）

```bash
# 1. 生成自签名证书
openssl req -x509 -newkey rsa:4096 -keyout key.pem -out cert.pem -days 365 -nodes

# 2. 使用支持 HTTPS 的工具（如 mkcert）
# 或使用 nginx/Caddy 反向代理
```

**注意**: 自签名证书需要在手机上手动信任，不太方便。

### 方案 5: Chrome 开发者模式（仅 Android Chrome，仅开发用）

1. 在手机 Chrome 中访问 `chrome://flags`
2. 搜索 "unsafely-treat-insecure-origin-as-secure"
3. 启用该标志
4. 添加你的服务器地址（如 `http://192.168.1.100:8080`）
5. 重启 Chrome

**警告**: 这只适用于开发环境，不应在生产环境中使用。

## 局域网访问设置

### 获取服务器 IP 地址

```bash
# Linux
ip addr show | grep "inet "

# 通常会显示类似 192.168.x.x 或 10.x.x.x 的地址
```

### 防火墙设置

确保服务器防火墙允许 8080 端口（或你使用的端口）：

```bash
# Ubuntu/Debian (ufw)
sudo ufw allow 8080/tcp

# CentOS/RHEL (firewalld)
sudo firewall-cmd --add-port=8080/tcp --permanent
sudo firewall-cmd --reload
```

## 完整的 ngrok 示例

```bash
# 终端 1: 启动 cimbar 服务器
cd ~/workdir/libcimbar
./build-cgo/bin/cimbar-server --addr :8080 --output-dir /tmp/cimbar

# 终端 2: 启动 ngrok
ngrok http 8080

# 输出示例：
# Forwarding  https://abc123.ngrok.io -> http://localhost:8080

# 用手机访问显示的 HTTPS 地址即可
```

## 推荐的 mkcert 方案（本地开发）

[mkcert](https://github.com/FiloSottile/mkcert) 可以创建受信任的自签名证书：

```bash
# 1. 安装 mkcert
# macOS
brew install mkcert nss

# Linux
sudo apt install libnss3-tools
curl -JLO "https://github.com/FiloSottile/mkcert/releases/latest/download/mkcert-linux-amd64"
sudo cp mkcert-linux-amd64 /usr/local/bin/mkcert

# 2. 安装本地 CA
mkcert -install

# 3. 为你的服务器创建证书
mkcert localhost 127.0.0.1 ::1

# 4. 使用支持 HTTPS 的工具
# 例如使用 caddy 或 nginx 反向代理
# 或使用 go 的内置 HTTPS 服务器
```

## 总结

| 方案 | 难度 | 适用场景 |
|------|------|----------|
| ngrok | ⭐ | 快速测试，临时演示 |
| Caddy | ⭐⭐ | 长期使用，有自己的域名 |
| nginx + Let's Encrypt | ⭐⭐⭐ | 生产环境 |
| Chrome 开发者模式 | ⭐ | 仅开发测试 |
| 自签名证书 | ⭐⭐ | 内部测试 |

**推荐**:
- 开发测试：使用 localhost 或 ngrok
- 生产环境：使用 Caddy 或 nginx + Let's Encrypt
