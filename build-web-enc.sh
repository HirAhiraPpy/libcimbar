#!/bin/bash

# Cimbar Web 编码器一键构建脚本
# 功能：本地编译 WASM + 加密 + 生成独立 HTML

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

print_step() {
    echo -e "${BLUE}==>${NC} $1"
}

print_success() {
    echo -e "${GREEN}✓${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}⚠${NC} $1"
}

print_error() {
    echo -e "${RED}✗${NC} $1"
}

print_info() {
    echo -e "${CYAN}ℹ${NC} $1"
}

show_help() {
    cat << EOF
Cimbar Web 编码器一键构建脚本

用法：$0 [选项]

选项:
    -h, --help          显示帮助信息
    -p, --password      设置加密口令 (或使用环境变量 CIMBAR_PASSWORD)
    -d, --docker        使用 Docker 构建 (默认：本地编译)
    -n, --no-confirm    不生成独立 HTML 文件 (非交互模式)
    --skip-encrypt      跳过加密步骤 (仅构建 WASM)

环境变量:
    CIMBAR_PASSWORD     加密口令
    ENCRYPTION_PASSWORD 默认口令 (默认：cimbar2024)

示例:
    $0                              # 本地编译，使用默认口令
    $0 -p "my-secret"               # 使用指定口令
    $0 -d                           # 使用 Docker 构建
    CIMBAR_PASSWORD="xxx" $0        # 通过环境变量设置口令

前置条件 (本地编译):
    - Emscripten SDK (emsdk)
    - CMake 3.10+
    - Python 3
    - OpenCV4 源码 (opencv4 子模块)

安装 Emscripten:
    git clone https://github.com/emscripten-core/emsdk.git
    cd emsdk
    ./emsdk install 3.1.39
    ./emsdk activate 3.1.39
    source ./emsdk_env.sh

获取 OpenCV 源码:
    git clone --branch 4.x https://github.com/opencv/opencv.git opencv4

EOF
    exit 0
}

# 配置
PASSWORD="${CIMBAR_PASSWORD:-}"
USE_DOCKER=0
NO_CONFIRM=0
SKIP_ENCRYPT=0
ENCRYPTION_PASSWORD="${ENCRYPTION_PASSWORD:-cimbar2024}"

# 解析参数
while [[ $# -gt 0 ]]; do
    case $1 in
        -h|--help)
            show_help
            ;;
        -p|--password)
            PASSWORD="$2"
            shift 2
            ;;
        -d|--docker)
            USE_DOCKER=1
            shift
            ;;
        -n|--no-confirm)
            NO_CONFIRM=1
            shift
            ;;
        --skip-encrypt)
            SKIP_ENCRYPT=1
            shift
            ;;
        *)
            print_error "未知参数：$1"
            echo "使用 -h 或 --help 查看帮助"
            exit 1
            ;;
    esac
done

echo "========================================"
echo "  Cimbar Web 编码器构建工具"
echo "========================================"
echo ""

# 检查 Node.js
print_step "检查构建环境..."
if ! command -v node &> /dev/null; then
    print_error "Node.js 未安装，请先安装 Node.js 14+"
    exit 1
fi
print_success "Node.js 版本：$(node -v)"

# 检查 emscripten
EMSDK_AVAILABLE=0
if command -v emcc &> /dev/null; then
    EMCC_VERSION=$(emcc -v 2>&1 | head -1)
    print_success "Emscripten: $EMCC_VERSION"
    EMSDK_AVAILABLE=1
elif [ "$USE_DOCKER" -eq 1 ]; then
    print_info "将使用 Docker 进行 WASM 编译"
else
    print_warning "Emscripten 未找到"
fi

# Docker 检查
if [ "$USE_DOCKER" -eq 1 ]; then
    if command -v docker &> /dev/null && docker ps &> /dev/null; then
        print_success "Docker 可用"
    else
        print_error "Docker 不可用，请检查安装或启用 WSL 集成"
        exit 1
    fi
fi

# 检查 OpenCV 源码
if [ -d "opencv4" ]; then
    print_success "OpenCV 源码已存在"
elif [ "$USE_DOCKER" -eq 1 ]; then
    print_info "将在 Docker 容器中克隆 OpenCV"
else
    print_warning "opencv4 目录不存在"
    echo ""
    echo "获取 OpenCV 源码："
    echo "  git clone --branch 4.x https://github.com/opencv/opencv.git opencv4"
    echo ""
fi

# 步骤 1: 构建 WASM
print_step "WASM 编译流程"
echo ""

