package fn_test

import (
	"encoding/json"
	"errors"
	"testing"

	"go.proteos.ai/functions-sdk-go/fn"
	"go.proteos.ai/functions-sdk-go/internal/dispatch"
)

// ----------------------------------------------------------------------
// Typed payload used by the register helpers under test.

type invoice struct {
	Amount   int    `json:"amount"`
	Customer string `json:"customer"`
}

// ----------------------------------------------------------------------
// Register helpers — full round-trip from envelope JSON into the typed
// handler and back through json.Marshal.

func TestOnBeforeCreate_RoundTrip(t *testing.T) {
	dispatch.ResetForTest()

	var seen invoice
	fn.OnBeforeCreate[invoice](func(_ fn.Context, inv invoice) (invoice, error) {
		seen = inv
		inv.Amount = inv.Amount * 2
		return inv, nil
	})

	out, err := dispatch.RunHook([]byte(`{"event":"before_create","entity":"invoice","record":{"amount":50,"customer":"acme"},"org_id":"o","source":{"id":"u","type":"user"}}`))
	if err != nil {
		t.Fatalf("RunHook: %v", err)
	}
	if seen.Amount != 50 || seen.Customer != "acme" {
		t.Errorf("seen = %+v", seen)
	}
	var got invoice
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("decode out: %v", err)
	}
	if got.Amount != 100 {
		t.Errorf("out.Amount = %d, want 100", got.Amount)
	}
}

func TestOnBeforeUpdate_RoundTrip(t *testing.T) {
	dispatch.ResetForTest()

	var seenRec, seenCur invoice
	fn.OnBeforeUpdate[invoice](func(_ fn.Context, rec, cur invoice) (invoice, error) {
		seenRec, seenCur = rec, cur
		return rec, nil
	})

	_, err := dispatch.RunHook([]byte(`{"event":"before_update","entity":"invoice","record":{"amount":99,"customer":"acme"},"current_record":{"amount":50,"customer":"acme"},"org_id":"o","source":{"id":"u","type":"user"}}`))
	if err != nil {
		t.Fatalf("RunHook: %v", err)
	}
	if seenRec.Amount != 99 {
		t.Errorf("seenRec.Amount = %d, want 99", seenRec.Amount)
	}
	if seenCur.Amount != 50 {
		t.Errorf("seenCur.Amount = %d, want 50", seenCur.Amount)
	}
}

func TestOnAfterUpdate_PreviousRecordNotDropped(t *testing.T) {
	dispatch.ResetForTest()

	var seenRec, seenPrev invoice
	fn.OnAfterUpdate[invoice](func(_ fn.Context, rec, prev invoice) error {
		seenRec, seenPrev = rec, prev
		return nil
	})

	_, err := dispatch.RunHook([]byte(`{"event":"after_update","entity":"invoice","record":{"amount":99,"customer":"acme"},"previous_record":{"amount":50,"customer":"acme"},"org_id":"o","source":{"id":"u","type":"user"}}`))
	if err != nil {
		t.Fatalf("RunHook: %v", err)
	}
	if seenRec.Amount != 99 {
		t.Errorf("seenRec.Amount = %d, want 99", seenRec.Amount)
	}
	if seenPrev.Amount != 50 {
		t.Errorf("seenPrev.Amount = %d, want 50", seenPrev.Amount)
	}
}

func TestOnBeforeDelete_NoReturnPayload(t *testing.T) {
	dispatch.ResetForTest()
	called := false
	fn.OnBeforeDelete[invoice](func(_ fn.Context, inv invoice) error {
		called = true
		if inv.Amount != 50 {
			t.Errorf("inv.Amount = %d", inv.Amount)
		}
		return nil
	})
	out, err := dispatch.RunHook([]byte(`{"event":"before_delete","entity":"invoice","record":{"amount":50}}`))
	if err != nil {
		t.Fatalf("RunHook: %v", err)
	}
	if !called {
		t.Fatal("handler never invoked")
	}
	if out != nil {
		t.Errorf("out = %s, want nil", out)
	}
}

func TestOnBeforeCreate_AuthorErrorPropagatesAsUserError(t *testing.T) {
	dispatch.ResetForTest()
	fn.OnBeforeCreate[invoice](func(_ fn.Context, inv invoice) (invoice, error) {
		if inv.Amount <= 0 {
			return inv, fn.UserError("amount must be > 0")
		}
		return inv, nil
	})

	_, err := dispatch.RunHook([]byte(`{"event":"before_create","entity":"invoice","record":{"amount":0}}`))
	if err == nil {
		t.Fatal("expected error from handler")
	}
	if !fn.IsUserError(err) {
		t.Errorf("IsUserError(%v) = false; want true", err)
	}
	if err.Error() != "amount must be > 0" {
		t.Errorf("err.Error() = %q", err.Error())
	}
}

