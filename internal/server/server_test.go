package server

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mhingston/skillrouter/internal/catalog"
)

func TestNewEngineNormalizesDefaultFileLimit(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "review")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: review\ndescription: Review code\n---\nbody\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "reference.md"), []byte("reference"), 0o600); err != nil {
		t.Fatal(err)
	}
	engine, err := NewEngine([]string{root}, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if engine.maxBytes != catalog.DefaultMaxFileBytes {
		t.Fatalf("maxBytes=%d", engine.maxBytes)
	}
	if _, err := engine.snapshot.ReadResource("review", "reference.md", engine.maxBytes); err != nil {
		t.Fatal(err)
	}
}

func TestStaleCheckReportsMissingRoot(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "review")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: review\ndescription: Review code\n---\nbody\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	engine, err := NewEngine([]string{root}, catalog.DefaultMaxFileBytes, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(root); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.isStale(); err == nil {
		t.Fatal("expected missing root to surface as drift error")
	}
}
