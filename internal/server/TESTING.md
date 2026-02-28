# Go Web Server 集成测试文档

## 概述

本目录包含 cimbar Go Web 服务器的集成测试套件，用于验证服务器端到端的功能正确性。

## 测试文件

### 1. `internal/encoder/encoder.go`
CGO 封装的 cimbar encoder，提供以下功能：
- `NewEncoder(mode, compression)` - 创建编码器实例
- `EncodeFile(filename, data)` - 编码文件并返回所有帧
- `Configure()` - 配置编码器模式
- `InitEncode(filename, encodeID)` - 初始化编码
- `Encode(data)` - 编码数据
- `NextFrame()` - 生成下一帧
- `GetFrameBuffer()` - 获取帧缓冲区

**注意**: Encoder 需要 OpenGL 和 OpenCV 依赖，在无头环境中可能无法运行。

### 2. `internal/server/ws_client_test.go`
WebSocket 测试客户端，提供以下功能：
- `Connect(url)` - 连接到 WebSocket 服务器
- `SendFrame(imgData, format)` - 发送帧数据
- `ReceiveResponse()` - 接收服务器响应
- `SendPing()` - 发送心跳消息
- `Close()` - 关闭连接

### 3. `internal/server/server_integration_test.go`
主集成测试文件，包含以下测试用例：

#### 测试用例列表

1. **TestIntegration_SingleFileDecode** - 单文件解码测试
   - 测试不同大小的文件（1KB, 50KB, 100KB）
   - 验证 WebSocket 连接和响应

2. **TestIntegration_ServerLifecycle** - 服务器生命周期测试
   - 测试服务器启动和关闭
   - 测试多次连接

3. **TestIntegration_ResponseTypes** - 响应类型测试
   - 测试无效帧的错误响应

4. **TestIntegration_ConcurrentClients** - 并发客户端测试
   - 测试 5 个并发 WebSocket 连接

5. **TestIntegration_KeepAlive** - 保持连接测试
   - 测试定时发送 ping 消息保持连接

6. **BenchmarkServerThroughput** - 服务器吞吐量基准测试
   - 测量帧发送性能

## 运行测试

### 基本测试命令

```bash
cd go/

# 运行所有测试
go test -v ./internal/server/...

# 运行特定测试
go test -v ./internal/server/... -run TestIntegration_ServerLifecycle

# 运行基准测试
go test -v ./internal/server/... -bench=.
```

### 环境变量配置

```bash
# 指定测试超时
go test -v -timeout 60s ./internal/server/...

# 显示测试覆盖率
go test -v -cover ./internal/server/...
```

## 预期输出

### 成功示例

```
=== RUN   TestIntegration_SingleFileDecode
=== RUN   TestIntegration_SingleFileDecode/small.txt
2026/02/28 14:40:43 Server initialized with 2 workers, mode: B, output: /tmp/cimbar-test-xxx
2026/02/28 14:40:43 WebSocket request received from [::1]:59496
2026/02/28 14:40:43 ✓ Client connected: [::1]:59496
    server_integration_test.go:146: response type: decode
--- PASS: TestIntegration_SingleFileDecode (0.37s)
    --- PASS: TestIntegration_SingleFileDecode/small.txt (0.15s)
--- PASS: TestIntegration_ServerLifecycle (0.11s)
PASS
ok      github.com/HirAhiraPpy/libcimbar/go/internal/server     6.813s
```

### 失败示例

```
=== RUN   TestIntegration_SingleFileDecode/small.txt
    server_integration_test.go:120: ✗ failed to connect: connection refused
--- FAIL: TestIntegration_SingleFileDecode (1.23s)
FAIL
```

## 依赖

- Go 1.21+
- `gorilla/websocket` (已在 go.mod 中)
- libcimbar C++ 库（已构建为 `build-cgo/lib/libcimbar_decoder.a`）
- OpenCV 库（用于 encoder 渲染）

## 未来扩展

### 完整的 Encoder 集成测试

当编码器环境配置完成后，可以添加以下测试：

```go
func TestIntegration_FullRoundtrip(t *testing.T) {
    // 1. 使用 encoder 编码文件
    enc := encoder.NewEncoder("B", 16)
    frames, err := enc.EncodeFile("test.txt", []byte("hello world"))

    // 2. 连接 WebSocket 服务器
    client, _ := Connect("ws://localhost:8080/ws")

    // 3. 发送所有帧
    for _, frame := range frames {
        client.SendFrame(frame.Data, decoder.FormatRGB)
        resp, _ := client.ReceiveResponse()

        // 4. 验证响应
        if resp.Type == "complete" {
            break
        }
    }

    // 5. 验证保存的文件内容
    verifySavedFile(t, outputDir, "test.txt", []byte("hello world"))
}
```

## 故障排除

### 问题：测试连接失败

```
failed to connect: dial tcp [::1]:8080: connect: connection refused
```

**解决方案**: 确保服务器在客户端连接前已启动。`startTestServer()` 函数已包含 100ms 的启动延迟。

### 问题：CGO 编译错误

```
error: opencv4/opencv2/imgcodecs.hpp: No such file or directory
```

**解决方案**: 安装 OpenCV 开发库：
```bash
sudo apt install libopencv-dev
```

### 问题：OpenGL 初始化失败

```
failed to create window
```

**解决方案**: 在无头环境中，需要使用虚拟显示或禁用 OpenGL：
```bash
# 使用 Xvfb
Xvfb :99 -screen 0 1024x768x24 &
export DISPLAY=:99
```

## 架构说明

```
┌─────────────────────────────────────────────────────────┐
│              集成测试 (Go + CGO)                        │
│  ┌────────────────────────────────────────────────┐     │
│  │ WSClient → 发送帧 → WebSocket Handler          │     │
│  │                  ↓                             │     │
│  │              Decoder (CGO)                     │     │
│  │                  ↓                             │     │
│  │              文件保存到 outputDir              │     │
│  └────────────────────────────────────────────────┘     │
└─────────────────────────────────────────────────────────┘
```

## 响应类型说明

| Type | 说明 | 字段 |
|------|------|------|
| `decode` | 正常解码响应 | Bytes, Extracted, FileSize, BytesRecv, Progress |
| `complete` | 文件解码完成 | Filename, FileSize, Success |
| `file_complete` | 文件完成通知（客户端应停止发送） | Success |
| `error` | 错误响应 | Error |
| `decode` + `NoData: true` | 未检测到数据 | NoData |
