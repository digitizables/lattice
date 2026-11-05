//go:build e2e

package e2e_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"testing"
	"time"
)

// Smoke exercises a running Compose (or host) stack:
//
//	LATTICE_E2E=1 go test -tags e2e ./test/e2e -count=1
//
// Requires teamserver on LATTICE_API_URL (default http://127.0.0.1:8080) and at
// least one agent checked in (e.g. docker-compose.lab.yml).
func TestSmokeCheckInAndWhoami(t *testing.T) {
	if os.Getenv("LATTICE_E2E") == "" {
		t.Skip("set LATTICE_E2E=1 to run against a live stack")
	}
	base := envOr("LATTICE_API_URL", "http://127.0.0.1:8080")
	client := &http.Client{Timeout: 30 * time.Second}
	token := mustLogin(t, client, base)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	var agentID string
	deadline := time.Now().Add(45 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		default:
		}
		sessions, err := listSessions(client, base, token)
		if err != nil {
			t.Logf("list sessions: %v", err)
			time.Sleep(2 * time.Second)
			continue
		}
		if len(sessions) > 0 {
			agentID = sessions[0].AgentID
			break
		}
		time.Sleep(2 * time.Second)
	}
	if agentID == "" {
		t.Fatal("no agent sessions; start packager/docker/docker-compose.lab.yml")
	}
	t.Logf("using agent %s", agentID)

	taskID, err := submitTask(client, base, token, agentID, "whoami", nil)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	t.Logf("task %s", taskID)

	var stdout string
	for i := 0; i < 60; i++ {
		task, err := getTask(client, base, token, taskID)
		if err != nil {
			t.Fatalf("get task: %v", err)
		}
		switch task.Status {
		case "completed":
			stdout = task.Stdout
			if task.ExitCode != 0 {
				t.Fatalf("whoami exit=%d stderr=%q", task.ExitCode, task.Stderr)
			}
			if stdout == "" {
				t.Fatal("empty stdout")
			}
			t.Logf("whoami ok: %q", truncate(stdout, 80))
			return
		case "failed", "cancelled":
			t.Fatalf("task %s", task.Status)
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatal("timeout waiting for whoami")
}

func TestSmokeListListeners(t *testing.T) {
	if os.Getenv("LATTICE_E2E") == "" {
		t.Skip("set LATTICE_E2E=1 to run against a live stack")
	}
	base := envOr("LATTICE_API_URL", "http://127.0.0.1:8080")
	client := &http.Client{Timeout: 15 * time.Second}
	token := mustLogin(t, client, base)
	listeners, err := listListeners(client, base, token)
	if err != nil {
		t.Fatal(err)
	}
	if len(listeners) == 0 {
		t.Fatal("expected at least one listener")
	}
	found := false
	for _, l := range listeners {
		t.Logf("listener %s bind=%s tls=%v status=%s", l.Name, l.Bind, l.TLS, l.Status)
		if l.Name == "http" && l.Status == "running" {
			found = true
		}
	}
	if !found {
		t.Fatal("http listener not running")
	}
}

type sessionDTO struct {
	AgentID string `json:"agentId"`
}

type taskDTO struct {
	TaskID   string `json:"taskId"`
	Status   string `json:"status"`
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int32  `json:"exitCode"`
}

type listenerDTO struct {
	Name   string `json:"name"`
	Bind   string `json:"bind"`
	TLS    bool   `json:"tls"`
	Status string `json:"status"`
}

func mustLogin(t *testing.T, client *http.Client, base string) string {
	t.Helper()
	user := envOr("LATTICE_BOOTSTRAP_OPERATOR", "admin")
	pass := envOr("LATTICE_BOOTSTRAP_PASSWORD", "lattice-change-me")
	var out struct {
		AccessToken string `json:"accessToken"`
	}
	err := postJSON(client, base+"/lattice.operator.v1.OperatorService/Login", map[string]any{
		"username": user,
		"password": pass,
	}, "", &out)
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if out.AccessToken == "" {
		t.Fatal("empty access token")
	}
	return out.AccessToken
}

func listSessions(client *http.Client, base, token string) ([]sessionDTO, error) {
	var out struct {
		Sessions []sessionDTO `json:"sessions"`
	}
	if err := postJSON(client, base+"/lattice.operator.v1.OperatorService/ListSessions", map[string]any{"pageSize": 20}, token, &out); err != nil {
		return nil, err
	}
	return out.Sessions, nil
}

func listListeners(client *http.Client, base, token string) ([]listenerDTO, error) {
	var out struct {
		Listeners []listenerDTO `json:"listeners"`
	}
	if err := postJSON(client, base+"/lattice.operator.v1.OperatorService/ListListeners", map[string]any{}, token, &out); err != nil {
		return nil, err
	}
	return out.Listeners, nil
}

func submitTask(client *http.Client, base, token, agentID, name string, args []string) (string, error) {
	if args == nil {
		args = []string{}
	}
	var out struct {
		TaskID string `json:"taskId"`
	}
	err := postJSON(client, base+"/lattice.operator.v1.OperatorService/SubmitTask", map[string]any{
		"agentId": agentID,
		"name":    name,
		"args":    args,
	}, token, &out)
	return out.TaskID, err
}

func getTask(client *http.Client, base, token, taskID string) (taskDTO, error) {
	var out struct {
		Task taskDTO `json:"task"`
	}
	err := postJSON(client, base+"/lattice.operator.v1.OperatorService/GetTask", map[string]any{"taskId": taskID}, token, &out)
	return out.Task, err
}

func postJSON(client *http.Client, url string, body any, token string, out any) error {
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Connect-Protocol-Version", "1")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(res.Body)
		return errStatus(res.StatusCode, buf.String())
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(res.Body).Decode(out)
}

type statusError struct {
	code int
	body string
}

func (e statusError) Error() string { return e.body }

func errStatus(code int, body string) error {
	return statusError{code: code, body: body}
}

func envOr(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func truncate(s string, n int) string {
	s = string(bytes.TrimSpace([]byte(s)))
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