// ----------------------------------------------------------------------
// Actions.

type sendInvoiceParams struct {
	RecipientEmail string `json:"recipient_email"`
}

type sendInvoiceResult struct {
	MessageId string `json:"message_id"`
}

func TestRegisterAction_EntityScoped(t *testing.T) {
	dispatch.ResetForTest()

	var seenRecId string
	var seenParams sendInvoiceParams
	fn.RegisterAction[sendInvoiceParams, sendInvoiceResult](func(_ fn.Context, recId string, p sendInvoiceParams) (sendInvoiceResult, error) {
		seenRecId = recId
		seenParams = p
		return sendInvoiceResult{MessageId: "msg-1"}, nil
	})

	out, err := dispatch.RunAction([]byte(`{"entity":"invoice","record_id":"r-42","action":"send-invoice","parameters":{"recipient_email":"a@b.com"}}`))
	if err != nil {
		t.Fatalf("RunAction: %v", err)
	}
	if seenRecId != "r-42" {
		t.Errorf("recordId = %q", seenRecId)
	}
	if seenParams.RecipientEmail != "a@b.com" {
		t.Errorf("params.RecipientEmail = %q", seenParams.RecipientEmail)
	}
	var got sendInvoiceResult
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("decode out: %v", err)
	}
	if got.MessageId != "msg-1" {
		t.Errorf("messageId = %q", got.MessageId)
	}
}

func TestRegisterBatchAction_RecordIdsReachHandler(t *testing.T) {
	dispatch.ResetForTest()

	var seenIds []string
	var seenParams sendInvoiceParams
	fn.RegisterBatchAction[sendInvoiceParams, sendInvoiceResult](func(_ fn.Context, ids []string, p sendInvoiceParams) (sendInvoiceResult, error) {
		seenIds = ids
		seenParams = p
		return sendInvoiceResult{MessageId: "msg-batch"}, nil
	})

	out, err := dispatch.RunAction([]byte(`{"entity":"invoice","record_ids":["r-1","r-2"],"action":"send-invoices","parameters":{"recipient_email":"a@b.com"}}`))
	if err != nil {
		t.Fatalf("RunAction: %v", err)
	}
	if len(seenIds) != 2 || seenIds[0] != "r-1" || seenIds[1] != "r-2" {
		t.Errorf("recordIds = %v", seenIds)
	}
	if seenParams.RecipientEmail != "a@b.com" {
		t.Errorf("params.RecipientEmail = %q", seenParams.RecipientEmail)
	}
	var got sendInvoiceResult
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("decode out: %v", err)
	}
	if got.MessageId != "msg-batch" {
		t.Errorf("messageId = %q", got.MessageId)
	}
}

func TestRegisterGlobalAction_NoRecordIdReachesHandler(t *testing.T) {
	dispatch.ResetForTest()

	called := false
	fn.RegisterGlobalAction[sendInvoiceParams, sendInvoiceResult](func(_ fn.Context, _ sendInvoiceParams) (sendInvoiceResult, error) {
		called = true
		return sendInvoiceResult{MessageId: "msg-g"}, nil
	})

	out, err := dispatch.RunAction([]byte(`{"action":"rebuild-index","parameters":{"recipientEmail":"ignored"}}`))
	if err != nil {
		t.Fatalf("RunAction: %v", err)
	}
	if !called {
		t.Fatal("global handler not invoked")
	}
	var got sendInvoiceResult
	_ = json.Unmarshal(out, &got)
	if got.MessageId != "msg-g" {
		t.Errorf("messageId = %q", got.MessageId)
	}
}

func TestRegisterAction_AndGlobal_ShareSlotAndPanic(t *testing.T) {
	dispatch.ResetForTest()
	fn.RegisterAction[sendInvoiceParams, sendInvoiceResult](func(_ fn.Context, _ string, _ sendInvoiceParams) (sendInvoiceResult, error) {
		return sendInvoiceResult{}, nil
	})
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic when registering global action after entity action")
		}
	}()
	fn.RegisterGlobalAction[sendInvoiceParams, sendInvoiceResult](func(_ fn.Context, _ sendInvoiceParams) (sendInvoiceResult, error) {
		return sendInvoiceResult{}, nil
	})
}

// ----------------------------------------------------------------------
// UserError API.

