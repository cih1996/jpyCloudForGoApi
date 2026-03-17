package devicews

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/ghp3000/logs"
)

// 配置常量
const (
	InitTimeout       = 3 * time.Second   // 初始化超时（连接后必须在此时间内发送 INIT）
	HeartbeatInterval = 30 * time.Second  // 心跳间隔
	HeartbeatTimeout  = 90 * time.Second  // 心跳超时（超过此时间无活动则断开）
	CleanupInterval   = 10 * time.Second  // 清理检查间隔
)

// DeviceManager 设备连接管理器
// 使用分片锁设计，支持高并发（上万连接）
type DeviceManager struct {
	// 分片数量（2的幂次方，便于取模）
	shardCount uint32
	// 分片数组，每个分片有独立的锁
	shards []*deviceShard

	// 统计信息
	totalConnections int64 // 总连接数（原子操作）
	totalMessages    int64 // 总消息数（原子操作）

	// 事件回调
	onConnect    func(dc *DeviceConn)
	onDisconnect func(dc *DeviceConn)
	onMessage    func(dc *DeviceConn, packet *Packet)

	// 运行状态
	running   int32
	stopChan  chan struct{}
	wg        sync.WaitGroup
}

// deviceShard 设备分片
type deviceShard struct {
	mu      sync.RWMutex
	devices map[uint32]*DeviceConn // deviceID -> DeviceConn
}

// NewDeviceManager 创建设备管理器
func NewDeviceManager() *DeviceManager {
	// 使用 256 个分片，足够支持上万连接的并发访问
	shardCount := uint32(256)
	shards := make([]*deviceShard, shardCount)
	for i := uint32(0); i < shardCount; i++ {
		shards[i] = &deviceShard{
			devices: make(map[uint32]*DeviceConn),
		}
	}

	return &DeviceManager{
		shardCount: shardCount,
		shards:     shards,
		stopChan:   make(chan struct{}),
	}
}

// getShard 根据 deviceID 获取对应的分片
func (dm *DeviceManager) getShard(deviceID uint32) *deviceShard {
	return dm.shards[deviceID%dm.shardCount]
}

// SetOnConnect 设置连接回调
func (dm *DeviceManager) SetOnConnect(fn func(dc *DeviceConn)) {
	dm.onConnect = fn
}

// SetOnDisconnect 设置断开回调
func (dm *DeviceManager) SetOnDisconnect(fn func(dc *DeviceConn)) {
	dm.onDisconnect = fn
}

// SetOnMessage 设置消息回调
func (dm *DeviceManager) SetOnMessage(fn func(dc *DeviceConn, packet *Packet)) {
	dm.onMessage = fn
}

// Start 启动管理器（启动清理协程）
func (dm *DeviceManager) Start() {
	if !atomic.CompareAndSwapInt32(&dm.running, 0, 1) {
		return
	}

	dm.wg.Add(1)
	go dm.cleanupLoop()

	logs.Info("[DeviceWS] 设备管理器已启动")
}

// Stop 停止管理器
func (dm *DeviceManager) Stop() {
	if !atomic.CompareAndSwapInt32(&dm.running, 1, 0) {
		return
	}

	close(dm.stopChan)
	dm.wg.Wait()

	// 关闭所有连接
	dm.CloseAll()

	logs.Info("[DeviceWS] 设备管理器已停止")
}

// cleanupLoop 清理循环（检测超时连接）
func (dm *DeviceManager) cleanupLoop() {
	defer dm.wg.Done()

	ticker := time.NewTicker(CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-dm.stopChan:
			return
		case <-ticker.C:
			dm.cleanupTimeoutConnections()
		}
	}
}

// cleanupTimeoutConnections 清理超时连接
func (dm *DeviceManager) cleanupTimeoutConnections() {
	now := time.Now()
	var toRemove []*DeviceConn

	// 遍历所有分片
	for _, shard := range dm.shards {
		shard.mu.RLock()
		for _, dc := range shard.devices {
			// 检查心跳超时
			if now.Sub(dc.LastSeen) > HeartbeatTimeout {
				toRemove = append(toRemove, dc)
			}
		}
		shard.mu.RUnlock()
	}

	// 移除超时连接
	for _, dc := range toRemove {
		logs.Info("[DeviceWS] 设备 %08X (%s) 心跳超时，断开连接", dc.DeviceID, dc.Serialno)
		dm.Remove(dc.DeviceID)
	}

	if len(toRemove) > 0 {
		logs.Info("[DeviceWS] 清理了 %d 个超时连接，当前连接数: %d", len(toRemove), dm.Count())
	}
}

