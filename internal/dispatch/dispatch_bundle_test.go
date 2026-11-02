package dispatch_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"go.proteos.ai/functions-sdk-go/internal/dispatch"
)

// A bundle registers the same event for two functions; the host's hook slug
// picks the right one, and a missing or unknown slug fails by name instead
// of running a sibling.
func TestRunHook_BundleRoutesByHookSlug(t *testing.T) {
	dispatch.ResetForTest()
	var ran []string
	register := func(slug string) {
		dispatch.WithSlug(slug, func() {
			dispatch.RegisterAfterUpdate(func(_ dispatch.Context, _, _ json.RawMessage) ([]byte, error) {
				ran = append(ran, slug)
				return nil, nil
			})
		})
	}
	register("sync-task")
	register("sync-action-item")

	envelope := func(hook string) []byte {
		return []byte(`{"event":"after_update","entity":"x","hook":"` + hook + `","record":{},"previous_record":{},"org_id":"org-1","source":{"id":"u","type":"person"}}`)
	}
	for _, slug := range []string{"sync-action-item", "sync-task"} {
		if _, err := dispatch.RunHook(envelope(slug)); err != nil {
			t.Fatalf("RunHook(%s): %v", slug, err)
		}
	}
	if got := strings.Join(ran, ","); got != "sync-action-item,sync-task" {
		t.Errorf("handlers ran = %q, want each slug's own handler in order", got)
	}

	for _, hook := range []string{"", "unknown"} {
		_, err := dispatch.RunHook(envelope(hook))
		if !errors.Is(err, dispatch.ErrNoHandler) {
			t.Errorf("RunHook(hook=%q) err = %v, want ErrNoHandler — a bundle must never fall back to a sibling", hook, err)
		}
		if hook != "" && !strings.Contains(err.Error(), `"unknown"`) {
			t.Errorf("error %q should name the unmatched hook slug", err)
		}
	}
}

// A standalone guest (unkeyed registration) keeps working whether or not the
// host sends a hook slug.
func TestRunHook_StandaloneIgnoresHookSlug(t *testing.T) {
	dispatch.ResetForTest()
	calls := 0
	dispatch.RegisterBeforeCreate(func(_ dispatch.Context, r json.RawMessage) ([]byte, error) {
		calls++
		return r, nil
	})
	for _, envelope := range []string{
		`{"event":"before_create","entity":"x","record":{"v":1},"org_id":"org-1","source":{"id":"u","type":"person"}}`,
		`{"event":"before_create","entity":"x","hook":"whatever","record":{"v":1},"org_id":"org-1","source":{"id":"u","type":"person"}}`,
	} {
		if _, err := dispatch.RunHook([]byte(envelope)); err != nil {
			t.Fatalf("RunHook: %v", err)
		}
	}
	if calls != 2 {
		t.Errorf("standalone handler ran %d times, want 2", calls)
	}
}

func TestRunAction_BundleRoutesByActionSlug(t *testing.T) {
	dispatch.ResetForTest()
	for _, slug := range []string{"close-ticket", "reopen-ticket"} {
		slug := slug
		dispatch.WithSlug(slug, func() {
			dispatch.RegisterAction(func(_ dispatch.Context, _ dispatch.ActionTarget, _ json.RawMessage) ([]byte, error) {
				return []byte(`"` + slug + `"`), nil
			})
		})
	}
	out, err := dispatch.RunAction([]byte(`{"action":"reopen-ticket","parameters":{},"org_id":"org-1","source":{"id":"u","type":"person"}}`))
	if err != nil {
		t.Fatalf("RunAction: %v", err)
	}
	if string(out) != `"reopen-ticket"` {
		t.Errorf("RunAction routed to %s, want reopen-ticket", out)
	}
	if _, err := dispatch.RunAction([]byte(`{"action":"missing","parameters":{},"org_id":"org-1","source":{"id":"u","type":"person"}}`)); !errors.Is(err, dispatch.ErrNoHandler) {
		t.Errorf("unknown action err = %v, want ErrNoHandler", err)
	}
}

