package fn

import (
	"encoding/json"
	"reflect"
	"sort"
	"testing"
	"time"
)

// ----------------------------------------------------------------------
// declaredFields — what a type claims jurisdiction over.

type plainRecord struct {
	Amount   int     `json:"amount"`
	Customer string  `json:"customer,omitempty"`
	Note     *string `json:"note,omitempty"`
	Untagged bool
	Skipped  string `json:"-"`
	Literal  string `json:"-,"`
	hidden   string //nolint:unused // exercises the unexported skip
}

type embedded struct {
	Shared string `json:"shared"`
	Inner  string `json:"inner"`
}

type outerPromoting struct {
	embedded
	Own string `json:"own"`
}

type outerTagged struct {
	Embedded embedded `json:"embedded"`
	Own      string   `json:"own"`
}

// The embedded struct carries a json tag name, so encoding/json treats it as
// an ordinary named field instead of promoting its fields.
type outerNamedEmbed struct {
	embedded `json:"nested"`
	Own      string `json:"own"`
}

type outerPointerEmbed struct {
	*embedded
	Own string `json:"own"`
}

// Shadowing: the outer field is shallower, so it wins and stays declared.
type outerShadowing struct {
	embedded
	Shared string `json:"shared"`
}

// The colliding field is untagged on both branches: it still resolves to the
// same json name at the same depth, which is the condition under test, but
// keeps `go vet`'s structtag check from flagging a deliberate duplicate tag.
type leftBranch struct {
	Collide string
	OnlyL   string `json:"only_l"`
}

type rightBranch struct {
	Collide string
	OnlyR   string `json:"only_r"`
}

// Both branches emit "Collide" at the same depth — encoding/json drops it.
type outerAmbiguous struct {
	leftBranch
	rightBranch
}

type taggedBranch struct {
	Dominant string `json:"Dominant"`
	OnlyT    string `json:"only_t"`
}

type untaggedBranch struct {
	Dominant string
	OnlyU    string `json:"only_u"`
}

// Same depth, same emitted name, but only one side is tagged — encoding/json
// lets the tagged field win instead of dropping the name.
type outerTaggedWins struct {
	taggedBranch
	untaggedBranch
}

type deepTaggedBranch struct {
	taggedBranch
}

// Depth outranks tagging: the shallow untagged Dominant beats the tagged one
// nested a level deeper.
type outerShallowUntaggedWins struct {
	untaggedBranch
	deepTaggedBranch
}

// Exported so reflect.StructOf can embed them (it rejects unexported fields).
// Both claim "dominant" with a tag, so neither dominates.
type TaggedA struct {
	Dominant string `json:"dominant,omitempty"`
	OnlyA    string `json:"only_a"`
}

type TaggedB struct {
	Dominant string `json:"dominant,omitempty"`
	OnlyB    string `json:"only_b"`
}

type optionalTagged struct {
	Tone *string `json:"Tone,omitempty"`
}

type plainTone struct {
	Tone string
}

// "Tone" is claimed by a tagged omitempty field and an untagged one at the same
// depth — the tagged one wins, so clearing it must stick.
type outerOptionalTagged struct {
	optionalTagged
	plainTone
}

// An unusable tag name is discarded: encoding/json falls back to the Go field
// name and stops treating the field as tagged.
type badTagRecord struct {
	Amount int    `json:"a\"b"`
	Name   string `json:"name"`
}

type selfEmbedding struct {
	*selfEmbedding
	Name string `json:"name"`
}

// Unexported non-struct embed — encoding/json ignores it outright.
type embeddedValue string

type outerValueEmbed struct {
	embeddedValue
	Own string `json:"own"`
}

// Exported non-struct embed — encoding/json names it after its type.
type ExportedValue string

type outerExportedValueEmbed struct {
	ExportedValue
	Own string `json:"own"`
}

