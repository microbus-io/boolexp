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
	"strings"
	"testing"
	"time"

	"github.com/microbus-io/testarossa"
)

func TestFlattenSymbolsMap(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	// No op
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
	// Array
	assert.Equal(
		map[string]any{
			"arr_int":        []any{1.0, 2.0, 3.0},
			"arr_int.1":      true,
			"arr_int.2":      true,
			"arr_int.3":      true,
			"arr_string":     []any{"x", "y", "z"},
			"arr_string.x":   true,
			"arr_string.y":   true,
			"arr_string.z":   true,
			"arr_float":      []any{1.0, 2.5, 3.33},
			"arr_float.1":    true,
			"arr_float.2.5":  true,
			"arr_float.3.33": true,
		},
		flattenSymbolsMap(map[string]any{
			"arr_int":    []int{1, 2, 3},
			"arr_string": []string{"x", "y", "z"},
			"arr_float":  []float32{1.0, 2.5, 3.33},
		}),
	)
	// Map
	assert.Equal(
		map[string]any{
			"map_int": map[string]any{
				"a": 1.0, "b": 2.0, "c": 3.0,
			},
			"map_int.a": 1.0,
			"map_int.b": 2.0,
			"map_int.c": 3.0,
			"map_string": map[string]any{
				"a": "A", "b": "B", "c": "C",
			},
			"map_string.a": "A",
			"map_string.b": "B",
			"map_string.c": "C",
			"map_float": map[string]any{
				"a": 1.0, "b": 2.5, "c": 3.33,
			},
			"map_float.a": 1.0,
			"map_float.b": 2.5,
			"map_float.c": 3.33,
		},
		flattenSymbolsMap(map[string]any{
			"map_int": map[string]int{
				"a": 1, "b": 2, "c": 3,
			},
			"map_string": map[string]string{
				"a": "A", "b": "B", "c": "C",
			},
			"map_float": map[string]float32{
				"a": 1.0, "b": 2.5, "c": 3.33,
			},
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

	// Literal
	l := 9999
	v, err = Eval("i==5 && s=='hello' && obj.field=~'^[a-z0-9]+$'", l)
	assert.Expect(err, nil, v, false)
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
