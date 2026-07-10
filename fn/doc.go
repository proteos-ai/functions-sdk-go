// Host capabilities — the guest-side wasm-import wrappers a handler calls into.
// Part of package fn (see fn.go for the package overview). Authors call these
// from their hook/action handler:
//
//	rec, err := fn.GetRecord[domain.Invoice](ctx, "invoice", id)
//	resp, err := fn.HTTP.PostJSON(ctx, "https://...", body)
//	fn.Log.Info(ctx, "validated", map[string]any{"id": id})
//
// Every method declares a `//go:wasmimport fn X` stub (or wraps Extism PDK's
// built-in HTTP / Log API) and exchanges a JSON envelope with the host:
//
//	request : {"entity":"…","id":"…"}                          (per-fn shape)
//	response: {"ok":true,"data":<payload>}
//	      or  {"ok":false,"err":{"code":"not_found","message":"…"}}
//
// The host-side implementations live in function-service (host functions +
// SSRF denylist).
package fn
