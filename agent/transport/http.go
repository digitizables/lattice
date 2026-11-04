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
}

// Options configures the agent HTTP(S) transport.
type Options struct {
	// InsecureSkipVerify skips TLS certificate verification (lab self-signed certs).
	InsecureSkipVerify bool
}

// NewHTTP builds a transport pointed at baseURL, e.g. https://127.0.0.1:8443.
func NewHTTP(baseURL string, opts Options) *HTTP {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if opts.InsecureSkipVerify {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // lab default for self-signed agent certs
	}
	return &HTTP{
		baseURL: baseURL,
		client: &http.Client{
			Timeout:   30 * time.Second,
			Transport: transport,
		},
	}
}

// CheckIn posts an AgentCheckIn and returns the server response.
func (h *HTTP) CheckIn(ctx context.Context, in *agentv1.AgentCheckIn) (*agentv1.AgentCheckInResponse, error) {
	var out agentv1.AgentCheckInResponse
	if err := h.post(ctx, "/v1/checkin", in, &out); err != nil {
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
