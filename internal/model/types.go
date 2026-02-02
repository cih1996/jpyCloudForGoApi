package model

import "time"

// 映射信息结构体
type MappingSession struct {
	DeviceId            uint64    `json:"deviceId"`
	TbYunJiUserDeviceId int64     `json:"tbYunJiUserDeviceId"`
	LocalPort           int       `json:"localPort"`
	PhonePort           int       `json:"phonePort"`
	CreateTime          time.Time `json:"createTime"`
	Status              string    `json:"status"` // "active"
	Key                 string    `json:"key"`    // 所属的 Key
}

// 连接请求结构体
type ConnectRequest struct {
	Key                 string `json:"key"`
	DeviceId            uint64 `json:"deviceId"`
	TbYunJiUserDeviceId int64  `json:"tbYunJiUserDeviceId"`
	LocalPort           int    `json:"localPort"`
	PhonePort           int    `json:"phonePort"`
}

// 断开连接请求结构体
type DisconnectRequest struct {
	Key       string `json:"key"`
	LocalPort int    `json:"localPort"`
}

// 响应结构体
type Response struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// 设备列表请求
type GetDevicesRequest struct {
	Key string `json:"key" form:"key"`
}

// 映射列表请求
type GetMappingsRequest struct {
	Key string `json:"key" form:"key"`
}
