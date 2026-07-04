package storage

type InMemoryStorage struct {
	data map[string][]byte
}

func NewInMemoryStorage() *InMemoryStorage {
	return &InMemoryStorage{
		data: make(map[string][]byte),
	}
}

func (s *InMemoryStorage) Save() error {
	return nil
}

func (s *InMemoryStorage) Load() error {
	return nil
}

func (s *InMemoryStorage) Delete(key string) {
	delete(s.data, key)
}
