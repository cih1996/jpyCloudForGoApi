package coreClass

import (
	"encoding/binary"
	"fmt"
)

type PointerEvent struct {
	Bution byte  //按钮 1左键   2右键   3滚轮
	Type   byte  // 事件类型：1=按下, 2=移动, 3=释放
	X      int32 // X坐标（归一化值）
	Y      int32 // Y坐标（归一化值）
}

// InputEvent {"action":"input","data":"w"}
type InputEvent struct {
	Action string `json:"action"`
	Data   string `json:"data"`
}

func parsePointerEvent(bytes []byte) (*PointerEvent, error) {
	// 检查数据长度是否正确
	if len(bytes) < 5 {
		return nil, fmt.Errorf("数据长度不足，期望至少5字节，实际接收到%d字节", len(bytes))
	}

	// 创建一个新的指针事件
	event := &PointerEvent{
		Bution: bytes[0],
		Type:   bytes[1], // 第一个字节是事件类型
	}

	// 使用小端字节序读取X坐标 (字节1-2)
	event.X = int32(int16(binary.LittleEndian.Uint16(bytes[2:4])))
	//如果不是中键
	//if x<0   x=0  if x>10000 x=10000
	//if y<0   y=0  if y>10000 y=10000

	// 使用小端字节序读取Y坐标 (字节3-4)
	event.Y = int32(int16(binary.LittleEndian.Uint16(bytes[4:6])))

	return event, nil
}
