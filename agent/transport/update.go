package transport

import (
	"context"
	"net/http"
	"time"

	agentv1 "github.com/digitizables/lattice/gen/lattice/agent/v1"
)

// CheckUpdate asks the teamserver whether a newer agent build exists.
func (h *HTTP) CheckUpdate(ctx context.Context, agentID, goos, arch, current string) (*agentv1.AgentUpdateCheckResponse, error) {
	var out agentv1.AgentUpdateCheckResponse
	err := h.post(ctx, "/v1/update/check", &agentv1.AgentUpdateCheck{
		AgentId:        agentID,
		Os:             goos,
		Arch:           arch,
		CurrentVersion: current,
	}, &out)
	return &out, err
}

// DownloadUpdate pulls a published agent binary.
func (h *HTTP) DownloadUpdate(ctx context.Context, agentID, goos, arch, version string) (*agentv1.AgentUpdateDownloadResponse, error) {
	// Use a long-timeout client; update payloads are larger than normal RPCs.
	prev := h.client
	h.client = &http.Client{
		Timeout:   5 * time.Minute,
		Transport: prev.Transport,
	}
	defer func() { h.client = prev }()

	var out agentv1.AgentUpdateDownloadResponse
	err := h.post(ctx, "/v1/update/download", &agentv1.AgentUpdateDownload{
		AgentId: agentID,
		Os:      goos,
		Arch:    arch,
		Version: version,
	}, &out)
	return &out, err
}
