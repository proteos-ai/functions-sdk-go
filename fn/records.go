package fn

import (
	"encoding/json"

	datamodel "go.proteos.ai/model/data"
	sdkdata "go.proteos.ai/sdk/data"
)

// Records is the entry point for record CRUD against data-service. The
// host-side bridge in LUM-51 will translate each call into the matching
// sdk/data.RecordService method.
var Records = recordsAPI{}

type recordsAPI struct{}

// Get returns the record as data-service speaks it (a free-form map).
// Typed callers should use GetRecord[T] instead.
func (recordsAPI) Get(_ Context, entity, id string) (datamodel.Record, error) {
	req, err := json.Marshal(struct {
		Entity string `json:"entity"`
		Id     string `json:"id"`
	}{entity, id})
	if err != nil {
		return nil, err
	}
	raw, err := callDecode(transport.recordsGet, req)
	if err != nil {
		return nil, err
	}
	var rec datamodel.Record
	if err := json.Unmarshal(raw, &rec); err != nil {
		return nil, err
	}
	return rec, nil
}

// Create posts a new record to data-service. The author hands in a
// data.Record (= map[string]any); typed authors use CreateRecord[T].
func (recordsAPI) Create(_ Context, entity string, rec datamodel.Record) (datamodel.Record, error) {
	req, err := json.Marshal(struct {
		Entity string           `json:"entity"`
		Record datamodel.Record `json:"record"`
	}{entity, rec})
	if err != nil {
		return nil, err
	}
	raw, err := callDecode(transport.recordsCreate, req)
	if err != nil {
		return nil, err
	}
	var out datamodel.Record
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Update patches a record. Authors usually go through UpdateRecord[T].
func (recordsAPI) Update(_ Context, entity, id string, rec datamodel.Record) (datamodel.Record, error) {
	req, err := json.Marshal(struct {
		Entity string           `json:"entity"`
		Id     string           `json:"id"`
		Record datamodel.Record `json:"record"`
	}{entity, id, rec})
	if err != nil {
		return nil, err
	}
	raw, err := callDecode(transport.recordsUpdate, req)
	if err != nil {
		return nil, err
	}
	var out datamodel.Record
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Delete removes a record by id.
func (recordsAPI) Delete(_ Context, entity, id string) error {
	req, err := json.Marshal(struct {
		Entity string `json:"entity"`
		Id     string `json:"id"`
	}{entity, id})
	if err != nil {
		return err
	}
	_, err = callDecode(transport.recordsDelete, req)
	return err
}

// List returns a page of records as data-service speaks them. The opts
// arg is the same ListRecordsOptions the SDK takes — page / pageSize /
// sort / filters (bracket-syntax operator keys). Pass nil for defaults.
func (recordsAPI) List(_ Context, entity string, opts *sdkdata.ListRecordsOptions) (Many[datamodel.Record], error) {
	req, err := json.Marshal(struct {
		Entity  string                      `json:"entity"`
		Options *sdkdata.ListRecordsOptions `json:"options,omitempty"`
	}{entity, opts})
	if err != nil {
		return Many[datamodel.Record]{}, err
	}
	raw, err := callDecode(transport.recordsList, req)
	if err != nil {
		return Many[datamodel.Record]{}, err
	}
	var out Many[datamodel.Record]
	if err := json.Unmarshal(raw, &out); err != nil {
		return Many[datamodel.Record]{}, err
	}
	return out, nil
}

// GetRecord decodes the record into the caller's typed T (e.g.
// domain.Invoice). Round-trips the map through JSON.
func GetRecord[T any](ctx Context, entity, id string) (T, error) {
	var zero T
	rec, err := Records.Get(ctx, entity, id)
	if err != nil {
		return zero, err
	}
	return convertRecord[T](rec)
}

// CreateRecord marshals the typed T into a data.Record, sends it, and
// decodes the persisted record back into T.
func CreateRecord[T any](ctx Context, entity string, in T) (T, error) {
	var zero T
	rec, err := toRecord(in)
	if err != nil {
		return zero, err
	}
	out, err := Records.Create(ctx, entity, rec)
	if err != nil {
		return zero, err
	}
	return convertRecord[T](out)
}

// UpdateRecord is the typed sibling of Records.Update.
func UpdateRecord[T any](ctx Context, entity, id string, in T) (T, error) {
	var zero T
	rec, err := toRecord(in)
	if err != nil {
		return zero, err
	}
	out, err := Records.Update(ctx, entity, id, rec)
	if err != nil {
		return zero, err
	}
	return convertRecord[T](out)
}

// DeleteRecord removes a record by id. It is the free-function sibling of
// Records.Delete, completing the GetRecord/CreateRecord/UpdateRecord/ListRecords
// set — there is no type parameter because delete has no payload or response
// body to decode.
func DeleteRecord(ctx Context, entity, id string) error {
	return Records.Delete(ctx, entity, id)
}

// ListRecords decodes each row of the list response into T and returns
// the page as Many[T]. The Meta is preserved verbatim.
func ListRecords[T any](ctx Context, entity string, opts *sdkdata.ListRecordsOptions) (Many[T], error) {
	many, err := Records.List(ctx, entity, opts)
	if err != nil {
		return Many[T]{}, err
	}
	typed := make([]T, len(many.Data))
	for i, rec := range many.Data {
		t, err := convertRecord[T](rec)
		if err != nil {
			return Many[T]{}, err
		}
		typed[i] = t
	}
	return Many[T]{Meta: many.Meta, Data: typed}, nil
}

func convertRecord[T any](rec datamodel.Record) (T, error) {
	var zero T
	b, err := json.Marshal(rec)
	if err != nil {
		return zero, err
	}
	var out T
	if err := json.Unmarshal(b, &out); err != nil {
		return zero, err
	}
	return out, nil
}

func toRecord[T any](in T) (datamodel.Record, error) {
	b, err := json.Marshal(in)
	if err != nil {
		return nil, err
	}
	var rec datamodel.Record
	if err := json.Unmarshal(b, &rec); err != nil {
		return nil, err
	}
	return rec, nil
}
