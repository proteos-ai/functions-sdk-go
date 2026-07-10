package fn

import (
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestCache_Get_Hit(t *testing.T) {
	resetTransport()
	transport.cacheGet = func([]byte) ([]byte, error) {
		return okEnvelope(t, map[string]any{"value": "v-1", "hit": true}), nil
	}

	val, hit, err := Cache.Get(Context{}, "k")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !hit || val != "v-1" {
		t.Errorf("hit=%v val=%q", hit, val)
	}
}

func TestCache_Get_Miss(t *testing.T) {
	resetTransport()
	transport.cacheGet = func([]byte) ([]byte, error) {
		return okEnvelope(t, map[string]any{"hit": false}), nil
	}

	val, hit, err := Cache.Get(Context{}, "k")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if hit || val != "" {
		t.Errorf("expected miss; got hit=%v val=%q", hit, val)
	}
}

func TestCache_Set_TTLRoundTrip(t *testing.T) {
	resetTransport()
	var seen struct {
		Key        string `json:"key"`
		Value      string `json:"value"`
		TTLSeconds int    `json:"ttl_seconds"`
	}
	transport.cacheSet = func(in []byte) ([]byte, error) {
		_ = json.Unmarshal(in, &seen)
		return okEnvelope(t, nil), nil
	}

	if err := Cache.Set(Context{}, "k", "v", 30*time.Second); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if seen.Key != "k" || seen.Value != "v" || seen.TTLSeconds != 30 {
		t.Errorf("seen = %+v", seen)
	}
}

func TestCache_Set_InternalErr(t *testing.T) {
	resetTransport()
	transport.cacheSet = func([]byte) ([]byte, error) {
		return errEnvelope(t, "internal", "redis down"), nil
	}

	err := Cache.Set(Context{}, "k", "v", 0)
	if !errors.Is(err, ErrInternal) {
		t.Errorf("errors.Is(err, ErrInternal) = false; err = %v", err)
	}
}

func TestCache_Delete(t *testing.T) {
	resetTransport()
	var seen struct{ Key string }
	transport.cacheDelete = func(in []byte) ([]byte, error) {
		_ = json.Unmarshal(in, &seen)
		return okEnvelope(t, nil), nil
	}

	if err := Cache.Delete(Context{}, "k"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if seen.Key != "k" {
		t.Errorf("seen.Key = %q", seen.Key)
	}
}
