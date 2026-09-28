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

func TestStatSignatureChangesWhenResourceChanges(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "review", "Review", "body")
	resource := filepath.Join(root, "review", "reference.md")
	if err := os.WriteFile(resource, []byte("one"), 0o600); err != nil {
		t.Fatal(err)
	}
	before, err := StatSignature([]string{root})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(resource, []byte("two-and-longer"), 0o600); err != nil {
		t.Fatal(err)
	}
	after, err := StatSignature([]string{root})
	if err != nil {
		t.Fatal(err)
	}
	if before == after {
		t.Fatal("expected resource change to alter disk signature")
	}
}

func TestReadResourceRejectsSymlinkSwapAfterDiscovery(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "review", "Review", "body")
	resource := filepath.Join(root, "review", "reference.md")
	if err := os.WriteFile(resource, []byte("inside"), 0o600); err != nil {
		t.Fatal(err)
	}
	s, err := Load([]string{root}, DefaultMaxFileBytes)
	if err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "outside.md")
	if err := os.WriteFile(outside, []byte("outside"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(resource); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, resource); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := s.ReadResource("review", "reference.md", DefaultMaxFileBytes); err == nil {
		t.Fatal("expected symlink swap to be rejected")
	}
}

func TestReadResourceNormalizesNonPositiveLimit(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "review", "Review", "body")
	resource := filepath.Join(root, "review", "reference.md")
	if err := os.WriteFile(resource, []byte("inside"), 0o600); err != nil {
		t.Fatal(err)
	}
	s, err := Load([]string{root}, 0)
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.ReadResource("review", "reference.md", 0)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "inside" {
		t.Fatalf("content=%q", got)
	}
}
