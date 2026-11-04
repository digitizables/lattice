package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/digitizables/lattice/server/modules"
)

func main() {
	dir := flag.String("dir", "data/modules", "teamserver modules directory")
	name := flag.String("name", "", "module name (required)")
	kind := flag.String("kind", modules.KindElevate, "module kind")
	version := flag.String("version", "1.0.0", "module version")
	goos := flag.String("os", runtime.GOOS, "target GOOS")
	arch := flag.String("arch", runtime.GOARCH, "target GOARCH")
	file := flag.String("file", "", "module binary path (required)")
	desc := flag.String("desc", "", "short description")
	argsJSON := flag.String("args", "[]", "default args as JSON string array")
	flag.Parse()

	if strings.TrimSpace(*name) == "" || strings.TrimSpace(*file) == "" {
		fmt.Fprintln(os.Stderr, `usage: publish-module -name my-uac -file .\tool.exe [-version 1.0.0] [-os windows] [-arch amd64] [-desc "..."] [-args "[\"--silent\"]"]`)
		os.Exit(2)
	}
	var defaultArgs []string
	if err := json.Unmarshal([]byte(*argsJSON), &defaultArgs); err != nil {
		fmt.Fprintln(os.Stderr, "invalid -args JSON:", err)
		os.Exit(2)
	}
	data, err := os.ReadFile(*file)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	abs, _ := filepath.Abs(*dir)
	store, err := modules.New(abs)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	mod, err := store.Publish(modules.Module{
		Name:        *name,
		Kind:        *kind,
		Version:     *version,
		OS:          *goos,
		Arch:        *arch,
		Description: *desc,
		DefaultArgs: defaultArgs,
	}, data)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(mod)
}
