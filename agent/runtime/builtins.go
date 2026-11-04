package runtime

import "os"

func runBuiltinHostname() (int32, string, string) {
	h, err := os.Hostname()
	if err != nil {
		return 1, "", err.Error()
	}
	return 0, h, ""
}