func TestDeclaredFields(t *testing.T) {
	tests := []struct {
		name string
		got  map[string]bool
		want []string
	}{
		{
			name: "tags win, untagged falls back to field name, json:- skipped",
			got:  declaredFields[plainRecord](),
			want: []string{"-", "Untagged", "amount", "customer", "note"},
		},
		{
			name: "pointer T resolves to the same set as the value type",
			got:  declaredFields[*plainRecord](),
			want: []string{"-", "Untagged", "amount", "customer", "note"},
		},
		{
			name: "untagged embedded struct is promoted",
			got:  declaredFields[outerPromoting](),
			want: []string{"inner", "own", "shared"},
		},
		{
			name: "named struct field is not promoted",
			got:  declaredFields[outerTagged](),
			want: []string{"embedded", "own"},
		},
		{
			name: "tagged embed is a named field, not a promotion",
			got:  declaredFields[outerNamedEmbed](),
			want: []string{"nested", "own"},
		},
		{
			name: "embedded pointer-to-struct is promoted",
			got:  declaredFields[outerPointerEmbed](),
			want: []string{"inner", "own", "shared"},
		},
		{
			name: "shallower field shadows the promoted one and stays declared",
			got:  declaredFields[outerShadowing](),
			want: []string{"inner", "shared"},
		},
		{
			name: "name ambiguous at equal depth is dropped, siblings survive",
			got:  declaredFields[outerAmbiguous](),
			want: []string{"only_l", "only_r"},
		},
		{
			name: "a tagged field beats an untagged one at the same depth",
			got:  declaredFields[outerTaggedWins](),
			want: []string{"Dominant", "only_t", "only_u"},
		},
		{
			name: "depth outranks tagging",
			got:  declaredFields[outerShallowUntaggedWins](),
			want: []string{"Dominant", "only_t", "only_u"},
		},
		{
			name: "unusable tag name falls back to the Go field name",
			got:  declaredFields[badTagRecord](),
			want: []string{"Amount", "name"},
		},
		{
			name: "self-embedding type terminates",
			got:  declaredFields[selfEmbedding](),
			want: []string{"name"},
		},
		{
			name: "unexported non-struct embed is ignored",
			got:  declaredFields[outerValueEmbed](),
			want: []string{"own"},
		},
		{
			name: "exported non-struct embed is named after its type",
			got:  declaredFields[outerExportedValueEmbed](),
			want: []string{"ExportedValue", "own"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if diff := keys(test.got); !reflect.DeepEqual(diff, test.want) {
				t.Errorf("declaredFields = %v, want %v", diff, test.want)
			}
		})
	}
}

// TestDeclaredFields_NonStructIsNil: a handler typed on anything without field
// structure already round-trips the whole record, so there is no jurisdiction
// to compute and preserveUndeclared must stay out of the way.
func TestDeclaredFields_NonStructIsNil(t *testing.T) {
	if got := declaredFields[map[string]any](); got != nil {
		t.Errorf("map[string]any = %v, want nil", got)
	}
	if got := declaredFields[json.RawMessage](); got != nil {
		t.Errorf("json.RawMessage = %v, want nil", got)
	}
	if got := declaredFields[[]string](); got != nil {
		t.Errorf("[]string = %v, want nil", got)
	}
	if got := declaredFields[string](); got != nil {
		t.Errorf("string = %v, want nil", got)
	}
	if got := declaredFields[any](); got != nil {
		t.Errorf("any = %v, want nil", got)
	}
}

// TestDeclaredFields_MatchesEncodingJSON is the drift guard: for types where
// every field is emitted unconditionally, the declared set must equal the keys
// encoding/json actually produces. If Go's promotion rules shift under us, or
// collectDeclared drifts from them, this fails.
func TestDeclaredFields_MatchesEncodingJSON(t *testing.T) {
	tests := []struct {
		name     string
		value    any
		declared map[string]bool
	}{
		{"promoting", outerPromoting{}, declaredFields[outerPromoting]()},
		{"tagged", outerTagged{}, declaredFields[outerTagged]()},
		{"named embed", outerNamedEmbed{}, declaredFields[outerNamedEmbed]()},
		{"pointer embed", outerPointerEmbed{embedded: &embedded{}}, declaredFields[outerPointerEmbed]()},
		{"shadowing", outerShadowing{}, declaredFields[outerShadowing]()},
		{"ambiguous", outerAmbiguous{}, declaredFields[outerAmbiguous]()},
		{"tagged wins", outerTaggedWins{}, declaredFields[outerTaggedWins]()},
		{"shallow untagged wins", outerShallowUntaggedWins{}, declaredFields[outerShallowUntaggedWins]()},
		{"bad tag", badTagRecord{}, declaredFields[badTagRecord]()},
		{"self embedding", selfEmbedding{}, declaredFields[selfEmbedding]()},
		{"value embed", outerValueEmbed{}, declaredFields[outerValueEmbed]()},
		{"exported value embed", outerExportedValueEmbed{}, declaredFields[outerExportedValueEmbed]()},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			raw, err := json.Marshal(test.value)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			var emitted map[string]json.RawMessage
			if err := json.Unmarshal(raw, &emitted); err != nil {
				t.Fatalf("decode %s: %v", raw, err)
			}
			want := make([]string, 0, len(emitted))
			for key := range emitted {
				want = append(want, key)
			}
			sort.Strings(want)
			if got := keys(test.declared); !reflect.DeepEqual(got, want) {
				t.Errorf("declared = %v, encoding/json emits %v", got, want)
			}
		})
	}
}

