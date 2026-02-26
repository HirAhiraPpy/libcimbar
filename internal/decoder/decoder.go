// Package decoder provides CGO bindings for libcimbar decoder functions.
package decoder

/*
#cgo CFLAGS: -I${SRCDIR}/../../src/lib -I${SRCDIR}/../../src/third_party_lib
#cgo linux LDFLAGS: -L${SRCDIR}/../../build-cgo/lib -L${SRCDIR}/../../build/src/lib/cimb_translator -L${SRCDIR}/../../build/src/lib/extractor -L${SRCDIR}/../../build/src/third_party_lib/wirehair -L${SRCDIR}/../../build/src/third_party_lib/zstd -L${SRCDIR}/../../build/src/third_party_lib/libcorrect/lib -lcimbar_decoder -lcimb_translator -lextractor -lcorrect -lwirehair -lzstd -lopencv_core -lopencv_imgcodecs -lopencv_imgproc -lopencv_photo -lopencv_calib3d -lstdc++ -ldl -lm -lpthread
#cgo darwin LDFLAGS: -L${SRCDIR}/../../build-cgo/lib -lcimbar_decoder -lopencv_core -lopencv_imgcodecs -lopencv_imgproc -lopencv_photo -lopencv_calib3d -framework Accelerate -framework AVFoundation -framework CoreGraphics -framework CoreMedia -framework CoreVideo -lstdc++

#include <stdlib.h>
#include <string.h>
#include "decoder.h"
*/
import "C"
import (
	"errors"
	"fmt"
	"unsafe"
)

// ImageFormat represents the pixel format of the input image
type ImageFormat int

const (
	// FormatRGBA - 4 channels, RGBA order
	FormatRGBA ImageFormat = 4
	// FormatRGB - 3 channels, RGB order
	FormatRGB ImageFormat = 3
	// FormatNV12 - YUV 4:2:0 NV12 format
	FormatNV12 ImageFormat = 12
	// FormatI420 - YUV 4:2:0 I420 format (also known as YV12)
	FormatI420 ImageFormat = 420
)

// DecoderResult represents the result of a decode operation
type DecoderResult struct {
	Bytes     int    // Number of bytes decoded
	Extracted bool   // Whether extraction succeeded
	Failed    bool   // Whether decode failed
	Error     string // Error message if any
}

// FountainResult represents the result of fountain decode
type FountainResult struct {
	FileID   uint32 // File ID (if complete, >0)
	Progress []int  // Progress percentages for each file
}

// Decoder handles cimbar decoding
type Decoder struct {
	modeVal     int
	fountainBuf []byte
	fountainBufSize int
}

// NewDecoder creates a new decoder instance
func NewDecoder(mode string) *Decoder {
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

	d := &Decoder{
		modeVal: modeVal,
	}

	// Initialize decoder with mode
	if modeVal != 0 {
		C.cimbard_configure_decode(C.int(modeVal))
	}

	return d
}

// GetBufferSize returns the required buffer size for fountain decode
func GetBufferSize() int {
	return int(C.cimbard_get_bufsize())
}

// ScanExtractDecode performs scan, extract and decode on a single frame
// Returns the extracted fountain data and decode result
func (d *Decoder) ScanExtractDecode(imgData []byte, width, height int, format ImageFormat) (*DecoderResult, []byte, error) {
	if len(imgData) == 0 {
		return &DecoderResult{Failed: true, Error: "empty image data"}, nil, nil
	}

	bufSize := d.getExtractBufSize()
	extractBuf := make([]byte, bufSize)

	imgDataPtr := (*C.uchar)(unsafe.Pointer(&imgData[0]))
	extractBufPtr := (*C.uchar)(unsafe.Pointer(&extractBuf[0]))

	result := C.cimbard_scan_extract_decode(
		imgDataPtr,
		C.uint(width),
		C.uint(height),
		C.int(format),
		extractBufPtr,
		C.uint(bufSize),
	)

	if result < 0 {
		return &DecoderResult{
			Bytes:     0,
			Extracted: false,
			Failed:    true,
			Error:     fmt.Sprintf("decode error: %d", result),
		}, nil, nil
	}

	if result == 0 {
		return &DecoderResult{
			Bytes:     0,
			Extracted: true,
			Failed:    false,
			Error:     "",
		}, nil, nil
	}

	// Copy the extracted data
	extractedData := make([]byte, result)
	copy(extractedData, extractBuf[:result])

	return &DecoderResult{
		Bytes:     int(result),
		Extracted: true,
		Failed:    false,
		Error:     "",
	}, extractedData, nil
}

