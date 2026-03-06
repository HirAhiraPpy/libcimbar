FROM golang:1.21-bookworm

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
    cmake \
    pkg-config \
    git \
    python3 \
    python3-pip \
    python3-venv \
    nodejs \
    npm \
    libopencv-dev \
    libglfw3-dev \
    libgles2-mesa-dev \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /workspace

COPY . /workspace

RUN go mod download

CMD ["bash"]
