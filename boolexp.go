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
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/microbus-io/errors"
)

var (
	identifierRegexp = regexp.MustCompile(`^\w+(\.\w+)*$`)
)

// Eval evaluates a boolean expression against a set of key-value pairs which can be provided as a map or a struct.
func Eval(boolExp string, symbols any) (bool, error) {
	b, err := evaluateBoolExp(boolExp, flattenSymbolsMap(symbols))
	return b, errors.Trace(err)
}

// Validate validates that a boolean expression is syntactically correct.
func Validate(boolExp string) (err error) {
	_, err = evaluateBoolExp(boolExp, nil)
	return errors.Trace(err)
}

// flattenSymbolsMap flattens the symbols map into a shallow dot-notated map,
// while normalizing all arrays to []any and all maps to map[string]any.
func flattenSymbolsMap(symbols any) map[string]any {
	// Normalize all numbers to float64, all arrays to []any and all maps to map[string]any
	j, err := json.Marshal(symbols)
	if err != nil {
		return nil
	}
	var mappedSymbols map[string]any
	err = json.Unmarshal(j, &mappedSymbols)
	if err != nil {
		return nil
	}

	// Flatten the symbols into a shallow dot-notated map
	flattenedSymbols := map[string]any{}
	var flatten func(obj map[string]any, prefix string)
	flatten = func(obj map[string]any, prefix string) {
		for k, v := range obj {
			flattenedSymbols[prefix+k] = v
			switch vv := v.(type) {
			case map[string]any:
				flatten(vv, prefix+k+".")
			case []any:
				for _, a := range vv {
					flattenedSymbols[fmt.Sprintf("%s%s.%v", prefix, k, a)] = true
				}
			default:
				flattenedSymbols[prefix+k] = vv
			}
		}
	}
	flatten(mappedSymbols, "")
	return flattenedSymbols
}

// evaluateBoolExp evaluates the boolean expressing, assuming that the input symbols have been flattened and normalized.
func evaluateBoolExp(boolExp string, flattenedSymbols map[string]any) (bool, error) {
	// Resolve parenthesized sub-expressions
	parenLoop := true
	for parenLoop {
		parenStart := -1
		parenDepth := 0
		var quote byte
		for i := range boolExp {
			if boolExp[i] == '\'' || boolExp[i] == '"' || boolExp[i] == '`' {
				if boolExp[i] == quote {
					// Close quote
					quote = 0
				} else {
					// Open quote
					quote = boolExp[i]
				}
				continue
			}
			if quote != 0 {
				continue
			}
			if boolExp[i] == '(' {
				parenDepth++
				if parenStart < 0 {
					parenStart = i
				}
			} else if boolExp[i] == ')' {
				parenDepth--
				if parenDepth < 0 {
					// A close paren before any matching open. Without this, the depth nets back to 0
					// via a later '(' so the end-of-scan guard misses it, parenStart stays set, and the
					// outer loop re-runs on the unchanged string forever (e.g. ")(").
					return false, errors.New("invalid parenthesis pattern")
				}
				if parenDepth == 0 {
					subEval, err := evaluateBoolExp(boolExp[parenStart+1:i], flattenedSymbols)
					if err != nil {
						return false, errors.Trace(err)
					}
					boolExp = boolExp[:parenStart] + " " + strconv.FormatBool(subEval) + " " + boolExp[i+1:]
					break
				}
			}
		}
		if parenDepth != 0 {
			return false, errors.New("invalid parenthesis pattern")
		}
		if parenStart < 0 {
			break
		}
	}

	// True/false constants
	boolExp = strings.TrimSpace(boolExp)
	if strings.EqualFold(boolExp, "true") {
		return true, nil
	}
	if strings.EqualFold(boolExp, "false") {
		return false, nil
	}

	// Split by ||
	parts := strings.Split(boolExp, "||")
	if len(parts) > 1 {
		for _, part := range parts {
			subEval, err := evaluateBoolExp(part, flattenedSymbols)
			if err != nil {
				return false, errors.Trace(err)
			}
			if subEval {
				return true, nil
			}
		}
		return false, nil
	}

	// Split by &&
	parts = strings.Split(boolExp, "&&")
	if len(parts) > 1 {
		for _, part := range parts {
			subEval, err := evaluateBoolExp(part, flattenedSymbols)
			if err != nil {
				return false, errors.Trace(err)
			}
			if !subEval {
				return false, nil
			}
		}
		return true, nil
	}

	// Binary operators
	var before, after, operator string
	for _, op := range []string{"==", "!=", "<=", ">=", "=~", "!~", "<", ">"} {
		var found bool
		if before, after, found = strings.Cut(boolExp, op); found {
			operator = op
			break
		}
	}
	if operator != "" {
		if err := validateOperand(before); err != nil {
			return false, errors.Trace(err)
		}
		if err := validateOperand(after); err != nil {
			return false, errors.Trace(err)
		}
		x := evalValue(before, flattenedSymbols)
		y := evalValue(after, flattenedSymbols)
		switch operator {
		case "==":
			return sameType(x, y) && eq(x, y), nil
		case "!=":
			return !sameType(x, y) || !eq(x, y), nil
		case "<=":
			return sameType(x, y) && (eq(x, y) || lt(x, y)), nil
		case ">=":
			return sameType(x, y) && (eq(x, y) || lt(y, x)), nil
		case "=~": // regexp
			xs, ok := x.(string)
			if !ok {
				return false, nil
			}
			ys, ok := y.(string)
			if !ok {
				return false, nil
			}
			matched, err := regexp.MatchString(ys, xs)
			if err != nil {
				return false, errors.New("invalid regexp '%s'", y)
			}
			return matched, nil
		case "!~": // negative regexp
			xs, ok := x.(string)
			if !ok {
				return false, nil
			}
			ys, ok := y.(string)
			if !ok {
				return false, nil
			}
			matched, err := regexp.MatchString(ys, xs)
			if err != nil {
				return false, errors.New("invalid regexp '%s'", y)
			}
			return !matched, nil
		case "<":
			return sameType(x, y) && lt(x, y), nil
		case ">":
			return sameType(x, y) && lt(y, x), nil
		}
	}

	// Operator !
	not := false
	for strings.HasPrefix(boolExp, "!") {
		not = !not
		boolExp = strings.TrimSpace(boolExp[1:])
	}
	// Existence
	v := evalValue(boolExp, flattenedSymbols)
	if isNil(v) {
		// Verify it's an identifier x.y.z
		matched := identifierRegexp.MatchString(boolExp)
		if !matched {
			return false, errors.New("invalid identifier '%s'", boolExp)
		}
	}
	b, ok := v.(bool)
	if !ok {
		b = !empty(v)
	}
	if not {
		return !b, nil
	}
	return b, nil
}

