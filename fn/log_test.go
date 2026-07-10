package fn

import (
	"encoding/json"
	"testing"
)

type logCall struct {
	level   string
	payload []byte
}

func captureLog(t *testing.T) (capture *[]logCall, restore func()) {
	t.Helper()
	calls := []logCall{}
	prev := transport
	transport.logDebug = func(in []byte) ([]byte, error) {
		calls = append(calls, logCall{"debug", append([]byte(nil), in...)})
		return nil, nil
	}
	transport.logInfo = func(in []byte) ([]byte, error) {
		calls = append(calls, logCall{"info", append([]byte(nil), in...)})
		return nil, nil
	}
	transport.logWarn = func(in []byte) ([]byte, error) {
		calls = append(calls, logCall{"warn", append([]byte(nil), in...)})
		return nil, nil
	}
	transport.logError = func(in []byte) ([]byte, error) {
		calls = append(calls, logCall{"error", append([]byte(nil), in...)})
		return nil, nil
	}
	return &calls, func() { transport = prev }
}

func TestLog_RoutesByLevel(t *testing.T) {
	calls, restore := captureLog(t)
	defer restore()

	ctx := Context{}
	Log.Debug(ctx, "d", map[string]any{"k": "v"})
	Log.Info(ctx, "i", map[string]any{"k": "v"})
	Log.Warn(ctx, "w", map[string]any{"k": "v"})
	Log.Error(ctx, "e", map[string]any{"k": "v"})

	if len(*calls) != 4 {
		t.Fatalf("expected 4 calls, got %d", len(*calls))
	}
	wantLevels := []string{"debug", "info", "warn", "error"}
	for i, c := range *calls {
		if c.level != wantLevels[i] {
			t.Errorf("call %d level = %q, want %q", i, c.level, wantLevels[i])
		}
	}
}

func TestLog_PayloadShape(t *testing.T) {
	calls, restore := captureLog(t)
	defer restore()

	Log.Info(Context{}, "validated", map[string]any{
		"invoiceId":  "inv-1",
		"customerId": "cust-2",
	})

	if len(*calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(*calls))
	}
	var got struct {
		Msg    string         `json:"msg"`
		Fields map[string]any `json:"fields"`
	}
	if err := json.Unmarshal((*calls)[0].payload, &got); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if got.Msg != "validated" {
		t.Errorf("msg = %q", got.Msg)
	}
	if got.Fields["invoiceId"] != "inv-1" || got.Fields["customerId"] != "cust-2" {
		t.Errorf("fields = %+v", got.Fields)
	}
}

func TestLog_NilFieldsOmitted(t *testing.T) {
	calls, restore := captureLog(t)
	defer restore()

	Log.Info(Context{}, "msg only", nil)

	var got map[string]json.RawMessage
	if err := json.Unmarshal((*calls)[0].payload, &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, present := got["fields"]; present {
		t.Errorf("fields key should be omitted when nil, payload = %s", (*calls)[0].payload)
	}
}

func TestLog_HostBuild_SilentNoop(t *testing.T) {
	// With no transport wired (default !wasip1 state via resetTransport),
	// Log calls don't panic and don't reach a transport closure.
	resetTransport()
	Log.Info(Context{}, "msg", map[string]any{"k": "v"})
	// No assertion needed — the test passes if the call doesn't panic.
}
