package fn

import (
	"encoding/json"
	"errors"
	"testing"

	datamodel "go.proteos.ai/model/data"
	sdkdata "go.proteos.ai/sdk/data"
)

type invoice struct {
	Id     string `json:"id"`
	Amount int    `json:"amount"`
}

// okEnvelope wraps an arbitrary marshalable payload in the host
// success envelope shape.
func okEnvelope(t *testing.T, v any) []byte {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal data: %v", err)
	}
	out, err := json.Marshal(envelope{Ok: true, Data: data})
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}
	return out
}

// errEnvelope wraps a code+message in the host error envelope shape.
func errEnvelope(t *testing.T, code, message string) []byte {
	t.Helper()
	out, err := json.Marshal(envelope{Ok: false, Err: &envelopeErr{Code: code, Message: message}})
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}
	return out
}

func resetTransport() { transport = transportFns{} }

func TestRecords_Get_TypedHappyPath(t *testing.T) {
	resetTransport()
	var seen struct {
		Entity, Id string
	}
	transport.recordsGet = func(in []byte) ([]byte, error) {
		_ = json.Unmarshal(in, &seen)
		return okEnvelope(t, invoice{Id: "inv-1", Amount: 50}), nil
	}

	got, err := GetRecord[invoice](Context{}, "invoice", "inv-1")
	if err != nil {
		t.Fatalf("GetRecord: %v", err)
	}
	if seen.Entity != "invoice" || seen.Id != "inv-1" {
		t.Errorf("request shape = %+v", seen)
	}
	if got.Amount != 50 {
		t.Errorf("got.Amount = %d", got.Amount)
	}
}

func TestRecords_Get_NotFoundMapsToPhotonSentinel(t *testing.T) {
	resetTransport()
	transport.recordsGet = func([]byte) ([]byte, error) {
		return errEnvelope(t, "not_found", "invoice not found"), nil
	}

	_, err := GetRecord[invoice](Context{}, "invoice", "missing")
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("errors.Is(err, ErrNotFound) = false; err = %v", err)
	}
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("errors.Is(err, host.ErrNotFound) = false; sentinel identity broken")
	}
}

func TestRecords_Create_RoundTrip(t *testing.T) {
	resetTransport()
	var sent datamodel.Record
	transport.recordsCreate = func(in []byte) ([]byte, error) {
		var req struct {
			Entity string           `json:"entity"`
			Record datamodel.Record `json:"record"`
		}
		_ = json.Unmarshal(in, &req)
		sent = req.Record
		return okEnvelope(t, invoice{Id: "inv-new", Amount: 100}), nil
	}

	got, err := CreateRecord[invoice](Context{}, "invoice", invoice{Amount: 100})
	if err != nil {
		t.Fatalf("CreateRecord: %v", err)
	}
	if got.Id != "inv-new" || got.Amount != 100 {
		t.Errorf("got = %+v", got)
	}
	if sent["amount"] == nil {
		t.Errorf("amount missing from sent record: %+v", sent)
	}
}

func TestRecords_Delete_NoReturn(t *testing.T) {
	resetTransport()
	var seen struct{ Entity, Id string }
	transport.recordsDelete = func(in []byte) ([]byte, error) {
		_ = json.Unmarshal(in, &seen)
		return okEnvelope(t, nil), nil
	}

	if err := Records.Delete(Context{}, "invoice", "inv-1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if seen.Entity != "invoice" || seen.Id != "inv-1" {
		t.Errorf("request shape = %+v", seen)
	}
}

