// Package functionssdkgo is the author-facing Go SDK for the Proteos
// function-service: the package every hook/action's main.go imports to
// declare its handler and get auto-wired into the wasm guest's runHook /
// runAction Extism entry points.
//
// Sub-packages:
//
//   - fn — types and Register* helpers authors call from func init().
//   - runtime/autoexport — //go:wasmexport entry points (wasip1 only).
//     Imported for side-effects via `_ "…/runtime/autoexport"`.
//   - host — wrappers around //go:wasmimport stubs (records, query, http,
//     cache, secrets, log). Lands in LUM-48.
package functionssdkgo
