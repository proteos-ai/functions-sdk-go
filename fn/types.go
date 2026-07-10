package fn

import "go.proteos.ai/model/common"

// Many is the {meta, data} envelope every list / query host fn returns.
// Mirrors sdk.ListResult[T] field-for-field so the host-side bridge in
// LUM-51 can pass the SDK's ListPage result straight through. Unifying
// Many[T] with sdk.ListResult[T] is a follow-up — would require moving
// ListResult into model/common since the SDK package pulls in net/http
// and is not importable in wasip1 guests.
type Many[T any] struct {
	Meta common.ResponseMeta `json:"meta"`
	Data []T                 `json:"data"`
}
