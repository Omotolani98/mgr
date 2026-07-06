// Package storage provides the storage interface.
package storage

type Storage interface {
	Save(key string, value []byte) error
	Load(key string) ([]byte, error)
	Delete(key string) error
}
