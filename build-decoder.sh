#!/bin/bash
#
# Cimbar 解码器一键构建脚本
# 支持 Linux x86_64/arm64, macOS x86_64/arm64
#
# 用法:
#   ./build-decoder.sh [选项]
#
# 选项:
#   -p, --platform    目标平台 (linux/amd64, linux/arm64, darwin/amd64, darwin/arm64)
#   -o, --output      输出目录 (默认：dist/decoder)
#   --skip-cpp        跳过 C++ 构建 (已有预编译库)
#   --static          静态链接 (可移植二进制)
#   --clean           清理构建缓存
#   -h, --help        显示帮助
#

set -e

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

log_info()    { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; }
log_warning() { echo -e "${YELLOW}[WARNING]${NC} $1"; }
log_error()   { echo -e "${RED}[ERROR]${NC} $1" >&2; }

# 默认配置
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PLATFORM=""
OUTPUT_DIR="${SCRIPT_DIR}/dist/decoder"
SKIP_CPP=false
STATIC_LINK=false
CLEAN_BUILD=false
BUILD_TYPE="Release"

# 解析命令行参数
parse_args() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            -p|--platform)
                PLATFORM="$2"
                shift 2
                ;;
            -o|--output)
                OUTPUT_DIR="$2"
                shift 2
                ;;
            --skip-cpp)
                SKIP_CPP=true
                shift
                ;;
            --static)
                STATIC_LINK=true
                shift
                ;;
            --clean)
                CLEAN_BUILD=true
                shift
                ;;
            -h|--help)
                show_help
                exit 0
                ;;
            *)
                log_error "未知选项：$1"
                show_help
                exit 1
                ;;
        esac
    done
}

show_help() {
    cat << EOF
Cimbar 解码器一键构建脚本

用法：$(basename "$0") [选项]

选项:
  -p, --platform    目标平台
                    可选值：linux/amd64, linux/arm64, darwin/amd64, darwin/arm64
                    默认：当前系统平台
  -o, --output      输出目录 (默认：dist/decoder)
  --skip-cpp        跳过 C++ 构建，使用已有库
  --static          静态链接 C++ 标准库 (提高可移植性)
  --clean           清理构建缓存后重新构建
  -h, --help        显示此帮助信息

示例:
  $(basename "$0")                              # 使用默认设置构建
  $(basename "$0") -p linux/arm64               # 构建 ARM64 版本
  $(basename "$0") --static -o /opt/cimbar      # 静态链接，自定义输出目录
  $(basename "$0") --skip-cpp                   # 跳过 C++ 构建（已有库）
  $(basename "$0") --clean                      # 清理后重新构建

输出:
  构建完成后，所有文件将复制到 OUTPUT_DIR 目录：
  - bin/cimbar-server     Go 服务器二进制
  - lib/                  C++ 静态库
  - web/server/           Web 前端文件
  - README.md             部署说明

EOF
}

