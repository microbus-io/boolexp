# Boolean Expression Evaluator

The `boolexp` module is a utility for evaluating a boolean expression such as `foo=='bar' && (y==3 || y<0)` against a set of key-value pairs in the form of a `map[string]any` or fields of a `struct`.

Syntax:
- Logical operators `&&`, `||` and `!`
- Comparison operators `==`, `!=`, `<`, `<=`, `>` and `>=`
- Regexp operators `=~` and `!~`
- Parentheses `(` and `)`
- Constants `true` and `false`
- Dot notation `symbol.property`
- Quotation of string literals using `'` or `"`

Evaluate against a `map[string]any`:

```go
symbols := map[string]any{
    "i": 5,
    "s": "hello",
    "nested": map[string]any{
        "field": "alphanumeric",
    },
}
isTrue, err := Eval("(i==5 || i==7) && s=='hello' && nested.field=~'^[a-z0-9]+$'", symbols)
```

Evaluate against an object's fields:

```go
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
isTrue, err := Eval("(i==5 || i==7) && s=='hello' && nested.field=~'^[a-z0-9]+$'", object)
```
