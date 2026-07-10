package fn

import (
	"encoding/json"
	"time"
)

// Connections is the connector-platform surface for hooks and actions: fetch
// a connection's usable token, or invoke one of its connector's methods. The
// platform authorizes every call as the invoking user (connections read for
// tokens/read methods, write for write methods) — there is no ambient
// bypass. Connector-method AUTHORS don't need this: their token arrives
// pre-resolved on ConnectorContext.
var Connections = connectionsAPI{}

type connectionsAPI struct{}

// ConnectionToken is a connection's usable credential material: a live OAuth
// access token (broker-refreshed), an api key, a bot token, or
// "username:password" for basic auth. Never a refresh token or app secret.
type ConnectionToken struct {
	AccessToken string
	// ExpiresAt is zero for static credential kinds.
	ExpiresAt time.Time
}

// GetToken returns the connection's live token material.
func (connectionsAPI) GetToken(_ Context, connectionId string) (ConnectionToken, error) {
	req, err := json.Marshal(struct {
		ConnectionId string `json:"connection_id"`
	}{connectionId})
	if err != nil {
		return ConnectionToken{}, err
	}
	raw, err := callDecode(transport.connectionsGetToken, req)
	if err != nil {
		return ConnectionToken{}, err
	}
	var resp struct {
		AccessToken string `json:"access_token"`
		ExpiresAt   string `json:"expires_at"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return ConnectionToken{}, err
	}
	token := ConnectionToken{AccessToken: resp.AccessToken}
	if resp.ExpiresAt != "" {
		if expiresAt, parseErr := time.Parse(time.RFC3339, resp.ExpiresAt); parseErr == nil {
			token.ExpiresAt = expiresAt
		}
	}
	return token, nil
}

// InvokeMethod invokes a connector method on a connection and returns the
// raw result JSON (shaped by the method's returns schema). params may be any
// JSON-marshallable value matching the method's params schema.
func (connectionsAPI) InvokeMethod(_ Context, connectionId string, method string, params any) (json.RawMessage, error) {
	encodedParams, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	req, err := json.Marshal(struct {
		ConnectionId string          `json:"connection_id"`
		Method       string          `json:"method"`
		Params       json.RawMessage `json:"params"`
	}{connectionId, method, encodedParams})
	if err != nil {
		return nil, err
	}
	raw, err := callDecode(transport.connectionsInvokeMethod, req)
	if err != nil {
		return nil, err
	}
	return raw, nil
}
