// Package inventory manages mgr's local server inventory.
package inventory

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"

	"go.yaml.in/yaml/v3"
)

var ErrNotFound = errors.New("server not found")

type Server struct {
	ID           string    `yaml:"id" json:"id"`
	Name         string    `yaml:"name" json:"name"`
	Host         string    `yaml:"host" json:"host"`
	Port         int       `yaml:"port" json:"port"`
	User         string    `yaml:"user,omitempty" json:"user,omitempty"`
	IdentityFile string    `yaml:"identity_file,omitempty" json:"identity_file,omitempty"`
	Tags         []string  `yaml:"tags,omitempty" json:"tags,omitempty"`
	Group        string    `yaml:"group,omitempty" json:"group,omitempty"`
	Env          string    `yaml:"env,omitempty" json:"env,omitempty"`
	Source       string    `yaml:"source,omitempty" json:"source,omitempty"`
	SourceID     string    `yaml:"source_id,omitempty" json:"source_id,omitempty"`
	LastSeenAt   time.Time `yaml:"last_seen_at,omitempty" json:"last_seen_at,omitempty"`
	CreatedAt    time.Time `yaml:"created_at" json:"created_at"`
	UpdatedAt    time.Time `yaml:"updated_at" json:"updated_at"`
}

type FileStore struct {
	path string
}

type fileData struct {
	Version int      `yaml:"version"`
	Servers []Server `yaml:"servers"`
}

type Filter struct {
	Group  string
	Tag    string
	Source string
}

func NewFileStore(path string) *FileStore {
	return &FileStore{path: path}
}

func (s *FileStore) Path() string {
	return s.path
}

func (s *FileStore) List(filter Filter) ([]Server, error) {
	data, err := s.load()
	if err != nil {
		return nil, err
	}
	out := make([]Server, 0, len(data.Servers))
	for _, srv := range data.Servers {
		if filter.Group != "" && srv.Group != filter.Group {
			continue
		}
		if filter.Tag != "" && !hasTag(srv.Tags, filter.Tag) {
			continue
		}
		if filter.Source != "" && srv.Source != filter.Source {
			continue
		}
		out = append(out, srv)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out, nil
}

func (s *FileStore) Get(name string) (Server, error) {
	data, err := s.load()
	if err != nil {
		return Server{}, err
	}
	for _, srv := range data.Servers {
		if srv.Name == name || srv.ID == name {
			return srv, nil
		}
	}
	return Server{}, ErrNotFound
}

func (s *FileStore) Upsert(server Server) (Server, error) {
	if server.Name == "" {
		return Server{}, errors.New("server name is required")
	}
	if server.Host == "" {
		return Server{}, errors.New("server host is required")
	}
	if server.Port == 0 {
		server.Port = 22
	}
	server.ID = normalizeID(server.ID, server.Name)
	if server.SourceID == "" && server.Source != "" {
		server.SourceID = server.ID
	}
	server.Tags = normalizeTags(server.Tags)

	data, err := s.load()
	if err != nil {
		return Server{}, err
	}
	now := time.Now().UTC()
	for i, existing := range data.Servers {
		if existing.Name == server.Name || existing.ID == server.ID {
			if server.CreatedAt.IsZero() {
				server.CreatedAt = existing.CreatedAt
			}
			if server.LastSeenAt.IsZero() {
				server.LastSeenAt = existing.LastSeenAt
			}
			server.UpdatedAt = now
			data.Servers[i] = server
			return server, s.save(data)
		}
	}
	server.CreatedAt = now
	server.UpdatedAt = now
	if server.LastSeenAt.IsZero() && server.Source != "" {
		server.LastSeenAt = now
	}
	data.Servers = append(data.Servers, server)
	return server, s.save(data)
}

func (s *FileStore) Remove(name string) error {
	data, err := s.load()
	if err != nil {
		return err
	}
	for i, srv := range data.Servers {
		if srv.Name == name || srv.ID == name {
			data.Servers = append(data.Servers[:i], data.Servers[i+1:]...)
			return s.save(data)
		}
	}
	return ErrNotFound
}

func (s *FileStore) Import(servers []Server) (int, error) {
	count := 0
	for _, server := range servers {
		if _, err := s.Upsert(server); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

func (s *FileStore) load() (fileData, error) {
	data := fileData{Version: 1}
	raw, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return data, nil
	}
	if err != nil {
		return data, err
	}
	if err := yaml.Unmarshal(raw, &data); err != nil {
		return data, fmt.Errorf("parse inventory %s: %w", s.path, err)
	}
	if data.Version == 0 {
		data.Version = 1
	}
	return data, nil
}

func (s *FileStore) save(data fileData) error {
	sort.Slice(data.Servers, func(i, j int) bool {
		return data.Servers[i].Name < data.Servers[j].Name
	})
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	raw, err := yaml.Marshal(data)
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, raw, 0o600)
}

func normalizeID(id, name string) string {
	if id != "" {
		return id
	}
	var b strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(name) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

func normalizeTags(tags []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(tags))
	for _, tag := range tags {
		tag = strings.TrimSpace(strings.ToLower(tag))
		if tag == "" {
			continue
		}
		if _, ok := seen[tag]; ok {
			continue
		}
		seen[tag] = struct{}{}
		out = append(out, tag)
	}
	sort.Strings(out)
	return out
}

func hasTag(tags []string, tag string) bool {
	tag = strings.ToLower(strings.TrimSpace(tag))
	for _, t := range tags {
		if t == tag {
			return true
		}
	}
	return false
}
