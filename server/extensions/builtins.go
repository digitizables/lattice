package extensions

import (
	httpchannel "github.com/digitizables/lattice/extensions/channels/http"
	"github.com/digitizables/lattice/server/listen"
)

// RegisterBuiltins wires in-process factories for extensions shipped with Lattice.
func RegisterBuiltins(l *Loader) {
	l.RegisterChannel("http", func(deps ChannelDeps) (listen.Channel, error) {
		return httpchannel.New(deps.Core, deps.Log), nil
	})
}
