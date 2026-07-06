package env

import (
	"strings"
	"testing"
)

func TestRenderDotenvSorted(t *testing.T) {
	out, err := RenderDotenv(map[string]string{
		"DB_URL": "postgres://localhost/app",
		"API":    "ok",
	})
	if err != nil {
		t.Fatalf("RenderDotenv: %v", err)
	}
	got := string(out)
	want := "API=ok\nDB_URL=postgres://localhost/app\n"
	if got != want {
		t.Fatalf("dotenv = %q, want %q", got, want)
	}
}

func TestRenderDotenvRejectsUnsafeData(t *testing.T) {
	if _, err := RenderDotenv(map[string]string{"bad-key": "value"}); err == nil {
		t.Fatal("expected invalid key error")
	}
	_, err := RenderDotenv(map[string]string{"TOKEN": "line1\nline2"})
	if err == nil || !strings.Contains(err.Error(), "newline") {
		t.Fatalf("error = %v, want newline rejection", err)
	}
}
