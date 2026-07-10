// Package dispatch is the host-portable decoder + slot table that sits
// between the fn registration API (host- and wasip1-compileable) and
// the wasip1-only runtime/autoexport entry points.
//
// It is internal — only packages under this module's root may import it.
// Photon's Register* generics wrap a typed handler in a closure and call
// one of the Register* setters here. The wasip1 autoexport package calls
// RunHook / RunAction with the bytes from pdk.Input.
//
// Pure Go, no Extism deps, no build tag — so dispatch unit tests run
// under plain `go test` on the host.
package dispatch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

// ----------------------------------------------------------------------
// Envelopes (mirror the wire shapes the host runtime sends in).

// Source is the invocation source. Type ∈ {"person", "agent", "api", "system"}.
type Source struct {
	Id   string `json:"id"`
	Type string `json:"type"`
}

// Context is what the wrapper closures receive from RunHook / RunAction.
// Photon wraps it into fn.Context before calling the typed handler.
type Context struct {
	Ctx     context.Context
	OrgId   string
	Source  Source
	Headers map[string]string
}

// HookEnvelope — sent by the host into the guest for any hook dispatch.
// Fields populated based on Event:
//
//	beforeCreate, beforeDelete, afterCreate, afterDelete → Record only
//	beforeUpdate                                         → Record + CurrentRecord
//	afterUpdate                                          → Record + PreviousRecord
type HookEnvelope struct {
	Event          string          `json:"event"`
	Entity         string          `json:"entity"`
	Record         json.RawMessage `json:"record"`
	CurrentRecord  json.RawMessage `json:"current_record,omitempty"`
	PreviousRecord json.RawMessage `json:"previous_record,omitempty"`
	OrgId          string          `json:"org_id"`
	Source         Source          `json:"source"`
}

// ActionEnvelope — sent by the host into the guest for any action invocation.
// Entity + RecordId are empty for global actions. Headers carries the inbound
// HTTP request headers (flattened) for HTTP-originated dispatch — e.g. a
// webhook receiver reading a token header; empty otherwise.
type ActionEnvelope struct {
	Entity     string            `json:"entity,omitempty"`
	RecordId   string            `json:"record_id,omitempty"`
	Action     string            `json:"action"`
	Parameters json.RawMessage   `json:"parameters"`
	OrgId      string            `json:"org_id"`
	Source     Source            `json:"source"`
	Headers    map[string]string `json:"headers,omitempty"`
}

// ----------------------------------------------------------------------
// Slot table — one handler per slot. Three explicit fn types so the
// register call site for each event reads with the same field names that
// the wire envelope uses, and so the compiler catches slot mismatches.

type SingleRecordHookFn func(ctx Context, record json.RawMessage) ([]byte, error)
type BeforeUpdateHookFn func(ctx Context, record, currentRecord json.RawMessage) ([]byte, error)
type AfterUpdateHookFn func(ctx Context, record, previousRecord json.RawMessage) ([]byte, error)
type ActionFn func(ctx Context, recordId string, parameters json.RawMessage) ([]byte, error)

var (
	beforeCreate SingleRecordHookFn
	beforeUpdate BeforeUpdateHookFn
	beforeDelete SingleRecordHookFn
	afterCreate  SingleRecordHookFn
	afterUpdate  AfterUpdateHookFn
	afterDelete  SingleRecordHookFn
	action       ActionFn
)

// ----------------------------------------------------------------------
// Register* setters. Each panics on double-register with a slot-named
// message — the wasm guest carries exactly one handler per slot, so
// re-registration is always a programming mistake.

func RegisterBeforeCreate(fn SingleRecordHookFn) {
	if beforeCreate != nil {
		panic("fn: OnBeforeCreate handler already registered")
	}
	beforeCreate = fn
}

func RegisterBeforeUpdate(fn BeforeUpdateHookFn) {
	if beforeUpdate != nil {
		panic("fn: OnBeforeUpdate handler already registered")
	}
	beforeUpdate = fn
}

func RegisterBeforeDelete(fn SingleRecordHookFn) {
	if beforeDelete != nil {
		panic("fn: OnBeforeDelete handler already registered")
	}
	beforeDelete = fn
}

func RegisterAfterCreate(fn SingleRecordHookFn) {
	if afterCreate != nil {
		panic("fn: OnAfterCreate handler already registered")
	}
	afterCreate = fn
}

func RegisterAfterUpdate(fn AfterUpdateHookFn) {
	if afterUpdate != nil {
		panic("fn: OnAfterUpdate handler already registered")
	}
	afterUpdate = fn
}

func RegisterAfterDelete(fn SingleRecordHookFn) {
	if afterDelete != nil {
		panic("fn: OnAfterDelete handler already registered")
	}
	afterDelete = fn
}

func RegisterAction(fn ActionFn) {
	if action != nil {
		panic("fn: action handler already registered")
	}
	action = fn
}

// ----------------------------------------------------------------------
// Dispatch entry points called by runtime/autoexport.

// ErrNoHandler is returned when an envelope arrives for a slot no init()
// registered. Indicates a misrouted dispatch on the host side.
var ErrNoHandler = errors.New("fn: no handler registered for event")

