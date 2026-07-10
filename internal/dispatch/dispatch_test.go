package dispatch_test

import (
	"encoding/json"
	"errors"
	"testing"

	"go.proteos.ai/functions-sdk-go/internal/dispatch"
)

func TestRunHook_DispatchesEachEventToCorrectSlot(t *testing.T) {
	cases := []struct {
		event          string
		envelope       []byte
		wantRecord     string
		wantSecondary  string // currentRecord (beforeUpdate) or previousRecord (afterUpdate); empty otherwise
		register       func(captured *captured)
		secondaryLabel string
	}{
		{
			event:      "before_create",
			envelope:   []byte(`{"event":"before_create","entity":"invoice","record":{"v":1},"org_id":"org-1","source":{"id":"u","type":"user"}}`),
			wantRecord: `{"v":1}`,
			register: func(c *captured) {
				dispatch.RegisterBeforeCreate(func(ctx dispatch.Context, r json.RawMessage) ([]byte, error) {
					c.record = string(r)
					c.orgId = ctx.OrgId
					return []byte(`{"ok":true}`), nil
				})
			},
		},
		{
			event:          "before_update",
			envelope:       []byte(`{"event":"before_update","entity":"invoice","record":{"v":2},"current_record":{"v":1},"org_id":"org-1","source":{"id":"u","type":"user"}}`),
			wantRecord:     `{"v":2}`,
			wantSecondary:  `{"v":1}`,
			secondaryLabel: "current_record",
			register: func(c *captured) {
				dispatch.RegisterBeforeUpdate(func(_ dispatch.Context, r, cur json.RawMessage) ([]byte, error) {
					c.record = string(r)
					c.secondary = string(cur)
					return nil, nil
				})
			},
		},
		{
			event:      "before_delete",
			envelope:   []byte(`{"event":"before_delete","entity":"invoice","record":{"v":1},"org_id":"org-1","source":{"id":"u","type":"user"}}`),
			wantRecord: `{"v":1}`,
			register: func(c *captured) {
				dispatch.RegisterBeforeDelete(func(_ dispatch.Context, r json.RawMessage) ([]byte, error) {
					c.record = string(r)
					return nil, nil
				})
			},
		},
		{
			event:      "after_create",
			envelope:   []byte(`{"event":"after_create","entity":"invoice","record":{"v":1},"org_id":"org-1","source":{"id":"u","type":"user"}}`),
			wantRecord: `{"v":1}`,
			register: func(c *captured) {
				dispatch.RegisterAfterCreate(func(_ dispatch.Context, r json.RawMessage) ([]byte, error) {
					c.record = string(r)
					return nil, nil
				})
			},
		},
		{
			event:          "after_update",
			envelope:       []byte(`{"event":"after_update","entity":"invoice","record":{"v":2},"previous_record":{"v":1},"org_id":"org-1","source":{"id":"u","type":"user"}}`),
			wantRecord:     `{"v":2}`,
			wantSecondary:  `{"v":1}`,
			secondaryLabel: "previous_record",
			register: func(c *captured) {
				dispatch.RegisterAfterUpdate(func(_ dispatch.Context, r, prev json.RawMessage) ([]byte, error) {
					c.record = string(r)
					c.secondary = string(prev)
					return nil, nil
				})
			},
		},
		{
			event:      "after_delete",
			envelope:   []byte(`{"event":"after_delete","entity":"invoice","record":{"v":1},"org_id":"org-1","source":{"id":"u","type":"user"}}`),
			wantRecord: `{"v":1}`,
			register: func(c *captured) {
				dispatch.RegisterAfterDelete(func(_ dispatch.Context, r json.RawMessage) ([]byte, error) {
					c.record = string(r)
					return nil, nil
				})
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.event, func(t *testing.T) {
			dispatch.ResetForTest()
			var c captured
			tc.register(&c)

			if _, err := dispatch.RunHook(tc.envelope); err != nil {
				t.Fatalf("RunHook: %v", err)
			}
			if c.record != tc.wantRecord {
				t.Errorf("record = %s, want %s", c.record, tc.wantRecord)
			}
			if tc.wantSecondary != "" && c.secondary != tc.wantSecondary {
				t.Errorf("%s = %s, want %s", tc.secondaryLabel, c.secondary, tc.wantSecondary)
			}
		})
	}
}

func TestRunHook_NoHandlerRegistered(t *testing.T) {
	dispatch.ResetForTest()
	_, err := dispatch.RunHook([]byte(`{"event":"before_create","record":{}}`))
	if !errors.Is(err, dispatch.ErrNoHandler) {
		t.Fatalf("got err %v, want ErrNoHandler", err)
	}
}

func TestRunHook_UnknownEventRejected(t *testing.T) {
	dispatch.ResetForTest()
	_, err := dispatch.RunHook([]byte(`{"event":"justKidding","record":{}}`))
	if err == nil {
		t.Fatal("expected error for unknown event")
	}
}

func TestRunAction_DispatchesWithRecordId(t *testing.T) {
	dispatch.ResetForTest()
	var capturedRecordId string
	var capturedParams string
	dispatch.RegisterAction(func(_ dispatch.Context, recordId string, params json.RawMessage) ([]byte, error) {
		capturedRecordId = recordId
		capturedParams = string(params)
		return []byte(`{"sent":1}`), nil
	})
	out, err := dispatch.RunAction([]byte(`{"entity":"invoice","record_id":"r-42","action":"send","parameters":{"to":"a@b.com"},"org_id":"org-1","source":{"id":"u","type":"user"}}`))
	if err != nil {
		t.Fatalf("RunAction: %v", err)
	}
	if string(out) != `{"sent":1}` {
		t.Errorf("out = %s", out)
	}
	if capturedRecordId != "r-42" {
		t.Errorf("recordId = %q", capturedRecordId)
	}
	if capturedParams != `{"to":"a@b.com"}` {
		t.Errorf("params = %s", capturedParams)
	}
}

func TestRunAction_GlobalActionEmptyRecordId(t *testing.T) {
	dispatch.ResetForTest()
	var capturedRecordId = "PRESET-NOT-OVERWRITTEN"
	dispatch.RegisterAction(func(_ dispatch.Context, recordId string, _ json.RawMessage) ([]byte, error) {
		capturedRecordId = recordId
		return nil, nil
	})
	_, _ = dispatch.RunAction([]byte(`{"action":"rebuild","parameters":{},"org_id":"org-1","source":{"id":"s","type":"system"}}`))
	if capturedRecordId != "" {
		t.Errorf("recordId = %q, want empty for global action", capturedRecordId)
	}
}

func TestRegister_DoubleRegisterPanics(t *testing.T) {
	cases := []struct {
		name     string
		register func()
		wantSub  string
	}{
		{
			name: "BeforeCreate",
			register: func() {
				dispatch.RegisterBeforeCreate(func(_ dispatch.Context, _ json.RawMessage) ([]byte, error) { return nil, nil })
			},
			wantSub: "OnBeforeCreate",
		},
		{
			name: "BeforeUpdate",
			register: func() {
				dispatch.RegisterBeforeUpdate(func(_ dispatch.Context, _, _ json.RawMessage) ([]byte, error) { return nil, nil })
			},
			wantSub: "OnBeforeUpdate",
		},
		{
			name: "AfterUpdate",
			register: func() {
				dispatch.RegisterAfterUpdate(func(_ dispatch.Context, _, _ json.RawMessage) ([]byte, error) { return nil, nil })
			},
			wantSub: "OnAfterUpdate",
		},
		{
			name: "Action",
			register: func() {
				dispatch.RegisterAction(func(_ dispatch.Context, _ string, _ json.RawMessage) ([]byte, error) { return nil, nil })
			},
			wantSub: "action handler",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dispatch.ResetForTest()
			tc.register()
			defer func() {
				r := recover()
				if r == nil {
					t.Fatal("expected panic on double register")
				}
				msg, ok := r.(string)
				if !ok || msg == "" {
					t.Fatalf("panic value = %v, want non-empty string", r)
				}
				if !contains(msg, tc.wantSub) {
					t.Errorf("panic msg %q missing %q", msg, tc.wantSub)
				}
			}()
			tc.register()
		})
	}
}

type captured struct {
	record    string
	secondary string
	orgId     string
}

func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