// validateOperand checks that a binary operator operand is syntactically valid:
// an identifier, a quoted string, a number, or a boolean.
func validateOperand(v string) error {
	v = strings.TrimSpace(v)
	if v == "" {
		return errors.New("empty operand")
	}
	// Quoted string - must have matching closing quote
	if v[0] == '\'' || v[0] == '"' || v[0] == '`' {
		if len(v) < 2 || v[len(v)-1] != v[0] {
			return errors.New("unterminated string '%s'", v)
		}
		return nil
	}
	// Number
	if _, err := strconv.ParseFloat(v, 64); err == nil {
		return nil
	}
	// Boolean
	if _, err := strconv.ParseBool(v); err == nil {
		return nil
	}
	// Identifier (e.g. foo, foo.bar)
	if identifierRegexp.MatchString(v) {
		return nil
	}
	return errors.New("invalid operand '%s'", v)
}

// sameType returns true if x and y are of the same type.
func sameType(x any, y any) bool {
	return reflect.TypeOf(x) == reflect.TypeOf(y)
}

// empty returns true if x is nil or the zero value for its type.
func empty(x any) bool {
	if isNil(x) {
		return true
	}
	switch v := x.(type) {
	case string:
		return v == ""
	case float64:
		return v == 0
	case bool:
		return !v
	default:
		return false
	}
}

// eq returns true if x and y are of the same type and x==y.
func eq(x any, y any) bool {
	if reflect.TypeOf(x) != reflect.TypeOf(y) {
		return false
	}
	if isNil(x) && isNil(y) {
		return true
	}
	switch v := x.(type) {
	case string:
		return v == y.(string)
	case float64:
		return v == y.(float64)
	case bool:
		return v == y.(bool)
	default:
		return false
	}
}

// lt returns true if x and y are of the same type and x<y.
func lt(x any, y any) bool {
	if reflect.TypeOf(x) != reflect.TypeOf(y) {
		return false
	}
	switch v := x.(type) {
	case string:
		return v < y.(string)
	case float64:
		return v < y.(float64)
	default:
		return false
	}
}

// evalValue returns the value of a terminal expression.
func evalValue(v string, symbols map[string]any) any {
	v = strings.TrimSpace(v)

	// Symbol
	if symbols != nil {
		if s, ok := symbols[v]; ok {
			return s
		}
	}
	// String
	if strings.HasPrefix(v, `"`) && strings.HasSuffix(v, `"`) && len(v) >= 2 {
		return v[1 : len(v)-1]
	}
	if strings.HasPrefix(v, `'`) && strings.HasSuffix(v, `'`) && len(v) >= 2 {
		return v[1 : len(v)-1]
	}
	if strings.HasPrefix(v, "`") && strings.HasSuffix(v, "`") && len(v) >= 2 {
		return v[1 : len(v)-1]
	}
	// Number
	if f, err := strconv.ParseFloat(v, 64); err == nil {
		return f
	}
	// Boolean
	if b, err := strconv.ParseBool(v); err == nil {
		return b
	}
	return nil
}

// isNil returns true if x is nil or an interface holding nil.
func isNil(x any) bool {
	defer func() { recover() }()
	return x == nil || reflect.ValueOf(x).IsNil()
}