// Add 添加设备连接
func (dm *DeviceManager) Add(dc *DeviceConn) {
	shard := dm.getShard(dc.DeviceID)
	shard.mu.Lock()

	// 如果已存在旧连接，先关闭
	if old, ok := shard.devices[dc.DeviceID]; ok {
		old.Close()
		atomic.AddInt64(&dm.totalConnections, -1)
	}

	shard.devices[dc.DeviceID] = dc
	shard.mu.Unlock()

	atomic.AddInt64(&dm.totalConnections, 1)

	if dm.onConnect != nil {
		dm.onConnect(dc)
	}
}

// Remove 移除设备连接
func (dm *DeviceManager) Remove(deviceID uint32) {
	shard := dm.getShard(deviceID)
	shard.mu.Lock()

	dc, ok := shard.devices[deviceID]
	if ok {
		delete(shard.devices, deviceID)
		dc.Close()
		atomic.AddInt64(&dm.totalConnections, -1)
	}
	shard.mu.Unlock()

	if ok && dm.onDisconnect != nil {
		dm.onDisconnect(dc)
	}
}

// Get 获取设备连接
func (dm *DeviceManager) Get(deviceID uint32) (*DeviceConn, bool) {
	shard := dm.getShard(deviceID)
	shard.mu.RLock()
	dc, ok := shard.devices[deviceID]
	shard.mu.RUnlock()
	return dc, ok
}

// GetBySerialNo 根据序列号获取设备连接
func (dm *DeviceManager) GetBySerialNo(serialno string) (*DeviceConn, bool) {
	for _, shard := range dm.shards {
		shard.mu.RLock()
		for _, dc := range shard.devices {
			if dc.Serialno == serialno {
				shard.mu.RUnlock()
				return dc, true
			}
		}
		shard.mu.RUnlock()
	}
	return nil, false
}

// Count 获取当前连接数
func (dm *DeviceManager) Count() int64 {
	return atomic.LoadInt64(&dm.totalConnections)
}

// TotalMessages 获取总消息数
func (dm *DeviceManager) TotalMessages() int64 {
	return atomic.LoadInt64(&dm.totalMessages)
}

// IncrementMessages 增加消息计数
func (dm *DeviceManager) IncrementMessages() {
	atomic.AddInt64(&dm.totalMessages, 1)
}

// GetAll 获取所有设备连接（用于遍历，注意性能）
func (dm *DeviceManager) GetAll() []*DeviceConn {
	var result []*DeviceConn
	for _, shard := range dm.shards {
		shard.mu.RLock()
		for _, dc := range shard.devices {
			result = append(result, dc)
		}
		shard.mu.RUnlock()
	}
	return result
}

// GetAllDeviceIDs 获取所有设备ID
func (dm *DeviceManager) GetAllDeviceIDs() []uint32 {
	var result []uint32
	for _, shard := range dm.shards {
		shard.mu.RLock()
		for id := range shard.devices {
			result = append(result, id)
		}
		shard.mu.RUnlock()
	}
	return result
}

// CloseAll 关闭所有连接
func (dm *DeviceManager) CloseAll() {
	for _, shard := range dm.shards {
		shard.mu.Lock()
		for id, dc := range shard.devices {
			dc.Close()
			delete(shard.devices, id)
		}
		shard.mu.Unlock()
	}
	atomic.StoreInt64(&dm.totalConnections, 0)
}

// Broadcast 广播消息到所有设备
func (dm *DeviceManager) Broadcast(msgType, dataFormat uint8, payload []byte) int {
	sent := 0
	for _, shard := range dm.shards {
		shard.mu.RLock()
		for _, dc := range shard.devices {
			packet := BuildPacket(msgType, dataFormat, dc.DeviceID, dc.NextSeqNo(), payload)
			if err := dc.WritePacket(packet); err == nil {
				sent++
			}
		}
		shard.mu.RUnlock()
	}
	return sent
}

// BroadcastJSON 广播 JSON 消息到所有设备
func (dm *DeviceManager) BroadcastJSON(msgType uint8, data interface{}) int {
	sent := 0
	for _, shard := range dm.shards {
		shard.mu.RLock()
		for _, dc := range shard.devices {
			packet, err := BuildJSONPacket(msgType, dc.DeviceID, dc.NextSeqNo(), data)
			if err != nil {
				continue
			}
			if err := dc.WritePacket(packet); err == nil {
				sent++
			}
		}
		shard.mu.RUnlock()
	}
	return sent
}

// Stats 获取统计信息
func (dm *DeviceManager) Stats() map[string]interface{} {
	// 统计各状态设备数量
	stateCount := make(map[DeviceState]int)
	for _, shard := range dm.shards {
		shard.mu.RLock()
		for _, dc := range shard.devices {
			stateCount[dc.State]++
		}
		shard.mu.RUnlock()
	}

	return map[string]interface{}{
		"totalConnections": dm.Count(),
		"totalMessages":    dm.TotalMessages(),
		"shardCount":       dm.shardCount,
		"stateCount":       stateCount,
	}
}
