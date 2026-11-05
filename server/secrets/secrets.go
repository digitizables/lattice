package secrets

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// EnsureAgentToken returns LATTICE_AGENT_TOKEN if set, otherwise loads or creates
// data/secrets/agent_token (remix of ephub first-boot secret file).
func EnsureAgentToken(secretsDir, envToken string) (token string, generated bool, err error) {
	envToken = strings.TrimSpace(envToken)
	if envToken != "" {
		if len(envToken) < 16 {
			return "", false, fmt.Errorf("LATTICE_AGENT_TOKEN must be at least 16 characters")
		}
		return envToken, false, nil
	}
	if err := os.MkdirAll(secretsDir, 0o700); err != nil {
		return "", false, err
	}
	path := filepath.Join(secretsDir, "agent_token")
	if b, err := os.ReadFile(path); err == nil {
		tok := strings.TrimSpace(string(b))
		if len(tok) >= 16 {
			return tok, false, nil
		}
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", false, err
	}
	tok := hex.EncodeToString(buf)
	if err := os.WriteFile(path, []byte(tok+"\n"), 0o600); err != nil {
		return "", false, err
	}
	return tok, true, nil
}
