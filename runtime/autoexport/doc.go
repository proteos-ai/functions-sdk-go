// Package autoexport provides the //go:wasmexport entry points the
// function-service runtime calls into:
//
//   - runHook            — decodes a dispatch.HookEnvelope and invokes the
//     hook handler for the envelope's Event (and Hook slug in a bundle).
//   - runAction          — decodes a dispatch.ActionEnvelope and invokes the
//     action handler the envelope's Action slug selects.
//   - runConnectorMethod — decodes a dispatch.ConnectorMethodEnvelope and
//     invokes the connector-method handler.
//   - warm               — a no-op; the host calls it right after
//     instantiation so the guest's _initialize runs there instead of inside
//     the first real call.
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
