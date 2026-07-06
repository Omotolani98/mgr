package storage

import (
	"errors"
	"sync"
)

type InMemoryStorage struct {
	mu   sync.RWMutex
	data map[string][]byte
}

var ErrNotFound = errors.New("key not found")

func NewInMemoryStorage() *InMemoryStorage {
	return &InMemoryStorage{
		data: make(map[string][]byte),
	}
}

func (m *InMemoryStorage) Save(key string, value []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[key] = value
	return nil
}

func (m *InMemoryStorage) Load(key string) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.data[key]
	if !ok {
		return nil, ErrNotFound
	}
	return v, nil
}

func (m *InMemoryStorage) Delete(key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.data[key]
	if !ok {
		return ErrNotFound
	}
	delete(m.data, key)
	return nil
}
