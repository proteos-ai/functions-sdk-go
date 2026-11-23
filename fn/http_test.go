package fn

import (
	"encoding/json"
	"testing"
	"time"
)

func TestResponse_Header(t *testing.T) {
	r := Response{Headers: map[string]string{"X-Message-Id": "msg-1"}}
	if r.Header("X-Message-Id") != "msg-1" {
		t.Errorf("Header lookup failed: %q", r.Header("X-Message-Id"))
	}
	if r.Header("missing") != "" {
		t.Errorf("missing header should return empty string")
	}
}

func TestResponse_JSON_Decodes(t *testing.T) {
	r := Response{Body: []byte(`{"name":"alice","age":30}`)}
	var into struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}
	if err := r.JSON(&into); err != nil {
		t.Fatalf("JSON: %v", err)
	}
	if into.Name != "alice" || into.Age != 30 {
		t.Errorf("decoded = %+v", into)
	}
}

func TestHTTP_HostBuild_StubFailsLoudly(t *testing.T) {
	// On the host build the wasip1 init() never runs, so httpDo returns
	// the not-wired sentinel. This guards against a regression where the
	// stub silently returns Response{}, masking misuse from non-wasip1
	// callers (the SDK is wasm-only).
	_, err := HTTP.Get(Context{}, "https://example.com", nil)
	if err != errHostStubNotWired {
		t.Errorf("expected errHostStubNotWired, got %v", err)
	}
}

func TestHTTP_WithTimeout_SetsPerCallBudget(t *testing.T) {
	var sent []map[string]any
	transport.httpRequest = func(in []byte) ([]byte, error) {
		var envelope map[string]any
		if err := json.Unmarshal(in, &envelope); err != nil {
			t.Fatalf("decode request envelope: %v", err)
		}
		sent = append(sent, envelope)
		return []byte(`{"ok":true,"data":{"status_code":200}}`), nil
	}
	t.Cleanup(func() { transport.httpRequest = nil })

	if _, err := HTTP.Get(Context{}, "https://example.com", nil); err != nil {
		t.Fatalf("Get: %v", err)
	}
	if _, err := HTTP.PostJSON(Context{}, "https://example.com", map[string]string{"a": "b"}, WithTimeout(45*time.Second)); err != nil {
		t.Fatalf("PostJSON: %v", err)
	}

	if _, ok := sent[0]["timeout_ms"]; ok {
		t.Errorf("call without WithTimeout sent timeout_ms %v; want the host default", sent[0]["timeout_ms"])
	}
	if got := sent[1]["timeout_ms"]; got != float64(45000) {
		t.Errorf("timeout_ms = %v, want 45000", got)
	}

	// A sub-millisecond budget must stay short, not fall back to the default.
	if _, err := HTTP.Get(Context{}, "https://example.com", nil, WithTimeout(500*time.Microsecond)); err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got := sent[2]["timeout_ms"]; got != float64(1) {
		t.Errorf("sub-millisecond timeout_ms = %v, want 1", got)
	}
}
