package fn

import (
	"errors"
	"fmt"
)

// Sentinel errors returned by host calls. Authors compare with
// errors.Is. ErrNotFound is the same sentinel exposed by fn so
// `errors.Is(err, ErrNotFound)` works regardless of which package
// the caller imports.
var (
	ErrPermissionDenied = errors.New("fn: permission denied")
	ErrBadInput         = errors.New("fn: bad input")
	ErrConflict         = errors.New("fn: conflict")
	ErrInternal         = errors.New("fn: internal")
)

// codeToError maps a wire-envelope error code to the matching sentinel,
// wrapping the host's message via fmt.Errorf("%w: ...") so callers get
// both `errors.Is` matching and the human-readable detail.
func codeToError(code, message string) error {
	var sentinel error
	switch code {
	case "not_found":
		sentinel = ErrNotFound
	case "permission_denied":
		sentinel = ErrPermissionDenied
	case "bad_input":
		sentinel = ErrBadInput
	case "conflict":
		sentinel = ErrConflict
	case "internal":
		sentinel = ErrInternal
	default:
		return fmt.Errorf("fn: %s: %s", code, message)
	}
	if message == "" {
		return sentinel
	}
	return fmt.Errorf("%w: %s", sentinel, message)
}
