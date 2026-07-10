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
