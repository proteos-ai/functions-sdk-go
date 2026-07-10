//go:build wasip1

package fn

import (
	pdk "github.com/extism/go-pdk"
)

//go:wasmimport fn records_get
func extismRecordsGet(uint64) uint64

//go:wasmimport fn records_create
func extismRecordsCreate(uint64) uint64

//go:wasmimport fn records_update
func extismRecordsUpdate(uint64) uint64

//go:wasmimport fn records_delete
func extismRecordsDelete(uint64) uint64

//go:wasmimport fn records_list
func extismRecordsList(uint64) uint64

//go:wasmimport fn query_execute
func extismQueryExecute(uint64) uint64

//go:wasmimport fn cache_get
func extismCacheGet(uint64) uint64

//go:wasmimport fn cache_set
func extismCacheSet(uint64) uint64

//go:wasmimport fn cache_delete
func extismCacheDelete(uint64) uint64

//go:wasmimport fn secrets_read
func extismSecretsRead(uint64) uint64

//go:wasmimport fn storage_generate_download_url
func extismStorageGenerateDownloadUrl(uint64) uint64

//go:wasmimport fn log_debug
func extismLogDebug(uint64) uint64

//go:wasmimport fn log_info
func extismLogInfo(uint64) uint64

//go:wasmimport fn log_warn
func extismLogWarn(uint64) uint64

//go:wasmimport fn log_error
func extismLogError(uint64) uint64

//go:wasmimport fn http_request
func extismHTTPRequest(uint64) uint64

//go:wasmimport fn connections_get_token
func extismConnectionsGetToken(uint64) uint64

//go:wasmimport fn connections_invoke_method
func extismConnectionsInvokeMethod(uint64) uint64

// wasipCall allocates `in` into wasm memory, invokes the host fn with
// the input offset, and reads the response bytes from the offset the
// host returns. Both the input and output memory blocks are freed via
// defer — the host-side impl is responsible for not retaining pointers
// into our memory past the call's return.
func wasipCall(fn func(uint64) uint64, in []byte) ([]byte, error) {
	mem := pdk.AllocateBytes(in)
	defer mem.Free()
	outOff := fn(mem.Offset())
	out := pdk.FindMemory(outOff)
	defer out.Free()
	return out.ReadBytes(), nil
}

func init() {
	transport = transportFns{
		recordsGet:                 func(in []byte) ([]byte, error) { return wasipCall(extismRecordsGet, in) },
		recordsCreate:              func(in []byte) ([]byte, error) { return wasipCall(extismRecordsCreate, in) },
		recordsUpdate:              func(in []byte) ([]byte, error) { return wasipCall(extismRecordsUpdate, in) },
		recordsDelete:              func(in []byte) ([]byte, error) { return wasipCall(extismRecordsDelete, in) },
		recordsList:                func(in []byte) ([]byte, error) { return wasipCall(extismRecordsList, in) },
		queryExecute:               func(in []byte) ([]byte, error) { return wasipCall(extismQueryExecute, in) },
		cacheGet:                   func(in []byte) ([]byte, error) { return wasipCall(extismCacheGet, in) },
		cacheSet:                   func(in []byte) ([]byte, error) { return wasipCall(extismCacheSet, in) },
		cacheDelete:                func(in []byte) ([]byte, error) { return wasipCall(extismCacheDelete, in) },
		secretsRead:                func(in []byte) ([]byte, error) { return wasipCall(extismSecretsRead, in) },
		storageGenerateDownloadUrl: func(in []byte) ([]byte, error) { return wasipCall(extismStorageGenerateDownloadUrl, in) },
		logDebug:                   func(in []byte) ([]byte, error) { return wasipCall(extismLogDebug, in) },
		logInfo:                    func(in []byte) ([]byte, error) { return wasipCall(extismLogInfo, in) },
		logWarn:                    func(in []byte) ([]byte, error) { return wasipCall(extismLogWarn, in) },
		logError:                   func(in []byte) ([]byte, error) { return wasipCall(extismLogError, in) },
		httpRequest:                func(in []byte) ([]byte, error) { return wasipCall(extismHTTPRequest, in) },
		connectionsGetToken:        func(in []byte) ([]byte, error) { return wasipCall(extismConnectionsGetToken, in) },
		connectionsInvokeMethod:    func(in []byte) ([]byte, error) { return wasipCall(extismConnectionsInvokeMethod, in) },
	}
}
