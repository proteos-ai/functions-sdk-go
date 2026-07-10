package fn

import (
	"encoding/json"
	"time"

	"go.proteos.ai/functions-sdk-go/internal/dispatch"
)

// Connection is the resolved connection a connector method runs against,
// injected by the platform: connector-service resolves the credentials
// (broker-refreshed for OAuth) and function-service hands them to the guest.
// The method author never wires connection ids or tokens — and refresh
// tokens / OAuth app secrets are structurally absent from this type.
type Connection struct {
	Id                string
	ConnectorKey      string
	Scope             string // "org" | "user"
	ExternalAccountId string
	// Settings is the connection's configuration + machine-written sync state.
	Settings map[string]any
	// AccessToken is the usable credential material for the connection's
	// kind: a live OAuth access token, an api key, a bot token, or
	// "username:password" for basic auth.
	AccessToken string
	// TokenExpiresAt is the OAuth access token's expiry (zero for static
	// credential kinds). The platform hands out tokens with a freshness
	// buffer, so a method normally never observes an expired one.
	TokenExpiresAt time.Time
}

// ConnectorContext is the first argument of every connector-method handler:
// the usual per-call identity plus the resolved Connection.
type ConnectorContext struct {
	Context
	Connection Connection
}

// Token returns the pre-resolved access token (sugar over
// ConnectorContext.Connection.AccessToken).
func (ctx ConnectorContext) Token() string {
	return ctx.Connection.AccessToken
}

// RegisterConnectorMethod registers the typed handler of ONE connector
// method (one wasm per method; the name must match the method.json folder —
// codegen + the module build keep them aligned). Call from init() in the
// method's main.go:
//
//	func init() {
//		fn.RegisterConnectorMethod("get_event",
//			func(ctx ConnectorContext, params methods.GetEventParams) (methods.GetEventResult, error) {
//				token := ctx.Token()
//				...
//			})
//	}
func RegisterConnectorMethod[P any, R any](name string, h func(ctx ConnectorContext, params P) (R, error)) {
	dispatch.RegisterConnectorMethod(name, func(ctx dispatch.Context, connection dispatch.ConnectionInfo, raw json.RawMessage) ([]byte, error) {
		var params P
		if err := json.Unmarshal(raw, &params); err != nil {
			return nil, err
		}
		out, err := h(toConnectorCtx(ctx, connection), params)
		if err != nil {
			return nil, err
		}
		return json.Marshal(out)
	})
}

func toConnectorCtx(c dispatch.Context, info dispatch.ConnectionInfo) ConnectorContext {
	connection := Connection{
		Id:                info.Id,
		ConnectorKey:      info.ConnectorKey,
		Scope:             info.Scope,
		ExternalAccountId: info.ExternalAccountId,
		Settings:          info.Settings,
		AccessToken:       info.AccessToken,
	}
	if info.TokenExpiresAt != "" {
		if expiresAt, err := time.Parse(time.RFC3339, info.TokenExpiresAt); err == nil {
			connection.TokenExpiresAt = expiresAt
		}
	}
	return ConnectorContext{
		Context:    toPhotonCtx(c),
		Connection: connection,
	}
}