// Standalone action: the slug the host sends does not match any keyed entry
// (there are none), so the unkeyed handler serves it — exactly today's
// behaviour for every existing binary.
func TestRunAction_StandaloneServesAnySlug(t *testing.T) {
	dispatch.ResetForTest()
	dispatch.RegisterAction(func(_ dispatch.Context, _ dispatch.ActionTarget, _ json.RawMessage) ([]byte, error) {
		return []byte(`"ok"`), nil
	})
	out, err := dispatch.RunAction([]byte(`{"action":"anything","parameters":{},"org_id":"org-1","source":{"id":"u","type":"person"}}`))
	if err != nil || string(out) != `"ok"` {
		t.Fatalf("RunAction = %s, %v", out, err)
	}
}

func TestRunConnectorMethod_BundleRoutesByActionSlug(t *testing.T) {
	dispatch.ResetForTest()
	// Two connectors in one module may both have a method called "get".
	for _, connector := range []string{"crm", "erp"} {
		connector := connector
		dispatch.WithSlug(connector+"-get", func() {
			dispatch.RegisterConnectorMethod("get", func(_ dispatch.Context, _ dispatch.ConnectionInfo, _ json.RawMessage) ([]byte, error) {
				return []byte(`"` + connector + `"`), nil
			})
		})
	}
	envelope := `{"connector_key":"erp","method":"get","action":"erp-get","parameters":{},"org_id":"org-1","source":{"id":"u","type":"person"},"connection":{"id":"c","connector_key":"erp","scope":"org"}}`
	out, err := dispatch.RunConnectorMethod([]byte(envelope))
	if err != nil || string(out) != `"erp"` {
		t.Fatalf("RunConnectorMethod = %s, %v", out, err)
	}
	// The name integrity check survives bundling.
	wrongMethod := strings.Replace(envelope, `"method":"get"`, `"method":"list"`, 1)
	if _, err := dispatch.RunConnectorMethod([]byte(wrongMethod)); err == nil || !strings.Contains(err.Error(), "mis-deployed") {
		t.Errorf("method mismatch err = %v, want the mis-deployed-wasm error", err)
	}
}

func TestRegister_DoubleRegistrationPanicsPerSlugOnly(t *testing.T) {
	dispatch.ResetForTest()
	noop := func(_ dispatch.Context, r json.RawMessage) ([]byte, error) { return r, nil }

	dispatch.WithSlug("a", func() { dispatch.RegisterBeforeCreate(noop) })
	dispatch.WithSlug("b", func() { dispatch.RegisterBeforeCreate(noop) }) // same slot, other function: fine

	defer func() {
		recovered := recover()
		if recovered == nil {
			t.Fatal("re-registering before_create for the same bundle function must panic")
		}
		if !strings.Contains(recovered.(string), `"a"`) {
			t.Errorf("panic %q should name the bundle function", recovered)
		}
	}()
	dispatch.WithSlug("a", func() { dispatch.RegisterBeforeCreate(noop) })
}

func TestWithSlug_ClearsScopeAfterPanic(t *testing.T) {
	dispatch.ResetForTest()
	func() {
		defer func() { _ = recover() }()
		dispatch.WithSlug("boom", func() { panic("registration failed") })
	}()
	// A later unkeyed registration must land in the standalone slot, not
	// under the slug whose registration blew up.
	dispatch.RegisterBeforeCreate(func(_ dispatch.Context, r json.RawMessage) ([]byte, error) { return r, nil })
	if _, err := dispatch.RunHook([]byte(`{"event":"before_create","entity":"x","record":{},"org_id":"o","source":{"id":"u","type":"person"}}`)); err != nil {
		t.Fatalf("standalone dispatch after a failed bundle registration: %v", err)
	}
}
