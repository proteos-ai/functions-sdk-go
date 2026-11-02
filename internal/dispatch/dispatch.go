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
	Event  string `json:"event"`
	Entity string `json:"entity"`
	// Hook is the slug of the hook row being dispatched. A function bundle
	// serves many hooks from one wasm and routes by it; standalone guests
	// (one handler per slot) ignore it. Hosts predating bundles never send
	// it.
	Hook           string          `json:"hook,omitempty"`
	Record         json.RawMessage `json:"record"`
	CurrentRecord  json.RawMessage `json:"current_record,omitempty"`
	PreviousRecord json.RawMessage `json:"previous_record,omitempty"`
	OrgId          string          `json:"org_id"`
	Source         Source          `json:"source"`
}

// ActionEnvelope — sent by the host into the guest for any action invocation.
// Entity + RecordId are empty for global actions; RecordIds carries the whole
// set for an `entity_batch` action (one dispatch per invocation, never one per
// record) and is empty everywhere else. Headers carries the inbound
// HTTP request headers (flattened) for HTTP-originated dispatch — e.g. a
// webhook receiver reading a token header; empty otherwise.
type ActionEnvelope struct {
	Entity     string            `json:"entity,omitempty"`
	RecordId   string            `json:"record_id,omitempty"`
	RecordIds  []string          `json:"record_ids,omitempty"`
	Action     string            `json:"action"`
	Parameters json.RawMessage   `json:"parameters"`
	OrgId      string            `json:"org_id"`
	Source     Source            `json:"source"`
	Headers    map[string]string `json:"headers,omitempty"`
}

// ----------------------------------------------------------------------
// Registry. Three explicit fn types so the register call site for each
// event reads with the same field names that the wire envelope uses, and
// so the compiler catches slot mismatches.
//
// Two shapes of guest share this table:
//
//   - a standalone guest carries exactly one handler per slot (one hook,
//     one action or one connector method per wasm) and registers into the
//     unkeyed slots;
//   - a function bundle carries every function of a module in one wasm.
//     The generated bundle main runs each function's registration under
//     WithSlug, so its handlers land in the slug-keyed maps and the host's
//     per-row key (hook slug / action slug / action-row slug of a connector
//     method) selects them at dispatch.
//
// Dispatch prefers a keyed handler when the envelope names one and falls
// back to the unkeyed slot, so an old standalone binary keeps working on a
// host that sends keys, and a new standalone binary keeps working on a host
// that does not.

type SingleRecordHookFn func(ctx Context, record json.RawMessage) ([]byte, error)
type BeforeUpdateHookFn func(ctx Context, record, currentRecord json.RawMessage) ([]byte, error)
type AfterUpdateHookFn func(ctx Context, record, previousRecord json.RawMessage) ([]byte, error)
type ActionFn func(ctx Context, target ActionTarget, parameters json.RawMessage) ([]byte, error)

// ActionTarget is what an invocation is aimed at: RecordId for an `entity`
// action, RecordIds for an `entity_batch` action, neither for a global one.
// Carried as one struct so adding a future targeting mode doesn't churn every
// handler signature.
type ActionTarget struct {
	RecordId  string
	RecordIds []string
}

// hookHandlers is one function's worth of hook slots.
type hookHandlers struct {
	beforeCreate SingleRecordHookFn
	beforeUpdate BeforeUpdateHookFn
	beforeDelete SingleRecordHookFn
	afterCreate  SingleRecordHookFn
	afterUpdate  AfterUpdateHookFn
	afterDelete  SingleRecordHookFn
}

var (
	// bundleSlug is the function slug registrations are keyed under while a
	// bundle's generated main runs one function's registration (see
	// WithSlug). Empty outside that window — a standalone guest registers
	// unkeyed.
	bundleSlug string

	standaloneHooks hookHandlers
	bundledHooks    = map[string]*hookHandlers{}

	standaloneAction ActionFn
	bundledActions   = map[string]ActionFn{}
)

// WithSlug runs register with every registration it performs keyed under
// slug. Init-time only (not goroutine-safe); the generated bundle main is
// its only caller, through runtime/bundle.
func WithSlug(slug string, register func()) {
	if slug == "" {
		panic("fn: bundle function slug must not be empty")
	}
	if bundleSlug != "" {
		panic("fn: nested bundle registration for " + slug + " inside " + bundleSlug)
	}
	bundleSlug = slug
	defer func() { bundleSlug = "" }()
	register()
}

