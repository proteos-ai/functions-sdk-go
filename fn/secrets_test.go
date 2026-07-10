package fn

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestSecrets_Read_HappyPath(t *testing.T) {
	resetTransport()
	var seen struct{ Name string }
	transport.secretsRead = func(in []byte) ([]byte, error) {
		_ = json.Unmarshal(in, &seen)
		return okEnvelope(t, map[string]any{"value": "s3cr3t"}), nil
	}

	v, err := Secrets.Read(Context{}, "POSTMARK_TOKEN")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if seen.Name != "POSTMARK_TOKEN" {
		t.Errorf("seen.Name = %q", seen.Name)
	}
	if v != "s3cr3t" {
		t.Errorf("v = %q", v)
	}
}

func TestSecrets_Read_BadInputMapped(t *testing.T) {
	resetTransport()
	transport.secretsRead = func([]byte) ([]byte, error) {
		return errEnvelope(t, "bad_input", "secret name must be non-empty"), nil
	}
	_, err := Secrets.Read(Context{}, "")
	if !errors.Is(err, ErrBadInput) {
		t.Errorf("errors.Is(err, ErrBadInput) = false; err = %v", err)
	}
}

func TestSecrets_Read_NotFoundMapped(t *testing.T) {
	resetTransport()
	transport.secretsRead = func([]byte) ([]byte, error) {
		return errEnvelope(t, "not_found", "no such secret"), nil
	}
	_, err := Secrets.Read(Context{}, "missing")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("errors.Is(err, ErrNotFound) = false; err = %v", err)
	}
}
