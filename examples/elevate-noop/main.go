// Command elevate-noop is a lab-only elevate module that performs no privilege
// change. It proves the Lattice BYO elevation pipeline end-to-end.
package main

import (
	"fmt"
	"os"
	"os/user"
	"runtime"
	"strings"
)

func main() {
	u := "unknown"
	if cu, err := user.Current(); err == nil {
		u = cu.Username
	}
	host, _ := os.Hostname()
	fmt.Printf("lattice elevate-noop ok\n")
	fmt.Printf("user=%s host=%s goos=%s goarch=%s\n", u, host, runtime.GOOS, runtime.GOARCH)
	if len(os.Args) > 1 {
		fmt.Printf("args=%s\n", strings.Join(os.Args[1:], " "))
	}
	os.Exit(0)
}
