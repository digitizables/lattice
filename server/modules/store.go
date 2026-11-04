package modules

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// KindElevate is the catalog kind for BYO privilege-escalation modules.
const KindElevate = "elevate"

// Module is one published operator module.
type Module struct {
	Name        string    `json:"name"`
	Kind        string    `json:"kind"`
	Version     string    `json:"version"`
	OS          string    `json:"os"`
	Arch        string    `json:"arch"`
	SHA256      string    `json:"sha256"`
	Size        uint64    `json:"size"`
	Description string    `json:"description,omitempty"`
	DefaultArgs []string  `json:"default_args,omitempty"`
	PublishedAt time.Time `json:"published_at"`
	File        string    `json:"file"`
}

type catalogFile struct {
	Modules []Module `json:"modules"`
}

// Store keeps published modules under root.
type Store struct {
	root string
	mu   sync.RWMutex
}

// New creates/opens a module catalog.
func New(root string) (*Store, error) {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, err
	}
	s := &Store{root: root}
	if _, err := s.load(); err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	return s, nil
}

func (s *Store) catalogPath() string { return filepath.Join(s.root, "catalog.json") }

func (s *Store) load() (catalogFile, error) {
	raw, err := os.ReadFile(s.catalogPath())
	if err != nil {
		return catalogFile{}, err
	}
	var cat catalogFile
	if err := json.Unmarshal(raw, &cat); err != nil {
		return catalogFile{}, err
	}
	return cat, nil
}

func (s *Store) save(cat catalogFile) error {
	raw, err := json.MarshalIndent(cat, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.catalogPath() + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.catalogPath())
}

// Publish stores a module binary.
func (s *Store) Publish(m Module, data []byte) (Module, error) {
	m.Name = strings.ToLower(strings.TrimSpace(m.Name))
	m.Kind = strings.ToLower(strings.TrimSpace(m.Kind))
	m.Version = strings.TrimSpace(m.Version)
	m.OS = strings.ToLower(strings.TrimSpace(m.OS))
	m.Arch = strings.ToLower(strings.TrimSpace(m.Arch))
	if m.Name == "" || m.Kind == "" || m.Version == "" || m.OS == "" || m.Arch == "" {
		return Module{}, fmt.Errorf("name, kind, version, os, and arch are required")
	}
	if len(data) == 0 {
		return Module{}, fmt.Errorf("empty module")
	}
	sum := sha256.Sum256(data)
	m.SHA256 = hex.EncodeToString(sum[:])
	m.Size = uint64(len(data))
	m.PublishedAt = time.Now().UTC()

	dir := filepath.Join(s.root, m.Kind, m.Name, m.Version)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Module{}, err
	}
	base := m.OS + "_" + m.Arch
	if m.OS == "windows" {
		base += ".exe"
	}
	rel := filepath.ToSlash(filepath.Join(m.Kind, m.Name, m.Version, base))
	if err := os.WriteFile(filepath.Join(s.root, filepath.FromSlash(rel)), data, 0o755); err != nil {
		return Module{}, err
	}
	m.File = rel

	s.mu.Lock()
	defer s.mu.Unlock()
	cat, err := s.load()
	if err != nil && !os.IsNotExist(err) {
		return Module{}, err
	}
	filtered := cat.Modules[:0]
	for _, x := range cat.Modules {
		if x.Name == m.Name && x.Kind == m.Kind && x.OS == m.OS && x.Arch == m.Arch && x.Version == m.Version {
			continue
		}
		filtered = append(filtered, x)
	}
	cat.Modules = append(filtered, m)
	if err := s.save(cat); err != nil {
		return Module{}, err
	}
	return m, nil
}

// List returns modules, optionally filtered by kind.
func (s *Store) List(kind string) ([]Module, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cat, err := s.load()
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	kind = strings.ToLower(strings.TrimSpace(kind))
	out := make([]Module, 0, len(cat.Modules))
	for _, m := range cat.Modules {
		if kind != "" && m.Kind != kind {
			continue
		}
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Name == out[j].Name {
			return out[i].PublishedAt.After(out[j].PublishedAt)
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

// Get returns the newest matching module (or a specific version when set).
func (s *Store) Get(kind, name, osName, arch, version string) (Module, []byte, error) {
	list, err := s.List(kind)
	if err != nil {
		return Module{}, nil, err
	}
	name = strings.ToLower(name)
	osName = strings.ToLower(osName)
	arch = strings.ToLower(arch)
	version = strings.TrimSpace(version)
	var found *Module
	for i := range list {
		m := &list[i]
		if m.Name != name || m.OS != osName || m.Arch != arch {
			continue
		}
		if version != "" && m.Version != version {
			continue
		}
		found = m
		break
	}
	if found == nil {
		return Module{}, nil, fmt.Errorf("module %s/%s for %s/%s not found", kind, name, osName, arch)
	}
	data, err := os.ReadFile(filepath.Join(s.root, filepath.FromSlash(found.File)))
	if err != nil {
		return Module{}, nil, err
	}
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) != found.SHA256 {
		return Module{}, nil, fmt.Errorf("module hash mismatch")
	}
	return *found, data, nil
}