# 检查依赖
check_dependencies() {
    log_info "检查系统依赖..."

    local missing=()

    # 检查 Go
    if ! command -v go &> /dev/null; then
        missing+=("Go 1.20+")
    else
        local go_version=$(go version | awk '{print $3}')
        log_info "Go: ${go_version}"
    fi

    # 检查 CMake
    if ! command -v cmake &> /dev/null; then
        missing+=("CMake 3.10+")
    else
        local cmake_version=$(cmake --version | head -1 | awk '{print $3}')
        log_info "CMake: ${cmake_version}"
    fi

    # 检查 C++ 编译器
    if command -v g++ &> /dev/null; then
        log_info "G++ $(g++ --version | head -1)"
    elif command -v clang++ &> /dev/null; then
        log_info "Clang++ $(clang++ --version | head -1)"
    else
        missing+=("GCC 或 Clang")
    fi

    # 检查 OpenCV
    if pkg-config --exists opencv4 2>/dev/null; then
        local opencv_version=$(pkg-config --modversion opencv4)
        log_info "OpenCV: ${opencv_version}"
    elif pkg-config --exists opencv 2>/dev/null; then
        local opencv_version=$(pkg-config --modversion opencv)
        log_info "OpenCV: ${opencv_version}"
    else
        log_warning "OpenCV 未通过 pkg-config 找到，构建可能会失败"
    fi

    # 报告缺失的依赖
    if [[ ${#missing[@]} -gt 0 ]]; then
        log_error "缺少以下依赖："
        for dep in "${missing[@]}"; do
            echo "  - $dep"
        done
        echo ""
        echo "安装指南:"
        echo "  Ubuntu/Debian:"
        echo "    sudo apt install libopencv-dev libglfw3-dev libgles2-mesa-dev cmake build-essential golang"
        echo ""
        echo "  macOS:"
        echo "    brew install opencv glfw cmake go"
        echo ""
        exit 1
    fi

    log_success "所有依赖检查通过"
}

# 确定目标平台
detect_platform() {
    if [[ -z "$PLATFORM" ]]; then
        local os=$(uname -s | tr '[:upper:]' '[:lower:]')
        local arch=$(uname -m)

        case $arch in
            x86_64) arch="amd64" ;;
            aarch64|arm64) arch="arm64" ;;
            *) log_error "不支持的架构：$arch"; exit 1 ;;
        esac

        PLATFORM="${os}/${arch}"
    fi

    log_info "目标平台：$PLATFORM"

    # 解析平台
    GO_OS=$(echo "$PLATFORM" | cut -d'/' -f1)
    GO_ARCH=$(echo "$PLATFORM" | cut -d'/' -f2)

    case "$PLATFORM" in
        linux/amd64|linux/arm64)
            log_info "Linux 平台，使用 CGO 和 OpenCV"
            ;;
        darwin/amd64|darwin/arm64)
            log_info "macOS 平台，需要额外配置 CGO"
            if ! xcrun --show-sdk-path &> /dev/null; then
                log_warning "Xcode 命令行工具可能未安装"
                log_info "运行 'xcode-select --install' 安装"
            fi
            ;;
        *)
            log_error "不支持的平台：$PLATFORM"
            log_info "支持的平台：linux/amd64, linux/arm64, darwin/amd64, darwin/arm64"
            exit 1
            ;;
    esac
}

# 清理构建目录
clean_build() {
    if [[ "$CLEAN_BUILD" == true ]]; then
        log_info "清理构建目录..."
        rm -rf "${SCRIPT_DIR}/build"
        rm -rf "${SCRIPT_DIR}/build-cgo"
        rm -rf "${OUTPUT_DIR}"
        log_success "清理完成"
    fi
}

# 构建 C++ 库
build_cpp() {
    if [[ "$SKIP_CPP" == true ]]; then
        log_info "跳过 C++ 构建 (--skip-cpp)"

        # 验证库是否存在
        if [[ ! -f "${SCRIPT_DIR}/build-cgo/lib/libcimbar_decoder.a" ]]; then
            log_error "预编译库不存在：build-cgo/lib/libcimbar_decoder.a"
            log_info "请先运行不带 --skip-cpp 的构建，或移除该选项"
            exit 1
        fi
        return
    fi

    log_info "构建 C++ 库..."

    cd "$SCRIPT_DIR"

    # 创建构建目录
    mkdir -p build-cgo

    # 配置 CMake
    local cmake_args=()
    cmake_args+=("-DBUILD_CGO=1")
    cmake_args+=("-DCMAKE_BUILD_TYPE=${BUILD_TYPE}")

    # 静态链接选项
    if [[ "$STATIC_LINK" == true ]]; then
        cmake_args+=("-DBUILD_PORTABLE_LINUX=1")
        log_info "启用静态链接 (可移植二进制)"
    fi

    log_info "运行 CMake 配置..."
    cmake "${cmake_args[@]}" .

    # 构建库
    log_info "编译 C++ 库 (使用 $(nproc 2>/dev/null || echo 4) 个线程)..."
    make -j$(nproc 2>/dev/null || echo 4) cimbar_decoder

    # 验证构建
    if [[ ! -f "${SCRIPT_DIR}/build-cgo/lib/libcimbar_decoder.a" ]]; then
        log_error "C++ 库构建失败"
        exit 1
    fi

    log_success "C++ 库构建完成"
}

