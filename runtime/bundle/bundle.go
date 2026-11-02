// Package bundle is the entry point `pro module build` generates for a
// function bundle: one wasm serving every hook, action and connector method
// of a module. The generated main.go imports each function's package and
// calls Register once per function; hand-written function code never
// imports this package.
//
// It carries no build tag so the generated tree compiles on the host too
// (go vet, go test ./...).
package bundle

import "go.proteos.ai/functions-sdk-go/internal/dispatch"

// Register runs one function's registration under its slug, so every
// fn.On* / fn.Register* call inside it lands in the slug-keyed registry the
// host dispatches by (hook slug, action slug, or the action-row slug of a
// connector method).
func Register(slug string, register func()) {
	dispatch.WithSlug(slug, register)
}