// hooksForRegistration is where the current registration lands: the keyed
// entry while a bundle function registers, the unkeyed slots otherwise.
func hooksForRegistration() *hookHandlers {
	if bundleSlug == "" {
		return &standaloneHooks
	}
	handlers, ok := bundledHooks[bundleSlug]
	if !ok {
		handlers = &hookHandlers{}
		bundledHooks[bundleSlug] = handlers
	}
	return handlers
}

// hooksForDispatch resolves an envelope's hook key: the keyed entry when
// one exists, else the unkeyed slots (standalone guest, or a host that
// sends no key).
func hooksForDispatch(hookSlug string) *hookHandlers {
	if hookSlug != "" {
		if handlers, ok := bundledHooks[hookSlug]; ok {
			return handlers
		}
	}
	return &standaloneHooks
}

// registrationLabel names the bundle function in panic messages so a
// double registration inside a bundle points at the offending function.
func registrationLabel() string {
	if bundleSlug == "" {
		return ""
	}
	return fmt.Sprintf(" (bundle function %q)", bundleSlug)
}

// dispatchLabel names the envelope key in ErrNoHandler wraps.
func dispatchLabel(key string) string {
	if key == "" {
		return ""
	}
	return fmt.Sprintf(" (%q)", key)
}

// ----------------------------------------------------------------------
// Register* setters. Each panics on double-register with a slot-named
// message — a guest carries exactly one handler per slot per function, so
// re-registration is always a programming mistake.

func RegisterBeforeCreate(fn SingleRecordHookFn) {
	handlers := hooksForRegistration()
	if handlers.beforeCreate != nil {
		panic("fn: OnBeforeCreate handler already registered" + registrationLabel())
	}
	handlers.beforeCreate = fn
}

func RegisterBeforeUpdate(fn BeforeUpdateHookFn) {
	handlers := hooksForRegistration()
	if handlers.beforeUpdate != nil {
		panic("fn: OnBeforeUpdate handler already registered" + registrationLabel())
	}
	handlers.beforeUpdate = fn
}

func RegisterBeforeDelete(fn SingleRecordHookFn) {
	handlers := hooksForRegistration()
	if handlers.beforeDelete != nil {
		panic("fn: OnBeforeDelete handler already registered" + registrationLabel())
	}
	handlers.beforeDelete = fn
}

func RegisterAfterCreate(fn SingleRecordHookFn) {
	handlers := hooksForRegistration()
	if handlers.afterCreate != nil {
		panic("fn: OnAfterCreate handler already registered" + registrationLabel())
	}
	handlers.afterCreate = fn
}

func RegisterAfterUpdate(fn AfterUpdateHookFn) {
	handlers := hooksForRegistration()
	if handlers.afterUpdate != nil {
		panic("fn: OnAfterUpdate handler already registered" + registrationLabel())
	}
	handlers.afterUpdate = fn
}

func RegisterAfterDelete(fn SingleRecordHookFn) {
	handlers := hooksForRegistration()
	if handlers.afterDelete != nil {
		panic("fn: OnAfterDelete handler already registered" + registrationLabel())
	}
	handlers.afterDelete = fn
}

func RegisterAction(fn ActionFn) {
	if bundleSlug != "" {
		if _, exists := bundledActions[bundleSlug]; exists {
			panic("fn: action handler already registered" + registrationLabel())
		}
		bundledActions[bundleSlug] = fn
		return
	}
	if standaloneAction != nil {
		panic("fn: action handler already registered")
	}
	standaloneAction = fn
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
	handlers := hooksForDispatch(env.Hook)
	label := dispatchLabel(env.Hook)
	switch env.Event {
	case "before_create":
		if handlers.beforeCreate == nil {
			return nil, fmt.Errorf("%w: beforeCreate%s", ErrNoHandler, label)
		}
		return handlers.beforeCreate(ctx, env.Record)
	case "before_update":
		if handlers.beforeUpdate == nil {
			return nil, fmt.Errorf("%w: beforeUpdate%s", ErrNoHandler, label)
		}
		return handlers.beforeUpdate(ctx, env.Record, env.CurrentRecord)
	case "before_delete":
		if handlers.beforeDelete == nil {
			return nil, fmt.Errorf("%w: beforeDelete%s", ErrNoHandler, label)
		}
		return handlers.beforeDelete(ctx, env.Record)
	case "after_create":
		if handlers.afterCreate == nil {
			return nil, fmt.Errorf("%w: afterCreate%s", ErrNoHandler, label)
		}
		return handlers.afterCreate(ctx, env.Record)
	case "after_update":
		if handlers.afterUpdate == nil {
			return nil, fmt.Errorf("%w: afterUpdate%s", ErrNoHandler, label)
		}
		return handlers.afterUpdate(ctx, env.Record, env.PreviousRecord)
	case "after_delete":
		if handlers.afterDelete == nil {
			return nil, fmt.Errorf("%w: afterDelete%s", ErrNoHandler, label)
		}
		return handlers.afterDelete(ctx, env.Record)
	default:
		return nil, fmt.Errorf("fn: unknown hook event %q", env.Event)
	}
}