func TestUserError_IsUserError(t *testing.T) {
	err := fn.UserError("bad input")
	if !fn.IsUserError(err) {
		t.Error("IsUserError(UserError) = false")
	}
	wrapped := errors.New("plain")
	if fn.IsUserError(wrapped) {
		t.Error("IsUserError(plain) = true")
	}
}

func TestUserErrorf_FormatsMessage(t *testing.T) {
	err := fn.UserErrorf("amount %d below %d", 0, 1)
	if err.Error() != "amount 0 below 1" {
		t.Errorf("err.Error() = %q", err.Error())
	}
}

// ----------------------------------------------------------------------
// Declared-field jurisdiction — end to end through dispatch.RunHook.
//
// A compiled hook only knows the attributes its type declares. Anything else
// the record carries — an attribute added to the entity after the hook was
// built, or one contributed by an entity-extension from another module — must
// reach the host untouched instead of being cleared by the round-trip.

func TestOnBeforeCreate_UndeclaredAttributesSurvive(t *testing.T) {
	dispatch.ResetForTest()

	fn.OnBeforeCreate[invoice](func(_ fn.Context, inv invoice) (invoice, error) {
		inv.Amount = inv.Amount * 2
		return inv, nil
	})

	out, err := dispatch.RunHook([]byte(`{"event":"before_create","entity":"invoice","record":{"amount":50,"customer":"acme","plus_score":7,"legacy_note":"keep me","address":{"city":"Berlin"}},"org_id":"o","source":{"id":"u","type":"user"}}`))
	if err != nil {
		t.Fatalf("RunHook: %v", err)
	}

	want := `{"address":{"city":"Berlin"},"amount":100,"customer":"acme","legacy_note":"keep me","plus_score":7}`
	if string(out) != want {
		t.Errorf("got  %s\nwant %s", out, want)
	}
}

func TestOnBeforeUpdate_UndeclaredAttributesSurvive(t *testing.T) {
	dispatch.ResetForTest()

	fn.OnBeforeUpdate[invoice](func(_ fn.Context, inv invoice, _ invoice) (invoice, error) {
		inv.Customer = "globex"
		return inv, nil
	})

	out, err := dispatch.RunHook([]byte(`{"event":"before_update","entity":"invoice","record":{"amount":50,"customer":"acme","plus_score":7},"current_record":{"amount":10,"customer":"acme","plus_score":1},"org_id":"o","source":{"id":"u","type":"user"}}`))
	if err != nil {
		t.Fatalf("RunHook: %v", err)
	}

	want := `{"amount":50,"customer":"globex","plus_score":7}`
	if string(out) != want {
		t.Errorf("got  %s\nwant %s", out, want)
	}
}

// TestOnBeforeUpdate_CurrentRecordIsNotEchoed: currentRecord is read-only
// input. Only the record being written carries attributes forward — an
// attribute that exists on the stored row but was dropped from the write must
// not be resurrected by the hook.
func TestOnBeforeUpdate_CurrentRecordIsNotEchoed(t *testing.T) {
	dispatch.ResetForTest()

	fn.OnBeforeUpdate[invoice](func(_ fn.Context, inv invoice, _ invoice) (invoice, error) {
		return inv, nil
	})

	out, err := dispatch.RunHook([]byte(`{"event":"before_update","entity":"invoice","record":{"amount":50,"customer":"acme"},"current_record":{"amount":10,"customer":"acme","stored_only":"do not resurrect"},"org_id":"o","source":{"id":"u","type":"user"}}`))
	if err != nil {
		t.Fatalf("RunHook: %v", err)
	}

	want := `{"amount":50,"customer":"acme"}`
	if string(out) != want {
		t.Errorf("got  %s\nwant %s", out, want)
	}
}

// TestOnBeforeCreate_DeclaredFieldStillClearable: the declared set comes from
// the TYPE, not from which keys the handler's output happens to contain, so a
// handler can still clear a field it owns. This is the property that keeps the
// change invisible to hook authors.
func TestOnBeforeCreate_DeclaredFieldStillClearable(t *testing.T) {
	dispatch.ResetForTest()

	type optionalInvoice struct {
		Amount   int     `json:"amount"`
		Customer *string `json:"customer,omitempty"`
	}

	fn.OnBeforeCreate[optionalInvoice](func(_ fn.Context, inv optionalInvoice) (optionalInvoice, error) {
		inv.Customer = nil // clear a declared attribute
		return inv, nil
	})

	out, err := dispatch.RunHook([]byte(`{"event":"before_create","entity":"invoice","record":{"amount":50,"customer":"acme","plus_score":7},"org_id":"o","source":{"id":"u","type":"user"}}`))
	if err != nil {
		t.Fatalf("RunHook: %v", err)
	}

	want := `{"amount":50,"plus_score":7}`
	if string(out) != want {
		t.Errorf("got  %s\nwant %s", out, want)
	}
}

