package storage

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/Omotolani98/mgr/internals/config"
)

type FileStore struct {
	Path string
}

func (f *FileStore) Save(key string, value []byte) error {
	path := f.path(key)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, value, 0o600)
}

func (f *FileStore) Load(key string) ([]byte, error) {
	data, err := os.ReadFile(f.path(key))
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotFound
	}
	return data, err
}

func (f *FileStore) Delete(key string) error {
	err := os.Remove(f.path(key))
	if errors.Is(err, os.ErrNotExist) {
		return ErrNotFound
	}
	return err
}

func (f *FileStore) path(key string) string {
	if f.Path != "" {
		return f.Path
	}
	if config.Home == "" {
		config.InitConfig()
	}
	return filepath.Join(config.Home, config.STOREFILENAME)
}
