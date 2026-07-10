//go:build wasip1

// Package main is the wasip1-only smoke target used to verify that the
// SDK compiles end-to-end with the Extism toolchain and that every
// //go:wasmimport / //go:wasmexport symbol the host needs to provide
// ends up in the binary.
//
// Build:
//
//	GOOS=wasip1 GOARCH=wasm go build -buildmode=c-shared \
//	    -o /tmp/smoke.wasm \
//	    ./examples/smoke-hook
package main

import (
	_ "go.proteos.ai/functions-sdk-go/runtime/autoexport"

	"go.proteos.ai/functions-sdk-go/fn"
)

type Invoice struct {
	Id         string `json:"id"`
	Amount     int    `json:"amount"`
	CustomerId string `json:"customer_id"`
}

type Customer struct {
	Id     string `json:"id"`
	Status string `json:"status"`
}

func init() {
	fn.OnBeforeCreate[Invoice](validateInvoice)
}

func validateInvoice(ctx fn.Context, inv Invoice) (Invoice, error) {
	if inv.Amount <= 0 {
		return inv, fn.UserError("amount must be > 0")
	}

	customer, err := fn.GetRecord[Customer](ctx, "customer", inv.CustomerId)
	if err != nil {
		return inv, err
	}
	if customer.Status == "blocked" {
		return inv, fn.UserErrorf("cannot invoice blocked customer %s", customer.Id)
	}

	fn.Log.Info(ctx, "invoice validated", map[string]any{
		"invoice_id":  inv.Id,
		"customer_id": customer.Id,
	})

	return inv, nil
}

func main() {}
