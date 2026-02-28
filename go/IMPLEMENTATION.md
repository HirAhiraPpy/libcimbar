# Go CIMBAR Web Server 实现总结

## 概述

实现了一个基于 Go 语言的 Web 服务器，允许手机浏览器（Chrome/Safari）通过摄像头扫描动态 Cimbar 条码，在服务端进行解码，并将解码后的文件保存到指定位置。

## 架构设计

```
┌─────────────────────┐
│   手机浏览器        │
│ (Chrome/Safari)     │
│ 摄像头 → WebSocket  │
└──────────┬──────────┘
           │
           ▼
┌─────────────────────┐
│   Go Web Server     │
│ - HTTP 静态服务     │
│ - WebSocket 帧处理  │
│ - CGO 解码器        │
└──────────┬──────────┘
           │
           ▼
┌─────────────────────┐
│   libcimbar C++     │
│ - OpenCV 图像处理   │
│ - Reed Solomon ECC  │
│ - Fountain 码       │
└─────────────────────┘
```

## 创建的文件

### Go 代码
| 文件 | 说明 |
|------|------|
| `go.mod` | Go 模块定义 |
| `cmd/cimbar-server/main.go` | Web 服务入口，命令行参数处理 |
| `internal/decoder/decoder.go` | CGO 绑定，封装 `cimbard_*` 函数 |
| `internal/decoder/decoder.h` | C 头文件封装 |
| `internal/server/websocket.go` | WebSocket 处理，视频帧解码 |

### 前端代码
| 文件 | 说明 |
|------|------|
| `web/server/index.html` | 移动端优化页面 |
| `web/server/capture.js` | 摄像头捕获和 WebSocket 通信 |

### 构建配置
| 文件 | 说明 |
|------|------|
| `CMakeLists.txt` | 添加了 `BUILD_CGO=1` 选项 |
| `build-go.sh` | 自动化构建脚本 |
| `go/README.md` | Go 服务器文档 |

## 构建步骤

### 方式一：使用构建脚本

```bash
bash build-go.sh
```

### 方式二：手动构建

**步骤 1：构建 C++ 库**

```bash
# 构建所有库和可执行文件
cmake -DBUILD_CGO=1 .
make -j$(nproc)
```

**步骤 2：构建 Go 服务器**

```bash
cd cmd/cimbar-server
go build -o ../../build-cgo/bin/cimbar-server
```

二进制文件位置：`build-cgo/bin/cimbar-server`

## 使用方法

### 启动服务器

```bash
./build-cgo/bin/cimbar-server --addr :8080 --output-dir /tmp/cimbar-downloads --mode Auto --workers 4
```

### 命令行参数

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `--addr` | `:8080` | HTTP 监听地址 |
| `--output-dir` | `/tmp/cimbar-downloads` | 解码文件保存目录 |
| `--mode` | `Auto` | 解码模式：Auto, B, Bu, Bm, 4C |
| `--workers` | `4` | 解码 worker 数量 |
| `--web-dir` | `./web/server` | 静态网页文件目录 |

### 解码模式

- **Auto**: 自动检测模式
- **B**: Mode B（默认，6 位，最可靠）
- **Bu**: Mode Bu（6 位变体）
- **Bm**: Mode Bm（6 位变体）
- **4C**: Mode 4C（传统 4 色，6 位）

## 连接步骤

1. 在计算机上启动服务器
2. 确保手机与计算机在同一网络
3. 手机浏览器访问：`http://<计算机 IP>:8080`
4. 授予摄像头权限
5. 将摄像头对准正在显示的 Cimbar 条码

## WebSocket 协议

### 客户端发送帧格式

二进制帧结构：

```
偏移    大小   字段
------  ----   -----
0       1      格式 (4=RGBA, 3=RGB, 12=NV12, 420=I420)
1       1      模式 (保留)
2       2      宽度 (小端 uint16)
4       2      高度 (小端 uint16)
6       N      像素数据
```

### 服务器响应

JSON 格式响应：

```json
// 解码结果
{"type": "decode", "bytes": 744, "extracted": true, "progress": [25, 50, 75]}

// 文件完成
{"type": "complete", "success": true, "filename": "file.txt", "file_size": 12345}

// 错误
{"type": "error", "error": "decode failed"}
```

## 解码流程

1. **捕获**: 手机浏览器通过 `getUserMedia()` 获取视频流
2. **发送**: 帧通过 WebSocket 以二进制消息发送
3. **扫描/提取**: C++ 库定位 Cimbar 网格，提取图块
4. **解码**: Reed Solomon 纠错，符号匹配
5. **Fountain 解码**: 从多帧重组文件
6. **解压**: Zstd 解压缩
7. **保存**: 将文件写入输出目录

## 系统依赖

```bash
# Ubuntu/Debian
sudo apt install libopencv-dev libglfw3-dev libgles2-mesa-dev cmake build-essential

# macOS
brew install opencv glfw cmake
```

## 性能指标

- 帧率：~10-15 fps（限速以减少带宽）
- 解码延迟：~50-100ms/帧
- 吞吐量：取决于 Cimbar 模式和网络

## 故障排除

### 摄像头无法访问

**问题**: 页面显示黑色屏幕，无法访问摄像头

**原因**: 现代浏览器（Chrome/Safari）在非 HTTPS 环境下会限制摄像头访问

**解决方案**:

1. **使用 localhost** (推荐用于测试)
   - localhost 被视为安全上下文，允许摄像头访问

2. **配置 HTTPS** (推荐用于生产)
   - 使用反向代理（如 nginx）配置 SSL 证书
   - 或使用 Caddy 等自动 HTTPS 服务器

3. **Chrome 临时方案** (仅用于开发)
   - 访问 `chrome://flags/#unsafely-treat-insecure-origin-as-secure`
   - 启用该标志并添加你的服务器地址

4. **Safari iOS**
   - iOS Safari 对非 HTTPS 页面的摄像头限制更严格
   - 必须使用 HTTPS 或 localhost

### 摄像头权限被拒绝

- 检查浏览器权限设置
- 确保没有其他程序占用摄像头
- 刷新页面重新请求权限

### 解码失败

- 确保良好照明
- 保持设备稳定
- 填满扫描框引导线
- 尝试不同模式

### Ctrl+C 无法终止服务器

- 已修复：现在按 Ctrl+C 会优雅关闭服务器
- 服务器会关闭所有 WebSocket 连接后退出

## 改进历史

### v2.0 改进

1. **优雅关闭**: 支持 Ctrl+C 终止，自动关闭 WebSocket 连接
2. **摄像头兼容性**:
   - 移除 `requestVideoFrameCallback` 依赖（需要 HTTPS）
   - 使用 `setInterval` 实现帧捕获（更兼容）
3. **HTTPS 提示**: 非 HTTPS 环境显示警告信息
4. **视频显示**:
   - 添加 `object-fit: cover` 确保视频填满容器
   - 添加加载指示器
5. **错误处理**: 更详细的错误消息和故障排除提示

## 许可证

Mozilla Public License v2.0
