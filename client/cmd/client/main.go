package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
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
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	switch os.Args[1] {
	case "sessions":
		res, err := rpc.ListSessions(ctx, &operatorv1.ListSessionsRequest{})
		if err != nil {
			fail(err)
		}
		if len(res.GetSessions()) == 0 {
			fmt.Println("no sessions")
			return
		}
		for _, s := range res.GetSessions() {
			fmt.Printf("%s  %s@%s  %s/%s  last_seen=%s\n",
				s.GetAgentId(), s.GetUsername(), s.GetHostname(), s.GetOs(), s.GetArch(),
				s.GetLastSeen().AsTime().UTC().Format(time.RFC3339),
			)
		}
	case "task":
		if len(os.Args) < 4 {
			fmt.Fprintln(os.Stderr, "usage: client task <agent_id> <name> [args...]")
			os.Exit(2)
		}
		res, err := rpc.SubmitTask(ctx, &operatorv1.SubmitTaskRequest{
			AgentId: os.Args[2],
			Name:    os.Args[3],
			Args:    os.Args[4:],
		})
		if err != nil {
			fail(err)
		}
		fmt.Println(res.GetTaskId())
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
