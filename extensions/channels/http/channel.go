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
	core     *core.Core
	log      *slog.Logger
	server   *http.Server
	certFile string
	keyFile  string
	useTLS   bool
}

// New builds an HTTP channel bound to core (TLS configured via WithTLS).
func New(c *core.Core, log *slog.Logger) *Channel {
	if log == nil {
		log = slog.Default()
	}
	return &Channel{core: c, log: log}
}

// WithTLS enables ListenAndServeTLS using the given certificate files.
func (c *Channel) WithTLS(certFile, keyFile string) *Channel {
	c.certFile = certFile
	c.keyFile = keyFile
	c.useTLS = true
	return c
}

// Name identifies the channel to the listener manager.
func (c *Channel) Name() string { return "http" }

// Start listens on addr. Uses TLS when WithTLS was called.
func (c *Channel) Start(ctx context.Context, addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/checkin", c.handleCheckIn)
	mux.HandleFunc("POST /v1/result", c.handleResult)
	mux.HandleFunc("POST /v1/task_running", c.handleTaskRunning)
	mux.HandleFunc("POST /v1/task_cancel_poll", c.handleTaskCancelPoll)
	mux.HandleFunc("POST /v1/update/check", c.handleUpdateCheck)
	mux.HandleFunc("POST /v1/update/download", c.handleUpdateDownload)

	c.server = &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	if c.useTLS {
		if c.certFile == "" || c.keyFile == "" {
			return fmt.Errorf("tls enabled but cert/key paths are empty")
		}
		c.server.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12}
		go func() {
			c.log.Info("http channel listening", "addr", addr, "tls", true)
			err := c.server.ListenAndServeTLS(c.certFile, c.keyFile)
			if err != nil && err != http.ErrServerClosed {
				c.log.Error("http channel stopped", "err", err)
			}
		}()
	} else {
		go func() {
			c.log.Info("http channel listening", "addr", addr, "tls", false)
			err := c.server.ListenAndServe()
			if err != nil && err != http.ErrServerClosed {
				c.log.Error("http channel stopped", "err", err)
			}
		}()
	}

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

func (c *Channel) handleTaskRunning(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		http.Error(w, "read body", http.StatusBadRequest)
		return
	}
	var in agentv1.AgentTaskRunning
	if err := proto.Unmarshal(body, &in); err != nil {
		http.Error(w, "bad protobuf", http.StatusBadRequest)
		return
	}
	cancelled, err := c.core.MarkTaskRunning(r.Context(), in.GetAgentId(), in.GetTaskId())
	if err != nil {
		c.log.Error("task running failed", "err", err)
		http.Error(w, "task running failed", http.StatusInternalServerError)
		return
	}
	raw, err := proto.Marshal(&agentv1.AgentTaskRunningResponse{Ok: true, Cancelled: cancelled})
	if err != nil {
		http.Error(w, "marshal", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/protobuf")
	_, _ = w.Write(raw)
}

func (c *Channel) handleUpdateCheck(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		http.Error(w, "read body", http.StatusBadRequest)
		return
	}
	var in agentv1.AgentUpdateCheck
	if err := proto.Unmarshal(body, &in); err != nil {
		http.Error(w, "bad protobuf", http.StatusBadRequest)
		return
	}
	ok, info, err := c.core.CheckUpdate(in.GetOs(), in.GetArch(), in.GetCurrentVersion())
	if err != nil {
		http.Error(w, "update check failed", http.StatusInternalServerError)
		return
	}
	out := &agentv1.AgentUpdateCheckResponse{Available: ok, Update: info}
	raw, err := proto.Marshal(out)
	if err != nil {
		http.Error(w, "marshal", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/protobuf")
	_, _ = w.Write(raw)
}

func (c *Channel) handleUpdateDownload(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		http.Error(w, "read body", http.StatusBadRequest)
		return
	}
	var in agentv1.AgentUpdateDownload
	if err := proto.Unmarshal(body, &in); err != nil {
		http.Error(w, "bad protobuf", http.StatusBadRequest)
		return
	}
	info, data, err := c.core.DownloadUpdate(in.GetOs(), in.GetArch(), in.GetVersion())
	if err != nil {
		c.log.Error("update download failed", "err", err)
		http.Error(w, "update download failed", http.StatusNotFound)
		return
	}
	// Cap absurd uploads in memory responses (lab binaries should be small).
	if len(data) > 64<<20 {
		http.Error(w, "update too large", http.StatusBadRequest)
		return
	}
	out := &agentv1.AgentUpdateDownloadResponse{Update: info, Data: data}
	raw, err := proto.Marshal(out)
	if err != nil {
		http.Error(w, "marshal", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/protobuf")
	_, _ = w.Write(raw)
}

func (c *Channel) handleTaskCancelPoll(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		http.Error(w, "read body", http.StatusBadRequest)
		return
	}
	var in agentv1.AgentTaskCancelPoll
	if err := proto.Unmarshal(body, &in); err != nil {
		http.Error(w, "bad protobuf", http.StatusBadRequest)
		return
	}
	cancelled, err := c.core.IsTaskCancelled(r.Context(), in.GetAgentId(), in.GetTaskId())
	if err != nil {
		c.log.Error("cancel poll failed", "err", err)
		http.Error(w, "cancel poll failed", http.StatusInternalServerError)
		return
	}
	raw, err := proto.Marshal(&agentv1.AgentTaskCancelPollResponse{Cancelled: cancelled})
	if err != nil {
		http.Error(w, "marshal", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/protobuf")
	_, _ = w.Write(raw)
}

var _ listen.Channel = (*Channel)(nil)
