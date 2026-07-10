package fn

import (
	"encoding/json"
	"time"
)

// Cache is the guest's KV cache surface, backed by data-service /
// function-service's shared Redis. Values are arbitrary strings —
// authors marshal/unmarshal JSON themselves if they want structured data.
var Cache = cacheAPI{}

type cacheAPI struct{}

// Get returns (value, true, nil) on hit, ("", false, nil) on miss, or
// ("", false, err) on transport / authorization failure.
func (cacheAPI) Get(_ Context, key string) (string, bool, error) {
	req, err := json.Marshal(struct {
		Key string `json:"key"`
	}{key})
	if err != nil {
		return "", false, err
	}
	raw, err := callDecode(transport.cacheGet, req)
	if err != nil {
		return "", false, err
	}
	var resp struct {
		Value string `json:"value"`
		Hit   bool   `json:"hit"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return "", false, err
	}
	return resp.Value, resp.Hit, nil
}

// Set writes a value with an optional TTL. Pass 0 for no expiry.
func (cacheAPI) Set(_ Context, key, value string, ttl time.Duration) error {
	req, err := json.Marshal(struct {
		Key        string `json:"key"`
		Value      string `json:"value"`
		TTLSeconds int    `json:"ttl_seconds"`
	}{key, value, int(ttl.Seconds())})
	if err != nil {
		return err
	}
	_, err = callDecode(transport.cacheSet, req)
	return err
}

// Delete removes a key.
func (cacheAPI) Delete(_ Context, key string) error {
	req, err := json.Marshal(struct {
		Key string `json:"key"`
	}{key})
	if err != nil {
		return err
	}
	_, err = callDecode(transport.cacheDelete, req)
	return err
}