// RunAction decodes an ActionEnvelope and invokes the action handler the
// envelope's Action slug selects — keyed in a bundle, the single unkeyed
// handler in a standalone guest. The handler decides what to do with the
// target: entity actions read RecordId, batch actions RecordIds, global
// actions neither.
func RunAction(input []byte) ([]byte, error) {
	var env ActionEnvelope
	if err := json.Unmarshal(input, &env); err != nil {
		return nil, fmt.Errorf("fn: decode ActionEnvelope: %w", err)
	}
	handler := standaloneAction
	if env.Action != "" {
		if bundled, ok := bundledActions[env.Action]; ok {
			handler = bundled
		}
	}
	if handler == nil {
		return nil, fmt.Errorf("%w: action%s", ErrNoHandler, dispatchLabel(env.Action))
	}
	ctx := Context{Ctx: context.Background(), OrgId: env.OrgId, Source: env.Source, Headers: env.Headers}
	return handler(ctx, ActionTarget{RecordId: env.RecordId, RecordIds: env.RecordIds}, env.Parameters)
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
	ConnectorKey string `json:"connector_key"`
	Method       string `json:"method"`
	// Action is the slug of the action row the method is deployed as
	// (`<connector_key>-<method>`). A function bundle routes by it;
	// standalone guests ignore it. Hosts predating bundles never send it.
	Action     string          `json:"action,omitempty"`
	Parameters json.RawMessage `json:"parameters"`
	OrgId      string          `json:"org_id"`
	Source     Source          `json:"source"`
	Connection ConnectionInfo  `json:"connection"`
}

type ConnectorMethodFn func(ctx Context, connection ConnectionInfo, parameters json.RawMessage) ([]byte, error)

// connectorMethodHandler pairs a method handler with the method name it was
// registered under, kept for the deploy-integrity check in
// RunConnectorMethod.
type connectorMethodHandler struct {
	name string
	fn   ConnectorMethodFn
}

var (
	standaloneConnectorMethod connectorMethodHandler
	bundledConnectorMethods   = map[string]connectorMethodHandler{}
)

// RegisterConnectorMethod registers one connector method's handler: the
// single unkeyed slot in a standalone guest (one wasm per method), or the
// entry keyed by the function slug inside a bundle. The name is retained as
// a deploy-integrity check: an envelope whose method doesn't match the
// registered name means a mis-deployed binary (a method row pointing at the
// wrong file) and fails precisely instead of running the wrong handler.
func RegisterConnectorMethod(name string, fn ConnectorMethodFn) {
	if name == "" {
		panic("fn: connector-method name must not be empty")
	}
	handler := connectorMethodHandler{name: name, fn: fn}
	if bundleSlug != "" {
		if _, exists := bundledConnectorMethods[bundleSlug]; exists {
			panic("fn: connector-method handler already registered" + registrationLabel())
		}
		bundledConnectorMethods[bundleSlug] = handler
		return
	}
	if standaloneConnectorMethod.fn != nil {
		panic("fn: connector-method handler already registered")
	}
	standaloneConnectorMethod = handler
}

// RunConnectorMethod decodes a ConnectorMethodEnvelope and invokes the
// method handler the envelope's Action slug selects (keyed in a bundle, the
// unkeyed slot otherwise) after the name integrity check.
func RunConnectorMethod(input []byte) ([]byte, error) {
	var env ConnectorMethodEnvelope
	if err := json.Unmarshal(input, &env); err != nil {
		return nil, fmt.Errorf("fn: decode ConnectorMethodEnvelope: %w", err)
	}
	handler := standaloneConnectorMethod
	if env.Action != "" {
		if bundled, ok := bundledConnectorMethods[env.Action]; ok {
			handler = bundled
		}
	}
	if handler.fn == nil {
		return nil, fmt.Errorf("%w: connectorMethod%s", ErrNoHandler, dispatchLabel(env.Action))
	}
	if env.Method != handler.name {
		return nil, fmt.Errorf("fn: this binary implements connector method %q, got dispatch for %q (mis-deployed wasm?)",
			handler.name, env.Method)
	}
	ctx := Context{Ctx: context.Background(), OrgId: env.OrgId, Source: env.Source}
	return handler.fn(ctx, env.Connection, env.Parameters)
}
