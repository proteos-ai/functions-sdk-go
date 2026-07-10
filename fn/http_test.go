package fn

import (
	"testing"
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
