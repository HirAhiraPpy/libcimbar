# Cimbar Web 编码器构建指南

## 快速开始

### 一键构建（推荐）

```bash
# 本地编译（需要 emsdk）
./build-web-enc.sh -p "your-password"

# 使用 Docker 编译（需要 Docker Desktop）
./build-web-enc.sh -d -p "your-password"

# 非交互模式
./build-web-enc.sh -n -p "your-password"
```

---

## 构建方式

### 方式 1: 本地编译

**前置条件：**
- Emscripten SDK 3.1.39
- CMake 3.10+
- Python 3
- 约 10GB 磁盘空间

**安装 Emscripten：**
```bash
# 克隆 emsdk
git clone https://github.com/emscripten-core/emsdk.git
cd emsdk

# 安装并激活
./emsdk install 3.1.39
./emsdk activate 3.1.39

# 设置环境变量
source ./emsdk_env.sh

# 验证安装
emcc -v
```

**运行构建：**
```bash
# 返回项目根目录
cd /path/to/libcimbar

# 运行构建脚本
./build-web-enc.sh -p "your-password"
```

### 方式 2: Docker 编译

**前置条件：**
- Docker Desktop（启用 WSL 集成）

**运行构建：**
```bash
./build-web-enc.sh -d -p "your-password"
```

构建时间：约 10-20 分钟（首次需要拉取镜像）

---

## 输出文件

构建完成后生成以下文件：

```
web/
├── cimbar_js.wasm          # 原始 WASM 文件
├── cimbar_js.wasm.enc      # 加密的 WASM 文件
├── cimbar_js.wasm.enc.b64  # Base64 编码版本
└── encoder-encrypted.html  # 独立 HTML (可选)

build-cgo/web-encoder/      # 部署包
├── index.html
├── main.js
├── wasm-decrypt.js
├── cimbar_js.wasm.enc
└── README.txt
```

---

## 命令行选项

| 选项 | 说明 |
|------|------|
| `-h, --help` | 显示帮助信息 |
| `-p, --password` | 设置加密口令 |
| `-d, --docker` | 使用 Docker 构建 |
| `-n, --no-confirm` | 非交互模式 |
| `--skip-encrypt` | 跳过加密（仅构建 WASM） |

---

## 环境变量

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `CIMBAR_PASSWORD` | 加密口令 | - |
| `ENCRYPTION_PASSWORD` | 默认口令 | `cimbar2024` |

---

## 本地测试

```bash
cd build-cgo/web-encoder
python3 -m http.server 8080

# 访问 http://localhost:8080/
# 输入构建时设置的口令
```

---

## 部署

### 文件清单

部署 `build-cgo/web-encoder/` 目录中的所有文件：

- `index.html` - 主页面
- `main.js` - 编码器逻辑
- `wasm-decrypt.js` - WASM 解密模块
- `cimbar_js.wasm.enc` - 加密的 WASM 文件
- `sw.js` - Service Worker（可选）

### Nginx 配置示例

```nginx
server {
    listen 443 ssl;
    server_name cimbar.example.com;

    ssl_certificate /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;

    root /var/www/cimbar;
    index index.html;

    # WASM 和加密文件
    location ~* \.(wasm|enc)$ {
        add_header Content-Type application/octet-stream;
    }

    # 安全头
    add_header X-Frame-Options "SAMEORIGIN";
    add_header X-Content-Type-Options "nosniff";
}
```

---

## 故障排除

### "emcc 未找到"

确保已正确安装并激活 emsdk：
```bash
cd /path/to/emsdk
source ./emsdk_env.sh
emcc -v  # 应显示版本信息
```

### "opencv4 目录不存在"

脚本会自动克隆 OpenCV，也可以手动克隆：
```bash
git clone --depth 1 --branch 4.9.1 https://github.com/opencv/opencv.git opencv4
```

### "Docker 不可用"

在 WSL 2 中使用 Docker Desktop：
1. 安装 Docker Desktop
2. 设置 → Resources → WSL Integration
3. 启用你的 WSL 发行版

### "加密失败"

检查 Node.js 是否安装：
```bash
node -v  # 应显示 v14+
```

---

## 构建时间参考

| 方式 | 时间 | 说明 |
|------|------|------|
| 本地编译 (首次) | 15-25 分钟 | 需编译 OpenCV |
| 本地编译 (增量) | 2-5 分钟 | 仅编译 libcimbar |
| Docker 编译 (首次) | 15-25 分钟 | 需拉取镜像 + 编译 |
| Docker 编译 (缓存) | 10-15 分钟 | 镜像已缓存 |

---

## 高级选项

