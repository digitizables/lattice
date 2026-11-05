package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/digitizables/lattice/server/generate"
	"github.com/digitizables/lattice/server/secrets"
)

func main() {
	server := flag.String("server", "https://127.0.0.1:8443", "agent callback URL")
	goos := flag.String("os", "", "target GOOS (default host)")
	arch := flag.String("arch", "", "target GOARCH (default host)")
	sleep := flag.Uint64("sleep", 5, "sleep seconds")
	jitter := flag.Uint("jitter", 0, "jitter percent 0-100")
	insecure := flag.Bool("insecure", true, "bake insecure TLS skip-verify")
	embedToken := flag.Bool("embed-token", true, "embed agent token from env or data/secrets")
	outName := flag.String("o", "", "output filename")
	outDir := flag.String("dir", "data/generated", "output directory")
	root := flag.String("root", ".", "module root (go.mod)")
	flag.Parse()

	token := ""
	if *embedToken {
		var err error
		token, _, err = secrets.EnsureAgentToken("data/secrets", os.Getenv("LATTICE_AGENT_TOKEN"))
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	absRoot, err := filepath.Abs(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	res, err := generate.Build(ctx, generate.Request{
		ServerURL:       *server,
		OS:              *goos,
		Arch:            *arch,
		SleepSeconds:    *sleep,
		JitterPercent:   uint32(*jitter),
		InsecureTLS:     *insecure,
		AgentToken:      token,
		EmbedAgentToken: *embedToken && token != "",
		OutputName:      *outName,
		OutDir:          *outDir,
		ModuleRoot:      absRoot,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("wrote %s\n  os=%s arch=%s size=%d sha256=%s\n", res.Path, res.OS, res.Arch, res.Size, res.SHA256)
}
