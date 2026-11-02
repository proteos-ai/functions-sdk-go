//go:build wasip1

package autoexport

import (
	pdk "github.com/extism/go-pdk"

	"go.proteos.ai/functions-sdk-go/internal/dispatch"
)

//go:wasmexport runHook
func runHook() int32 {
	out, err := dispatch.RunHook(pdk.Input())
	if err != nil {
		pdk.SetError(err)
		return 1
	}
	pdk.Output(out)
	return 0
}

//go:wasmexport runAction
func runAction() int32 {
	out, err := dispatch.RunAction(pdk.Input())
	if err != nil {
		pdk.SetError(err)
		return 1
	}
	pdk.Output(out)
	return 0
}

//go:wasmexport runConnectorMethod
func runConnectorMethod() int32 {
	out, err := dispatch.RunConnectorMethod(pdk.Input())
	if err != nil {
		pdk.SetError(err)
		return 1
	}
	pdk.Output(out)
	return 0
}

// warm is a no-op entry point. Calling any export runs the guest's
// _initialize first (Go runtime boot plus every package init), so the host
// uses this one to pay that cost right after instantiation — measurable and
// ahead of the first real call — instead of inside it.
//
//go:wasmexport warm
func warm() int32 {
	return 0
}
