package centControlPlatform

import "adminApi"

// GetApi 返回内部的 AdminApi 实例，供外部直接调用集控平台 API
// 注意：这是为 jpyCloudForGoApi 项目添加的扩展方法，JpyApiAgent 原始代码不包含此文件
func (c *Core) GetApi() *adminApi.AdminApi {
	return c.server.api
}
