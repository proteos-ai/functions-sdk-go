package fn

import (
	"encoding/json"
	"fmt"
)

// envelope is the wasm-import wire protocol — every host call's response
// bytes decode into this shape. See doc.go for the rationale.
type envelope struct {
	Ok   bool            `json:"ok"`
	Data json.RawMessage `json:"data,omitempty"`
	Err  *envelopeErr    `json:"err,omitempty"`
}

type envelopeErr struct {
	Code    string          `json:"code"`
	Message string          `json:"message"`
	Details json.RawMessage `json:"details,omitempty"`
}

// decode returns the data payload bytes if ok=true, or a typed sentinel
// error (wrapped with the host message) if ok=false. Bytes that don't
// parse as the envelope return a generic decode error.
func decode(raw []byte) (json.RawMessage, error) {
	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("host: decode envelope: %w", err)
	}
	if env.Ok {
		return env.Data, nil
	}
	if env.Err == nil {
		return nil, fmt.Errorf("host: ok=false but no err payload")
	}
	return nil, codeToError(env.Err.Code, env.Err.Message)
}

// callDecode is the common pattern: send `req` over the transport, decode
// the response envelope, return either the data bytes or the typed error.
func callDecode(fn func([]byte) ([]byte, error), req []byte) (json.RawMessage, error) {
	raw, err := call(fn, req)
	if err != nil {
		return nil, err
	}
	return decode(raw)
}
