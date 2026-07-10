package fn

import "errors"

// transportFns is the package-internal indirection between the host-portable
// wrappers (records.go, query.go, cache.go, secrets.go) and the wasm-import
// stubs declared in transport_wasip1.go. Tests override individual fields
// to exercise the JSON envelope contract without a wasm runtime.
type transportFns struct {
	recordsGet                 func([]byte) ([]byte, error)
	recordsCreate              func([]byte) ([]byte, error)
	recordsUpdate              func([]byte) ([]byte, error)
	recordsDelete              func([]byte) ([]byte, error)
	recordsList                func([]byte) ([]byte, error)
	queryExecute               func([]byte) ([]byte, error)
	cacheGet                   func([]byte) ([]byte, error)
	cacheSet                   func([]byte) ([]byte, error)
	cacheDelete                func([]byte) ([]byte, error)
	secretsRead                func([]byte) ([]byte, error)
	storageGenerateDownloadUrl func([]byte) ([]byte, error)
	logDebug                   func([]byte) ([]byte, error)
	logInfo                    func([]byte) ([]byte, error)
	logWarn                    func([]byte) ([]byte, error)
	logError                   func([]byte) ([]byte, error)
	httpRequest                func([]byte) ([]byte, error)
	connectionsGetToken        func([]byte) ([]byte, error)
	connectionsInvokeMethod    func([]byte) ([]byte, error)
}

var transport transportFns

// errHostStubNotWired is returned by the host-side stub transport so a
// call from a non-wasip1 binary fails loudly instead of silently no-oping.
var errHostStubNotWired = errors.New("host: wasm-import transport not wired (compile under GOOS=wasip1)")

// call invokes one transport fn and returns the response envelope bytes,
// or any transport-level error. The caller decodes the envelope via
// decode() (see envelope.go).
func call(fn func([]byte) ([]byte, error), in []byte) ([]byte, error) {
	if fn == nil {
		return nil, errHostStubNotWired
	}
	return fn(in)
}
