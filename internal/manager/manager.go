package manager

import (
	"port-mapping-demo/coreClass"
	"port-mapping-demo/internal/model"
	"sync"
)

type MappingManager struct {
	activeMappings map[int]model.MappingSession
	lock           sync.RWMutex
	Core           *coreClass.Core
}

var (
	instance *MappingManager
	once     sync.Once
)

func GetInstance() *MappingManager {
	once.Do(func() {
		instance = &MappingManager{
			activeMappings: make(map[int]model.MappingSession),
			Core:           coreClass.NewCore(),
		}
	})
	return instance
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
