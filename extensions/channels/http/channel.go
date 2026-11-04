package httpchannel

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"google.golang.org/protobuf/proto"

	agentv1 "github.com/digitizables/lattice/gen/lattice/agent/v1"
	"github.com/digitizables/lattice/server/core"
	"github.com/digitizables/lattice/server/listen"
)

// Channel serves agent check-in and task-result over HTTP(S) with Protobuf bodies.
type Channel struct {
	core   *core.Core
	log    *slog.Logger
	server *http.Server
}

// New builds an HTTP channel bound to core.
func New(c *core.Core, log *slog.Logger) *Channel {
	if log == nil {
		log = slog.Default()
	}
	return &Channel{core: c, log: log}
}

// Name identifies the channel to the listener manager.
func (c *Channel) Name() string { return "http" }

// Start listens on addr. Use http:// for lab cleartext or configure TLS later.
func (c *Channel) Start(ctx context.Context, addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/checkin", c.handleCheckIn)
	mux.HandleFunc("POST /v1/result", c.handleResult)

	c.server = &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		c.log.Info("http channel listening", "addr", addr)
		err := c.server.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			c.log.Error("http channel stopped", "err", err)
		}
	}()

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = c.Stop(shutdownCtx)
	}()

	return nil
}

// StartTLS is like Start but with a certificate.
func (c *Channel) StartTLS(ctx context.Context, addr, certFile, keyFile string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/checkin", c.handleCheckIn)
	mux.HandleFunc("POST /v1/result", c.handleResult)

	c.server = &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		TLSConfig:         &tls.Config{MinVersion: tls.VersionTLS12},
	}

	go func() {
		c.log.Info("http channel listening tls", "addr", addr)
		err := c.server.ListenAndServeTLS(certFile, keyFile)
		if err != nil && err != http.ErrServerClosed {
			c.log.Error("http channel stopped", "err", err)
		}
	}()

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = c.Stop(shutdownCtx)
	}()

	return nil
}

// Stop closes the listener.
func (c *Channel) Stop(ctx context.Context) error {
	if c.server == nil {
		return nil
	}
	return c.server.Shutdown(ctx)
}

func (c *Channel) handleCheckIn(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		http.Error(w, "read body", http.StatusBadRequest)
		return
	}
	var in agentv1.AgentCheckIn
	if err := proto.Unmarshal(body, &in); err != nil {
		http.Error(w, "bad protobuf", http.StatusBadRequest)
		return
	}

	out, err := c.core.HandleCheckIn(r.Context(), &in)
	if err != nil {
		c.log.Error("check-in failed", "err", err)
		http.Error(w, "check-in failed", http.StatusInternalServerError)
		return
	}

	raw, err := proto.Marshal(out)
	if err != nil {
		http.Error(w, "marshal", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/protobuf")
	_, _ = w.Write(raw)
}

func (c *Channel) handleResult(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		http.Error(w, "read body", http.StatusBadRequest)
		return
	}
	var in agentv1.AgentTaskResult
	if err := proto.Unmarshal(body, &in); err != nil {
		http.Error(w, "bad protobuf", http.StatusBadRequest)
		return
	}

	out, err := c.core.HandleTaskResult(r.Context(), &in)
	if err != nil {
		c.log.Error("task result failed", "err", err)
		http.Error(w, "result failed", http.StatusInternalServerError)
		return
	}

	raw, err := proto.Marshal(out)
	if err != nil {
		http.Error(w, "marshal", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/protobuf")
	_, _ = w.Write(raw)
}

// Ensure Channel implements listen.Channel.
var _ listen.Channel = (*Channel)(nil)

// ErrNotConfigured is reserved for profile-driven startup failures.
var ErrNotConfigured = fmt.Errorf("http channel not configured")
