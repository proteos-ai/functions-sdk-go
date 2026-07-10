package fn

import (
	"encoding/json"
	"testing"

	dataapi "go.proteos.ai/model/data/api"
)

func TestQuery_Execute_RawShape(t *testing.T) {
	resetTransport()
	var seen struct {
		SQL string `json:"sql"`
	}
	transport.queryExecute = func(in []byte) ([]byte, error) {
		_ = json.Unmarshal(in, &seen)
		return okEnvelope(t, dataapi.QueryExecuteResponse{
			Data: []dataapi.QueryRow{
				{"id": "a", "n": float64(1)},
				{"id": "b", "n": float64(2)},
			},
			Meta: &dataapi.QueryExecuteMeta{Columns: []string{"id", "n"}, Items: 2},
		}), nil
	}

	resp, err := Query.Execute(Context{}, "SELECT id, n FROM invoices")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if seen.SQL != "SELECT id, n FROM invoices" {
		t.Errorf("sql = %q", seen.SQL)
	}
	if len(resp.Data) != 2 || resp.Meta == nil || resp.Meta.Items != 2 {
		t.Errorf("resp = %+v", resp)
	}
}

func TestQueryRecords_TypedDecode(t *testing.T) {
	resetTransport()
	type row struct {
		Id string `json:"id"`
		N  int    `json:"n"`
	}
	transport.queryExecute = func([]byte) ([]byte, error) {
		return okEnvelope(t, dataapi.QueryExecuteResponse{
			Data: []dataapi.QueryRow{
				{"id": "a", "n": float64(1)},
				{"id": "b", "n": float64(2)},
			},
			Meta: &dataapi.QueryExecuteMeta{Items: 2},
		}), nil
	}

	rows, meta, err := QueryRecords[row](Context{}, "SELECT …")
	if err != nil {
		t.Fatalf("QueryRecords: %v", err)
	}
	if len(rows) != 2 || rows[0].Id != "a" || rows[1].N != 2 {
		t.Errorf("rows = %+v", rows)
	}
	if meta == nil || meta.Items != 2 {
		t.Errorf("meta = %+v", meta)
	}
}
