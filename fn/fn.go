// Package fn is what hook/action authors import from their main.go.
// It exposes:
//
//   - Context — what every handler receives.
//   - Six generic hook type aliases (OnBeforeCreate[T] etc.).
//   - Action[P,R] / GlobalAction[P,R] aliases.
//   - Register helpers that hook into the //go:wasmexport entry points
//     in runtime/autoexport.
//   - UserError / IsUserError so authors can surface 4xx-shaped errors
//     to callers without ending up classified as infrastructure failures.
//
// Authors don't import dispatch directly — these generics are the only
// supported registration surface.
package fn

import (
	"encoding/json"
	"errors"
	"fmt"
)

// ErrNotFound is the sentinel host.Get* and host.Query helpers return
// when a record / row isn't found. Authors compare with errors.Is.
var ErrNotFound = errors.New("fn: not found")

// userError is the wrapper around author-surfaced errors. The host
// runtime (LUM-50) checks with IsUserError to decide whether the
// envelope returned to the original caller should be classified as
// GUEST_ERROR (author-controlled) vs UNKNOWN/TRAP (infrastructure).
type userError struct{ msg string }

func (e *userError) Error() string { return e.msg }

// UserError wraps a message into an error that the host runtime will
// surface to the calling client as code=GUEST_ERROR.
func UserError(msg string) error { return &userError{msg: msg} }

// UserErrorf is the fmt.Sprintf variant of UserError.
func UserErrorf(format string, args ...any) error {
	return &userError{msg: fmt.Sprintf(format, args...)}
}

// IsUserError reports whether err (or anything in its unwrap chain) was
// produced by UserError / UserErrorf. Used host-side; safe for authors.
func IsUserError(err error) bool {
	var ue *userError
	return errors.As(err, &ue)
}

// JSON marshals v and panics on error. Convenience for tests + examples
// where the value is statically known to marshal. Do not use in handler
// bodies — return json.Marshal's error to the host instead.
func JSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("fn.JSON: %v", err))
	}
	return b
}
