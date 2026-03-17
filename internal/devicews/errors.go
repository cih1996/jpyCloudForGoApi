package devicews

import "errors"

// 错误定义
var (
	ErrDeviceNotFound = errors.New("device not found")
	ErrDeviceClosed   = errors.New("device connection closed")
	ErrInvalidPacket  = errors.New("invalid packet")
	ErrTimeout        = errors.New("operation timeout")
)
