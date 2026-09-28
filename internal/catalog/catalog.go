package catalog

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

const DefaultMaxFileBytes int64 = 1 << 20

type Skill struct {
	Name        string
	Description string
	Body        string
	Path        string
	Dir         string
	Root        string
	Resources   []string
	Fingerprint string
}

type Snapshot struct {
	Skills    map[string]Skill
	Names     []string
	Shadowed  map[string][]string
	Signature string
	Roots     []string
}

type frontmatter struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	WhenToUse   string `yaml:"when_to_use"`
}

func Load(roots []string, maxFileBytes int64) (*Snapshot, error) {
	if maxFileBytes <= 0 {
		maxFileBytes = DefaultMaxFileBytes
	}
	cleanRoots := make([]string, 0, len(roots))
	for _, root := range roots {
		root = strings.TrimSpace(root)
		if root == "" {
			continue
		}
		abs, err := filepath.Abs(expandHome(root))
		if err != nil {
			return nil, fmt.Errorf("resolve skills root %q: %w", root, err)
		}
		cleanRoots = append(cleanRoots, filepath.Clean(abs))
	}
	if len(cleanRoots) == 0 {
		return nil, errors.New("at least one skills directory is required")
	}

	s := &Snapshot{
		Skills:   map[string]Skill{},
		Shadowed: map[string][]string{},
		Roots:    cleanRoots,
	}

	for _, root := range cleanRoots {
		info, err := os.Stat(root)
		if err != nil {
			return nil, fmt.Errorf("skills root %q: %w", root, err)
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("skills root %q is not a directory", root)
		}
		var paths []string
		err = filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.Type()&os.ModeSymlink != 0 {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if !d.IsDir() && strings.EqualFold(d.Name(), "SKILL.md") {
				paths = append(paths, path)
			}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("scan skills root %q: %w", root, err)
		}
		sort.Strings(paths)
		for _, path := range paths {
			skill, err := parseSkill(root, path, maxFileBytes)
			if err != nil {
				continue
			}
			if existing, ok := s.Skills[skill.Name]; ok {
				s.Shadowed[skill.Name] = append(s.Shadowed[skill.Name], skill.Path)
				if len(s.Shadowed[skill.Name]) == 1 {
					s.Shadowed[skill.Name] = append([]string{existing.Path}, s.Shadowed[skill.Name]...)
				}
				continue
			}
			s.Skills[skill.Name] = skill
			s.Names = append(s.Names, skill.Name)
		}
	}
	sort.Strings(s.Names)
	s.Signature = signature(s)
	return s, nil
}

func parseSkill(root, path string, maxFileBytes int64) (Skill, error) {
	info, err := os.Stat(path)
	if err != nil {
		return Skill{}, err
	}
	if !info.Mode().IsRegular() || info.Size() > maxFileBytes {
		return Skill{}, fmt.Errorf("invalid SKILL.md size or type")
	}
	rawBytes, err := os.ReadFile(path)
	if err != nil {
		return Skill{}, err
	}
	raw := strings.ReplaceAll(string(rawBytes), "\r\n", "\n")
	fmText, body, ok := splitFrontmatter(raw)
	if !ok {
		return Skill{}, fmt.Errorf("missing YAML frontmatter")
	}
	var fm frontmatter
	if err := yaml.Unmarshal([]byte(fmText), &fm); err != nil {
		return Skill{}, fmt.Errorf("parse frontmatter: %w", err)
	}
	name := strings.TrimSpace(fm.Name)
	if name == "" {
		name = filepath.Base(filepath.Dir(path))
	}
	if name == "" {
		return Skill{}, fmt.Errorf("skill name is empty")
	}
	description := strings.TrimSpace(fm.Description)
	if extra := strings.TrimSpace(fm.WhenToUse); extra != "" {
		if description != "" {
			description += " "
		}
		description += extra
	}
	dir := filepath.Dir(path)
	resources, err := discoverResources(dir, path, maxFileBytes)
	if err != nil {
		return Skill{}, err
	}
	h := sha256.Sum256(rawBytes)
	return Skill{
		Name:        name,
		Description: description,
		Body:        strings.TrimSpace(body),
		Path:        path,
		Dir:         dir,
		Root:        root,
		Resources:   resources,
		Fingerprint: hex.EncodeToString(h[:]),
	}, nil
}

func splitFrontmatter(raw string) (string, string, bool) {
	if !strings.HasPrefix(raw, "---\n") {
		return "", "", false
	}
	rest := raw[4:]
	idx := strings.Index(rest, "\n---\n")
	if idx < 0 {
		if strings.HasSuffix(rest, "\n---") {
			return strings.TrimSuffix(rest, "\n---"), "", true
		}
		return "", "", false
	}
	return rest[:idx], rest[idx+5:], true
}

func discoverResources(dir, skillPath string, maxFileBytes int64) ([]string, error) {
	var resources []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.Type()&os.ModeSymlink != 0 {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() || path == skillPath {
			return nil
		}
		info, err := d.Info()
		if err != nil || !info.Mode().IsRegular() || info.Size() > maxFileBytes {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return nil
		}
		resources = append(resources, filepath.ToSlash(rel))
		return nil
	})
	sort.Strings(resources)
	return resources, err
}

func (s *Snapshot) ReadResource(skillName, resource string, maxFileBytes int64) ([]byte, error) {
	skill, ok := s.Skills[skillName]
	if !ok {
		return nil, fmt.Errorf("unknown skill %q", skillName)
	}
	resource = filepath.FromSlash(strings.TrimSpace(resource))
	if resource == "" || filepath.IsAbs(resource) {
		return nil, fmt.Errorf("resource path must be relative")
	}
	clean := filepath.Clean(resource)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return nil, fmt.Errorf("resource path escapes skill directory")
	}
	allowed := false
	for _, candidate := range skill.Resources {
		if filepath.Clean(filepath.FromSlash(candidate)) == clean {
			allowed = true
			break
		}
	}
	if !allowed {
		return nil, fmt.Errorf("resource %q is not declared by the discovered skill", resource)
	}
	path := filepath.Join(skill.Dir, clean)
	resolvedDir, err := filepath.EvalSymlinks(skill.Dir)
	if err != nil {
		return nil, err
	}
	resolvedPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		return nil, err
	}
	rel, err := filepath.Rel(resolvedDir, resolvedPath)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return nil, fmt.Errorf("resource path escapes skill directory")
	}
	info, err := os.Stat(resolvedPath)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > maxFileBytes {
		return nil, fmt.Errorf("resource is not a readable regular file within size limit")
	}
	content, err := os.ReadFile(resolvedPath)
	if err != nil {
		return nil, err
	}
	if !utf8.Valid(content) {
		return nil, fmt.Errorf("resource is not valid UTF-8 text")
	}
	return content, nil
}

func signature(s *Snapshot) string {
	h := sha256.New()
	for _, name := range s.Names {
		skill := s.Skills[name]
		_, _ = fmt.Fprintf(h, "%s\x00%s\x00%s\n", name, skill.Path, skill.Fingerprint)
	}
	return hex.EncodeToString(h.Sum(nil))
}

func expandHome(path string) string {
	if path == "~" || strings.HasPrefix(path, "~/") || strings.HasPrefix(path, "~\\") {
		if home, err := os.UserHomeDir(); err == nil {
			if path == "~" {
				return home
			}
			return filepath.Join(home, path[2:])
		}
	}
	return path
}
