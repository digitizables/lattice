package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"

	"github.com/digitizables/lattice/client/console"
	operatorv1 "github.com/digitizables/lattice/gen/lattice/operator/v1"
	"github.com/digitizables/lattice/gen/lattice/operator/v1/operatorv1connect"
)

func main() {
	if len(os.Args) < 2 {
		console.Usage()
		os.Exit(2)
	}

	baseURL := envOr("LATTICE_API_URL", "http://127.0.0.1:8080")
	rpc := operatorv1connect.NewOperatorServiceClient(
		connect.NewClient(connecthttp.NewTransport(http.DefaultClient, baseURL)),
	)

	switch os.Args[1] {
	case "sessions":
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		query := ""
		if len(os.Args) > 2 {
			query = strings.Join(os.Args[2:], " ")
		}
		var token string
		var shown int
		for {
			res, err := rpc.ListSessions(ctx, &operatorv1.ListSessionsRequest{
				PageSize:  100,
				PageToken: token,
				Query:     query,
			})
			if err != nil {
				fail(err)
			}
			if shown == 0 {
				fmt.Printf("total=%d\n", res.GetTotalCount())
			}
			if len(res.GetSessions()) == 0 && shown == 0 {
				fmt.Println("no sessions")
				return
			}
			for _, s := range res.GetSessions() {
				fmt.Printf("%s  %s  %s/%s  last_seen=%s\n",
					s.GetAgentId(), userAtHost(s.GetUsername(), s.GetHostname()), s.GetOs(), s.GetArch(),
					s.GetLastSeen().AsTime().UTC().Format(time.RFC3339),
				)
				shown++
			}
			token = res.GetNextPageToken()
			if token == "" {
				return
			}
		}
	case "task":
		if len(os.Args) < 4 {
			fmt.Fprintln(os.Stderr, "usage: client task <agent_id> <name> [args...]")
			os.Exit(2)
		}
		agentID := os.Args[2]
		name := os.Args[3]
		args := os.Args[4:]

		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		stream, err := rpc.StreamEvents(ctx, &operatorv1.StreamEventsRequest{AgentId: agentID})
		if err != nil {
			fail(err)
		}
		defer stream.Close()

		res, err := rpc.SubmitTask(ctx, &operatorv1.SubmitTaskRequest{
			AgentId: agentID,
			Name:    name,
			Args:    args,
		})
		if err != nil {
			fail(err)
		}
		taskID := res.GetTaskId()
		fmt.Println(taskID)

		for {
			ev, err := stream.Receive()
			if err == io.EOF {
				fail(fmt.Errorf("stream ended before task %s completed", taskID))
			}
			if err != nil {
				fail(err)
			}
			tu := ev.GetTaskUpdated()
			if tu == nil || tu.GetTaskId() != taskID {
				continue
			}
			fmt.Printf("status=%s\n", tu.GetStatus())
			if tu.GetStatus() != "completed" && tu.GetStatus() != "failed" {
				continue
			}
			if tu.GetStdout() != "" {
				fmt.Print(tu.GetStdout())
				if tu.GetStdout()[len(tu.GetStdout())-1] != '\n' {
					fmt.Println()
				}
			}
			if tu.GetStderr() != "" {
				fmt.Fprint(os.Stderr, tu.GetStderr())
				if tu.GetStderr()[len(tu.GetStderr())-1] != '\n' {
					fmt.Fprintln(os.Stderr)
				}
			}
			if tu.GetExitCode() != 0 {
				os.Exit(int(tu.GetExitCode()))
			}
			return
		}
	default:
		console.Usage()
		os.Exit(2)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func userAtHost(username, hostname string) string {
	user := username
	if idx := strings.LastIndex(username, `\`); idx >= 0 {
		domain := username[:idx]
		account := username[idx+1:]
		if hostname != "" && strings.EqualFold(domain, hostname) && account != "" {
			user = account
		}
	}
	if hostname == "" {
		return user
	}
	if user == "" {
		return hostname
	}
	return user + "@" + hostname
}
