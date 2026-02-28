# 故障排查指南

## 已修复的问题

### 1. WebSocket 连接失败（一直显示"连接中..."）

**原因**: HTTPS 页面不能使用 `ws://` 协议，必须使用 `wss://`

**修复**:
- 已修改 `capture.js` 自动检测协议
- HTTPS 页面使用 `wss://`
- HTTP 页面使用 `ws://`

### 2. 摄像头画面被水平翻转

**原因**: CSS 样式 `transform: scaleX(-1)` 用于前置摄像头镜像效果

**修复**:
- 已移除该样式，后置摄像头不需要镜像

## 问题排查步骤

### 步骤 1: 检查服务器是否运行

```bash
# 检查进程
ps aux | grep cimbar-server

# 检查端口
netstat -tlnp | grep 8080
# 或
ss -tlnp | grep 8080
```

如果服务器没有运行：
```bash
cd ~/workdir/libcimbar
./build-cgo/bin/cimbar-server --addr :8080 --output-dir /tmp/cimbar
```

### 步骤 2: 检查 Nginx 配置

```bash
# 测试配置
sudo nginx -t

# 查看错误日志
sudo tail -f /var/log/nginx/error.log

# 重新加载配置
sudo nginx -s reload
```

确保 Nginx 配置包含 WebSocket 支持：
```nginx
location / {
    proxy_pass http://127.0.0.1:8080;
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection $connection_upgrade;
}
```

### 步骤 3: 测试 WebSocket 连接

在服务器上直接测试（绕过 Nginx）：
```bash
# 安装 wscat
npm install -g wscat

# 测试本地连接
wscat -c ws://localhost:8080/ws

# 如果看到 "connected"，说明服务器正常
```

通过 Nginx 测试：
```bash
# 测试 HTTPS 连接
wscat -c wss://your-domain.com/ws
```

### 步骤 4: 检查防火墙

```bash
# Ubuntu/Debian
sudo ufw status
sudo ufw allow 443/tcp  # HTTPS

# CentOS/RHEL
sudo firewall-cmd --list-all
sudo firewall-cmd --add-port=443/tcp --permanent
sudo firewall-cmd --reload
```

### 步骤 5: 查看服务器日志

启动服务器时查看输出：
```bash
./build-cgo/bin/cimbar-server --addr :8080 --output-dir /tmp/cimbar

# 应该看到：
# Starting cimbar web server on :8080
# Web UI: http://:8080
# Output directory: /tmp/cimbar
```

当手机连接时，应该看到：
```
WebSocket request received from xxx.xxx.xxx.xxx:port
Upgrade header: websocket
Connection header: Upgrade
✓ Client connected: xxx.xxx.xxx.xxx:port
```

### 步骤 6: 手机浏览器调试

**Chrome for Android**:

1. 在手机 Chrome 中访问 `chrome://inspect`
2. 启用 USB 调试
3. 用 USB 连接手机和电脑
4. 在电脑的 Chrome 中访问 `chrome://inspect/#devices`
5. 可以看到手机浏览器页面，点击"inspect"查看控制台

**查看控制台错误**:
- 按 F12 打开开发者工具
- 查看 Console 标签
- 查找 WebSocket 相关错误

### 步骤 7: 测试摄像头权限

在浏览器地址栏输入：
```
chrome://settings/content/camera
```

确保：
- 摄像头权限已开启
- 你的网站没有被阻止

## 常见错误及解决方案

### 错误 1: "WebSocket connection failed"

**可能原因**:
1. Nginx 没有 WebSocket 配置
2. 防火墙阻止连接
3. 服务器没有运行

**解决**:
1. 检查 Nginx 配置（见步骤 2）
2. 检查防火墙（见步骤 4）
3. 重启服务器

### 错误 2: "Failed to execute 'getUserMedia' on 'Navigator'"

**可能原因**:
1. 非 HTTPS 环境
2. 摄像头权限被拒绝
3. 摄像头被其他程序占用

**解决**:
1. 使用 HTTPS（见 HTTPS_SETUP.md）
2. 在浏览器设置中允许摄像头
3. 关闭其他使用摄像头的程序

### 错误 3: "502 Bad Gateway"

**可能原因**:
- cimbar 服务器没有运行

**解决**:
```bash
# 检查进程
ps aux | grep cimbar-server

# 如果没有，启动服务器
./build-cgo/bin/cimbar-server --addr :8080 --output-dir /tmp/cimbar
```

### 错误 4: 连接成功但无法解码

**可能原因**:
1. 图像质量差
2. 光线不足
3. 距离太远或太近

**解决**:
1. 确保 Cimbar 条码清晰显示
2. 增加环境光线
3. 调整距离，使条码填满扫描框

## 快速诊断命令

```bash
# 1. 检查服务器
ps aux | grep cimbar-server && echo "✓ Server running" || echo "✗ Server not running"

# 2. 检查端口
ss -tlnp | grep 8080 && echo "✓ Port 8080 listening" || echo "✗ Port 8080 not listening"

# 3. 检查 Nginx
sudo nginx -t && echo "✓ Nginx config OK" || echo "✗ Nginx config error"

# 4. 检查防火墙
sudo ufw status | grep 443 && echo "✓ Port 443 allowed" || echo "✗ Port 443 blocked"

# 5. 测试 WebSocket
curl -i -N -H "Connection: Upgrade" -H "Upgrade: websocket" http://localhost:8080/ws
```

## 完整测试流程

1. **启动服务器**
   ```bash
   ./build-cgo/bin/cimbar-server --addr :8080 --output-dir /tmp/cimbar
   ```

2. **查看服务器日志**
   - 应该看到 "Starting cimbar web server on :8080"

3. **手机访问**
   - 打开 https://your-domain.com
   - 允许摄像头权限
   - 查看控制台日志

4. **检查连接**
   - 服务器日志应该显示 "Client connected"
   - 页面左上角应该显示"已连接"

5. **测试解码**
   - 用 cimbar_send 或其他工具显示 Cimbar 条码
   - 用手机摄像头对准
   - 观察扫描框颜色变化（绿→黄→蓝）

## 联系支持

如果以上步骤都无法解决问题，请提供以下信息：

1. 服务器日志输出
2. Nginx 错误日志 (`/var/log/nginx/error.log`)
3. 手机浏览器控制台截图
4. 你的 Nginx 配置（隐藏敏感信息）
