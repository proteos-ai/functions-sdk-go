package fn

import (
	"encoding/json"
)

// HTTP is the guest's outbound HTTP surface. Backed by the
// `//go:wasmimport fn http_request` host fn (LUM-51), which
// function-service runs through its SSRF denylist + pin-IP transport
// before dialing. Same wire-envelope shape as every other fn host
// call — `{ok, data, err}` with the per-fn payload below.
//
// Originally (LUM-47) backed by Extism's built-in `extism:host/env
// http_request`, but that uses an allow-list semantics — we pivoted to
// a fn-namespace host fn so the host-side denylist is the source
// of truth.
var HTTP = httpAPI{}

type httpAPI struct{}

// Response carries the result of an HTTP host call. Body is the raw
// response bytes; JSON / Header are convenience accessors.
type Response struct {
	StatusCode int
	Headers    map[string]string
	Body       []byte
}

// Header returns the named response header. Empty if not set.
// Lookup is case-insensitive — the host normalizes to canonical form.
func (r Response) Header(name string) string { return r.Headers[name] }

// JSON unmarshals the response body into the caller's target.
func (r Response) JSON(into any) error { return json.Unmarshal(r.Body, into) }

// httpRequestEnvelope is the JSON shape the fn.http_request host fn
// expects. Body is base64-encoded inside the JSON because random bytes
// don't survive UTF-8 string encoding round-trip.
type httpRequestEnvelope struct {
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    []byte            `json:"body,omitempty"`
}

// httpResponseEnvelope is the JSON shape the host fn returns inside the
// {ok, data, err} envelope's `data` field.
type httpResponseEnvelope struct {
	StatusCode int               `json:"status_code"`
	Headers    map[string]string `json:"headers,omitempty"`
	Body       []byte            `json:"body,omitempty"`
}

// Get issues GET <url> with the given headers (pass nil for none) — e.g. an
// Authorization bearer token. Symmetric with Post/Put/Patch/Delete.
func (httpAPI) Get(ctx Context, url string, headers map[string]string) (Response, error) {
	return httpDo(ctx, "GET", url, nil, headers)
}

// Post issues POST <url> with the given body and headers.
func (httpAPI) Post(ctx Context, url string, body []byte, headers map[string]string) (Response, error) {
	return httpDo(ctx, "POST", url, body, headers)
}

// PostJSON marshals body to JSON, sets Content-Type, and POSTs.
func (httpAPI) PostJSON(ctx Context, url string, body any) (Response, error) {
	marshaled, err := json.Marshal(body)
	if err != nil {
		return Response{}, err
	}
	return httpDo(ctx, "POST", url, marshaled, map[string]string{"Content-Type": "application/json"})
}

// Put issues PUT <url>.
func (httpAPI) Put(ctx Context, url string, body []byte, headers map[string]string) (Response, error) {
	return httpDo(ctx, "PUT", url, body, headers)
}

// Patch issues PATCH <url>.
func (httpAPI) Patch(ctx Context, url string, body []byte, headers map[string]string) (Response, error) {
	return httpDo(ctx, "PATCH", url, body, headers)
}

// Delete issues DELETE <url>.
func (httpAPI) Delete(ctx Context, url string, headers map[string]string) (Response, error) {
	return httpDo(ctx, "DELETE", url, nil, headers)
}

// httpDo serializes the request to JSON, routes it through
// `transport.httpRequest` (the fn.http_request wasm import), and
// decodes the response envelope back into the caller's Response shape.
//
// Lives in this file (not _wasip1.go) because the transport indirection
// means the same code path works on both wasip1 and the host-side stub
// transport used by unit tests.
func httpDo(_ Context, method, url string, body []byte, headers map[string]string) (Response, error) {
	reqBytes, err := json.Marshal(httpRequestEnvelope{
		Method:  method,
		URL:     url,
		Headers: headers,
		Body:    body,
	})
	if err != nil {
		return Response{}, err
	}
	dataBytes, err := callDecode(transport.httpRequest, reqBytes)
	if err != nil {
		return Response{}, err
	}
	var resp httpResponseEnvelope
	if err := json.Unmarshal(dataBytes, &resp); err != nil {
		return Response{}, err
	}
	return Response{
		StatusCode: resp.StatusCode,
		Headers:    resp.Headers,
		Body:       resp.Body,
	}, nil
}
