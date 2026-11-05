// install-extension copies a channel package into the extensions tree from a
// local directory, a local/remote archive, or a GitHub repository zipball.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/digitizables/lattice/server/extensions"
)

func main() {
	src := flag.String("src", "", "local channel package directory containing manifest.yaml")
	url := flag.String("url", "", "https archive URL, or local .zip/.tar.gz path")
	github := flag.String("github", "", "owner/repo or owner/repo@ref (downloads GitHub zipball)")
	ref := flag.String("ref", "", "git ref for -github when @ref is omitted (default main)")
	subdir := flag.String("subdir", "", "path inside the archive to the channel package")
	dir := flag.String("extensions-dir", envOr("LATTICE_EXTENSIONS_DIR", "extensions"), "extensions root")
	flag.Parse()

	n := 0
	if *src != "" {
		n++
	}
	if *url != "" {
		n++
	}
	if *github != "" {
		n++
	}
	if n != 1 {
		fmt.Fprintln(os.Stderr, "usage:")
		fmt.Fprintln(os.Stderr, "  install-extension -src ./path/to/channel")
		fmt.Fprintln(os.Stderr, "  install-extension -url https://example.com/channel.zip [-subdir path]")
		fmt.Fprintln(os.Stderr, "  install-extension -github owner/repo[@ref] [-subdir path] [-ref main]")
		os.Exit(2)
	}

	var (
		dst string
		err error
	)
	switch {
	case *src != "":
		dst, err = extensions.InstallLocalChannel(*dir, *src)
	default:
		dst, err = extensions.InstallRemoteChannel(*dir, extensions.RemoteOptions{
			URL:    *url,
			GitHub: *github,
			Ref:    *ref,
			Subdir: *subdir,
		})
	}
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
