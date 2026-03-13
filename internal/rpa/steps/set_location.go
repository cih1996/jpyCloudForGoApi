package steps

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"port-mapping-demo/internal/database"
	"port-mapping-demo/internal/rpa"
	"port-mapping-demo/internal/service"
	"port-mapping-demo/pkg/logger"
	"time"
)

// SetLocationStep 设置定位步骤
// 参数:
//   - lat: 纬度，支持变量引用如 {{proxyData.lat}}
//   - lng: 经度，支持变量引用如 {{proxyData.lng}}
//   - randomRange: 随机范围（公里），0表示不随机
type SetLocationStep struct{}

func init() {
	rpa.RegisterStep(&SetLocationStep{})
}

func (s *SetLocationStep) Type() string {
	return "set_location"
}

func (s *SetLocationStep) Name() string {
	return "设置定位"
}

func (s *SetLocationStep) SubSteps() []string {
	return []string{"设置定位"}
}

func (s *SetLocationStep) Execute(deviceID int, params map[string]interface{}, subStep int, ctx database.StepContext) rpa.StepResult {
	lat, _ := params["lat"].(float64)
	lng, _ := params["lng"].(float64)
	randomRange, _ := params["randomRange"].(float64) // 随机范围（公里）

	if lat == 0 && lng == 0 {
		return rpa.StepResult{
			Completed: true,
			Success:   false,
			Error:     "缺少必要参数: lat, lng",
		}
	}

	// 如果设置了随机范围，在原坐标基础上随机偏移
	if randomRange > 0 {
		lat, lng = randomizeLocation(lat, lng, randomRange)
	}

	req := &service.UnifiedRequest{
		Type: "setLocation",
		Seq:  int(time.Now().Unix()),
		Data: []map[string]interface{}{
			{
				"deviceId": deviceID,
				"lat":      lat,
				"lng":      lng,
			},
		},
	}

	res, err := service.HandleUnifiedRequestHTTP(context.Background(), req)
	if err != nil {
		return rpa.StepResult{
			Completed: true,
			Success:   false,
			Error:     fmt.Sprintf("设置定位失败: %v", err),
		}
	}

	if res.Code != 200 {
		return rpa.StepResult{
			Completed: true,
			Success:   false,
			Error:     fmt.Sprintf("设置定位失败: %s", res.Msg),
		}
	}

	logger.LogInfo("[RPA] 设备 %d 设置定位: %.6f, %.6f (随机范围: %.1fkm)", deviceID, lat, lng, randomRange)

	return rpa.StepResult{
		Completed: true,
		Success:   true,
		Output: map[string]interface{}{
			"lat": lat,
			"lng": lng,
		},
	}
}

// randomizeLocation 在指定范围内随机偏移经纬度
// rangeKm: 随机范围（公里）
func randomizeLocation(lat, lng, rangeKm float64) (float64, float64) {
	// 地球半径（公里）
	const earthRadius = 6371.0

	// 随机距离（0 到 rangeKm）
	distance := rand.Float64() * rangeKm
	// 随机角度（0 到 2π）
	bearing := rand.Float64() * 2 * math.Pi

	// 将纬度转换为弧度
	latRad := lat * math.Pi / 180
	lngRad := lng * math.Pi / 180

	// 计算新的纬度
	newLatRad := math.Asin(
		math.Sin(latRad)*math.Cos(distance/earthRadius) +
			math.Cos(latRad)*math.Sin(distance/earthRadius)*math.Cos(bearing),
	)

	// 计算新的经度
	newLngRad := lngRad + math.Atan2(
		math.Sin(bearing)*math.Sin(distance/earthRadius)*math.Cos(latRad),
		math.Cos(distance/earthRadius)-math.Sin(latRad)*math.Sin(newLatRad),
	)

	// 转换回度数
	newLat := newLatRad * 180 / math.Pi
	newLng := newLngRad * 180 / math.Pi

	return newLat, newLng
}
