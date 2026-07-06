package inventory

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestFileStoreUpsertListGetRemove(t *testing.T) {
	store := NewFileStore(filepath.Join(t.TempDir(), "inventory.yaml"))

	saved, err := store.Upsert(Server{
		Name: "prod-api",
		Host: "10.0.0.5",
		User: "ubuntu",
		Tags: []string{"prod", "api", "prod"},
	})
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if saved.ID != "prod-api" {
		t.Fatalf("ID = %q, want prod-api", saved.ID)
	}

	got, err := store.Get("prod-api")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Port != 22 {
		t.Fatalf("Port = %d, want 22", got.Port)
	}
	if len(got.Tags) != 2 {
		t.Fatalf("Tags = %#v, want deduplicated tags", got.Tags)
	}

	if _, err := store.Upsert(Server{Name: "dev-db", Host: "127.0.0.1", Group: "dev", Tags: []string{"db"}}); err != nil {
		t.Fatalf("upsert second: %v", err)
	}
	filtered, err := store.List(Filter{Tag: "db"})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(filtered) != 1 || filtered[0].Name != "dev-db" {
		t.Fatalf("filtered = %#v, want dev-db only", filtered)
	}

	if err := store.Remove("prod-api"); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if _, err := store.Get("prod-api"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("get removed error = %v, want ErrNotFound", err)
	}
}

func TestFileStoreRejectsIncompleteServer(t *testing.T) {
	store := NewFileStore(filepath.Join(t.TempDir(), "inventory.yaml"))
	if _, err := store.Upsert(Server{Host: "localhost"}); err == nil {
		t.Fatal("expected missing name error")
	}
	if _, err := store.Upsert(Server{Name: "local"}); err == nil {
		t.Fatal("expected missing host error")
	}
}
