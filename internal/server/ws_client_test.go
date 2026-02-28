package server

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"time"

	"github.com/HirAhiraPpy/libcimbar/go/internal/decoder"
	"github.com/gorilla/websocket"
)

// WSClient 测试用 WebSocket 客户端
type WSClient struct {
	conn    *websocket.Conn
	url     string
	timeout time.Duration
}

// Connect 连接到服务器
func Connect(url string) (*WSClient, error) {
	dialer := websocket.Dialer{
		ReadBufferSize:  1024 * 1024,
		WriteBufferSize: 1024 * 1024,
		HandshakeTimeout: time.Second * 10,
	}
	conn, _, err := dialer.Dial(url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}

	return &WSClient{
		conn:    conn,
		url:     url,
		timeout: time.Second * 30,
	}, nil
}

// SetTimeout 设置读写超时
func (c *WSClient) SetTimeout(timeout time.Duration) {
	c.timeout = timeout
}

// SendFrame 发送帧数据
// 帧格式：[format:1][mode:1][width:2][height:2][data:...]
func (c *WSClient) SendFrame(imgData []byte, format decoder.ImageFormat) error {
	// Build frame header
	header := make([]byte, 6)
	header[0] = byte(format)
	header[1] = 0 // mode (reserved)
	binary.LittleEndian.PutUint16(header[2:4], uint16(1024))  // width
	binary.LittleEndian.PutUint16(header[4:6], uint16(1024)) // height

	// Combine header and pixel data
	frame := append(header, imgData...)

	c.conn.SetWriteDeadline(time.Now().Add(c.timeout))
	return c.conn.WriteMessage(websocket.BinaryMessage, frame)
}

// ReceiveResponse 接收响应
func (c *WSClient) ReceiveResponse() (*DecodeResponse, error) {
	c.conn.SetReadDeadline(time.Now().Add(c.timeout))

	msgType, data, err := c.conn.ReadMessage()
	if err != nil {
		return nil, fmt.Errorf("read message error: %w", err)
	}

	if msgType != websocket.TextMessage {
		return nil, fmt.Errorf("unexpected message type: %d", msgType)
	}

	var resp DecodeResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response error: %w", err)
	}

	return &resp, nil
}

// ReceiveResponseWithTimeout 接收响应（带超时）
func (c *WSClient) ReceiveResponseWithTimeout(timeout time.Duration) (*DecodeResponse, error) {
	c.conn.SetReadDeadline(time.Now().Add(timeout))

	msgType, data, err := c.conn.ReadMessage()
	if err != nil {
		return nil, fmt.Errorf("read message error: %w", err)
	}

	if msgType != websocket.TextMessage {
		return nil, fmt.Errorf("unexpected message type: %d", msgType)
	}

	var resp DecodeResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response error: %w", err)
	}

	return &resp, nil
}

// SendPing 发送心跳消息
func (c *WSClient) SendPing() error {
	msg := `{"type":"ping"}`
	c.conn.SetWriteDeadline(time.Now().Add(c.timeout))
	return c.conn.WriteMessage(websocket.TextMessage, []byte(msg))
}

// Close 关闭连接
func (c *WSClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}
