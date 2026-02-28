// Package encoder provides CGO bindings for libcimbar encoder functions.
package encoder

/*
#cgo CFLAGS: -I${SRCDIR}/../../src/lib -I${SRCDIR}/../../src/third_party_lib
#cgo linux LDFLAGS: -L${SRCDIR}/../../build-cgo/lib -L${SRCDIR}/../../build/src/lib/cimb_translator -L${SRCDIR}/../../build/src/lib/extractor -L${SRCDIR}/../../build/src/lib/cimbar_js -L${SRCDIR}/../../build/src/third_party_lib/wirehair -L${SRCDIR}/../../build/src/third_party_lib/zstd -L${SRCDIR}/../../build/src/third_party_lib/libcorrect/lib -lcimbar_js -lcimb_translator -lextractor -lcorrect -lwirehair -lzstd -lopencv_core -lopencv_imgcodecs -lopencv_imgproc -lopencv_photo -lopencv_calib3d -lopencv_highgui -lglfw -lGL -lstdc++ -ldl -lm -lpthread
#cgo darwin LDFLAGS: -L${SRCDIR}/../../build-cgo/lib -lcimbar_js -lopencv_core -lopencv_imgcodecs -lopencv_imgproc -lopencv_photo -lopencv_calib3d -framework Accelerate -framework AVFoundation -framework CoreGraphics -framework CoreMedia -framework CoreVideo -lstdc++

#include <stdlib.h>
#include <string.h>
#include "cimbar_js/cimbar_js.h"
*/
import "C"
import (
	"fmt"
	"unsafe"
)

// Frame 表示一帧 cimbar 图像
type Frame struct {
	Data   []byte // RGB 像素数据
	Width  int    // 1024
	Height int    // 1024
}

// Encoder cimbar 编码器
type Encoder struct {
	modeVal     int
	compression int
}

// NewEncoder 创建编码器
func NewEncoder(mode string, compression int) *Encoder {
	modeVal := 68 // default mode B
	switch mode {
	case "Auto", "auto":
		modeVal = 0
	case "B":
		modeVal = 68
	case "Bu":
		modeVal = 66
	case "Bm":
		modeVal = 67
	case "4C":
		modeVal = 4
	}

	return &Encoder{
		modeVal:     modeVal,
		compression: compression,
	}
}

// Configure 配置编码器模式
func (e *Encoder) Configure() error {
	result := C.cimbare_configure(C.int(e.modeVal), C.int(e.compression))
	if result < 0 {
		return fmt.Errorf("configure error: %d", result)
	}
	return nil
}

// InitEncode 初始化编码
func (e *Encoder) InitEncode(filename string, encodeID int) error {
	fnSize := len(filename)
	fnPtr := C.CString(filename)
	defer C.free(unsafe.Pointer(fnPtr))

	result := C.cimbare_init_encode(fnPtr, C.uint(fnSize), C.int(encodeID))
	if result < 0 {
		return fmt.Errorf("init encode error: %d", result)
	}
	return nil
}

// Encode 编码数据
func (e *Encoder) Encode(data []byte) (int, error) {
	if len(data) == 0 {
		return 0, nil
	}

	dataPtr := (*C.uchar)(unsafe.Pointer(&data[0]))
	result := C.cimbare_encode(dataPtr, C.uint(len(data)))
	if result < 0 {
		return 0, fmt.Errorf("encode error: %d", result)
	}
	return int(result), nil
}

// Finalize 完成编码，准备生成帧
func (e *Encoder) Finalize() error {
	// Encode with empty data to signal completion
	result := C.cimbare_encode(nil, 0)
	if result < 0 {
		return fmt.Errorf("finalize error: %d", result)
	}
	return nil
}

// NextFrame 生成下一帧
func (e *Encoder) NextFrame() bool {
	result := C.cimbare_next_frame_default()
	return result > 0
}

// GetFrameBuffer 获取帧缓冲区
func (e *Encoder) GetFrameBuffer() ([]byte, error) {
	var buff *C.uchar
	result := C.cimbare_get_frame_buff(&buff)
	if result < 0 {
		return nil, fmt.Errorf("get frame buffer error: %d", result)
	}

	// Copy data from C buffer
	// Frame buffer is RGB data, result is the byte count
	frameData := unsafe.Slice((*byte)(unsafe.Pointer(buff)), result)
	output := make([]byte, len(frameData))
	copy(output, frameData)

	return output, nil
}

// EncodeFile 编码文件，返回所有帧
func (e *Encoder) EncodeFile(filename string, data []byte) ([]Frame, error) {
	var frames []Frame

	// Step 1: Configure
	if err := e.Configure(); err != nil {
		return nil, err
	}

	// Step 2: Initialize encode
	if err := e.InitEncode(filename, -1); err != nil {
		return nil, err
	}

	// Step 3: Encode data in chunks
	chunkSize := int(C.cimbare_encode_bufsize())
	for len(data) > 0 {
		chunk := data
		if len(chunk) > chunkSize {
			chunk = chunk[:chunkSize]
		}

		_, err := e.Encode(chunk)
		if err != nil {
			return nil, err
		}
		data = data[len(chunk):]
	}

	// Step 4: Finalize
	if err := e.Finalize(); err != nil {
		return nil, err
	}

	// Step 5: Generate frames
	for e.NextFrame() {
		frameData, err := e.GetFrameBuffer()
		if err != nil {
			return nil, err
		}
		frames = append(frames, Frame{
			Data:   frameData,
			Width:  1024,
			Height: 1024,
		})
	}

	return frames, nil
}

// GetEncodeBufsize 返回编码缓冲区大小
func GetEncodeBufsize() int {
	return int(C.cimbare_encode_bufsize())
}
