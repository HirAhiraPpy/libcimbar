#!/bin/sh
#
# Cimbar 解码器服务器 Docker 入口脚本
#
# 功能:
# - 等待依赖就绪
# - 创建必要的目录
# - 执行迁移 (如果有)
# - 启动服务器
#

set -e

log_info() {
    echo "[INFO] $1"
}

log_error() {
    echo "[ERROR] $1" >&2
}

# 等待 OpenCV 库就绪
wait_for_libs() {
    log_info "检查系统依赖..."

    # 检查 opencv 库
    if ! ldconfig -p | grep -q opencv_core; then
        log_error "OpenCV 库未找到，请确保镜像正确构建"
        exit 1
    fi

    log_info "系统依赖检查通过"
}

# 创建必要的目录
setup_directories() {
    log_info "设置目录结构..."

    # 创建输出目录
    if [[ -n "$CIMBAR_OUTPUT_DIR" ]]; then
        mkdir -p "$CIMBAR_OUTPUT_DIR"
        chmod 755 "$CIMBAR_OUTPUT_DIR"
        log_info "输出目录：$CIMBAR_OUTPUT_DIR"
    fi

    # 创建缓存目录
    if [[ -n "$CIMBAR_CACHE_DIR" ]]; then
        mkdir -p "$CIMBAR_CACHE_DIR"
        chmod 755 "$CIMBAR_CACHE_DIR"
        log_info "缓存目录：$CIMBAR_CACHE_DIR"
    fi

    # 创建数据库目录
    db_dir=$(dirname "$CIMBAR_DB_PATH")
    if [[ -n "$db_dir" && "$db_dir" != "." ]]; then
        mkdir -p "$db_dir"
        chmod 755 "$db_dir"
        log_info "数据库目录：$db_dir"
    fi
}

# 显示启动信息
show_startup_info() {
    echo ""
    echo "========================================"
    echo "  Cimbar 解码器服务器"
    echo "========================================"
    echo ""
    echo "配置:"
    echo "  监听地址：   ${CIMBAR_ADDR:-:8080}"
    echo "  输出目录：   ${CIMBAR_OUTPUT_DIR:-/data/output}"
    echo "  缓存目录：   ${CIMBAR_CACHE_DIR:-/data/cache}"
    echo "  数据库路径： ${CIMBAR_DB_PATH:-/data/cimbar.db}"
    echo "  解码模式：   ${CIMBAR_MODE:-Auto}"
    echo "  工作线程：   ${CIMBAR_WORKERS:-4}"
    echo ""
    echo "Web 界面：http://localhost:${CIMBAR_ADDR#:}"
    echo ""
    echo "========================================"
    echo ""
}

# 主函数
main() {
    wait_for_libs
    setup_directories
    show_startup_info

    # 构建命令行参数
    args=""
    [[ -n "$CIMBAR_ADDR" ]] && args="$args --addr $CIMBAR_ADDR"
    [[ -n "$CIMBAR_OUTPUT_DIR" ]] && args="$args --output-dir $CIMBAR_OUTPUT_DIR"
    [[ -n "$CIMBAR_CACHE_DIR" ]] && args="$args --cache-dir $CIMBAR_CACHE_DIR"
    [[ -n "$CIMBAR_DB_PATH" ]] && args="$args --db-path $CIMBAR_DB_PATH"
    [[ -n "$CIMBAR_MODE" ]] && args="$args --mode $CIMBAR_MODE"
    [[ -n "$CIMBAR_WORKERS" ]] && args="$args --workers $CIMBAR_WORKERS"
    [[ -n "$CIMBAR_WEB_DIR" ]] && args="$args --web-dir $CIMBAR_WEB_DIR"

    # 启动服务器
    exec /app/cimbar-server $args
}

main "$@"
