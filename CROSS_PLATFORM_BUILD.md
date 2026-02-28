# Cimbar 解码器跨平台构建评估报告

**日期**: 2026-02-28
**版本**: 1.0

---

## 执行摘要

本报告评估了 Cimbar 解码器服务器在不同目标平台上的构建可行性和所需配置。

### 推荐部署方案

| 方案 | 推荐度 | 适用场景 |
|------|--------|----------|
| Docker (Linux) | ⭐⭐⭐⭐⭐ | 生产环境、快速部署 |
| 静态链接二进制 (Linux) | ⭐⭐⭐⭐ | 裸机部署、无 Docker 环境 |
| 动态链接二进制 (Linux) | ⭐⭐⭐⭐ | 标准 Linux 服务器 |
| macOS (实验性) | ⭐⭐ | 开发测试 |
| Windows | ❌ | 不推荐 |

---

## 平台兼容性矩阵

### 目标平台评估

| 平台 | 架构 | 可行性 | CGO 支持 | OpenCV | 推荐度 |
|------|------|--------|----------|--------|--------|
| Linux | x86_64 | ✅ 完全 | 原生 | pkg-config | ⭐⭐⭐⭐⭐ |
| Linux | arm64 | ✅ 完全 | 原生 | pkg-config | ⭐⭐⭐⭐⭐ |
| Linux | armv7l | ⚠️ 有限 | 原生 | 需编译 | ⭐⭐⭐ |
| macOS | x86_64 | ⚠️ 复杂 | 需配置 | Homebrew | ⭐⭐ |
| macOS | arm64 (M1/M2) | ⚠️ 复杂 | 需配置 | Homebrew | ⭐⭐ |
| Windows | x86_64 | ❌ 困难 | MSVC 复杂 | 需编译 | ❌ |
| Docker | multi-arch | ✅ 完全 | 原生 | 预装 | ⭐⭐⭐⭐⭐ |

---

## 详细平台分析

### 1. Linux x86_64 (主要目标)

**状态**: ✅ 完全支持

**依赖**:
- OpenCV 4.x (`libopencv-dev`)
- GLFW3 (`libglfw3-dev`)
- GLES2 (`libgles2-mesa-dev`)
- GCC/G++ 7+ 或 Clang
- Go 1.21+

**构建命令**:
```bash
./build-decoder.sh
```

**注意事项**:
- 无特殊配置要求
- 推荐使用 Ubuntu 20.04+ 或 Debian 11+
- 静态链接可提高可移植性

---

### 2. Linux ARM64 (Raspberry Pi/ARM 服务器)

**状态**: ✅ 完全支持

**依赖**: 同 x86_64

**构建命令**:
```bash
./build-decoder.sh -p linux/arm64
```

**或使用 Docker**:
```bash
docker buildx build --platform linux/arm64 -t cimbar-decoder:arm64 .
```

**注意事项**:
- Raspberry Pi 4 性能足够
- 可能需要增加 swap 空间
- 建议使用官方 Raspberry Pi OS 64-bit

**性能参考** (Raspberry Pi 4 8GB):
- 解码速度：~5-10 fps
- 内存占用：~300MB
- 推荐线程数：2-4

---

### 3. macOS x86_64

**状态**: ⚠️ 实验性支持

**依赖**:
```bash
brew install opencv glfw cmake go
```

**构建命令**:
```bash
./build-decoder.sh -p darwin/amd64
```

**已知问题**:
1. CGO 链接器可能需要额外配置
2. OpenCV 路径可能需要手动指定
3. 部分 OpenCV 功能可能受限

**解决方案**:
```bash
export CGO_CFLAGS="-I$(brew --prefix opencv)/include"
export CGO_LDFLAGS="-L$(brew --prefix opencv)/lib"
```

---

### 4. macOS ARM64 (M1/M2)

**状态**: ⚠️ 实验性支持

**依赖**: 同 macOS x86_64 (使用 ARM64 版本)

**构建命令**:
```bash
./build-decoder.sh -p darwin/arm64
```

**已知问题**:
1. Rosetta 2 可能影响性能
2. 部分库可能需要原生 ARM 版本
3. CGO 链接问题与 x86_64 类似

**建议**:
- 使用 Docker Desktop with ARM64 支持
- 优先使用 Docker 部署方案

---

### 5. Windows

**状态**: ❌ 不推荐

**问题**:
1. CGO 与 MSVC 兼容性复杂
2. OpenCV Windows 构建复杂
3. 路径分隔符问题
4. 缺乏测试环境

**替代方案**:
- 使用 WSL2 (Windows Subsystem for Linux)
- 使用 Docker Desktop with WSL2
- 使用 Linux VM

**WSL2 方案**:
```bash
# 在 WSL2 中执行
./build-decoder.sh
```

---

## Docker 多平台构建

### 使用 docker buildx

**前提条件**:
- Docker 20.10+
- buildx 插件
- QEMU (用于模拟非原生架构)

**安装 QEMU**:
```bash
docker run --rm --privileged multiarch/qemu-user-static --reset -p yes
```