// ----------------------------------------------------------------------
// preserveUndeclared — the echo itself.

func TestPreserveUndeclared(t *testing.T) {
	declared := map[string]bool{"amount": true, "customer": true}

	tests := []struct {
		name     string
		input    string
		typedOut string
		declared map[string]bool
		want     string
	}{
		{
			name:     "attributes outside the type survive the round-trip",
			input:    `{"amount":50,"customer":"acme","plus_score":7,"legacy_note":"keep me"}`,
			typedOut: `{"amount":100,"customer":"acme"}`,
			declared: declared,
			want:     `{"amount":100,"customer":"acme","legacy_note":"keep me","plus_score":7}`,
		},
		{
			name:     "a declared field the handler dropped stays cleared",
			input:    `{"amount":50,"customer":"acme","plus_score":7}`,
			typedOut: `{"amount":100}`,
			declared: declared,
			want:     `{"amount":100,"plus_score":7}`,
		},
		{
			name:     "handler output wins over the input for declared keys",
			input:    `{"amount":50,"customer":"acme"}`,
			typedOut: `{"amount":100,"customer":"globex"}`,
			declared: declared,
			want:     `{"amount":100,"customer":"globex"}`,
		},
		{
			name:     "handler output wins even for an undeclared key it emitted",
			input:    `{"amount":50,"extra":"from input"}`,
			typedOut: `{"amount":100,"extra":"from handler"}`,
			declared: declared,
			want:     `{"amount":100,"extra":"from handler"}`,
		},
		{
			name:     "undeclared null is carried, not swallowed",
			input:    `{"amount":50,"plus_note":null}`,
			typedOut: `{"amount":100}`,
			declared: declared,
			want:     `{"amount":100,"plus_note":null}`,
		},
		{
			name:     "undeclared nested object and array are carried whole",
			input:    `{"amount":50,"address":{"city":"Berlin"},"tags":["a","b"]}`,
			typedOut: `{"amount":100}`,
			declared: declared,
			want:     `{"address":{"city":"Berlin"},"amount":100,"tags":["a","b"]}`,
		},
		{
			name:     "nothing undeclared to carry",
			input:    `{"amount":50}`,
			typedOut: `{"amount":100,"customer":""}`,
			declared: declared,
			want:     `{"amount":100,"customer":""}`,
		},
		{
			name:     "nil declared set (non-struct T) returns the output verbatim",
			input:    `{"amount":50,"plus_score":7}`,
			typedOut: `{"amount":100}`,
			declared: nil,
			want:     `{"amount":100}`,
		},
		{
			name:     "non-object input has nothing keyed to echo",
			input:    `[1,2,3]`,
			typedOut: `{"amount":100}`,
			declared: declared,
			want:     `{"amount":100}`,
		},
		{
			name:     "json null input is not an object",
			input:    `null`,
			typedOut: `{"amount":100}`,
			declared: declared,
			want:     `{"amount":100}`,
		},
		{
			name:     "self-marshaling T produces a non-object output, left alone",
			input:    `{"amount":50,"plus_score":7}`,
			typedOut: `"2026-09-16T00:00:00Z"`,
			declared: declared,
			want:     `"2026-09-16T00:00:00Z"`,
		},
		{
			name:     "empty output is left alone",
			input:    `{"amount":50}`,
			typedOut: ``,
			declared: declared,
			want:     ``,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := preserveUndeclared(json.RawMessage(test.input), []byte(test.typedOut), test.declared)
			if err != nil {
				t.Fatalf("preserveUndeclared: %v", err)
			}
			if string(got) != test.want {
				t.Errorf("got  %s\nwant %s", got, test.want)
			}
		})
	}
}

// TestPreserveUndeclared_PrecisionIsByteIdentical: carried values must never be
// laundered through float64. An int64 id or a decimal amount that a hook never
// touched has to reach the host with the digits it arrived with.
func TestPreserveUndeclared_PrecisionIsByteIdentical(t *testing.T) {
	input := json.RawMessage(`{"amount":1,"external_id":12345678901234567890,"price":"10.010000000000000001","ratio":1.0}`)

	got, err := preserveUndeclared(input, []byte(`{"amount":2}`), map[string]bool{"amount": true})
	if err != nil {
		t.Fatalf("preserveUndeclared: %v", err)
	}

	want := `{"amount":2,"external_id":12345678901234567890,"price":"10.010000000000000001","ratio":1.0}`
	if string(got) != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
}

