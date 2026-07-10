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
