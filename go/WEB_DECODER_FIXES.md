# Cimbar Web 解码器进度和多文件接收修复

## 修复概述

本次修复解决了两个主要问题：
1. **进度条显示错误**：后端日志显示进度 0.5902（59%），但前端显示 100%
2. **无法接收第二个文件**：接收完一个文件后，状态转移有问题，无法接收新文件

## 修改文件列表

### 1. C++ 代码修改

#### `src/lib/cimbar_js/cimbar_recv_js.h`
- 添加 `cimbard_reset_sink()` 函数声明，用于重置解码器状态

#### `src/lib/cimbar_js/cimbar_recv_js.cpp`
- 实现 `cimbard_reset_sink()` 函数，重置 `_sink`、`_dec`、`_reassembled` 和 `_decId`

### 2. Go 后端修改

#### `internal/decoder/decoder.h`
- 添加 `cimbard_reset_sink()` C 函数声明

#### `internal/decoder/decoder.go`
- 添加 `Reset()` 函数，调用 C 函数重置解码器
- 修改 `getProgress()` 返回 `[]float64` 而非 `[]int`，正确解析小数进度值
- 修改 `FountainResult.Progress` 类型为 `[]float64`

#### `internal/server/websocket.go`
- 添加 `CompletedFile` 结构体，记录已完成文件信息
- 在 `Server` 结构体中添加 `completedFiles []CompletedFile` 字段
- 修改 `WSHandler`：
  - 移除 `fileCompletePending` 标志
  - 添加 `reset_decoder` 控制消息处理
  - 文件完成后不再自动停止客户端
- 修改 `handleFileComplete()`：
  - 文件完成后添加到 `completedFiles` 列表
- 添加 `FilesHandler()` API 端点，返回已完成文件列表
- 添加 `DownloadHandler()` API 端点，提供文件下载

#### `cmd/cimbar-server/main.go`
- 添加 `/api/files` 和 `/api/download` 路由

### 3. 前端修改

#### `web/server/index.html`
- 添加侧边栏 UI 样式
- 添加已完成文件侧边栏 HTML
- 添加侧边栏切换按钮

#### `web/server/capture.js`
- 添加 `completedFiles` 数组，跟踪已完成文件
- 修改 `updateProgress()`：
  - 正确处理进度值（支持小数和百分比格式）
  - 确保进度条显示正确的百分比
- 修改 `handleServerResponse()`：
  - `complete` 类型：添加到本地已完成列表，不自动停止扫描
  - `file_complete` 类型：仅更新状态，不自动停止
- 修改 `toggleScanning()`：
  - 开始扫描时发送 `reset_decoder` 消息到后端
- 添加 `updateCompletedFilesList()` 函数，更新侧边栏文件列表
- 添加 `downloadFile()` 函数，处理文件下载
- 添加 `loadCompletedFiles()` 函数，从服务器加载历史记录
- 添加侧边栏切换事件监听器

## 新增功能

### 1. 多文件接收
- 接收完一个文件后，解码器自动重置
- 用户点击"停止识别"后，再次点击"开始识别"可以接收新文件
- 每次开始扫描时，后端解码器状态会被重置

### 2. 进度条正确显示
- 后端返回小数格式进度（0.0-1.0）
- 前端正确解析并显示百分比
- 支持多个流的进度条显示

### 3. 已完成文件列表
- 侧边栏显示所有已完成文件
- 显示文件名、大小、完成时间
- 支持点击下载文件

## API 端点

### `/api/files` (GET)
返回已完成文件列表

响应示例：
```json
[
  {
    "filename": "example.txt",
    "file_size": 1024,
    "path": "/tmp/cimbar-downloads/example.txt",
    "completed_at": "2026-02-28T10:30:00Z"
  }
]
```

### `/api/download?file=<filename>` (GET)
下载指定文件

## 构建说明

```bash
# 完整构建（包含 C++ 库和 Go 服务器）
bash build-go.sh

# 或者分步构建
# 1. 编译 C++ 库
cmake -DBUILD_CGO=1 .
make -j$(nproc) cimbar_decoder

# 2. 编译 Go 服务器
cd cmd/cimbar-server
go build -o ../../build-cgo/bin/cimbar-server
```

## 测试方法

### 单元测试
```bash
cd go/
go test -v ./internal/server/...
```

### 手动测试
1. 启动服务器：
   ```bash
   ./build-cgo/bin/cimbar-server --addr :8080 --output-dir /tmp/cimbar
   ```

2. 用手机浏览器访问 `http://<server-ip>:8080`

3. 测试进度显示：
   - 扫描 cimbar 图像
   - 验证进度条正确显示百分比（不是 100%）

4. 测试多文件接收：
   - 扫描第一个图像，等待完成
   - 点击"停止识别"
   - 再次点击"开始识别"
   - 扫描第二个图像，验证可以正常接收

5. 测试文件列表：
   - 点击右上角"已完成"按钮
   - 验证侧边栏显示已完成文件
   - 点击下载按钮，验证文件下载

## 预期行为

### 修复前
- ❌ 进度条始终显示 100%
- ❌ 第一个文件完成后无法接收第二个文件

### 修复后
- ✅ 进度条正确显示每个流的进度（如 59%）
- ✅ 文件完成后自动重置，用户点击"开始识别"后可以接收新文件
- ✅ 侧边栏显示已完成文件列表，可下载

## 注意事项

1. **向后兼容**：保持现有 WebSocket 协议兼容
2. **存储管理**：已完成文件列表存储在服务器内存中，重启后丢失。文件本身保存在输出目录
3. **安全性**：下载 API 包含路径遍历防护（使用 `filepath.Base()`）
4. **性能**：文件列表当前未分页，如果文件很多可能需要优化
