package identity

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DefaultIDFile returns the durable agent ID path.
// Prefer LATTICE_AGENT_ID_FILE, otherwise ~/.config/lattice/agent.id (or OS equivalent).
func DefaultIDFile() string {
	if v := strings.TrimSpace(os.Getenv("LATTICE_AGENT_ID_FILE")); v != "" {
		return v
	}
	dir, err := os.UserConfigDir()
	if err != nil || dir == "" {
		return "agent.id"
	}
	return filepath.Join(dir, "lattice", "agent.id")
}

// Load reads a previously assigned agent ID from path.
func Load(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	id := strings.TrimSpace(string(raw))
	return id, nil
}

// Save writes agent ID to path, creating parent directories as needed.
func Save(path, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("refusing to save empty agent id")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(id+"\n"), 0o600)
}
