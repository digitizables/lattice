//go:build unix

package runtime

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func runBuiltinPS() (int32, string, string) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return 1, "", "process list: " + err.Error()
	}

	var b strings.Builder
	b.WriteString("PID\tPPID\tNAME\n")
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(e.Name())
		if err != nil || pid <= 0 {
			continue
		}
		comm, _ := os.ReadFile(filepath.Join("/proc", e.Name(), "comm"))
		name := strings.TrimSpace(string(comm))
		if name == "" {
			name = "?"
		}
		ppid := 0
		if st, err := os.ReadFile(filepath.Join("/proc", e.Name(), "stat")); err == nil {
			// /proc/pid/stat: pid (comm) state ppid ... — comm may contain spaces.
			s := string(st)
			if i := strings.LastIndex(s, ")"); i >= 0 && i+1 < len(s) {
				rest := strings.Fields(s[i+1:])
				// rest[0]=state, rest[1]=ppid
				if len(rest) >= 2 {
					if p, err := strconv.Atoi(rest[1]); err == nil {
						ppid = p
					}
				}
			}
		}
		fmt.Fprintf(&b, "%d\t%d\t%s\n", pid, ppid, name)
	}
	return 0, b.String(), ""
}