# 构建 Go 服务器
build_go() {
    log_info "构建 Go 服务器..."

    cd "$SCRIPT_DIR"

    # 设置 Go 环境变量
    export CGO_ENABLED=1
    export GOOS="$GO_OS"
    export GOARCH="$GO_ARCH"

    # macOS 特定配置
    if [[ "$GO_OS" == "darwin" ]]; then
        # 使用 Go 1.21+ 的 -ldflags=-extldflags=-Wl,-rpath,@executable_path/../lib
        export CGO_CFLAGS="-I${SCRIPT_DIR}/src/lib -I${SCRIPT_DIR}/src/third_party_lib"
        export CGO_LDFLAGS="-L${SCRIPT_DIR}/build-cgo/lib -L${SCRIPT_DIR}/build/src/lib/cimb_translator -L${SCRIPT_DIR}/build/src/lib/extractor -L${SCRIPT_DIR}/build/src/third_party_lib/wirehair -L${SCRIPT_DIR}/build/src/third_party_lib/zstd -L${SCRIPT_DIR}/build/src/third_party_lib/libcorrect/lib -lcimbar_decoder -lcimb_translator -lextractor -lcorrect -lwirehair -lzstd -lopencv_core -lopencv_imgcodecs -lopencv_imgproc -lopencv_photo -lopencv_calib3d -lstdc++ -framework Accelerate -framework AVFoundation -framework CoreGraphics -framework CoreMedia -framework CoreVideo"
    fi

    # 创建输出目录
    mkdir -p "${OUTPUT_DIR}/bin"

    # 构建
    cd "${SCRIPT_DIR}/cmd/cimbar-server"

    local ldflags="-s -w"  # 减小二进制大小

    log_info "编译 Go 服务器..."
    go build -trimpath -ldflags="$ldflags" -o "${OUTPUT_DIR}/bin/cimbar-server" .

    if [[ ! -f "${OUTPUT_DIR}/bin/cimbar-server" ]]; then
        log_error "Go 服务器构建失败"
        exit 1
    fi

    log_success "Go 服务器构建完成: ${OUTPUT_DIR}/bin/cimbar-server"
}

# 复制 Web 前端文件
copy_web_files() {
    log_info "复制 Web 前端文件..."

    mkdir -p "${OUTPUT_DIR}/web/server"

    # 复制 Web 文件
    cp -r "${SCRIPT_DIR}/web/server/"* "${OUTPUT_DIR}/web/server/"

    # 验证
    if [[ ! -f "${OUTPUT_DIR}/web/server/index.html" ]]; then
        log_error "Web 文件复制失败"
        exit 1
    fi

    log_success "Web 前端文件复制完成"
}

# 复制库文件
copy_libs() {
    log_info "复制库文件..."

    mkdir -p "${OUTPUT_DIR}/lib"

    # 复制主要库
    if [[ -f "${SCRIPT_DIR}/build-cgo/lib/libcimbar_decoder.a" ]]; then
        cp "${SCRIPT_DIR}/build-cgo/lib/libcimbar_decoder.a" "${OUTPUT_DIR}/lib/"
    fi

    # 复制其他依赖库（如果存在）
    for lib in libcimb_translator.a libextractor.a libimage_hash.a libwirehair.a libzstd.a libcorrect.a; do
        if [[ -f "${SCRIPT_DIR}/build-cgo/lib/$lib" ]]; then
            cp "${SCRIPT_DIR}/build-cgo/lib/$lib" "${OUTPUT_DIR}/lib/"
        fi
    done

    log_success "库文件复制完成"
}

# 创建配置文件模板
create_config() {
    log_info "创建配置文件模板..."

    cat > "${OUTPUT_DIR}/config.env" << 'EOF'
# Cimbar 解码器服务器配置文件
# 复制此文件为 config.env 并根据需要修改

# HTTP 监听地址
CIMBAR_ADDR=:8080

# 解码文件输出目录
CIMBAR_OUTPUT_DIR=/tmp/cimbar-downloads

# 缓存目录
CIMBAR_CACHE_DIR=./data/cache

# 数据库路径
CIMBAR_DB_PATH=./data/cimbar.db

# 解码模式：Auto, B, Bu, Bm, 4C
CIMBAR_MODE=Auto

# 工作线程数
CIMBAR_WORKERS=4

# Web 目录 (相对于二进制文件)
CIMBAR_WEB_DIR=./web/server
EOF

    log_success "配置文件模板创建完成"
}

