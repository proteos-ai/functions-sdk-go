package fn

import (
	"encoding/json"
)

// Log is the guest's structured-log surface. Like every other host
// capability (Records, Query, HTTP, Cache, Secrets), it's a
// package-level value backed by `//go:wasmimport fn log_*` stubs.
// Function-service's binding (LUM-51) tags each log line with
// org_id/slug/sha/trace_id server-side from invocation UserData.
//
// Usage:
//
//	host.Log.Info(ctx, "validated", map[string]any{
//	    "invoice_id":  inv.Id,
//	    "customer_id": customer.Id,
//	})
//
// ctx is accepted (and currently unused beyond consistency with the
// rest of host.X) so cancellation / trace context can be threaded
// through later without an API change.
var Log = logAPI{}

type logAPI struct{}

func (logAPI) Debug(_ Context, msg string, fields map[string]any) {
	emitLog(transport.logDebug, msg, fields)
}

func (logAPI) Info(_ Context, msg string, fields map[string]any) {
	emitLog(transport.logInfo, msg, fields)
}

func (logAPI) Warn(_ Context, msg string, fields map[string]any) {
	emitLog(transport.logWarn, msg, fields)
}

func (logAPI) Error(_ Context, msg string, fields map[string]any) {
	emitLog(transport.logError, msg, fields)
}

func emitLog(fn func([]byte) ([]byte, error), msg string, fields map[string]any) {
	if fn == nil {
		return // host build: no transport wired — silent no-op
	}
	payload, err := json.Marshal(struct {
		Msg    string         `json:"msg"`
		Fields map[string]any `json:"fields,omitempty"`
	}{msg, fields})
	if err != nil {
		return
	}
	_, _ = fn(payload)
}
