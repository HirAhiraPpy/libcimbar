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
