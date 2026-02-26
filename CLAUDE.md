# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Test

### Native Build (Linux)
```bash
# Install dependencies
sudo apt install libopencv-dev libglfw3-dev libgles2-mesa-dev

# Build
cmake .
make -j7
make install  # installs to ./dist/bin/
```

### WASM Build (encoder only, via emscripten docker)
```bash
docker run --mount type=bind,source="$(pwd)",target="/usr/src/app" -it emscripten/emsdk:3.1.39
# Inside container:
bash /usr/src/app/package-wasm.sh
```

### Go Web Server Build (decoder for mobile browsers)
```bash
# Install Go dependencies
go mod download

# Build C++ library for CGO
cmake -DBUILD_CGO=1 .
make -j$(nproc) cimbar_decoder

# Build Go server
cd cmd/cimbar-server
go build -o ../../build-cgo/bin/cimbar-server

# Or use the build script:
bash build-go.sh
```

### Run Tests
```bash
ctest  # runs all tests from build directory
./build/lib/<component>/test/<component>_test  # run specific test
```

## High-Level Architecture

### Core Libraries (`src/lib/`)

| Component | Purpose |
|-----------|---------|
| `cimb_translator` | Core encoder/decoder logic - `CimbEncoder`, `CimbDecoder`, `CimbWriter`, `CimbReader` |
| `encoder` / `decoder` | Reed Solomon error correction, aligned stream handling |
| `extractor` | Image scanning, corner detection, perspective transform, deskewing |
| `image_hash` | 8x8 tile hash extraction, symbol matching via hamming distance |
| `fountain` | Fountain codes (wirehair) for multi-frame file encoding/decoding |
| `compression` | Zstd compression wrappers |
| `bit_file` | Bit-level I/O: `bitreader`, `bitbuffer`, `bitmatrix` |
| `chromatic_adaptation` | Color correction for 4-color / 8-color modes |
| `gui` | OpenGL/GLFW display for `cimbar_send` |
| `cimbar_js` | WASM bindings for web encoder |

### Executables (`src/exe/`)

| Binary | Purpose |
|--------|---------|
| `cimbar` | CLI encode/decode static images |
| `cimbar_send` | Live encoder - displays animated barcode to window |
| `cimbar_recv` / `cimbar_recv2` | Live decoder from camera input |
| `cimbar_extract` | Extract data from PNG files |
| `build_image_assets` | Generate tile/symbol assets |

### Key Third-Party Libraries (`src/third_party_lib/`)

- `wirehair` - fountain codes (encoder/decoder)
- `libcorrect` - Reed Solomon error correction
- `zstd` - compression
- `opencv4` - computer vision (external dependency)

### Web Encoder (`web/`)

- `main.js` - UI logic for cimbar.org
- `recv.js` / `recv-worker.js` - decoder web worker (WIP)
- `sw.js` - service worker for PWA
- Built artifacts output to `web/` via `package-wasm.sh`

### Go Web Server (`go/`)

- `cmd/cimbar-server/main.go` - Server entry point
- `internal/decoder/decoder.go` - CGO bindings for cimbar decoder
- `internal/server/websocket.go` - WebSocket frame handling
- `web/server/index.html` - Mobile browser UI
- `web/server/capture.js` - Camera capture and WebSocket client

**Usage:**
```bash
./build-cgo/bin/cimbar-server --addr :8080 --output-dir /tmp/cimbar
```

Mobile browsers connect via WebSocket, streaming video frames for server-side decoding.
Decoded files are saved to the specified output directory.

### Cimbar Format

- Grid: 1024x1024 pixels, 9x9 tile grid (112x112 data cells, 8x8 pixels each)
- Mode B (default): 4-bit symbol + 2-bit color = 6 bits/tile
- ECC: Reed Solomon 30/155 (30 parity bytes per 125-byte block)
- Capacity: 7500 bytes/image after ECC
- Max file size: ~33MB (wirehair limitation)
- Interleaving: spreads ECC blocks across image to handle localized errors

### Decoder Pipeline

1. **Scan** - locate 3 corner markers, triangulate 4th corner
2. **Extract** - perspective transform to 1024x1024
3. **Tile decode** - for each 8x8 cell: compute image hash, match to symbol dictionary by hamming distance
4. **Drift tracking** - maintain per-cell (x,y) offset for local distortion
5. **Confidence ordering** - decode high-confidence cells first, use their drift for neighbors
6. **Deinterleave** - reverse encoder's skip pattern
7. **Error correction** - Reed Solomon on byte blocks
8. **Fountain decode** - reassemble file from multiple frames (order-independent)

### Encoder Pipeline

1. Compress input with zstd
2. Fountain encode (6-byte header per 744-byte block)
3. For each frame: Reed Solomon ECC, interleave, map bits to tile symbols + colors
4. Render tiles to 1024x1024 PNG