func TestRecords_List_DecodesManyT(t *testing.T) {
	resetTransport()
	var seen struct {
		Entity  string                      `json:"entity"`
		Options *sdkdata.ListRecordsOptions `json:"options,omitempty"`
	}
	transport.recordsList = func(in []byte) ([]byte, error) {
		_ = json.Unmarshal(in, &seen)
		// Many[Record] shape — meta + data[]
		page := map[string]any{
			"meta": map[string]any{
				"page": 0, "page_size": 2, "items_total": 2, "pages_total": 1,
			},
			"data": []invoice{{Id: "i-1", Amount: 10}, {Id: "i-2", Amount: 20}},
		}
		return okEnvelope(t, page), nil
	}

	out, err := ListRecords[invoice](Context{}, "invoice", &sdkdata.ListRecordsOptions{
		Page: 0, PageSize: 2, Sort: "createdAt:desc",
	})
	if err != nil {
		t.Fatalf("ListRecords: %v", err)
	}
	if seen.Options == nil || seen.Options.PageSize != 2 || seen.Options.Sort != "createdAt:desc" {
		t.Errorf("options not round-tripped: %+v", seen.Options)
	}
	if len(out.Data) != 2 {
		t.Fatalf("len(Data) = %d", len(out.Data))
	}
	if out.Data[0].Id != "i-1" || out.Data[1].Amount != 20 {
		t.Errorf("Data = %+v", out.Data)
	}
	if out.Meta.ItemsTotal != 2 {
		t.Errorf("Meta.ItemsTotal = %d", out.Meta.ItemsTotal)
	}
}

func TestRecords_BatchUpsert_TypedRoundTrip(t *testing.T) {
	resetTransport()
	var seen struct {
		Entity       string                           `json:"entity"`
		Transactions []sdkdata.BatchUpsertTransaction `json:"transactions"`
	}
	transport.recordsBatchUpsert = func(in []byte) ([]byte, error) {
		_ = json.Unmarshal(in, &seen)
		return okEnvelope(t, sdkdata.BatchUpsertRecordsResponse{
			Results: []sdkdata.BatchUpsertTransactionResult{
				{TransactionID: "0", Status: sdkdata.BatchTransactionSuccess, Record: datamodel.Record{"id": "i-1", "amount": 10}},
				{TransactionID: "1", Status: sdkdata.BatchTransactionError, Error: &sdkdata.BatchTransactionErr{Code: "upsert_error", Message: "boom"}},
			},
		}), nil
	}

	out, err := BatchUpsertRecords(Context{}, "invoice", []invoice{{Amount: 10}, {Id: "i-2", Amount: 20}})
	if err != nil {
		t.Fatalf("BatchUpsertRecords: %v", err)
	}
	if seen.Entity != "invoice" || len(seen.Transactions) != 2 {
		t.Fatalf("request shape = %+v", seen)
	}
	if seen.Transactions[0].TransactionID != "0" || seen.Transactions[1].TransactionID != "1" {
		t.Errorf("transaction ids = %q, %q", seen.Transactions[0].TransactionID, seen.Transactions[1].TransactionID)
	}
	if seen.Transactions[1].Data["id"] != "i-2" {
		t.Errorf("transaction data not round-tripped: %+v", seen.Transactions[1].Data)
	}
	if len(out.Results) != 2 {
		t.Fatalf("len(Results) = %d", len(out.Results))
	}
	if out.Results[0].Status != sdkdata.BatchTransactionSuccess || out.Results[1].Error == nil {
		t.Errorf("Results = %+v", out.Results)
	}
}

func TestRecords_BatchUpsert_ErrorEnvelope(t *testing.T) {
	resetTransport()
	transport.recordsBatchUpsert = func([]byte) ([]byte, error) {
		return errEnvelope(t, "permission_denied", "no access"), nil
	}

	_, err := Records.BatchUpsert(Context{}, "invoice", []sdkdata.BatchUpsertTransaction{{TransactionID: "0", Data: map[string]any{"amount": 1}}})
	if !errors.Is(err, ErrPermissionDenied) {
		t.Errorf("errors.Is(err, ErrPermissionDenied) = false; err = %v", err)
	}
}

func TestRecords_PermissionDeniedMapsToSentinel(t *testing.T) {
	resetTransport()
	transport.recordsGet = func([]byte) ([]byte, error) {
		return errEnvelope(t, "permission_denied", "no access"), nil
	}

	_, err := GetRecord[invoice](Context{}, "invoice", "x")
	if !errors.Is(err, ErrPermissionDenied) {
		t.Errorf("errors.Is(err, ErrPermissionDenied) = false; err = %v", err)
	}
}