build_wasm_local() {
    print_step "本地编译 WASM..."

    # 激活 emsdk 环境
    if [ -f "$HOME/emsdk/emsdk_env.sh" ]; then
        source "$HOME/emsdk/emsdk_env.sh"
    elif [ -f "./emsdk/emsdk_env.sh" ]; then
        source "./emsdk/emsdk_env.sh"
    fi

    # 检查 emcc
    if ! command -v emcc &> /dev/null; then
        print_error "emcc 未找到，请先安装并激活 Emscripten SDK"
        echo ""
        echo "安装步骤:"
        echo "  git clone https://github.com/emscripten-core/emsdk.git"
        echo "  cd emsdk"
        echo "  ./emsdk install 3.1.39"
        echo "  ./emsdk activate 3.1.39"
        echo "  source ./emsdk_env.sh"
        echo ""
        exit 1
    fi

    # 检查 OpenCV
    if [ ! -d "opencv4" ]; then
        print_step "克隆 OpenCV 源码..."
        git clone --depth 1 --branch 4.9.1 https://github.com/opencv/opencv.git opencv4
    fi

    # 构建 OpenCV for WASM
    if [ ! -d "opencv4/opencv-build-wasm" ]; then
        print_step "编译 OpenCV for WASM..."
        cd opencv4
        mkdir -p opencv-build-wasm
        cd opencv-build-wasm
        python3 ../platforms/js/build_js.py build_wasm --emscripten_dir=$(dirname $(which emcc))
        cd ../..
    fi

    # 构建 libcimbar WASM
    print_step "编译 libcimbar WASM..."
    mkdir -p build-wasm
    cd build-wasm
    emcmake cmake .. -DUSE_WASM=1 -DOPENCV_DIR=$SCRIPT_DIR/opencv4
    make -j$(nproc) install
    cd ..

    # 压缩 WASM
    if [ -f "web/wasmgz.sh" ]; then
        print_step "压缩 WASM 文件..."
        cd web && bash wasmgz.sh && cd ..
    fi

    if [ -f "web/cimbar_js.wasm" ]; then
        WASM_SIZE=$(ls -lh web/cimbar_js.wasm | awk '{print $5}')
        print_success "WASM 文件已生成 ($WASM_SIZE)"
    else
        print_error "WASM 文件生成失败"
        exit 1
    fi
}

build_wasm_docker() {
    print_step "使用 Docker 编译 WASM..."

    # 拉取镜像
    if ! docker images emscripten/emsdk:3.1.39 &> /dev/null; then
        print_step "拉取 emscripten 镜像 (约 1GB)..."
        docker pull emscripten/emsdk:3.1.39
    fi

    # 运行构建
    print_step "在容器中构建 (可能需要 10-20 分钟)..."
    docker run --rm \
        --mount type=bind,source="$SCRIPT_DIR",target="/usr/src/app" \
        emscripten/emsdk:3.1.39 \
        bash -c "
            cd /usr/src/app

            # 克隆 OpenCV
            if [ ! -d opencv4 ]; then
                git clone --depth 1 --branch 4.9.1 https://github.com/opencv/opencv.git opencv4
            fi

            # 运行构建脚本
            export CIMBAR_ROOT=/usr/src/app
            bash package-wasm.sh
        "

    if [ -f "web/cimbar_js.wasm" ]; then
        WASM_SIZE=$(ls -lh web/cimbar_js.wasm | awk '{print $5}')
        print_success "WASM 文件已生成 ($WASM_SIZE)"
    else
        print_error "WASM 文件生成失败"
        exit 1
    fi
}

# 选择构建方式
if [ "$USE_DOCKER" -eq 1 ]; then
    build_wasm_docker
elif [ "$EMSDK_AVAILABLE" -eq 1 ] || command -v emcc &> /dev/null; then
    build_wasm_local
else
    # 都没有，询问用户
    echo ""
    print_warning "未检测到 Emscripten 环境"
    echo ""
    echo "请选择构建方式:"
    echo "  1) 本地编译 (需要先安装 emsdk)"
    echo "  2) Docker 编译 (需要 Docker Desktop)"
    echo "  3) 跳过 WASM 编译 (使用已有文件)"
    echo ""

    if [ "$NO_CONFIRM" -eq 0 ]; then
        read -p "请选择 (1/2/3): " BUILD_CHOICE

        case $BUILD_CHOICE in
            1)
                print_info "请先安装 Emscripten SDK:"
                echo ""
                echo "  git clone https://github.com/emscripten-core/emsdk.git"
                echo "  cd emsdk && ./emsdk install 3.1.39"
                echo "  source ./emsdk_env.sh"
                echo ""
                exit 1
                ;;
            2)
                USE_DOCKER=1
                build_wasm_docker
                ;;
            3)
                if [ -f "web/cimbar_js.wasm" ]; then
                    print_success "使用已有 WASM 文件"
                else
                    print_error "未找到 WASM 文件，无法跳过"
                    exit 1
                fi
                ;;
            *)
                print_error "无效选择"
                exit 1
                ;;
        esac
    else
        print_error "非交互模式下需要有效的构建环境"
        exit 1
    fi
