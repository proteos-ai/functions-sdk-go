// Package autoexport provides the two //go:wasmexport entry points the
// function-service runtime calls into:
//
//   - runHook   — decodes a dispatch.HookEnvelope and invokes the
//     registered hook handler for the envelope's Event.
//   - runAction — decodes a dispatch.ActionEnvelope and invokes the
//     registered action handler.
//
// Authors blank-import this package once in their main.go so the export
// symbols end up in the compiled .wasm:
//
//	import _ "go.proteos.ai/functions-sdk-go/runtime/autoexport"
//
// The actual implementation is gated behind //go:build wasip1 — on the
// host the package is empty, so `go test` / `go build ./...` work
// without needing the extism/go-pdk dependency to resolve.
package autoexport
