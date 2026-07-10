package fn

import (
	"encoding/json"

	"go.proteos.ai/functions-sdk-go/internal/dispatch"
)

// OnBeforeCreate registers a typed beforeCreate handler. Call from init().
// Panics if called twice in the same process.
func OnBeforeCreate[T any](h func(ctx Context, record T) (T, error)) {
	dispatch.RegisterBeforeCreate(func(ctx dispatch.Context, record json.RawMessage) ([]byte, error) {
		var rec T
		if err := json.Unmarshal(record, &rec); err != nil {
			return nil, err
		}
		out, err := h(toPhotonCtx(ctx), rec)
		if err != nil {
			return nil, err
		}
		return json.Marshal(out)
	})
}

// OnBeforeUpdate registers a typed beforeUpdate handler. Call from init().
func OnBeforeUpdate[T any](h func(ctx Context, record T, currentRecord T) (T, error)) {
	dispatch.RegisterBeforeUpdate(func(ctx dispatch.Context, record, currentRecord json.RawMessage) ([]byte, error) {
		var rec, cur T
		if err := json.Unmarshal(record, &rec); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(currentRecord, &cur); err != nil {
			return nil, err
		}
		out, err := h(toPhotonCtx(ctx), rec, cur)
		if err != nil {
			return nil, err
		}
		return json.Marshal(out)
	})
}

// OnBeforeDelete registers a typed beforeDelete handler. Returns no
// payload — the host expects nil bytes back on success.
func OnBeforeDelete[T any](h func(ctx Context, record T) error) {
	dispatch.RegisterBeforeDelete(func(ctx dispatch.Context, record json.RawMessage) ([]byte, error) {
		var rec T
		if err := json.Unmarshal(record, &rec); err != nil {
			return nil, err
		}
		return nil, h(toPhotonCtx(ctx), rec)
	})
}

// OnAfterCreate registers a typed afterCreate handler. No return payload.
func OnAfterCreate[T any](h func(ctx Context, record T) error) {
	dispatch.RegisterAfterCreate(func(ctx dispatch.Context, record json.RawMessage) ([]byte, error) {
		var rec T
		if err := json.Unmarshal(record, &rec); err != nil {
			return nil, err
		}
		return nil, h(toPhotonCtx(ctx), rec)
	})
}

// OnAfterUpdate registers a typed afterUpdate handler. Receives both the
// just-persisted record and the previousRecord (the row's pre-update
// state). No return payload.
func OnAfterUpdate[T any](h func(ctx Context, record T, previousRecord T) error) {
	dispatch.RegisterAfterUpdate(func(ctx dispatch.Context, record, previousRecord json.RawMessage) ([]byte, error) {
		var rec, prev T
		if err := json.Unmarshal(record, &rec); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(previousRecord, &prev); err != nil {
			return nil, err
		}
		return nil, h(toPhotonCtx(ctx), rec, prev)
	})
}

// OnAfterDelete registers a typed afterDelete handler. No return payload.
func OnAfterDelete[T any](h func(ctx Context, record T) error) {
	dispatch.RegisterAfterDelete(func(ctx dispatch.Context, record json.RawMessage) ([]byte, error) {
		var rec T
		if err := json.Unmarshal(record, &rec); err != nil {
			return nil, err
		}
		return nil, h(toPhotonCtx(ctx), rec)
	})
}

// RegisterAction registers a typed entity-scoped action handler. The
// wasm carries exactly one action — this and RegisterGlobalAction share
// the same slot, so calling either after the other panics.
func RegisterAction[P any, R any](h func(ctx Context, recordId string, params P) (R, error)) {
	dispatch.RegisterAction(func(ctx dispatch.Context, recordId string, raw json.RawMessage) ([]byte, error) {
		var params P
		if err := json.Unmarshal(raw, &params); err != nil {
			return nil, err
		}
		out, err := h(toPhotonCtx(ctx), recordId, params)
		if err != nil {
			return nil, err
		}
		return json.Marshal(out)
	})
}

// RegisterGlobalAction registers a typed global action handler. recordId
// from the dispatch envelope is ignored (it'll be empty anyway). Shares
// the action slot with RegisterAction.
func RegisterGlobalAction[P any, R any](h func(ctx Context, params P) (R, error)) {
	dispatch.RegisterAction(func(ctx dispatch.Context, _ string, raw json.RawMessage) ([]byte, error) {
		var params P
		if err := json.Unmarshal(raw, &params); err != nil {
			return nil, err
		}
		out, err := h(toPhotonCtx(ctx), params)
		if err != nil {
			return nil, err
		}
		return json.Marshal(out)
	})
}

func toPhotonCtx(c dispatch.Context) Context {
	return Context{
		Ctx:     c.Ctx,
		OrgId:   c.OrgId,
		Source:  Source{Id: c.Source.Id, Type: c.Source.Type},
		Headers: c.Headers,
	}
}
