package catalog

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadUsesRootOrderForCollisionsAndDiscoversResources(t *testing.T) {
	first, second := t.TempDir(), t.TempDir()
	writeSkill(t, first, "review", "First", "first body")
	writeSkill(t, second, "review", "Second", "second body")
	if err := os.WriteFile(filepath.Join(first, "review", "references.md"), []byte("ref"), 0o600); err != nil {
		t.Fatal(err)
	}
	s, err := Load([]string{first, second}, DefaultMaxFileBytes)
	if err != nil {
		t.Fatal(err)
	}
	if got := s.Skills["review"].Description; got != "First" {
		t.Fatalf("description=%q", got)
	}
	if len(s.Shadowed["review"]) != 2 {
		t.Fatalf("shadowed=%v", s.Shadowed["review"])
	}
	if len(s.Skills["review"].Resources) != 1 || s.Skills["review"].Resources[0] != "references.md" {
		t.Fatalf("resources=%v", s.Skills["review"].Resources)
	}
}

func TestReadResourceRejectsTraversal(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "review", "Review", "body")
	s, err := Load([]string{root}, DefaultMaxFileBytes)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ReadResource("review", "../secret", DefaultMaxFileBytes); err == nil {
		t.Fatal("expected traversal error")
	}
}

func writeSkill(t *testing.T, root, name, description, body string) {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "---\nname: " + name + "\ndescription: " + description + "\n---\n\n" + body + "\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
