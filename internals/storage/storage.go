// Package storage provides the storage interface.
package storage

type Storage interface {
	Save() error
	Load() error
	Delete() error
}
