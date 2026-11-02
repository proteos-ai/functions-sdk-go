package dispatch

// ResetForTest clears every registered handler, keyed and unkeyed, and any
// in-progress bundle registration. Test-only — guest wasms register exactly
// once per process lifetime, so this is never needed outside `go test`.
func ResetForTest() {
	bundleSlug = ""
	standaloneHooks = hookHandlers{}
	bundledHooks = map[string]*hookHandlers{}
	standaloneAction = nil
	bundledActions = map[string]ActionFn{}
	standaloneConnectorMethod = connectorMethodHandler{}
	bundledConnectorMethods = map[string]connectorMethodHandler{}
}
