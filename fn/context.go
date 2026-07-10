package fn

import (
	"context"
	"strings"
)

// Source identifies the invocation origin. Type ∈ {"person", "agent", "api",
// "system"}; Id is the user id (the "system" sentinel for bootstrap writes).
type Source struct {
	Id   string `json:"id"`
	Type string `json:"type"`
}

// Context is the first argument every fn handler receives. It bundles
// the request-scoped context.Context with the OrgId + Source claims.
// Host-provided capabilities (records, query, http, cache, secrets, log)
// live in the host package as package-level values — not on Context —
// to keep Context focused on per-call identity.
//
// Headers carries the inbound HTTP request headers (flattened to single
// values) for HTTP-originated action dispatch — e.g. a public webhook
// action verifying a token header. It is nil for hook dispatch and for
// non-HTTP invocations. Header keys are canonicalized (e.g.
// "X-Goog-Channel-Token"); read them case-insensitively via Context.Header.
type Context struct {
	Ctx     context.Context
	OrgId   string
	Source  Source
	Headers map[string]string
}

// Header returns the inbound request header named `name`, matched
// case-insensitively, or "" if absent. Convenience over indexing Headers
// directly (whose keys are HTTP-canonicalized).
func (c Context) Header(name string) string {
	if c.Headers == nil {
		return ""
	}
	if v, ok := c.Headers[name]; ok {
		return v
	}
	for key, value := range c.Headers {
		if strings.EqualFold(key, name) {
			return value
		}
	}
	return ""
}