# 创建部署说明
create_readme() {
    log_info "创建部署说明..."

    cat > "${OUTPUT_DIR}/README.md" << 'EOF'
# Cimbar 解码器服务器部署包

## 目录结构

```
dist/decoder/
├── bin/
│   └── cimbar-server      # 主程序
├── lib/                    # C++ 静态库
├── web/server/            # Web 前端文件
│   ├── index.html
│   ├── capture.js
│   └── login.js
├── config.env            # 配置文件模板
└── README.md             # 本文件
```

## 快速开始

### 1. 准备输出目录

```bash
mkdir -p /var/cimbar/output
chmod 755 /var/cimbar/output
```

### 2. 启动服务器

```bash
# 使用默认配置
./bin/cimbar-server

# 自定义配置
./bin/cimbar-server \
  --addr :8080 \
  --output-dir /var/cimbar/output \
  --mode Auto \
  --workers 4
```

### 3. 访问 Web 界面

在浏览器中访问：`http://localhost:8080`

## 配置选项

| 选项 | 默认值 | 说明 |
|------|--------|------|
| --addr | :8080 | HTTP 监听地址 |
| --output-dir | /tmp/cimbar-downloads | 解码文件输出目录 |
| --cache-dir | ./data/cache | 缓存目录 |
| --db-path | ./data/cimbar.db | SQLite 数据库路径 |
| --mode | Auto | 解码模式 (Auto/B/Bu/Bm/4C) |
| --workers | 4 | 解码工作线程数 |
| --web-dir | ./web/server | Web 文件目录 |

## 环境变量

也可以通过环境变量配置：

```bash
export CIMBAR_ADDR=:8080
export CIMBAR_OUTPUT_DIR=/var/cimbar/output
export CIMBAR_MODE=Auto
export CIMBAR_WORKERS=4
./bin/cimbar-server
```

## 解码模式

- **Auto**: 自动检测（推荐）
- **B**: 标准 4 色模式 (6 位/像素)
- **Bu**: 变体 B (6 位)
- **Bm**: 变体 M (6 位)
- **4C**: 传统 4 色模式

## 系统要求

- Linux x86_64 或 ARM64
- 已安装 OpenCV 4.x (动态链接版本)
- 200MB 可用磁盘空间

## Docker 部署

使用 Docker 部署（推荐）：

```bash
# 构建镜像
docker build -t cimbar-decoder .

# 运行
docker run -d \
  -p 8080:8080 \
  -v /var/cimbar/output:/data/output \
  -e CIMBAR_OUTPUT_DIR=/data/output \
  cimbar-decoder
```

## 故障排除

### 无法启动服务器

检查端口是否被占用：
```bash
netstat -tlnp | grep 8080
```

### 解码失败

1. 确保摄像头画面清晰
2. 调整光线条件
3. 确保 Cimbar 码完整可见

### CGO 错误

确保 OpenCV 已正确安装：
```bash
pkg-config --modversion opencv4
```

## 许可证

Mozilla Public License v2.0
EOF

    log_success "部署说明创建完成"
}

