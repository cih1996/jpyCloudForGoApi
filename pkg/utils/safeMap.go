package utils

import (
	"sync"
)

// SafeMap 泛型线程安全 Map，接口兼容 sync.Map
type SafeMap[K comparable, V any] struct {
	mu sync.Mutex
	m  map[K]V
}

// NewSafeMap 创建一个新的 SafeMap
func NewSafeMap[K comparable, V any]() *SafeMap[K, V] {
	return &SafeMap[K, V]{
		m: make(map[K]V),
	}
}

// Load 返回键对应的值和是否存在
func (s *SafeMap[K, V]) Load(key K) (value V, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	value, ok = s.m[key]
	return
}

// Store 设置键值对
func (s *SafeMap[K, V]) Store(key K, value V) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[key] = value
}

// LoadOrStore 如果键存在，返回现有值和 true；否则存入新值并返回新值和 false
func (s *SafeMap[K, V]) LoadOrStore(key K, value V) (actual V, loaded bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	actual, loaded = s.m[key]
	if !loaded {
		s.m[key] = value
		actual = value
	}
	return
}

// Delete 删除指定键
func (s *SafeMap[K, V]) Delete(key K) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, key)
}

// Range 遍历所有键值对，f 返回 false 时停止遍历
func (s *SafeMap[K, V]) Range(f func(key K, value V) bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, v := range s.m {
		if !f(k, v) {
			break
		}
	}
}

// LoadAndDelete 返回键对应的值和是否存在，并删除该键
func (s *SafeMap[K, V]) LoadAndDelete(key K) (value V, loaded bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	value, loaded = s.m[key]
	if loaded {
		delete(s.m, key)
	}
	return
}

// Swap 交换键对应的值，返回旧值和是否存在
func (s *SafeMap[K, V]) Swap(key K, newValue V) (oldValue V, loaded bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	oldValue, loaded = s.m[key]
	s.m[key] = newValue
	return
}

// Clear 清空所有键值对
func (s *SafeMap[K, V]) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	clear(s.m)
	// 重新初始化底层 map 清空所有数据
	s.m = make(map[K]V)
}