// FountainDecode performs fountain decode on extracted data
func (d *Decoder) FountainDecode(data []byte) (*FountainResult, error) {
	if len(data) == 0 {
		return nil, errors.New("empty data")
	}

	dataPtr := (*C.uchar)(unsafe.Pointer(&data[0]))
	result := C.cimbard_fountain_decode(dataPtr, C.uint(len(data)))

	if result < 0 {
		return nil, fmt.Errorf("fountain decode error: %d", result)
	}

	if result > 0 {
		// File complete! result is the file ID
		fileID := uint32(result & 0xFFFFFFFF)
		return &FountainResult{
			FileID:   fileID,
			Progress: d.getProgress(),
		}, nil
	}

	return &FountainResult{
		FileID:   0,
		Progress: d.getProgress(),
	}, nil
}

// GetFilename retrieves the filename for a completed file
func GetFilename(fileID uint32) (string, error) {
	bufSize := 1024
	buf := make([]byte, bufSize)
	bufPtr := (*C.char)(unsafe.Pointer(&buf[0]))

	result := C.cimbard_get_filename(C.uint(fileID), bufPtr, C.uint(bufSize))
	if result <= 0 {
		return "", fmt.Errorf("get filename error: %d", result)
	}

	// Find null terminator
	nullIdx := 0
	for nullIdx < len(buf) && buf[nullIdx] != 0 {
		nullIdx++
	}

	return string(buf[:nullIdx]), nil
}

// DecompressRead reads decompressed data for a file
func DecompressRead(fileID uint32) ([]byte, error) {
	bufSize := int(C.cimbard_get_decompress_bufsize())
	buf := make([]byte, bufSize)
	bufPtr := (*C.uchar)(unsafe.Pointer(&buf[0]))

	result := C.cimbard_decompress_read(C.uint(fileID), bufPtr, C.uint(bufSize))
	if result <= 0 {
		return nil, fmt.Errorf("decompress read error: %d", result)
	}

	return buf[:result], nil
}

// GetFileSize returns the size of a file by ID
func GetFileSize(fileID uint32) uint32 {
	return uint32(C.cimbard_get_filesize(C.uint(fileID)))
}

// getProgress retrieves decode progress
func (d *Decoder) getProgress() []int {
	bufSize := 1024
	buf := make([]byte, bufSize)
	bufPtr := (*C.uchar)(unsafe.Pointer(&buf[0]))

	result := C.cimbard_get_report(bufPtr, C.uint(bufSize))
	if result == 0 {
		return []int{}
	}

	// Parse JSON-like progress string
	// Format is typically something like: [25,50,75,100]
	progressStr := string(buf[:result])
	progress := make([]int, 0)

	// Simple parser for [num,num,num] format
	var num int
	inNum := false
	for _, ch := range progressStr {
		if ch >= '0' && ch <= '9' {
			num = num*10 + int(ch-'0')
			inNum = true
		} else if ch == ',' || ch == ']' {
			if inNum {
				progress = append(progress, num)
				num = 0
				inNum = false
			}
		}
	}

	return progress
}

// getExtractBufSize calculates the required extract buffer size
func (d *Decoder) getExtractBufSize() int {
	// Based on fountain_chunks_per_frame * fountain_chunk_size
	// This should match the C implementation
	return 8244 // Default for mode B (6-bit, 744 byte chunks, 11 chunks per frame)
}

// Configure sets the decode mode
func Configure(mode string) {
	modeVal := 68
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
	C.cimbard_configure_decode(C.int(modeVal))
}
