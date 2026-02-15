package manager

import (
	// 使用 JpyApiAgent 的集控平台核心模块替代旧的 coreClass
	"port-mapping-demo/internal/model"
	centCtl "port-mapping-demo/third_party/JpyApiAgent/table/coreClass/centControlPlatform"
	"sync"
)

type MappingManager struct {
	activeMappings map[int]model.MappingSession
	lock           sync.RWMutex
	// Core 替换为 JpyApiAgent 的 centControlPlatform.Core
	// 内部已封装登录、中间件RTC连接、端口映射等全部通讯逻辑
	Core *centCtl.Core
}

var (
	instance *MappingManager
	once     sync.Once
)

// GetInstance 获取单例管理器
// 注意：Core 初始化延迟到 EnsureLogin 时通过 SetCore 设置
func GetInstance() *MappingManager {
	once.Do(func() {
		instance = &MappingManager{
			activeMappings: make(map[int]model.MappingSession),
		}
	})
	return instance
}

// SetCore 设置 JpyApiAgent Core 实例（登录成功后调用）
func (m *MappingManager) SetCore(core *centCtl.Core) {
	m.Core = core
}
func (m *MappingManager) AddMapping(port int, session model.MappingSession) {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.activeMappings[port] = session
}

func (m *MappingManager) RemoveMapping(port int) {
	m.lock.Lock()
	defer m.lock.Unlock()
	delete(m.activeMappings, port)
}

func (m *MappingManager) GetMapping(port int) (model.MappingSession, bool) {
	m.lock.RLock()
	defer m.lock.RUnlock()
	val, ok := m.activeMappings[port]
	return val, ok
}

func (m *MappingManager) GetAllMappings() []model.MappingSession {
	m.lock.RLock()
	defer m.lock.RUnlock()
	list := make([]model.MappingSession, 0, len(m.activeMappings))
	for _, v := range m.activeMappings {
		list = append(list, v)
	}
	return list
}

func (m *MappingManager) HasMapping(port int) bool {
	m.lock.RLock()
	defer m.lock.RUnlock()
	_, ok := m.activeMappings[port]
	return ok
}