# 创建 Dockerfile
create_dockerfile() {
    log_info "创建 Dockerfile..."

    cat > "${SCRIPT_DIR}/Dockerfile" << 'EOF'
# Cimbar 解码器服务器 Dockerfile
# 多阶段构建，优化镜像大小

# ============================================
# 阶段 1: 构建 OpenCV (如果需要自定义编译)
# ============================================
FROM ubuntu:22.04 AS opencv-builder
LABEL stage=opencv

RUN apt-get update && apt-get install -y \
    build-essential \
    cmake \
    git \
    libgtk2.0-dev \
    pkg-config \
    libavcodec-dev \
    libavformat-dev \
    libswscale-dev \
    libjpeg-dev \
    libpng-dev \
    libtiff-dev \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /usr/src
RUN git clone --depth 1 --branch 4.9.1 https://github.com/opencv/opencv.git
WORKDIR /usr/src/opencv
RUN mkdir build && cd build \
    && cmake -DCMAKE_BUILD_TYPE=Release \
             -DCMAKE_INSTALL_PREFIX=/usr/local \
             -DBUILD_SHARED_LIBS=OFF \
             -DOPENCV_GENERATE_PKGCONFIG=ON \
             .. \
    && make -j$(nproc) \
    && make install

# ============================================
# 阶段 2: 构建 libcimbar C++ 库
# ============================================
FROM ubuntu:22.04 AS cpp-builder
LABEL stage=cpp

RUN apt-get update && apt-get install -y \
    build-essential \
    cmake \
    libopencv-dev \
    libglfw3-dev \
    libgles2-mesa-dev \
    pkg-config \
    git \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /usr/src/libcimbar
COPY . .

RUN cmake -DBUILD_CGO=1 -DCMAKE_BUILD_TYPE=Release . \
    && make -j$(nproc) cimbar_decoder

# ============================================
# 阶段 3: 构建 Go 服务器
# ============================================
FROM golang:1.21 AS go-builder
LABEL stage=go

RUN apt-get update && apt-get install -y \
    libopencv-dev \
    libglfw3-dev \
    libgles2-mesa-dev \
    pkg-config \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /usr/src/libcimbar

# 从 cpp-builder 阶段复制库文件
COPY --from=cpp-builder /usr/src/libcimbar/build-cgo ./build-cgo
COPY --from=cpp-builder /usr/src/libcimbar/build ./build
COPY --from=cpp-builder /usr/src/libcimbar/src ./src
COPY --from=cpp-builder /usr/src/libcimbar/cmd ./cmd
COPY --from=cpp-builder /usr/src/libcimbar/go.mod ./go.mod
COPY --from=cpp-builder /usr/src/libcimbar/go.sum ./go.sum

WORKDIR /usr/src/libcimbar/cmd/cimbar-server

ENV CGO_ENABLED=1
RUN go build -trimpath -ldflags="-s -w" -o /usr/src/libcimbar/build-cgo/bin/cimbar-server .

# ============================================
# 阶段 4: 运行时镜像
# ============================================
FROM debian:bullseye-slim
LABEL maintainer="Cimbar Team"

# 安装运行时依赖
RUN apt-get update && apt-get install -y \
    libopencv-core4.5 \
    libopencv-imgproc4.5 \
    libopencv-imgcodecs4.5 \
    libopencv-photo4.5 \
    libopencv-calib3d4.5 \
    libstdc++6 \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# 复制二进制文件
COPY --from=go-builder /usr/src/libcimbar/build-cgo/bin/cimbar-server /app/

# 复制 Web 文件
COPY web/server /app/web/server

# 创建数据目录
RUN mkdir -p /data/output /data/cache

# 环境变量
ENV CIMBAR_ADDR=:8080
ENV CIMBAR_OUTPUT_DIR=/data/output
ENV CIMBAR_CACHE_DIR=/data/cache
ENV CIMBAR_MODE=Auto
ENV CIMBAR_WORKERS=4

# 暴露端口
EXPOSE 8080

# 健康检查
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/ || exit 1

# 启动命令
ENTRYPOINT ["/app/cimbar-server"]
EOF

    log_success "Dockerfile 创建完成"
}

# 创建 docker-compose.yml
create_docker_compose() {
    log_info "创建 docker-compose.yml..."

    cat > "${SCRIPT_DIR}/docker-compose.yml" << 'EOF'
version: '3.8'

services:
  cimbar-decoder:
    build:
      context: .
      dockerfile: Dockerfile
    image: cimbar-decoder:latest
    container_name: cimbar-decoder
    ports:
      - "8080:8080"
    volumes:
      # 解码文件输出
      - cimbar-output:/data/output
      # 缓存目录
      - cimbar-cache:/data/cache
      # 可选：配置文件
      # - ./config.env:/app/config.env
    environment:
      - CIMBAR_ADDR=:8080
      - CIMBAR_OUTPUT_DIR=/data/output
      - CIMBAR_CACHE_DIR=/data/cache
      - CIMBAR_MODE=Auto
      - CIMBAR_WORKERS=4
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "wget", "--no-verbose", "--tries=1", "--spider", "http://localhost:8080/"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 10s

volumes:
  cimbar-output:
    driver: local
  cimbar-cache:
    driver: local
EOF

    log_success "docker-compose.yml 创建完成"
}

