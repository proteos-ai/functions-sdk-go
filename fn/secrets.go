package fn

import (
	"encoding/json"
)

// Secrets is the read-only secrets surface. Writes happen via the
// platform's secret-store admin path, never from inside a hook/action.
var Secrets = secretsAPI{}

type secretsAPI struct{}

// Read returns the value of the named secret, or an error if it doesn't
// exist (ErrNotFound) or the caller lacks access (ErrPermissionDenied).
func (secretsAPI) Read(_ Context, name string) (string, error) {
	req, err := json.Marshal(struct {
		Name string `json:"name"`
	}{name})
	if err != nil {
		return "", err
	}
	raw, err := callDecode(transport.secretsRead, req)
	if err != nil {
		return "", err
	}
	var resp struct {
		Value string `json:"value"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return "", err
	}
	return resp.Value, nil
}
