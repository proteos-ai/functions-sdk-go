package fn

import (
	"encoding/json"

	dataapi "go.proteos.ai/model/data/api"
)

// Query is the raw-SQL surface backed by data-service's /query/execute
// endpoint. The server rewrites bare attribute references into JSONB
// accessors and authorizes per-table — same auth the SDK enforces.
var Query = queryAPI{}

type queryAPI struct{}

// Execute mirrors sdk.QueryService.Execute exactly — raw SQL in, typed
// rows + meta out.
func (queryAPI) Execute(_ Context, sql string) (dataapi.QueryExecuteResponse, error) {
	req, err := json.Marshal(struct {
		SQL string `json:"sql"`
	}{sql})
	if err != nil {
		return dataapi.QueryExecuteResponse{}, err
	}
	raw, err := callDecode(transport.queryExecute, req)
	if err != nil {
		return dataapi.QueryExecuteResponse{}, err
	}
	var out dataapi.QueryExecuteResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return dataapi.QueryExecuteResponse{}, err
	}
	return out, nil
}

// QueryRecords runs the SQL and decodes each row into the typed T via
// JSON round-trip. Meta (columns, items, executionTimeMs) is preserved
// on the returned Many[T].Meta — note this is dataapi.QueryExecuteMeta,
// not common.ResponseMeta, so QueryRecords returns the rows + meta as a
// pair rather than reusing the list-page Many[T] shape.
func QueryRecords[T any](ctx Context, sql string) ([]T, *dataapi.QueryExecuteMeta, error) {
	resp, err := Query.Execute(ctx, sql)
	if err != nil {
		return nil, nil, err
	}
	out := make([]T, len(resp.Data))
	for i, row := range resp.Data {
		b, err := json.Marshal(row)
		if err != nil {
			return nil, nil, err
		}
		if err := json.Unmarshal(b, &out[i]); err != nil {
			return nil, nil, err
		}
	}
	return out, resp.Meta, nil
}
