package fn

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestStorage_GenerateDownloadUrl_HappyPath(t *testing.T) {
	resetTransport()
	var seen struct {
		FileId         string `json:"file_id"`
		AllowsMultiUse bool   `json:"allows_multi_use"`
	}
	transport.storageGenerateDownloadUrl = func(in []byte) ([]byte, error) {
		_ = json.Unmarshal(in, &seen)
		return okEnvelope(t, map[string]any{
			"url":        "https://api.proteos.ai/storage/v1/files/file-1/download/url/tok",
			"expires_at": "2026-06-12T15:30:00Z",
		}), nil
	}

	url, expiresAt, err := Storage.GenerateDownloadUrl(Context{}, "file-1", GenerateDownloadUrlOptions{})
	if err != nil {
		t.Fatalf("GenerateDownloadUrl: %v", err)
	}
	if seen.FileId != "file-1" {
		t.Errorf("seen.FileId = %q", seen.FileId)
	}
	if seen.AllowsMultiUse {
		t.Errorf("seen.AllowsMultiUse = true; want false by default")
	}
	if url != "https://api.proteos.ai/storage/v1/files/file-1/download/url/tok" {
		t.Errorf("url = %q", url)
	}
	if expiresAt != "2026-06-12T15:30:00Z" {
		t.Errorf("expiresAt = %q", expiresAt)
	}
}

func TestStorage_GenerateDownloadUrl_MultiUseFlagSent(t *testing.T) {
	resetTransport()
	var seen struct {
		AllowsMultiUse bool `json:"allows_multi_use"`
	}
	transport.storageGenerateDownloadUrl = func(in []byte) ([]byte, error) {
		_ = json.Unmarshal(in, &seen)
		return okEnvelope(t, map[string]any{"url": "u", "expires_at": "t"}), nil
	}

	if _, _, err := Storage.GenerateDownloadUrl(Context{}, "file-1", GenerateDownloadUrlOptions{AllowsMultiUse: true}); err != nil {
		t.Fatalf("GenerateDownloadUrl: %v", err)
	}
	if !seen.AllowsMultiUse {
		t.Errorf("seen.AllowsMultiUse = false; want true")
	}
}

func TestStorage_GenerateDownloadUrl_NotFoundMapped(t *testing.T) {
	resetTransport()
	transport.storageGenerateDownloadUrl = func([]byte) ([]byte, error) {
		return errEnvelope(t, "not_found", "no such file"), nil
	}
	_, _, err := Storage.GenerateDownloadUrl(Context{}, "missing", GenerateDownloadUrlOptions{})
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("errors.Is(err, ErrNotFound) = false; err = %v", err)
	}
}

func TestStorage_GenerateDownloadUrl_PermissionDeniedMapped(t *testing.T) {
	resetTransport()
	transport.storageGenerateDownloadUrl = func([]byte) ([]byte, error) {
		return errEnvelope(t, "permission_denied", "files:read required"), nil
	}
	_, _, err := Storage.GenerateDownloadUrl(Context{}, "file-1", GenerateDownloadUrlOptions{})
	if !errors.Is(err, ErrPermissionDenied) {
		t.Errorf("errors.Is(err, ErrPermissionDenied) = false; err = %v", err)
	}
}