fi

# 跳过加密则退出
if [ "$SKIP_ENCRYPT" -eq 1 ]; then
    print_success "WASM 编译完成，已跳过加密步骤"
    exit 0
fi

# 步骤 2: 加密 WASM
print_step "加密 WASM 文件..."

# 检查是否设置了密码
if [ -z "$PASSWORD" ]; then
    print_warning "未设置 CIMBAR_PASSWORD 环境变量，使用默认密码"
    PASSWORD="$ENCRYPTION_PASSWORD"
fi

print_info "使用密码：$PASSWORD"
node scripts/encrypt-wasm.js web/cimbar_js.wasm web/cimbar_js.wasm.enc "$PASSWORD"

if [ -f "web/cimbar_js.wasm.enc" ]; then
    ENC_SIZE=$(ls -lh web/cimbar_js.wasm.enc | awk '{print $5}')
    print_success "加密文件已生成 ($ENC_SIZE)"
else
    print_error "加密文件生成失败"
    exit 1
fi

# 步骤 3: 生成 Base64 版本
print_step "生成 Base64 编码..."
base64 web/cimbar_js.wasm.enc > web/cimbar_js.wasm.enc.b64
B64_SIZE=$(ls -lh web/cimbar_js.wasm.enc.b64 | awk '{print $5}')
print_success "Base64 文件已生成 ($B64_SIZE)"

# 步骤 4: 生成独立 HTML
if [ "$NO_CONFIRM" -eq 0 ]; then
    echo ""
    print_step "是否生成独立 HTML 文件？"
    echo "   (将所有资源嵌入单一 HTML 文件)"
    echo "   注意：文件较大，加载较慢"
    echo ""
    read -p "生成独立 HTML？(y/N): " GENERATE_HTML

    if [[ "$GENERATE_HTML" =~ ^[Yy]$ ]]; then
        print_step "生成独立 HTML..."
        node scripts/generate-encrypted-html.js "$PASSWORD"

        if [ -f "web/encoder-encrypted.html" ]; then
            HTML_SIZE=$(ls -lh web/encoder-encrypted.html | awk '{print $5}')
            print_success "独立 HTML 已生成 ($HTML_SIZE)"
        fi
    fi
fi

# 步骤 5: 创建部署包
print_step "创建部署包..."
DEPLOY_DIR="build-cgo/web-encoder"
rm -rf "$DEPLOY_DIR"
mkdir -p "$DEPLOY_DIR"

cp web/index.html "$DEPLOY_DIR/"
cp web/main.js "$DEPLOY_DIR/"
cp web/wasm-decrypt.js "$DEPLOY_DIR/"
cp web/cimbar_js.wasm.enc "$DEPLOY_DIR/"
cp web/sw.js "$DEPLOY_DIR/" 2>/dev/null || true
cp web/cimbar_js.js "$DEPLOY_DIR/" 2>/dev/null || true

cat > "$DEPLOY_DIR/README.txt" << EOF
Cimbar Web 编码器 - 部署包

访问口令：$PASSWORD

使用方法:
1. 将此目录所有文件部署到 Web 服务器
2. 确保支持 .wasm 和 .enc MIME 类型
3. 访问 index.html
4. 输入访问口令

文件说明:
- index.html: 主页面 (深色霓虹主题)
- main.js: 编码器逻辑
- wasm-decrypt.js: WASM 解密模块
- cimbar_js.wasm.enc: 加密的 WASM 文件
- sw.js: Service Worker (可选)

注意：请使用 HTTPS 部署
EOF

DEPLOY_SIZE=$(du -sh "$DEPLOY_DIR" | cut -f1)
print_success "部署包已创建 ($DEPLOY_SIZE)"

# 输出总结
echo ""
echo "========================================"
echo "  构建完成!"
echo "========================================"
echo ""
echo -e "访问口令：${GREEN}$PASSWORD${NC}"
echo ""
echo "输出文件:"
echo "  - web/cimbar_js.wasm.enc (加密 WASM)"
echo "  - web/cimbar_js.wasm.enc.b64 (Base64)"
if [ -f "web/encoder-encrypted.html" ]; then
    echo "  - web/encoder-encrypted.html (独立 HTML)"
fi
echo "  - $DEPLOY_DIR/ (部署包)"
echo ""
echo "本地测试:"
echo "  cd $DEPLOY_DIR"
echo "  python3 -m http.server 8080"
echo "  访问 http://localhost:8080/"
echo ""
echo "========================================"
