/*
Copyright (c) 2023-2026 Microbus LLC and various contributors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package boolexp

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/microbus-io/testarossa"
)

// Normalize converts arrays and maps to []any / map[string]any, and numbers to int64 or float64 -
// following the JSON LITERAL, not the Go type: float32(1.0) marshals to "1" and comes back an int64. That
// is invisible to an expression - all numbers compare as one type - and is what keeps a 64-bit id exact.
func TestNormalize(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	// No-op: already normalized values pass through unchanged.
	m := map[string]any{"x": "1", "y": "2", "z": "3"}
	out, err := Normalize(m)
	assert.NoError(err)
	assert.Equal(m, out)

	out, err = Normalize(map[string]any{
		"arr_int":    []int{1, 2, 3},
		"arr_string": []string{"x", "y", "z"},
		"arr_float":  []float32{1.0, 2.5, 3.33},
		"map_int":    map[string]int{"a": 1, "b": 2, "c": 3},
		"map_string": map[string]string{"a": "A", "b": "B", "c": "C"},
		"map_float":  map[string]float32{"a": 1.0, "b": 2.5, "c": 3.33},
	})
	assert.NoError(err)
	assert.Equal(map[string]any{
		"arr_int":    []any{int64(1), int64(2), int64(3)},
		"arr_string": []any{"x", "y", "z"},
		"arr_float":  []any{int64(1), 2.5, 3.33},
		"map_int":    map[string]any{"a": int64(1), "b": int64(2), "c": int64(3)},
		"map_string": map[string]any{"a": "A", "b": "B", "c": "C"},
		"map_float":  map[string]any{"a": int64(1), "b": 2.5, "c": 3.33},
	}, out)
}

// Proven by mutation rather than by comparing the return value, since equal-but-copied maps would also
// satisfy assert.Equal.
func TestNormalize_NoOpReturnsSameMap(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	m := map[string]any{"a": int64(1)}
	out, err := Normalize(m)
	assert.NoError(err)
	m["b"] = "added later"
	assert.Equal("added later", out["b"])
}

func TestFlattenSymbolsMap(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	assert.Equal(
		map[string]any{
			"x": "1",
			"y": "2",
			"z": "3",
		},
		flattenSymbolsMap(map[string]any{
			"x": "1",
			"y": "2",
			"z": "3",
		}),
	)

	assert.Equal(
		map[string]any{
			"arr":      []any{"x", "y", "z"},
			"arr.x":    true,
			"arr.y":    true,
			"arr.z":    true,
			"nested":   map[string]any{"a": int64(1), "b": "B"},
			"nested.a": int64(1),
			"nested.b": "B",
		},
		flattenSymbolsMap(map[string]any{
			"arr":    []any{"x", "y", "z"},
			"nested": map[string]any{"a": int64(1), "b": "B"},
		}),
	)
}

func TestArray(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	symbols := map[string]any{
		"str": []string{"a", "b", "c"},
		"int": []int{1, 2, 3},
		"any": []any{"x", "y", 99},
	}
	tcTrue := []string{
		"str.a",
		"int.1",
		"any.x",
		"any.99",
	}
	for _, tc := range tcTrue {
		b, err := Eval(tc, symbols)
		if assert.NoError(err, tc) {
			assert.True(b, tc)
		}
	}
	tcFalse := []string{
		"str.d",
		"int.4",
		"any.xxx",
		"any.999",
	}
	for _, tc := range tcFalse {
		b, err := Eval(tc, symbols)
		if assert.NoError(err, tc) {
			assert.False(b, tc)
		}
	}
}

func TestSyntax(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	symbols := map[string]any{
		"foo":       "bar",
		"num":       5.0,
		"nil":       nil,
		"copyright": "(c)",
		"roles":     []any{"admin", "manager", 100},
		"level":     5,
		"fib":       []int{1, 2, 3, 5, 8},
		"nested":    map[string]any{"a": 1, "b": "one"},
	}

	tcTrue := []string{
		"true",
		"(TRUE)",
		"( ( true )) ",
		"true || false",
		"false || true",
		"true || true",
		"true && true",
		"(true) && ((true))",
		"1==1",
		"1 != 2",
		` "x" == 'x' `,
		` "x" != 'y'`,
		` "x" != ' x'`,
		`"x"`,
		`'x'`,
		"1",
		"1.0",
		"true==true",
		"false==false",
		"!false",
		"!!true",
		"!! !false",
		`!""`,
		`!''`,
		"!nil",
		"nothing==nil",

		"foo!=bar",
		"foo!='baz'",
		"foo=='bar'",
		"foo",
		"copyright=='(c)'",
		"copyright!='(r)'",
		"!foo.nothing",
		"foo.nothing==nil",

		"num==5",
		"num==5.0",
		"num>=5.0",
		"num>4",
		"num!=4",
		"level==5.0",
		"num==level",

		// Simple operators
		"foo=='bar'",
		"foo!='baz'",
		"foo<='bar'",
		"foo<'baz'",
		"foo>='bar'",
		"foo>'bam'",

		// Regular expressions
		"foo=~'bar'",
		"foo=~'(bar|not)'",
		"foo!~'baz'",
		"foo!~'....'",
		"roles.guest || foo=~'b'",

		// Array
		"roles.admin",
		"roles.manager",
		"roles.100",

		// Nested
		"nested.a==1",
		"nested.b==`one`",
		"!nested.c",
	}
	for _, tc := range tcTrue {
		b, err := Eval(tc, symbols)
		if assert.NoError(err, tc) {
			assert.True(b, tc)
		}
	}

	tcFalse := []string{
		"false",
		"(FALSE)",
		"( ( false )) ",
		"false || false",
		"false && true",
		"true && false",
		"false && false",
		"(false) && ((false))",
		"1==2",
		"1 != 1",
		` "x" != 'x' `,
		` "x" == 'y'`,
		` "x" == ' x'`,
		`!"x"`,
		`!'x'`,
		"!1",
		"!1.0",
		"true==false",
		"false==true",
		"!true",
		"!!false",
		"! ! ! true",
		`""`,
		`''`,
		"nil",
		"nothing!=nil",

		"foo==bar",
		"foo=='baz'",
		"foo!='bar'",
		"!foo",
		"copyright!='(c)'",
		"copyright=='(r)'",
		"foo.nothing",
		"foo.nothing!=nil",

		"num!=5",
		"num!=5.0",
		"num<5",
		"num<5.0",
		"num>5",
		"num>5.0",
		"level!=5.0",
		"num!=level",

		// Regular expressions
		"foo!~'bar'",
		"foo!~'(bar|not)'",
		"foo=~'baz'",
		"foo=~'....'",
		"roles.admin && foo=~'x'",

		// Array
		"roles.user",
		"roles.20",

		// Nested
		"nested.a!=1",
		"nested.b!=`one`",
		"nested.c",
	}
	for _, tc := range tcFalse {
		b, err := Eval(tc, symbols)
		if assert.NoError(err, tc) {
			assert.False(b, tc)
		}
	}

	tcErr := []string{
		"(",
		")",
		"(true))",
		"(true && (false) && (true)",
		"aa++bb || false",
		"!",
		"",
		"a & b",
		"a | b",
		"a || b.",
		"valid ==",
		"== ==",
		"a > > b",
		"a =~ '[invalid",
		// Close paren before its matching open: previously infinite-looped the paren-resolution
		// pass (depth nets back to 0 so the end-of-scan guard missed it). Now rejected.
		")(",
		")()(",
		"a)(b",
		"))((",
		"a && )(",
		")( || true",
	}
	for _, tc := range tcErr {
		_, err := Eval(tc, nil)
		assert.Error(err, tc)
	}

	_, err := Eval("true", nil) // Nil symbols
	assert.NoError(err)
}

func TestSymbols(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	// Symbols as a map
	symbols := map[string]any{
		"i": 5,
		"s": "hello",
		"nested": map[string]any{
			"field": "alphanumeric",
		},
	}
	v, err := Eval("(i==5 || i==7) && s=='hello' && nested.field=~'^[a-z0-9]+$'", symbols)
	assert.Expect(err, nil, v, true)

	// Symbols as an object
	type NestedObj struct {
		Field string `json:"field"`
	}
	type Obj struct {
		I      int       `json:"i"`
		S      string    `json:"s"`
		Nested NestedObj `json:"nested"`
	}
	object := Obj{
		I: 5,
		S: "hello",
		Nested: NestedObj{
			Field: "alphanumeric",
		},
	}
	v, err = Eval("(i==5 || i==7) && s=='hello' && nested.field=~'^[a-z0-9]+$'", object)
	assert.Expect(err, nil, v, true)

	// Literal - a scalar can't decode into named symbols, so Eval surfaces Normalize's error.
	l := 9999
	v, err = Eval("i==5 && s=='hello' && obj.field=~'^[a-z0-9]+$'", l)
	assert.Error(err)
	assert.False(v)
}

// FuzzTerminates asserts Validate and Eval always RETURN on arbitrary input — they must never hang.
// It guards termination directly (the property), rather than pre-validating syntax (which would have
// to re-implement the parser's quote-aware paren tracking and could diverge from it). A malformed
// expression must come back as an error, never as an infinite loop: boolexp evaluates author-supplied
// `when` expressions on a caller's hot path, so a hang wedges the calling goroutine permanently.
//
// Regression seed: ")(" (a close paren before its matching open) once span-looped the paren pass
// forever; the parenDepth<0 guard now rejects it. The watchdog turns any future hang class into a
// bounded test failure instead of a hung suite.
func FuzzTerminates(f *testing.F) {
	seeds := []string{
		")(", ")()(", "a)(b", "))((", "a && )(", ")( || true",
		"", "(", ")", "((((((((((", "true", "false",
		`x == "())("`, `a =~ '('`, "(a || b) && (c)", "!!!(true)",
		"a && (b || (c && (d)))", "\t\n)(", "a" + strings.Repeat("(", 64),
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, expr string) {
		symbols := map[string]any{"a": 1, "b": "x", "c": true}
		done := make(chan struct{})
		go func() {
			defer close(done)
			// Return values are irrelevant; the assertion is that these calls return AT ALL.
			_ = Validate(expr)
			_, _ = Eval(expr, symbols)
		}()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatalf("boolexp did not terminate on %q", expr)
		}
	})
}

// Numbers are compared exactly, not through a float64 - which holds integers only up to 2^53, so
// neighboring 64-bit identifiers collapse onto the same value and two DIFFERENT ids compare equal.
// Integers and fractional numbers still compare as one type, so an expression need not know how a
// number happened to be spelled.
func TestNumericPrecision(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	// Two distinct Snowflake-scale ids, 1 apart: the same float64 rounds both to ...768.
	const id = 1234567890123456789
	const neighbor = 1234567890123456788
	assert.Equal(float64(id), float64(neighbor)) // the trap, stated outright

	symbols := map[string]any{"id": int64(id), "other": int64(neighbor)}

	// Symbol vs literal: the literal side is parsed exactly too, or the comparison rounds there instead.
	v, err := Eval("id==1234567890123456789", symbols)
	assert.Expect(err, nil, v, true)
	v, err = Eval("id==1234567890123456788", symbols)
	assert.Expect(err, nil, v, false)

	// Symbol vs symbol.
	v, err = Eval("id==other", symbols)
	assert.Expect(err, nil, v, false)
	v, err = Eval("id>other", symbols)
	assert.Expect(err, nil, v, true)

	// Ordering is exact at full width, not at float64 resolution.
	v, err = Eval("id>1234567890123456788 && id<1234567890123456790", symbols)
	assert.Expect(err, nil, v, true)
}

func TestIsNormalized(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	tcTrue := []any{
		nil,
		true,
		"hello",
		int64(5),
		float64(5.5),
		json.Number("99999999999999999999999999999999"), // beyond even float64's range
		map[string]any{},
		[]any{},
		map[string]any{"a": int64(1), "b": []any{"x", float64(2.5), nil}},
		[]any{map[string]any{"a": int64(1)}, []any{int64(2)}},
	}
	for _, tc := range tcTrue {
		assert.True(isNormalized(tc), "%#v", tc)
	}

	tcFalse := []any{
		5,
		int32(5),
		float32(5),
		uint(5),
		[]int{1, 2},
		map[string]int{"a": 1},
		struct{ A int }{A: 1},
		map[string]any{"a": 5},
		map[string]any{"a": []any{"x", 5}},
		[]any{map[string]any{"a": 5}},
		map[string]any{"a": map[string]any{"b": 5}},
	}
	for _, tc := range tcFalse {
		assert.False(isNormalized(tc), "%#v", tc)
	}
}

// The fast and slow paths must produce identical results.
func TestNormalize_FastPathMatchesSlowPath(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	normalized := map[string]any{
		"str":    "hello",
		"num":    int64(5),
		"frac":   2.5,
		"nested": map[string]any{"a": int64(1), "b": "B"},
		"arr":    []any{"x", int64(2)},
	}
	raw := map[string]any{
		"str":    "hello",
		"num":    5,
		"frac":   2.5,
		"nested": map[string]int{"a": 1, "b": 2}, // "b" differs from normalized's, proving this took its own round trip
		"arr":    []any{"x", 2},
	}

	assert.True(isNormalized(normalized))
	assert.False(isNormalized(raw))

	fast, err := Normalize(normalized)
	assert.NoError(err)
	slow, err := Normalize(raw)
	assert.NoError(err)

	assert.Equal(normalized, fast) // no-op

	assert.Equal(map[string]any{
		"str":    "hello",
		"num":    int64(5),
		"frac":   2.5,
		"nested": map[string]any{"a": int64(1), "b": int64(2)},
		"arr":    []any{"x", int64(2)},
	}, slow)
}

// The documented usage pattern: Normalize once, then Eval repeatedly against the result.
func TestEval_PreDecodedMapReused(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	const id = 1234567890123456789
	type obj struct {
		ID     int64    `json:"id"`
		Name   string   `json:"name"`
		Active bool     `json:"active"`
		Tags   []string `json:"tags"`
	}
	decoded, err := Normalize(obj{ID: id, Name: "widget", Active: true, Tags: []string{"a", "b"}})
	assert.NoError(err)
	assert.True(isNormalized(decoded))

	for _, tc := range []struct {
		expr string
		want bool
	}{
		{"id==1234567890123456789", true},
		{"id==1234567890123456788", false},
		{"name=='widget'", true},
		{"active", true},
		{"tags.a", true},
		{"tags.c", false},
	} {
		v, err := Eval(tc.expr, decoded)
		assert.NoError(err, tc.expr)
		assert.Equal(tc.want, v, tc.expr)
	}
}

// An integer symbol and a fractional literal (and vice versa) are one number domain: a comparison must
// not gate on whether the value arrived as an int64 or a float64.
func TestMixedIntAndFloatComparison(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	symbols := map[string]any{
		"qty":   3,    // int -> int64
		"score": 88.5, // float64
		"zero":  0,    // int64 zero, for the existence/empty path
		"ratio": 2.0,  // marshals to "2" -> int64: JSON has one number type
	}

	for _, exp := range []string{
		"qty==3", "qty>2.5", "qty<3.5", "qty>=3.0", "qty<=3",
		"score>88", "score<89", "score>=88.5", "score!=88",
		"ratio==2", "ratio==2.0", "ratio>1.5",
		"qty", "!zero", // existence: a non-zero number is truthy, a zero one is not
	} {
		v, err := Eval(exp, symbols)
		assert.NoError(err, "%s", exp)
		assert.True(v, "expected %s to evaluate true", exp)
	}

	// A number and a non-number are still different types.
	v, err := Eval("qty=='3'", symbols)
	assert.Expect(err, nil, v, false)
}

// Compares an already-normalized map (fast path) against the equivalent struct (forces the round trip).
func BenchmarkEval_PreNormalizedMap(b *testing.B) {
	symbols := map[string]any{
		"id":     int64(1234567890123456789),
		"name":   "widget",
		"active": true,
		"nested": map[string]any{"a": int64(1), "b": "B"},
		"tags":   []any{"a", "b", "c"},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Eval("id==1234567890123456789 && name=='widget' && active && nested.a==1 && tags.b", symbols)
	}
}

func BenchmarkEval_Struct(b *testing.B) {
	type nested struct {
		A int    `json:"a"`
		B string `json:"b"`
	}
	type obj struct {
		ID     int64    `json:"id"`
		Name   string   `json:"name"`
		Active bool     `json:"active"`
		Nested nested   `json:"nested"`
		Tags   []string `json:"tags"`
	}
	symbols := obj{
		ID:     1234567890123456789,
		Name:   "widget",
		Active: true,
		Nested: nested{A: 1, B: "B"},
		Tags:   []string{"a", "b", "c"},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Eval("id==1234567890123456789 && name=='widget' && active && nested.a==1 && tags.b", symbols)
	}
}