### 自定义 Emscripten 版本

编辑 `build-web-enc.sh`，修改 Docker 镜像版本：
```bash
docker pull emscripten/emsdk:3.1.39  # 改为其他版本
```

### 仅构建 WASM（不加密）

```bash
./build-web-enc.sh --skip-encrypt
```

### 自定义部署包路径

编辑脚本中的 `DEPLOY_DIR` 变量：
```bash
DEPLOY_DIR="/path/to/your/deploy"
```

---

## 相关文档

- [scripts/README.md](scripts/README.md) - 加密工具说明
- [DEPLOYMENT.md](DEPLOYMENT.md) - 部署指南
- [IMPLEMENTATION_SUMMARY.md](IMPLEMENTATION_SUMMARY.md) - 实现总结

---

## 附录：Go 解码器服务器构建

### 一键构建（推荐）

```bash
# 本地构建
./build-decoder.sh

# 查看帮助
./build-decoder.sh --help

# 构建特定平台
./build-decoder.sh -p linux/arm64

# 静态链接（提高可移植性）
./build-decoder.sh --static

# 跳过 C++ 构建（使用已有库）
./build-decoder.sh --skip-cpp

# 清理后重新构建
./build-decoder.sh --clean
```

### 前置条件

**Ubuntu/Debian:**
```bash
sudo apt install libopencv-dev libglfw3-dev libgles2-mesa-dev cmake build-essential golang
```

**macOS:**
```bash
brew install opencv glfw cmake go
```

### 输出文件

构建完成后生成以下文件：

```
dist/decoder/
├── bin/
│   └── cimbar-server      # Go 服务器二进制
├── lib/                    # C++ 静态库
├── web/server/            # Web 前端文件
│   ├── index.html
│   ├── capture.js
│   └── login.js
├── config.env            # 配置文件模板
└── README.md             # 部署说明
```

### 命令行选项

| 选项 | 说明 |
|------|------|
| `-p, --platform` | 目标平台 (linux/amd64, linux/arm64, darwin/amd64, darwin/arm64) |
| `-o, --output` | 输出目录 (默认：dist/decoder) |
| `--skip-cpp` | 跳过 C++ 构建 |
| `--static` | 静态链接 C++ 标准库 |
| `--clean` | 清理构建缓存 |
| `-h, --help` | 显示帮助信息 |

### Docker 部署

**构建镜像:**
```bash
docker build -t cimbar-decoder:latest .
```

**运行容器:**
```bash
docker run -d \
  -p 8080:8080 \
  -v /tmp/cimbar-output:/data/output \
  -e CIMBAR_MODE=Auto \
  cimbar-decoder
```

**使用 Docker Compose:**
```bash
docker-compose up -d
```

### 跨平台构建

| 平台 | 状态 | 说明 |
|------|------|------|
| Linux x86_64 | ✅ 完全支持 | 主要目标平台 |
| Linux ARM64 | ✅ 完全支持 | Raspberry Pi, ARM 服务器 |
| macOS x86_64 | ⚠️ 实验性 | CGO 链接需额外配置 |
| macOS ARM64 | ⚠️ 实验性 | M1/M2 芯片 |
| Windows | ❌ 不支持 | OpenCV+CGO 复杂度太高 |

**使用 Docker 进行交叉编译:**
```bash
# 构建 ARM64 镜像
docker buildx build --platform linux/arm64 -t cimbar-decoder:arm64 .

# 构建多平台镜像
docker buildx build --platform linux/amd64,linux/arm64 -t cimbar-decoder:multiarch .
```

### 故障排除

**"opencv4 未找到"**
```bash
# Ubuntu/Debian
pkg-config --modversion opencv4

# 如果未找到，安装:
sudo apt install libopencv-dev
```

**"CGO 编译失败"**
```bash
# 检查 CGO 是否启用
go env CGO_ENABLED  # 应输出 1

# 检查库路径
export CGO_CFLAGS="-I/usr/include/opencv4"
export CGO_LDFLAGS="-L/usr/lib -lopencv_core -lopencv_imgproc"
```

**"架构不匹配"**
```bash
# 检查当前架构
uname -m
go env GOARCH

# 交叉编译时设置目标架构
export GOOS=linux
export GOARCH=arm64
./build-decoder.sh -p linux/arm64
```

---

## 相关文档

- [scripts/README.md](scripts/README.md) - 加密工具说明
- [DEPLOYMENT.md](DEPLOYMENT.md) - 部署指南
- [IMPLEMENTATION_SUMMARY.md](IMPLEMENTATION_SUMMARY.md) - 实现总结

---

**最后更新**: 2026-02-28
