# AGENTS.md

This repository should be developed and tested inside the project container.

## Build the development image

```bash
docker build -t libcimbar-dev .
```

## Start a container for development

```bash
docker run --rm -it -v "$(pwd)":/workspace -w /workspace libcimbar-dev bash
```

## Build commands (inside container)

### C++ tools and libraries

```bash
cmake .
make -j"$(nproc)"
```

### Go web server

```bash
cmake -DBUILD_CGO=1 -DCMAKE_BUILD_TYPE=Release .
make -j"$(nproc)" cimbar_decoder
cd cmd/cimbar-server
go build ./...
```

## Test commands (inside container)

### Python CLI tests

```bash
python3 -m unittest discover -s test/py -v
```

### Go server tests

```bash
go test ./internal/server/...
```

## Rules for coding agents

- If new system packages are needed, update `Dockerfile` and rebuild the image.
- Do **not** install apt/pip/npm packages ad-hoc during runtime as a primary workflow.
- Run all builds and tests inside the container to keep results reproducible.
