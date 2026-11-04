package extensions

import (
	"fmt"

	httpchannel "github.com/digitizables/lattice/extensions/channels/http"
	"github.com/digitizables/lattice/server/listen"
)

// RegisterBuiltins wires in-process factories for extensions shipped with Lattice.
func RegisterBuiltins(l *Loader) {
	l.RegisterChannel("http", func(deps ChannelDeps) (listen.Channel, error) {
		ch := httpchannel.New(deps.Core, deps.Log)
		if deps.Cleartext {
			return ch, nil
		}
		if deps.CertFile == "" || deps.KeyFile == "" {
			return nil, fmt.Errorf("http channel requires TLS cert/key (or LATTICE_AGENT_CLEARTEXT=1)")
		}
		return ch.WithTLS(deps.CertFile, deps.KeyFile), nil
	})
}
