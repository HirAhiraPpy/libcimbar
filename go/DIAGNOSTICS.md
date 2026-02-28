# 诊断指南 - 摄像头识别问题

## 已添加的调试功能

### 前端调试信息

**页面左下角调试面板**:
```
帧：XXX | 响应：XXX | 尺寸：1280x720
```

- **帧**: 已发送到服务器的帧数
- **响应**: 收到服务器响应的次数
- **尺寸**: 摄像头分辨率

**状态栏信息**（页面底部）:
- `已发送 XX 帧` - 每 30 帧更新一次
- `扫描中...` - 服务器收到帧但没有识别到数据
- `解码成功！XX 字节` - 服务器成功提取数据
- `解码中...` - 正在处理解码

**扫描框颜色**:
- 🟢 **绿色** - 等待/空闲状态
- 🟡 **黄色** - 正在扫描（服务器收到帧但没有数据）
- 🔵 **蓝色** - 解码成功（识别到 Cimbar 数据）

### 后端日志

**服务器启动日志**:
```
Starting cimbar web server on :8080
Web UI: http://:8080
Output directory: /tmp/cimbar
Press Ctrl+C to stop
```

**WebSocket 连接日志**:
```
WebSocket request received from 192.168.1.100:12345
Upgrade header: websocket
Connection header: Upgrade
✓ Client connected: 192.168.1.100:12345
```

**帧统计日志**（每 5 秒）:
```
Frame stats: received=150
```

**解码日志**（每 10 帧）:
```
Frame 150: 1280x720, result=0 bytes, extracted=false, failed=false
```

## 诊断步骤

### 步骤 1: 检查前端调试面板

打开手机浏览器，查看左下角的调试面板：

**情况 A: 帧数不断增加，响应数为 0**
```
帧：150 | 响应：0 | 尺寸：1280x720
```
**原因**: 服务器没有返回响应
**排查**:
1. 检查服务器日志，看是否收到帧
2. 检查服务器是否报错

**情况 B: 帧数和响应数都增加**
```
帧：150 | 响应：150 | 尺寸：1280x720
```
**原因**: 通信正常，但可能没有识别到 Cimbar
**排查**:
1. 确保 Cimbar 条码清晰显示
2. 调整距离和角度
3. 检查扫描框颜色变化

**情况 C: 帧数不增加**
```
帧：0 | 响应：0 | 尺寸：-
```
**原因**: 摄像头没有工作
**排查**:
1. 检查摄像头权限
2. 刷新页面重新授权
3. 检查控制台错误

### 步骤 2: 检查服务器日志

在运行服务器的终端查看日志：

**正常接收帧的日志**:
```
Frame stats: received=150
Frame 150: 1280x720, result=0 bytes, extracted=false, failed=false
```

**`result=0 bytes, extracted=false`** 表示：
- 服务器收到了帧
- 但没有识别到 Cimbar 网格
- 这是正常的，当没有 Cimbar 条码在画面中时

**识别到 Cimbar 的日志**:
```
Frame 200: 1280x720, result=744 bytes, extracted=true, failed=false
```

**`result=744 bytes, extracted=true`** 表示：
- 成功识别 Cimbar
- 提取了 744 字节数据
- 扫描框应该变成黄色或蓝色

### 步骤 3: 浏览器控制台调试

在 Chrome for Android 上：

1. 手机设置 → 开发者选项 → 启用 USB 调试
2. 用 USB 连接手机和电脑
3. 电脑 Chrome 访问 `chrome://inspect/#devices`
4. 点击你的手机页面 → "inspect"
5. 查看 Console 标签

**查找这些日志**:
```
Connecting to WebSocket: wss://your-domain.com/ws
WebSocket connected!
Frame 10: Sent 1280x720 (3686406 bytes)
Server response: {type: "decode", nodata: true}
```

### 步骤 4: 测试 Cimbar 识别

**使用 cimbar_send 显示条码**:
```bash
# 在另一台机器或同一台机器上
./build-cgo/bin/cimbar_send /path/to/file.txt
```

**或使用 cimbar.org**:
1. 访问 https://cimbar.org
2. 选择文件编码
3. 屏幕会显示 Cimbar 条码

**手机扫描**:
1. 打开你的网页
2. 对准显示 Cimbar 的屏幕
3. 距离约 20-30cm
4. 观察扫描框颜色变化

## 常见问题诊断

### 问题 1: 帧数增加但没有响应

**症状**:
```
帧：100 | 响应：0
```

**可能原因**:
1. 服务器解码错误
2. 图像格式问题
3. 服务器崩溃

**检查服务器日志**:
```bash
# 查看是否有错误
tail -f /path/to/server.log
```

### 问题 2: 扫描框一直是绿色

**症状**:
- 帧数正常增加
- 响应数也正常
- 但扫描框一直是绿色

**可能原因**:
1. 画面中没有 Cimbar 条码
2. Cimbar 条码太小或太远
3. 光线太暗或反光

**解决**:
1. 确保 Cimbar 条码填满扫描框
2. 增加环境光线
3. 避免反光

### 问题 3: 闪退或黑屏

**可能原因**:
1. 摄像头被其他应用占用
2. 内存不足
3. 浏览器 bug

**解决**:
1. 关闭其他使用摄像头的应用
2. 刷新页面
3. 重启浏览器

## 调试模式

### 启用详细日志

在服务器启动时设置环境变量：
```bash
DEBUG=1 ./build-cgo/bin/cimbar-server --addr :8080 --output-dir /tmp/cimbar
```

### 前端调试命令

在手机浏览器控制台输入：
```javascript
// 显示详细日志
localStorage.setItem('debug', 'true');
location.reload();

// 捕获单帧测试
captureFrame();

// 查看当前状态
console.log('WebSocket state:', ws.readyState);
console.log('Camera state:', video.readyState);
```

## 性能优化建议

### 如果帧率太低

1. 降低摄像头分辨率（在 capture.js 中修改 constraints）
2. 增加 CAPTURE_INTERVAL（降低帧率）
3. 减少服务器日志输出

### 如果解码太慢

1. 减少服务器 workers 数量
2. 检查服务器 CPU 使用率
3. 考虑升级服务器硬件

## 快速诊断脚本

```bash
#!/bin/bash
# save as diagnose.sh

echo "=== Cimbar Web Server Diagnosis ==="

echo -e "\n1. Checking server process..."
ps aux | grep cimbar-server | grep -v grep

echo -e "\n2. Checking port 8080..."
ss -tlnp | grep 8080

echo -e "\n3. Checking Nginx..."
sudo nginx -t

echo -e "\n4. Recent server logs..."
tail -20 /var/log/nginx/error.log

echo -e "\n5. Test WebSocket..."
timeout 2 bash -c 'echo > /dev/tcp/127.0.0.1/8080' && echo "Port 8080 open" || echo "Port 8080 closed"
```
