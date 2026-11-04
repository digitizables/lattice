package extensions

// Loader will discover and start installable extensions (channels, modules, hooks).
// The HTTP channel is registered directly from teamserver main until this lands.
type Loader struct{}

// New returns an empty loader stub.
func New() *Loader {
	return &Loader{}
}