# 创建 .dockerignore
create_dockerignore() {
    log_info "创建 .dockerignore..."

    cat > "${SCRIPT_DIR}/.dockerignore" << 'EOF'
# Git
.git
.gitignore
.gitmodules

# 构建产物
build/
build-cgo/
dist/
tmp/
*.o
*.a
*.so

# Go
go.sum
vendor/

# Python
__pycache__/
*.pyc
*.pyo

# 文档
*.md
!README.md
docs/

# 脚本
scripts/
*.sh
!docker-entrypoint.sh

# 测试
test/
*_test.go

# 其他
.claude/
*.log
.DS_Store
Thumbs.db
EOF

    log_success ".dockerignore 创建完成"
}

# 验证构建
verify_build() {
    log_info "验证构建结果..."

    local errors=0

    # 检查二进制文件
    if [[ ! -x "${OUTPUT_DIR}/bin/cimbar-server" ]]; then
        log_error "二进制文件不存在或不可执行"
        ((errors++))
    else
        log_success "二进制文件：${OUTPUT_DIR}/bin/cimbar-server"
    fi

    # 检查 Web 文件
    if [[ ! -f "${OUTPUT_DIR}/web/server/index.html" ]]; then
        log_error "Web 文件缺失"
        ((errors++))
    else
        log_success "Web 文件：${OUTPUT_DIR}/web/server/"
    fi

    # 检查库文件
    if [[ "$SKIP_CPP" != true ]] && [[ ! -f "${OUTPUT_DIR}/lib/libcimbar_decoder.a" ]]; then
        log_warning "库文件缺失 (可能是静态链接)"
    elif [[ -f "${OUTPUT_DIR}/lib/libcimbar_decoder.a" ]]; then
        log_success "库文件：${OUTPUT_DIR}/lib/"
    fi

    # 检查配置文件
    if [[ ! -f "${OUTPUT_DIR}/config.env" ]]; then
        log_warning "配置文件模板缺失"
    else
        log_success "配置文件：${OUTPUT_DIR}/config.env"
    fi

    # 显示文件大小
    local bin_size=$(du -h "${OUTPUT_DIR}/bin/cimbar-server" | cut -f1)
    log_info "二进制大小：${bin_size}"

    local total_size=$(du -sh "${OUTPUT_DIR}" | cut -f1)
    log_info "部署包总大小：${total_size}"

    if [[ $errors -gt 0 ]]; then
        log_error "验证失败，发现 $errors 个错误"
        exit 1
    fi

    log_success "验证通过"
}

# 打印完成信息
print_summary() {
    echo ""
    echo "========================================"
    log_success "构建完成!"
    echo "========================================"
    echo ""
    echo "部署包位置：${OUTPUT_DIR}"
    echo ""
    echo "目录结构:"
    find "${OUTPUT_DIR}" -type f | head -20 | sed 's|^|  |'
    echo ""
    echo "使用方法:"
    echo "  1. 启动服务器:"
    echo "     cd ${OUTPUT_DIR}"
    echo "     ./bin/cimbar-server --addr :8080 --output-dir /tmp/cimbar"
    echo ""
    echo "  2. 访问 Web 界面:"
    echo "     http://localhost:8080"
    echo ""
    echo "  3. Docker 部署 (如果已创建 Dockerfile):"
    echo "     cd ${SCRIPT_DIR}"
    echo "     docker build -t cimbar-decoder ."
    echo "     docker run -p 8080:8080 -v /tmp/cimbar:/data/output cimbar-decoder"
    echo ""
}

# 主函数
main() {
    echo "========================================"
    echo "  Cimbar 解码器一键构建脚本"
    echo "========================================"
    echo ""

    parse_args "$@"
    detect_platform
    check_dependencies
    clean_build

    build_cpp
    build_go
    copy_web_files
    copy_libs
    create_config
    create_readme
    create_dockerfile
    create_docker_compose
    create_dockerignore

    verify_build
    print_summary
}

# 运行主函数
main "$@"
