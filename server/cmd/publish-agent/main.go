package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/digitizables/lattice/server/updates"
)

func main() {
	dir := flag.String("dir", "data/updates", "teamserver updates directory")
	version := flag.String("version", "", "version label (required)")
	goos := flag.String("os", runtime.GOOS, "target GOOS")
	arch := flag.String("arch", runtime.GOARCH, "target GOARCH")
	file := flag.String("file", "", "agent binary path (required)")
	flag.Parse()

	if strings.TrimSpace(*version) == "" || strings.TrimSpace(*file) == "" {
		fmt.Fprintln(os.Stderr, "usage: publish-agent -version 0.2.1 -file ./bin/agent.exe [-os windows -arch amd64] [-dir data/updates]")
		os.Exit(2)
	}
	data, err := os.ReadFile(*file)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	abs, _ := filepath.Abs(*dir)
	store, err := updates.New(abs)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	art, err := store.Publish(*version, *goos, *arch, data)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(art)
}
