# Cimbar 编码器部署指南

## 快速部署（开发测试）

### 1. 构建 WASM 文件
```bash
# 使用 emscripten docker 容器
docker run --mount type=bind,source="$(pwd)",target="/usr/src/app" -it emscripten/emsdk:3.1.39 bash

# 在容器内执行:
bash package-wasm.sh
```

### 2. 加密 WASM
```bash
# 设置口令
PASSWORD="your-secret-password"

# 加密
node scripts/encrypt-wasm.js web/cimbar_js.wasm web/cimbar_js.wasm.enc "$PASSWORD"
```

### 3. 本地测试
```bash
# 使用 Python 简单 HTTP 服务器
cd web/
python3 -m http.server 8080

# 访问 http://localhost:8080/index.html
```

---

## 生产部署

### 文件清单
部署以下文件到 Web 服务器：

```
web/
├── index.html              # 主页面（深色霓虹主题）
├── main.js                 # 编码器逻辑
├── wasm-decrypt.js         # WASM 解密模块
├── cimbar_js.wasm.enc      # 加密的 WASM 文件
├── cimbar_js.js            # WASM 加载器（如需要）
└── sw.js                   # Service Worker
```

### Nginx 配置示例

```nginx
server {
    listen 80;
    server_name cimbar.example.com;

    # 强制 HTTPS
    return 301 https://$server_name$request_uri;
}

server {
    listen 443 ssl http2;
    server_name cimbar.example.com;

    ssl_certificate /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;

    root /var/www/cimbar;
    index index.html;

    # 文件类型
    location ~* \.(wasm|enc|js|css)$ {
        expires 7d;
        add_header Cache-Control "public, immutable";
    }

    location ~* \.html$ {
        expires -1;
        add_header Cache-Control "no-cache";
    }

    # Service Worker
    location = /sw.js {
        add_header Cache-Control "no-cache";
        add_header Service-Worker-Allowed "/";
    }

    # 安全头
    add_header X-Frame-Options "SAMEORIGIN";
    add_header X-Content-Type-Options "nosniff";
    add_header Referrer-Policy "strict-origin-when-cross-origin";

    # Content Security Policy (根据实际需求调整)
    add_header Content-Security-Policy "default-src 'self'; script-src 'self' 'unsafe-inline'; worker-src 'self' blob:; connect-src 'self' blob:; img-src 'self' blob: data:; style-src 'self' 'unsafe-inline';";
}
```

### Apache 配置示例

```apache
<VirtualHost *:443>
    ServerName cimbar.example.com

    DocumentRoot /var/www/cimbar

    SSLEngine on
    SSLCertificateFile /path/to/cert.pem
    SSLCertificateKeyFile /path/to/key.pem

    <Directory /var/www/cimbar>
        Options -Indexes
        AllowOverride None
        Require all granted

        # 安全头
        Header set X-Frame-Options "SAMEORIGIN"
        Header set X-Content-Type-Options "nosniff"
        Header set Referrer-Policy "strict-origin-when-cross-origin"
    </Directory>

    # WASM 和加密文件
    <FilesMatch "\.(wasm|enc)$">
        Header set Cache-Control "public, max-age=604800, immutable"
    </FilesMatch>

    # HTML 文件不缓存
    <FilesMatch "\.html$">
        Header set Cache-Control "no-cache"
    </FilesMatch>

    # Service Worker
    <Files "sw.js">
        Header set Cache-Control "no-cache"
        Header set Service-Worker-Allowed "/"
    </Files>
</VirtualHost>
```

---

## 使用单文件部署

如果需要单文件部署（所有资源嵌入 HTML）：

```bash
# 生成包含加密 WASM 的单文件
node scripts/generate-encrypted-html.js "your-password"

# 输出：web/encoder-encrypted.html
```

**注意**: 由于 WASM 文件较大（~2MB），生成的 HTML 文件也会很大。建议：
- 使用外部 `.enc` 文件方式
- 或将 HTML 和 `.enc` 文件一起部署

---

## 口令管理

### 设置强口令
- 至少 12 个字符
- 包含大小写字母、数字、符号
- 避免常见单词

### 更换口令
```bash
# 删除旧的加密文件
rm web/cimbar_js.wasm.enc

# 生成新的加密文件
node scripts/encrypt-wasm.js web/cimbar_js.wasm web/cimbar_js.wasm.enc "new-password"
```

### 分发口令
- 通过安全渠道分发给授权用户
- 考虑使用密码管理器
- 定期更换

---

## 故障排除

### WASM 加载失败
```
错误：无法加载加密 WASM 文件
```
**解决**：
1. 检查 `cimbar_js.wasm.enc` 是否存在
2. 确认 Web 服务器配置正确（MIME 类型）
3. 检查浏览器控制台错误

### 口令错误
```
错误：口令错误或数据损坏
```
**解决**：
1. 确认输入的口令与加密时一致
2. 检查是否有拼写错误（大小写敏感）
3. 如忘记口令，需要重新加密 WASM

### Service Worker 问题
```
Service Worker 注册失败
```
**解决**：
1. 确保使用 HTTPS 或 localhost
2. 检查 `sw.js` 路径正确
3. 清除浏览器缓存

### 移动端问题
```
编码器无法启动
```
**解决**：
1. 确保使用现代浏览器（Chrome、Safari）
2. 检查 WebGL 支持
3. 尝试横屏模式

---

## 性能优化

### 启用 Gzip/Brotli 压缩
```nginx
# Nginx Gzip
gzip on;
gzip_types application/wasm text/javascript;
gzip_min_length 1000;
```

```apache
# Apache Brotli
<IfModule mod_brotli.c>
    AddOutputFilterByType BROTLI_COMPRESS application/wasm text/javascript
</IfModule>
```

### CDN 部署
将静态资源部署到 CDN：
- Cloudflare
- AWS CloudFront
- Azure CDN

---

## 监控和日志

### Nginx 访问日志
```nginx
log_format cimbar '$remote_addr - $remote_user [$time_local] '
                  '"$request" $status $body_bytes_sent '
                  '"$http_referer" "$http_user_agent"';

access_log /var/log/nginx/cimbar-access.log cimbar;
error_log /var/log/nginx/cimbar-error.log;
```

### 监控指标
- 页面加载时间
- WASM 解密成功率
- 口令错误率
- 并发用户数

---

## 安全检查清单

- [ ] 使用 HTTPS
- [ ] 设置强口令
- [ ] 配置 CSP 头
- [ ] 禁用目录浏览
- [ ] 定期更新依赖
- [ ] 监控异常访问
- [ ] 备份加密文件
- [ ] 文档化部署流程

---

## 支持

遇到问题？
1. 查看 [IMPLEMENTATION_SUMMARY.md](IMPLEMENTATION_SUMMARY.md)
2. 查看 [scripts/README.md](scripts/README.md)
3. 检查浏览器控制台错误
4. 联系管理员

---

**最后更新**: 2026-02-28
