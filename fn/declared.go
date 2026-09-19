package fn

import (
	"encoding/json"
	"reflect"
	"strings"
	"unicode"
)

// Declared fields — the jurisdiction rule for record-returning hooks.
//
// A typed before-hook handler takes T and returns T, and the host treats the
// bytes it returns as the WHOLE record. Left alone, that means any attribute T
// doesn't declare — one added to the entity after the hook was compiled, or one
// contributed by an entity-extension the hook's module never saw — round-trips
// through json.Unmarshal/json.Marshal into nothing and is silently cleared on
// write.
//
// So T's json tags define its jurisdiction: the attributes the handler is
// allowed to write. Everything outside that set is echoed back from the input
// record verbatim. The author's mental model ("my hook edits what it edits")
// becomes true, and no author-visible behavior changes — clearing a declared
// field still works, because the declared set comes from the TYPE, not from
// which keys happen to be present in the output.
//
// The set is derived once per registration (T is known at Register time), never
// per dispatch.

// declaredFields returns the top-level JSON object keys type T governs.
//
// A nil result means "T has no field structure to reason about" — a
// map[string]any handler, or any non-struct T. Those round-trip everything
// already, so preserveUndeclared leaves their output untouched.
func declaredFields[T any]() map[string]bool {
	return declaredFieldsOf(reflect.TypeOf((*T)(nil)).Elem())
}

// declaredFieldsOf is declaredFields over a reflect.Type — the form tests use
// to build struct shapes that can't be spelled in source (see the ambiguous
// fixtures in declared_test.go).
func declaredFieldsOf(t reflect.Type) map[string]bool {
	t = deref(t)
	if t.Kind() != reflect.Struct {
		return nil
	}

	claims := map[string]fieldClaim{}
	collectDeclared(t, 0, claims, map[reflect.Type]bool{})

	declared := make(map[string]bool, len(claims))
	for name, claim := range claims {
		// A contested name is dropped by encoding/json in both directions —
		// the handler can neither read it nor write it — so it is NOT
		// declared, and the input value is echoed rather than lost.
		if claim.contested {
			continue
		}
		declared[name] = true
	}
	return declared
}

// fieldClaim is the best claim on one JSON name found so far, and whether an
// equally strong claim was found alongside it.
//
// encoding/json resolves same-name fields by embed depth first and an explicit
// json tag second (see dominantField): the shallowest wins, a tagged field
// beats an untagged one at the same depth, and only a tie on BOTH is ambiguous
// enough to drop the name entirely.
type fieldClaim struct {
	depth     int
	isTagged  bool
	contested bool
}

// beats reports whether this claim outranks other outright.
func (claim fieldClaim) beats(other fieldClaim) bool {
	if claim.depth != other.depth {
		return claim.depth < other.depth
	}
	return claim.isTagged && !other.isTagged
}

// ties reports whether this claim is exactly as strong as other, which is what
// makes a name ambiguous.
func (claim fieldClaim) ties(other fieldClaim) bool {
	return claim.depth == other.depth && claim.isTagged == other.isTagged
}

// collectDeclared walks t's fields the way encoding/json's typeFields does —
// promoting untagged anonymous struct fields, honoring `json:"-"`, skipping
// unexported — recording the strongest claim on each emitted name.
//
// A T that marshals itself (embeds time.Time, defines its own MarshalJSON)
// needs no special case here: its output isn't a JSON object at all, and
// preserveUndeclared short-circuits on that.
func collectDeclared(t reflect.Type, depth int, claims map[string]fieldClaim, visiting map[reflect.Type]bool) {
	// Self-embedding types (type Node struct{ *Node }) would recurse forever.
	if visiting[t] {
		return
	}
	visiting[t] = true
	defer delete(visiting, t)

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		tag := field.Tag.Get("json")

		// `json:"-"` means omitted entirely. `json:"-,"` means a field
		// literally named "-", so only the bare form skips.
		if tag == "-" {
			continue
		}
		name, _, _ := strings.Cut(tag, ",")
		// An unusable tag name is discarded outright: encoding/json falls back
		// to the Go field name AND stops treating the field as tagged, which
		// changes which field wins a same-name contest.
		if !isValidTag(name) {
			name = ""
		}
		isTagged := name != ""

		// The exported check runs differently for anonymous fields:
		// encoding/json keeps embedded structs of UNEXPORTED types (they may
		// carry exported fields) and drops only unexported non-struct embeds.
		if field.Anonymous {
			if !isExported(field) && deref(field.Type).Kind() != reflect.Struct {
				continue
			}
		} else if !isExported(field) {
			continue
		}

		// An anonymous struct field is promoted ONLY when it carries no tag
		// name. With one it is an ordinary named field — including when its
		// type is unexported.
		if field.Anonymous && name == "" && deref(field.Type).Kind() == reflect.Struct {
			collectDeclared(deref(field.Type), depth+1, claims, visiting)
			continue
		}

		if name == "" {
			// For an embed, reflect already reports Name as the type name,
			// which is the key json emits.
			name = field.Name
		}

		claim := fieldClaim{depth: depth, isTagged: isTagged}
		best, seen := claims[name]
		switch {
		case !seen || claim.beats(best):
			// A strictly better claim also clears any earlier ambiguity: the
			// contest it lost is no longer at the top.
			claims[name] = claim
		case claim.ties(best):
			best.contested = true
			claims[name] = best
		}
	}
}

// preserveUndeclared returns the handler's output with every input key outside
// the handler's jurisdiction carried through untouched.
//
// declared == nil (non-struct T) short-circuits: those handlers already see and
// return the whole record. Non-object input or output short-circuits too —
// there is nothing keyed to carry over.
//
// Values are carried as json.RawMessage so they reach the host byte-identical:
// an int64 id or a high-precision decimal must not be laundered through float64
// just because a hook ran.
func preserveUndeclared(input json.RawMessage, typedOut []byte, declared map[string]bool) ([]byte, error) {
	if declared == nil || len(typedOut) == 0 {
		return typedOut, nil
	}

	var in map[string]json.RawMessage
	if err := json.Unmarshal(input, &in); err != nil || in == nil {
		return typedOut, nil
	}
	var out map[string]json.RawMessage
	if err := json.Unmarshal(typedOut, &out); err != nil || out == nil {
		return typedOut, nil
	}

	for key, value := range in {
		if declared[key] {
			continue
		}
		// A custom MarshalJSON on T may already have emitted the key; the
		// handler's output always wins over the echo.
		if _, taken := out[key]; taken {
			continue
		}
		out[key] = value
	}
	return json.Marshal(out)
}

// isValidTag mirrors encoding/json's rule for a usable tag name: non-empty,
// and made only of letters, digits, and the punctuation json permits.
func isValidTag(name string) bool {
	if name == "" {
		return false
	}
	for _, char := range name {
		switch {
		case strings.ContainsRune("!#$%&()*+-./:;<=>?@[]^_{|}~ ", char):
			// Backslash and quote are reserved; other punctuation is allowed.
		case !unicode.IsLetter(char) && !unicode.IsDigit(char):
			return false
		}
	}
	return true
}

// deref unwraps pointer types down to the pointed-at type.
func deref(t reflect.Type) reflect.Type {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t
}

// isExported reports whether json will emit this field. PkgPath is empty for
// exported fields on every reflect implementation, TinyGo's included.
func isExported(field reflect.StructField) bool {
	return field.PkgPath == ""
}
