package dispatch

// ResetForTest clears every registered handler slot. Test-only — guest
// wasms register exactly once per process lifetime, so this is never
// needed outside `go test`.
func ResetForTest() {
	beforeCreate = nil
	beforeUpdate = nil
	beforeDelete = nil
	afterCreate = nil
	afterUpdate = nil
	afterDelete = nil
	action = nil
}
