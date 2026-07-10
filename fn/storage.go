package fn

import (
	"encoding/json"
)

// Storage is the storage-service surface available to a hook/action. Today
// it generates short-lived public download URLs for stored files — the URL
// a function hands to an external service (e.g. a transcription API) so it
// can fetch the bytes. Uploads still go through the platform's upload path,
// not from inside a hook/action.
var Storage = storageAPI{}

type storageAPI struct{}

// GenerateDownloadUrlOptions tunes a minted download URL.
type GenerateDownloadUrlOptions struct {
	// AllowsMultiUse mints a token that is NOT spent on first read, so the URL
	// survives an external consumer that probes it before downloading (HEAD+GET,
	// a redirect-follow, a retry) and gets a longer TTL. Default false yields a
	// single-use, ~5 min URL. Prefer the default unless the consumer is outside
	// our control.
	AllowsMultiUse bool
}

// GenerateDownloadUrl returns a public download URL for the stored file
// `fileId`, plus the URL's expiry as an RFC3339 timestamp. By default the URL
// is short-lived (~5 min) and single-use, so mint it immediately before handing
// it to the external consumer; pass GenerateDownloadUrlOptions{AllowsMultiUse:
// true} for a reusable, longer-lived URL. Errors mirror the records surface:
// ErrNotFound when the file is unknown, ErrPermissionDenied without the
// files:read grant on the principal the function runs as.
func (storageAPI) GenerateDownloadUrl(_ Context, fileId string, opts GenerateDownloadUrlOptions) (url string, expiresAt string, err error) {
	req, err := json.Marshal(struct {
		FileId         string `json:"file_id"`
		AllowsMultiUse bool   `json:"allows_multi_use"`
	}{fileId, opts.AllowsMultiUse})
	if err != nil {
		return "", "", err
	}
	raw, err := callDecode(transport.storageGenerateDownloadUrl, req)
	if err != nil {
		return "", "", err
	}
	var resp struct {
		Url       string `json:"url"`
		ExpiresAt string `json:"expires_at"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return "", "", err
	}
	return resp.Url, resp.ExpiresAt, nil
}
