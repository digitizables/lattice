// install-extension copies a local channel package into the extensions tree.
// Remote GitHub install is not implemented yet.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/digitizables/lattice/server/extensions"
)

func main() {
	src := flag.String("src", "", "path to a channel package directory containing manifest.yaml")
	dir := flag.String("extensions-dir", envOr("LATTICE_EXTENSIONS_DIR", "extensions"), "extensions root")
	flag.Parse()
	if *src == "" {
		fmt.Fprintln(os.Stderr, "usage: install-extension -src ./path/to/channel")
		os.Exit(2)
	}
	dst, err := extensions.InstallLocalChannel(*dir, *src)
	if err != nil {
		fmt.Fprintf(os.Stderr, "install-extension: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("installed channel package to %s\n", dst)
	fmt.Println("restart the teamserver to load new channel manifests (builtin factories still required for in-process channels)")
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