// RunHook decodes a HookEnvelope from input and invokes the registered
// handler for the envelope's Event. Returns the handler's output bytes
// (may be nil for void hooks).
func RunHook(input []byte) ([]byte, error) {
	var env HookEnvelope
	if err := json.Unmarshal(input, &env); err != nil {
		return nil, fmt.Errorf("fn: decode HookEnvelope: %w", err)
	}
	ctx := Context{Ctx: context.Background(), OrgId: env.OrgId, Source: env.Source}
	switch env.Event {
	case "before_create":
		if beforeCreate == nil {
			return nil, fmt.Errorf("%w: beforeCreate", ErrNoHandler)
		}
		return beforeCreate(ctx, env.Record)
	case "before_update":
		if beforeUpdate == nil {
			return nil, fmt.Errorf("%w: beforeUpdate", ErrNoHandler)
		}
		return beforeUpdate(ctx, env.Record, env.CurrentRecord)
	case "before_delete":
		if beforeDelete == nil {
			return nil, fmt.Errorf("%w: beforeDelete", ErrNoHandler)
		}
		return beforeDelete(ctx, env.Record)
	case "after_create":
		if afterCreate == nil {
			return nil, fmt.Errorf("%w: afterCreate", ErrNoHandler)
		}
		return afterCreate(ctx, env.Record)
	case "after_update":
		if afterUpdate == nil {
			return nil, fmt.Errorf("%w: afterUpdate", ErrNoHandler)
		}
		return afterUpdate(ctx, env.Record, env.PreviousRecord)
	case "after_delete":
		if afterDelete == nil {
			return nil, fmt.Errorf("%w: afterDelete", ErrNoHandler)
		}
		return afterDelete(ctx, env.Record)
	default:
		return nil, fmt.Errorf("fn: unknown hook event %q", env.Event)
	}
}

// RunAction decodes an ActionEnvelope and invokes the (single) registered
// action handler. The handler decides what to do with RecordId — entity
// actions use it; global actions ignore it.
func RunAction(input []byte) ([]byte, error) {
	var env ActionEnvelope
	if err := json.Unmarshal(input, &env); err != nil {
		return nil, fmt.Errorf("fn: decode ActionEnvelope: %w", err)
	}
	if action == nil {
		return nil, fmt.Errorf("%w: action", ErrNoHandler)
	}
	ctx := Context{Ctx: context.Background(), OrgId: env.OrgId, Source: env.Source, Headers: env.Headers}
	return action(ctx, env.RecordId, env.Parameters)
}

// ----------------------------------------------------------------------
// Connector methods (the wasm behind one custom-connector method).

// ConnectionInfo is the RESOLVED connection context the host
// (function-service, fed by connector-service) injects into a
// connector-method invocation: usable token material and settings — no
// refresh tokens, no OAuth app secrets, structurally.
type ConnectionInfo struct {
	Id                string         `json:"id"`
	ConnectorKey      string         `json:"connector_key"`
	Scope             string         `json:"scope"`
	ExternalAccountId string         `json:"external_account_id,omitempty"`
	Settings          map[string]any `json:"settings,omitempty"`
	AccessToken       string         `json:"access_token,omitempty"`
	TokenExpiresAt    string         `json:"token_expires_at,omitempty"`
}

// ConnectorMethodEnvelope — sent by the host into the guest through the
// dedicated runConnectorMethod export. Mirrors function-service's
// models.ConnectorMethodEnvelope (same drift contract as the other
// envelopes in this file).
type ConnectorMethodEnvelope struct {
	ConnectorKey string          `json:"connector_key"`
	Method       string          `json:"method"`
	Parameters   json.RawMessage `json:"parameters"`
	OrgId        string          `json:"org_id"`
	Source       Source          `json:"source"`
	Connection   ConnectionInfo  `json:"connection"`
}

type ConnectorMethodFn func(ctx Context, connection ConnectionInfo, parameters json.RawMessage) ([]byte, error)

var (
	connectorMethod     ConnectorMethodFn
	connectorMethodName string
)

// RegisterConnectorMethod fills the single connector-method slot (one wasm
// per method — the module build compiles each method directory separately).
// The name is retained as a deploy-integrity check: an envelope whose method
// doesn't match the registered name means a mis-deployed binary (a method
// row pointing at the wrong file) and fails precisely instead of running the
// wrong handler.
func RegisterConnectorMethod(name string, fn ConnectorMethodFn) {
	if connectorMethod != nil {
		panic("fn: connector-method handler already registered")
	}
	if name == "" {
		panic("fn: connector-method name must not be empty")
	}
	connectorMethod = fn
	connectorMethodName = name
}

// RunConnectorMethod decodes a ConnectorMethodEnvelope and invokes the
// registered method handler after the name integrity check.
func RunConnectorMethod(input []byte) ([]byte, error) {
	var env ConnectorMethodEnvelope
	if err := json.Unmarshal(input, &env); err != nil {
		return nil, fmt.Errorf("fn: decode ConnectorMethodEnvelope: %w", err)
	}
	if connectorMethod == nil {
		return nil, fmt.Errorf("%w: connectorMethod", ErrNoHandler)
	}
	if env.Method != connectorMethodName {
		return nil, fmt.Errorf("fn: this binary implements connector method %q, got dispatch for %q (mis-deployed wasm?)",
			connectorMethodName, env.Method)
	}
	ctx := Context{Ctx: context.Background(), OrgId: env.OrgId, Source: env.Source}
	return connectorMethod(ctx, env.Connection, env.Parameters)
}
