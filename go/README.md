# Go CIMBAR Web Server

A Go-based web server that decodes Cimbar barcodes from mobile device cameras.

## Architecture

The server uses CGO to integrate with libcimbar's C++ decoder library:

```
┌─────────────────────┐
│  Mobile Browser     │
│  (Chrome/Safari)    │
│  Camera → WebSocket │
└──────────┬──────────┘
           │
           ▼
┌─────────────────────┐
│  Go Web Server      │
│  - HTTP static      │
│  - WebSocket frames │
│  - CGO decoder      │
└──────────┬──────────┘
           │
           ▼
┌─────────────────────┐
│  libcimbar C++      │
│  - OpenCV image     │
│  - Reed Solomon ECC │
│  - Fountain codes   │
└─────────────────────┘
```

## Prerequisites

### System Dependencies

```bash
# Ubuntu/Debian
sudo apt install libopencv-dev libglfw3-dev libgles2-mesa-dev cmake build-essential

# macOS
brew install opencv glfw cmake
```

### Go Dependencies

```bash
go mod download
```

## Building

### Option 1: Use Build Script

```bash
bash build-go.sh
```

### Option 2: Manual Build

**Step 1: Build C++ Libraries**

```bash
# Build all libraries and executables
cmake -DBUILD_CGO=1 .
make -j$(nproc)
```

**Step 2: Build Go Server**

```bash
cd cmd/cimbar-server
go build -o ../../build-cgo/bin/cimbar-server
```

The binary will be at `build-cgo/bin/cimbar-server`.

## Usage

```bash
./build-cgo/bin/cimbar-server --addr :8080 --output-dir /tmp/cimbar-downloads --mode Auto --workers 4
```

### Command Line Options

| Flag | Default | Description |
|------|---------|-------------|
| `--addr` | `:8080` | HTTP listen address |
| `--output-dir` | `/tmp/cimbar-downloads` | Directory to save decoded files |
| `--mode` | `Auto` | Decode mode: Auto, B, Bu, Bm, 4C |
| `--workers` | `4` | Number of decoder workers |
| `--web-dir` | `./web/server` | Static web files directory |

### Modes

- **Auto**: Automatically detect mode from incoming frames
- **B**: Mode B (default, 6-bit, most reliable)
- **Bu**: Mode Bu (6-bit variant)
- **Bm**: Mode Bm (6-bit variant)
- **4C**: Mode 4C (legacy 4-color, 6-bit)

## Connecting from Mobile Device

1. Start the server on your computer
2. Ensure your mobile device is on the same network
3. Open browser on mobile: `http://<computer-ip>:8080`
4. Grant camera permission
5. Point camera at Cimbar barcode being displayed

## Protocol

### WebSocket Frame Format

Binary frames are sent with the following structure:

```
Offset  Size  Field
------  ----  -----
0       1     Format (4=RGBA, 3=RGB, 12=NV12, 420=I420)
1       1     Mode (reserved)
2       2     Width (little-endian uint16)
4       2     Height (little-endian uint16)
6       N     Pixel data
```

### Server Responses

JSON responses:

```json
// Decode result
{"type": "decode", "bytes": 744, "extracted": true, "progress": [25, 50, 75]}

// File complete
{"type": "complete", "success": true, "filename": "file.txt", "file_size": 12345}

// Error
{"type": "error", "error": "decode failed"}
```

## Decoding Pipeline

1. **Capture**: Mobile browser captures video frames via `getUserMedia()`
2. **Send**: Frames sent via WebSocket as binary messages
3. **Scan/Extract**: C++ library locates Cimbar grid, extracts tiles
4. **Decode**: Reed Solomon error correction, symbol matching
5. **Fountain Decode**: Reassemble file from multiple frames
6. **Decompress**: Zstd decompression
7. **Save**: Write file to output directory

## Performance

- Frame rate: ~15-30 fps (throttled to reduce bandwidth)
- Decode latency: ~50-100ms per frame
- Throughput: Depends on Cimbar mode and network

## Troubleshooting

### Camera not accessible

- Ensure HTTPS or localhost (browser security requirement)
- Check camera permissions

### Decode failures

- Ensure good lighting
- Keep device steady
- Fill the scan frame guide
- Try different modes

### CGO build errors

- Verify OpenCV installation: `pkg-config --modversion opencv4`
- Check library paths in `internal/decoder/decoder.go`

## License

Mozilla Public License v2.0