// TestOnBeforeCreate_UntypedHandlerUnchanged: a map-typed handler already sees
// and returns every attribute, so jurisdiction does not apply and its output
// passes through exactly as the handler built it.
func TestOnBeforeCreate_UntypedHandlerUnchanged(t *testing.T) {
	dispatch.ResetForTest()

	fn.OnBeforeCreate[map[string]any](func(_ fn.Context, rec map[string]any) (map[string]any, error) {
		delete(rec, "plus_score") // the handler CAN drop what it can see
		rec["added"] = "by hook"
		return rec, nil
	})

	out, err := dispatch.RunHook([]byte(`{"event":"before_create","entity":"invoice","record":{"amount":50,"plus_score":7},"org_id":"o","source":{"id":"u","type":"user"}}`))
	if err != nil {
		t.Fatalf("RunHook: %v", err)
	}

	want := `{"added":"by hook","amount":50}`
	if string(out) != want {
		t.Errorf("got  %s\nwant %s", out, want)
	}
}

// TestOnBeforeCreate_UndeclaredPrecisionPreserved: a carried value must reach
// the host byte-identical. Decoding an int64 id into float64 and back would
// corrupt it even though no hook ever touched the attribute.
func TestOnBeforeCreate_UndeclaredPrecisionPreserved(t *testing.T) {
	dispatch.ResetForTest()

	fn.OnBeforeCreate[invoice](func(_ fn.Context, inv invoice) (invoice, error) {
		return inv, nil
	})

	out, err := dispatch.RunHook([]byte(`{"event":"before_create","entity":"invoice","record":{"amount":1,"customer":"acme","external_id":12345678901234567890,"price":"10.010000000000000001"},"org_id":"o","source":{"id":"u","type":"user"}}`))
	if err != nil {
		t.Fatalf("RunHook: %v", err)
	}

	want := `{"amount":1,"customer":"acme","external_id":12345678901234567890,"price":"10.010000000000000001"}`
	if string(out) != want {
		t.Errorf("got  %s\nwant %s", out, want)
	}
}

// TestOnBeforeCreate_HandlerErrorSkipsPreservation: an aborting hook returns no
// payload at all — preservation must not manufacture one that looks like a
// successful write.
func TestOnBeforeCreate_HandlerErrorSkipsPreservation(t *testing.T) {
	dispatch.ResetForTest()

	fn.OnBeforeCreate[invoice](func(_ fn.Context, _ invoice) (invoice, error) {
		return invoice{}, errors.New("rejected")
	})

	out, err := dispatch.RunHook([]byte(`{"event":"before_create","entity":"invoice","record":{"amount":50,"plus_score":7},"org_id":"o","source":{"id":"u","type":"user"}}`))
	if err == nil {
		t.Fatal("expected the handler error to propagate")
	}
	if out != nil {
		t.Errorf("out = %s, want nil", out)
	}
}

// TestOnBeforeCreate_TaggedFieldWinsSameDepthContest: when two embedded structs
// claim the same JSON name at the same depth, encoding/json lets a tagged field
// beat an untagged one — so the handler really does own that attribute. It must
// be able to clear it, and the stale input value must not be echoed back over
// the clear.
func TestOnBeforeCreate_TaggedFieldWinsSameDepthContest(t *testing.T) {
	dispatch.ResetForTest()

	fn.OnBeforeCreate[contestedRecord](func(_ fn.Context, rec contestedRecord) (contestedRecord, error) {
		rec.optionalTone.Tone = nil // clear the attribute the tagged field owns
		return rec, nil
	})

	out, err := dispatch.RunHook([]byte(`{"event":"before_create","entity":"invoice","record":{"Tone":"formal","amount":50,"plus_score":7},"org_id":"o","source":{"id":"u","type":"user"}}`))
	if err != nil {
		t.Fatalf("RunHook: %v", err)
	}

	want := `{"amount":50,"plus_score":7}`
	if string(out) != want {
		t.Errorf("got  %s\nwant %s", out, want)
	}
}

type optionalTone struct {
	Tone *string `json:"Tone,omitempty"`
}

type plainTone struct {
	Tone string
}

type contestedRecord struct {
	optionalTone
	plainTone
	Amount int `json:"amount"`
}