// TestPreserveUndeclared_OutputIsDeterministic: Go sorts map keys on marshal,
// so the same input must always yield byte-identical output — hosts and tests
// can compare bytes.
func TestPreserveUndeclared_OutputIsDeterministic(t *testing.T) {
	input := json.RawMessage(`{"a":1,"z":2,"m":3,"b":4,"y":5}`)
	declared := map[string]bool{"a": true}

	first, err := preserveUndeclared(input, []byte(`{"a":9}`), declared)
	if err != nil {
		t.Fatalf("preserveUndeclared: %v", err)
	}
	for range 20 {
		next, err := preserveUndeclared(input, []byte(`{"a":9}`), declared)
		if err != nil {
			t.Fatalf("preserveUndeclared: %v", err)
		}
		if string(next) != string(first) {
			t.Fatalf("unstable output: %s vs %s", next, first)
		}
	}
	if want := `{"a":9,"b":4,"m":3,"y":5,"z":2}`; string(first) != want {
		t.Errorf("got %s, want %s", first, want)
	}
}

// TestPreserveUndeclared_TimeValuedAttributeIsDeclared guards the everyday
// codegen shape: a time.Time attribute is a named field, so the handler owns
// it — it must not be echoed back from the input after the handler cleared it.
func TestPreserveUndeclared_TimeValuedAttributeIsDeclared(t *testing.T) {
	type record struct {
		SeenAt *time.Time `json:"seen_at,omitempty"`
		Name   string     `json:"name"`
	}
	declared := declaredFields[record]()
	if !declared["seen_at"] {
		t.Fatalf("seen_at must be declared, got %v", keys(declared))
	}

	got, err := preserveUndeclared(
		json.RawMessage(`{"name":"a","seen_at":"2026-09-16T00:00:00Z","plus_note":"keep"}`),
		[]byte(`{"name":"a"}`),
		declared,
	)
	if err != nil {
		t.Fatalf("preserveUndeclared: %v", err)
	}
	if want := `{"name":"a","plus_note":"keep"}`; string(got) != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

// TestDeclaredFields_BothTaggedAtSameDepthIsAmbiguous covers the other half of
// the dominance rule: when neither field outranks the other, encoding/json
// drops the name and so must we.
//
// The shape is built with reflect.StructOf because a source-level struct
// embedding two branches that both carry `json:"dominant"` trips `go vet`'s
// structtag check — the very duplicate this test is about.
func TestDeclaredFields_BothTaggedAtSameDepthIsAmbiguous(t *testing.T) {
	ambiguous := reflect.StructOf([]reflect.StructField{
		{Name: "TaggedA", Type: reflect.TypeOf(TaggedA{}), Anonymous: true},
		{Name: "TaggedB", Type: reflect.TypeOf(TaggedB{}), Anonymous: true},
	})

	got := keys(declaredFieldsOf(ambiguous))
	if want := []string{"only_a", "only_b"}; !reflect.DeepEqual(got, want) {
		t.Errorf("declaredFieldsOf = %v, want %v (dominant must be contested)", got, want)
	}

	// Same drift guard as the source-level fixtures: whatever encoding/json
	// actually emits is the answer.
	raw, err := json.Marshal(reflect.New(ambiguous).Elem().Interface())
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if want := `{"only_a":"","only_b":""}`; string(raw) != want {
		t.Errorf("encoding/json emits %s, want %s", raw, want)
	}
}

// TestPreserveUndeclared_TaggedWinnerStaysCleared is the regression this
// dominance rule exists for. "Tone" is claimed by a tagged omitempty field and
// an untagged one at the same depth; the tagged field wins, so the handler owns
// the name. Clearing it must leave it cleared — treating the name as contested
// would echo the stale input value straight back over the handler's intent.
func TestPreserveUndeclared_TaggedWinnerStaysCleared(t *testing.T) {
	declared := declaredFields[outerOptionalTagged]()
	if !declared["Tone"] {
		t.Fatalf("Tone must be declared, got %v", keys(declared))
	}

	record := outerOptionalTagged{}
	if err := json.Unmarshal([]byte(`{"Tone":"formal","plus_note":"keep"}`), &record); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	// Spelled out because Go itself treats the selector as ambiguous at equal
	// depth — only encoding/json applies the tag-wins rule.
	if record.optionalTagged.Tone == nil || *record.optionalTagged.Tone != "formal" {
		t.Fatalf("Tone did not decode into the tagged field: %+v", record)
	}

	record.optionalTagged.Tone = nil // the handler clears the attribute it owns
	typedOut, err := json.Marshal(record)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	got, err := preserveUndeclared(json.RawMessage(`{"Tone":"formal","plus_note":"keep"}`), typedOut, declared)
	if err != nil {
		t.Fatalf("preserveUndeclared: %v", err)
	}
	if want := `{"plus_note":"keep"}`; string(got) != want {
		t.Errorf("got %s, want %s (Tone was restored over the handler's clear)", got, want)
	}
}

func keys(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for key := range set {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}
