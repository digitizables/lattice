package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/digitizables/lattice/server/store"
)

const (
	HeaderAuthorization = "Authorization"
	HeaderOperatorToken = "X-Lattice-Operator-Token"
	DefaultTTLHours     = 12
	MinPasswordLen      = 8
	MinTokenLen         = 16
)

// Auth manages operator passwords and bearer sessions.
type Auth struct {
	store *store.Store
	ttl   time.Duration

	mu   sync.Mutex
	toks map[string]session

	failMu sync.Mutex
	fails  map[string]loginFail

	disabled bool // LATTICE_AUTH_DISABLED=true for open lab only
}

type session struct {
	username string
	expiry   time.Time
}

type loginFail struct {
	count    int
	lockedUntil time.Time
}

// New builds an Auth helper. disabled skips token checks (lab escape hatch).
func New(st *store.Store, ttlHours int, disabled bool) *Auth {
	if ttlHours <= 0 {
		ttlHours = DefaultTTLHours
	}
	return &Auth{
		store:    st,
		ttl:      time.Duration(ttlHours) * time.Hour,
		toks:     map[string]session{},
		fails:    map[string]loginFail{},
		disabled: disabled,
	}
}

// Disabled reports whether operator auth is off.
func (a *Auth) Disabled() bool { return a == nil || a.disabled }

// BootstrapAdmin creates the first operator when the table is empty.
func (a *Auth) BootstrapAdmin(ctx context.Context, username, password string) (created bool, err error) {
	if a == nil || a.store == nil {
		return false, fmt.Errorf("auth not configured")
	}
	n, err := a.store.CountOperators(ctx)
	if err != nil {
		return false, err
	}
	if n > 0 {
		return false, nil
	}
	username = strings.TrimSpace(username)
	if username == "" {
		username = "admin"
	}
	if len(password) < MinPasswordLen {
		return false, fmt.Errorf("bootstrap password must be at least %d characters", MinPasswordLen)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return false, err
	}
	if err := a.store.CreateOperator(ctx, store.Operator{
		ID:           store.NewID(),
		Username:     username,
		PasswordHash: string(hash),
	}); err != nil {
		return false, err
	}
	return true, nil
}

// Login verifies credentials and returns a bearer token.
func (a *Auth) Login(ctx context.Context, username, password, clientIP string) (token string, expiresIn int64, err error) {
	if a.Disabled() {
		tok := a.issue("lab")
		return tok, int64(a.ttl.Seconds()), nil
	}
	lockKey := clientIP
	if lockKey == "" {
		lockKey = "user:" + strings.ToLower(strings.TrimSpace(username))
	}
	if err := a.checkLockout(lockKey); err != nil {
		return "", 0, err
	}
	op, err := a.store.GetOperatorByUsername(ctx, strings.TrimSpace(username))
	if err != nil {
		a.recordFail(lockKey)
		return "", 0, fmt.Errorf("invalid credentials")
	}
	if bcrypt.CompareHashAndPassword([]byte(op.PasswordHash), []byte(password)) != nil {
		a.recordFail(lockKey)
		return "", 0, fmt.Errorf("invalid credentials")
	}
	a.clearFail(lockKey)
	tok := a.issue(op.Username)
	return tok, int64(a.ttl.Seconds()), nil
}

// Logout revokes a bearer token.
func (a *Auth) Logout(token string) {
	if a == nil {
		return
	}
	a.mu.Lock()
	delete(a.toks, token)
	a.mu.Unlock()
}

// Valid reports whether token is an active operator session.
func (a *Auth) Valid(token string) bool {
	if a.Disabled() {
		return true
	}
	if token == "" {
		return false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	s, ok := a.toks[token]
	if !ok {
		return false
	}
	if time.Now().After(s.expiry) {
		delete(a.toks, token)
		return false
	}
	s.expiry = time.Now().Add(a.ttl)
	a.toks[token] = s
	return true
}

func (a *Auth) issue(username string) string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	tok := hex.EncodeToString(b)
	a.mu.Lock()
	a.toks[tok] = session{username: username, expiry: time.Now().Add(a.ttl)}
	a.mu.Unlock()
	return tok
}

func (a *Auth) checkLockout(ip string) error {
	a.failMu.Lock()
	defer a.failMu.Unlock()
	f := a.fails[ip]
	if time.Now().Before(f.lockedUntil) {
		return fmt.Errorf("too many failed logins; try again later")
	}
	return nil
}

func (a *Auth) recordFail(ip string) {
	a.failMu.Lock()
	defer a.failMu.Unlock()
	f := a.fails[ip]
	f.count++
	if f.count >= 8 {
		f.lockedUntil = time.Now().Add(2 * time.Minute)
		f.count = 0
	}
	a.fails[ip] = f
}

func (a *Auth) clearFail(ip string) {
	a.failMu.Lock()
	delete(a.fails, ip)
	a.failMu.Unlock()
}

// TokenFromRequest extracts the operator bearer token.
func TokenFromRequest(r *http.Request) string {
	if r == nil {
		return ""
	}
	h := r.Header.Get(HeaderAuthorization)
	if len(h) >= 7 && strings.EqualFold(h[:7], "bearer ") {
		return strings.TrimSpace(h[7:])
	}
	if t := r.Header.Get(HeaderOperatorToken); t != "" {
		return t
	}
	return ""
}

// ClientIP best-effort peer address for login limiting.
func ClientIP(r *http.Request) string {
	if r == nil {
		return ""
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// Context keys for authenticated username.
type ctxKey int

const usernameKey ctxKey = 1

// WithUsername stores the operator name on context.
func WithUsername(ctx context.Context, username string) context.Context {
	return context.WithValue(ctx, usernameKey, username)
}

// UsernameFromContext returns the authenticated operator if present.
func UsernameFromContext(ctx context.Context) string {
	v, _ := ctx.Value(usernameKey).(string)
	return v
}

// AgentTokenOK compares the presented agent token to the configured secret.
// Empty configured token means open lab (any/no token accepted).
func AgentTokenOK(configured, presented string) bool {
	configured = strings.TrimSpace(configured)
	if configured == "" {
		return true
	}
	presented = strings.TrimSpace(presented)
	if len(configured) != len(presented) {
		// still use ConstantTimeCompare on equal-length pads
		return false
	}
	return subtle.ConstantTimeCompare([]byte(configured), []byte(presented)) == 1
}

// EnsureAgentToken returns configured token or generates and persists one.
func EnsureAgentToken(path, configured string) (token string, generated bool, err error) {
	configured = strings.TrimSpace(configured)
	if configured != "" {
		if len(configured) < MinTokenLen {
			return "", false, fmt.Errorf("LATTICE_AGENT_TOKEN must be at least %d characters", MinTokenLen)
		}
		return configured, false, nil
	}
	return "", false, nil
}

// ErrUnauthorized is returned when a request lacks a valid operator token.
var ErrUnauthorized = errors.New("unauthorized")
