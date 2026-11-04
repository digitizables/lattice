package updates

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

// Artifact is one published agent binary.
type Artifact struct {
	Version     string    `json:"version"`
	OS          string    `json:"os"`
	Arch        string    `json:"arch"`
	SHA256      string    `json:"sha256"`
	Size        uint64    `json:"size"`
	PublishedAt time.Time `json:"published_at"`
	File        string    `json:"file"` // relative to root
}

type catalogFile struct {
	Artifacts []Artifact `json:"artifacts"`
}

// Store keeps published agent builds on disk under root.
type Store struct {
	root string
	mu   sync.RWMutex
}

// New opens (or creates) an update catalog at root.
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

func (s *Store) catalogPath() string {
	return filepath.Join(s.root, "catalog.json")
}

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

func platformKey(osName, arch string) string {
	return strings.ToLower(osName) + "_" + strings.ToLower(arch)
}

// Publish stores a binary and updates the catalog (replaces same os/arch/version).
func (s *Store) Publish(version, osName, arch string, data []byte) (Artifact, error) {
	version = strings.TrimSpace(version)
	osName = strings.ToLower(strings.TrimSpace(osName))
	arch = strings.ToLower(strings.TrimSpace(arch))
	if version == "" || osName == "" || arch == "" {
		return Artifact{}, fmt.Errorf("version, os, and arch are required")
	}
	if len(data) == 0 {
		return Artifact{}, fmt.Errorf("empty binary")
	}

	sum := sha256.Sum256(data)
	sha := hex.EncodeToString(sum[:])
	dir := filepath.Join(s.root, version)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Artifact{}, err
	}
	name := platformKey(osName, arch)
	if osName == "windows" {
		name += ".exe"
	}
	rel := filepath.ToSlash(filepath.Join(version, name))
	abs := filepath.Join(s.root, version, name)
	if err := os.WriteFile(abs, data, 0o755); err != nil {
		return Artifact{}, err
	}

	art := Artifact{
		Version:     version,
		OS:          osName,
		Arch:        arch,
		SHA256:      sha,
		Size:        uint64(len(data)),
		PublishedAt: time.Now().UTC(),
		File:        rel,
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	cat, err := s.load()
	if err != nil && !os.IsNotExist(err) {
		return Artifact{}, err
	}
	filtered := cat.Artifacts[:0]
	for _, a := range cat.Artifacts {
		if a.OS == osName && a.Arch == arch && a.Version == version {
			continue
		}
		filtered = append(filtered, a)
	}
	cat.Artifacts = append(filtered, art)
	if err := s.save(cat); err != nil {
		return Artifact{}, err
	}
	return art, nil
}

// List returns all artifacts, newest first.
func (s *Store) List() ([]Artifact, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cat, err := s.load()
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	out := append([]Artifact(nil), cat.Artifacts...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].PublishedAt.Equal(out[j].PublishedAt) {
			return out[i].Version > out[j].Version
		}
		return out[i].PublishedAt.After(out[j].PublishedAt)
	})
	return out, nil
}

// Latest returns the newest artifact for os/arch, if any.
func (s *Store) Latest(osName, arch string) (Artifact, bool, error) {
	list, err := s.List()
	if err != nil {
		return Artifact{}, false, err
	}
	osName = strings.ToLower(osName)
	arch = strings.ToLower(arch)
	for _, a := range list {
		if a.OS == osName && a.Arch == arch {
			return a, true, nil
		}
	}
	return Artifact{}, false, nil
}

// Get returns a specific version for os/arch, or latest when version is empty.
func (s *Store) Get(osName, arch, version string) (Artifact, []byte, error) {
	osName = strings.ToLower(osName)
	arch = strings.ToLower(arch)
	version = strings.TrimSpace(version)

	var art Artifact
	var ok bool
	var err error
	if version == "" {
		art, ok, err = s.Latest(osName, arch)
		if err != nil {
			return Artifact{}, nil, err
		}
		if !ok {
			return Artifact{}, nil, fmt.Errorf("no update published for %s/%s", osName, arch)
		}
	} else {
		list, err := s.List()
		if err != nil {
			return Artifact{}, nil, err
		}
		for _, a := range list {
			if a.OS == osName && a.Arch == arch && a.Version == version {
				art = a
				ok = true
				break
			}
		}
		if !ok {
			return Artifact{}, nil, fmt.Errorf("update %s for %s/%s not found", version, osName, arch)
		}
	}

	data, err := os.ReadFile(filepath.Join(s.root, filepath.FromSlash(art.File)))
	if err != nil {
		return Artifact{}, nil, err
	}
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) != art.SHA256 {
		return Artifact{}, nil, fmt.Errorf("catalog hash mismatch for %s", art.File)
	}
	return art, data, nil
}

// NewerThan reports whether latest is newer than current (string inequality / semver-ish compare).
func NewerThan(latest, current string) bool {
	latest = strings.TrimSpace(latest)
	current = strings.TrimSpace(current)
	if latest == "" || latest == current {
		return false
	}
	if current == "" {
		return true
	}
	return compareVersions(latest, current) > 0
}

func compareVersions(a, b string) int {
	as := strings.Split(strings.TrimPrefix(a, "v"), ".")
	bs := strings.Split(strings.TrimPrefix(b, "v"), ".")
	n := len(as)
	if len(bs) > n {
		n = len(bs)
	}
	for i := 0; i < n; i++ {
		var ai, bi int
		if i < len(as) {
			fmt.Sscanf(as[i], "%d", &ai)
		}
		if i < len(bs) {
			fmt.Sscanf(bs[i], "%d", &bi)
		}
		if ai > bi {
			return 1
		}
		if ai < bi {
			return -1
		}
	}
	return 0
}