**构建多平台镜像**:
```bash
docker buildx create --name cimbar-builder
docker buildx use cimbar-builder
docker buildx build --platform linux/amd64,linux/arm64 -t cimbar-decoder:multiarch .
```

**推送到镜像仓库**:
```bash
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  -t your-registry/cimbar-decoder:latest \
  --push \
  .
```

---

## 交叉编译策略

### 策略 A: Docker 多阶段构建 (推荐)

**优点**:
- 环境隔离，可重复
- 支持多平台
- 镜像大小优化

**缺点**:
- 需要 Docker
- 构建时间较长

### 策略 B: 交叉编译工具链

**Linux x86_64 → ARM64**:
```bash
# 安装交叉编译工具
sudo apt install gcc-aarch64-linux-gnu g++-aarch64-linux-gnu

# 设置环境变量
export CC=aarch64-linux-gnu-gcc
export CXX=aarch64-linux-gnu-g++
export CGO_ENABLED=1
export GOOS=linux
export GOARCH=arm64
export CC_FOR_TARGET=aarch64-linux-gnu-gcc

# 构建
./build-decoder.sh -p linux/arm64
```

**限制**:
- OpenCV ARM64 库需要单独编译
- 链接配置复杂
- 不推荐用于生产

### 策略 C: 目标平台原生构建

在目标平台（如 Raspberry Pi）上直接构建：

```bash
# 在 ARM64 设备上
sudo apt install libopencv-dev libglfw3-dev golang cmake
./build-decoder.sh
```

**优点**:
- 最简单
- 无交叉编译问题

**缺点**:
- 编译速度慢
- 需要设备访问

---

## 可移植性最佳实践

### 1. 使用静态链接

```bash
./build-decoder.sh --static
```

或在 CMake 中:
```cmake
set(CMAKE_CXX_FLAGS_RELWITHDEBINFO "${CMAKE_CXX_FLAGS_RELWITHDEBINFO} -static-libstdc++ -static-libgcc")
```

### 2. 使用 musl libc (可选)

```bash
# 安装 musl
sudo apt install musl-tools

# 使用 musl-gcc 构建
export CC=musl-gcc
export CXX=musl-g++
./build-decoder.sh --static
```

### 3. Docker 部署 (最佳)

```yaml
# docker-compose.yml
services:
  cimbar-decoder:
    image: cimbar-decoder:latest
    platform: linux/amd64  # 或 linux/arm64
```

---

## 构建时间参考

| 平台 | 方式 | 首次构建 | 增量构建 |
|------|------|----------|----------|
| Linux x86_64 | 原生 | 5-10 分钟 | 1-2 分钟 |
| Linux ARM64 | 原生 | 15-30 分钟 | 3-5 分钟 |
| Linux ARM64 | 交叉编译 | 10-15 分钟 | 2-3 分钟 |
| macOS | 原生 | 10-20 分钟 | 2-4 分钟 |
| Docker (多平台) | buildx | 20-40 分钟 | 5-10 分钟 |

*注：时间因硬件配置而异*

---

## 镜像大小优化

### 多阶段构建

```dockerfile
# 基础镜像大小对比
FROM ubuntu:22.04      # ~77MB
FROM debian:bullseye   # ~80MB
FROM alpine:3.18       # ~7MB (需要额外安装 glibc)

# 最终镜像大小
cimbar-decoder:ubuntu  # ~350MB
cimbar-decoder:debian  # ~300MB
cimbar-decoder:alpine  # ~150MB (需要特殊配置)
```

### 优化建议

1. 使用 Debian slim 基础镜像
2. 删除不必要的运行时依赖
3. 使用 UPX 压缩二进制 (可选)

---

## 测试矩阵

### 已测试环境

| 环境 | 版本 | 状态 | 备注 |
|------|------|------|------|
| Ubuntu | 20.04 LTS | ✅ 通过 | 推荐 |
| Ubuntu | 22.04 LTS | ✅ 通过 | 推荐 |
| Debian | 11 (Bullseye) | ✅ 通过 | 推荐 |
| Debian | 12 (Bookworm) | ✅ 通过 | 推荐 |
| Raspberry Pi OS | 64-bit | ✅ 通过 | ARM64 |
| macOS | 12+ (Monterey+) | ⚠️ 部分 | 需配置 |
| Docker | Ubuntu 22.04 | ✅ 通过 | 推荐 |

### 待测试环境

- Alpine Linux (需要 musl 适配)
- CentOS/Rocky Linux
- Fedora
- Arch Linux

---

## 结论与建议

### 推荐方案

1. **生产部署**: Docker (Debian slim 基础镜像)
2. **裸机部署**: Linux x86_64 静态链接二进制
3. **ARM 设备**: Docker 或原生构建
4. **开发测试**: 本地原生构建

### 不推荐方案

1. Windows 原生 (使用 WSL2 替代)
2. macOS 生产部署 (仅限开发)
3. 32 位系统 (内存限制)

### 未来改进

1. 添加 Alpine Linux 支持 (减小镜像)
2. 支持更多 ARM 设备 (Orange Pi, NanoPi 等)
3. 提供预编译二进制下载
4. 添加自动化跨平台 CI/CD

---

**最后更新**: 2026-02-28
