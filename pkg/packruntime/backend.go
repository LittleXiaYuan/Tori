package packruntime

import "net/http"

// BackendRoute describes one HTTP surface provided by a backend capability
// pack. The route is mounted by the host Gateway, while enablement is still
// controlled by the installed pack manifest.
type BackendRoute struct {
	Method  string
	Path    string
	Handler http.HandlerFunc
}

// BackendModule is implemented by backend capability packs that can be mounted
// into the host Gateway without hard-coding their individual routes there.
type BackendModule interface {
	PackID() string
	Routes() []BackendRoute
}
