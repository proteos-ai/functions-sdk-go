package bundle_test

import (
	"encoding/json"
	"testing"

	"go.proteos.ai/functions-sdk-go/fn"
	"go.proteos.ai/functions-sdk-go/internal/dispatch"
	"go.proteos.ai/functions-sdk-go/runtime/bundle"
)

type record struct {
	Name string `json:"name"`
}

// The public surface a generated bundle main uses: typed fn.On* registrations
// made inside bundle.Register are keyed by the slug the host dispatches with.
func TestRegister_KeysTypedRegistrations(t *testing.T) {
	dispatch.ResetForTest()
	bundle.Register("stamp", func() {
		fn.OnBeforeCreate[record](func(_ fn.Context, r record) (record, error) {
			r.Name = "stamped:" + r.Name
			return r, nil
		})
	})
	bundle.Register("upper", func() {
		fn.OnBeforeCreate[record](func(_ fn.Context, r record) (record, error) {
			r.Name = "UPPER"
			return r, nil
		})
	})

	out, err := dispatch.RunHook([]byte(`{"event":"before_create","entity":"x","hook":"stamp","record":{"name":"a"},"org_id":"o","source":{"id":"u","type":"person"}}`))
	if err != nil {
		t.Fatalf("RunHook: %v", err)
	}
	var got record
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Name != "stamped:a" {
		t.Errorf("hook %q ran the wrong handler: %+v", "stamp", got)
	}
}
