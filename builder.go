package mf2

import (
	"encoding/json"
	"fmt"
	"strings"
)

const (
	defaultSpacing = " "
	defaultNewline = "\n"
)

type Builder struct {
	spacing string // optional spacing [s]
	newline string
	err     error

	locals    []local
	inputs    []*Expression
	selectors []*Expression // matcher selectors
	variants  []variant     // matcher variants
	pattern   []any
}

func NewBuilder() *Builder {
	return &Builder{
		newline: defaultNewline,
		spacing: defaultSpacing,
	}
}

func (b *Builder) Build() (string, error) {
	if b.err != nil {
		return "", b.err
	}

	if err := b.validate(); err != nil {
		return "", err
	}

	var s string

	for _, v := range b.inputs {
		s += ".input" + b.spacing + v.build(b.spacing) + b.newline
	}

	for _, v := range b.locals {
		s += ".local" + b.spacing + string(v.variable) + b.spacing + "=" + b.spacing + v.expr.build(b.spacing) + b.newline
	}

	quotedPattern := (len(b.inputs) > 0 || len(b.locals) > 0) && (len(b.variants) == 0 && len(b.selectors) == 0)

	if len(b.pattern) > 0 {
		if v, ok := b.pattern[0].(string); ok && !hasSimpleStart(v) {
			switch {
			case len(b.pattern) == 1 && v == "": // simple message with empty text
				// noop
			case len(v) > 0 && []rune(v)[0] == '.': // complex message
				quotedPattern = true
			default:
				return "", fmt.Errorf("simple message MUST start with a simple start character: %s", v)
			}
		}
	}

	if quotedPattern {
		s += "{{"
	}

	for _, v := range b.pattern {
		switch v := v.(type) {
		case string:
			s += textEscape(v)
		case *Expression:
			s += v.build(b.spacing)
		default:
			return "", fmt.Errorf("unsupported pattern type: %T", v)
		}
	}

	if quotedPattern {
		s += "}}"
	}

	if len(b.selectors) > 0 {
		s += ".match"

		for _, v := range b.selectors {
			s += b.spacing + v.build(b.spacing)
		}

		s += b.newline
	}

	for i, v := range b.variants {
		s += v.build(b.spacing)

		if i != len(b.variants)-1 {
			s += b.newline
		}
	}

	return s, nil
}

func (b *Builder) MustBuild() string {
	s, err := b.Build()
	if err != nil {
		panic(err)
	}

	return s
}

func (b *Builder) validate() error {
	if len(b.variants) > 0 {
		if len(b.pattern) > 0 {
			return fmt.Errorf("complex message MUST have single complex body")
		}

		if len(b.selectors) == 0 {
			return fmt.Errorf("matcher MUST have at least one selector")
		}

		if !hasCatchAllVariant(b.variants) {
			return fmt.Errorf("matcher MUST have at least one variant with all catch-all keys")
		}
	}

	if len(b.selectors) > 0 && len(b.variants) == 0 {
		return fmt.Errorf("matcher MUST have at least one variant")
	}

	return nil
}

func (b *Builder) Newline(s string) *Builder {
	if b.err != nil {
		return b
	}

	b.newline = s

	return b
}

// TODO: add to all expressions.
func (b *Builder) Spacing(s string) *Builder {
	if b.err != nil {
		return b
	}

	b.spacing = s

	return b
}

func (b *Builder) Text(s string) *Builder {
	if b.err != nil {
		return b
	}

	if len(b.variants) > 0 {
		b.variants[len(b.variants)-1].pattern = append(b.variants[len(b.variants)-1].pattern, s)
	} else {
		b.pattern = append(b.pattern, s)
	}

	return b
}

func (b *Builder) Local(v string, expr *Expression) *Builder {
	if b.err != nil {
		return b
	}

	b.locals = append(b.locals, local{variable: Var(v), expr: expr})

	return b
}

func (b *Builder) Input(expr *Expression) *Builder {
	if b.err != nil {
		return b
	}

	b.inputs = append(b.inputs, expr)

	return b
}

func Expr(operand ...any) *Expression {
	if len(operand) == 0 {
		return new(Expression)
	}

	switch v := operand[0].(type) {
	default:
		return &Expression{operand: Literal(v)}
	case Var, Literal:
		return &Expression{operand: v}
	}
}

func (b *Builder) Expr(expr *Expression) *Builder {
	if b.err != nil {
		return b
	}

	if len(b.variants) > 0 {
		b.variants[len(b.variants)-1].pattern = append(b.variants[len(b.variants)-1].pattern, expr)
		return b
	}

	b.pattern = append(b.pattern, expr)

	return b
}

func (b *Builder) Match(selector *Expression, selectors ...*Expression) *Builder {
	if b.err != nil {
		return b
	}

	if len(b.pattern) > 0 {
		b.err = fmt.Errorf("complex message cannot be added after simple message")
		return b
	}

	b.selectors = append(b.selectors, selector)
	b.selectors = append(b.selectors, selectors...)

	return b
}

func (b *Builder) Keys(key any, keys ...any) *Builder {
	if b.err != nil {
		return b
	}

	if len(keys)+1 != len(b.selectors) {
		b.err = fmt.Errorf("number of keys in each variant MUST match the number of selectors in the matcher")
		return b
	}

	b.variants = append(b.variants, variant{keys: append([]any{key}, keys...)})

	return b
}

type variant struct {
	keys    []any
	pattern []any
}

func (v *variant) build(spacing string) string {
	var s string

	for i, k := range v.keys {
		if i > 0 {
			s += coalesce(spacing, defaultSpacing)
		}

		if k == "*" {
			s += "*"
		} else {
			s += printLiteral(k)
		}
	}

	s += spacing + "{{"

	for i := range v.pattern {
		switch p := v.pattern[i].(type) {
		case string:
			s += textEscape(p)
		case *Expression:
			s += p.build(spacing)
		default:
			panic(fmt.Sprintf("unsupported pattern type: %T", p))
		}
	}

	return s + "}}"
}

type local struct {
	expr     *Expression
	variable Var
}

type Literal any

type Var string

func (v Var) String() string {
	return "$" + string(v)
}

type function struct {
	name    string
	options []FuncOption
}

type Expression struct {
	operand  any // literal or variable
	function function
}

func (e *Expression) build(spacing string) string {
	s := "{" + spacing

	switch v := e.operand.(type) {
	case Var:
		s += v.String()
	case nil:
		// noop
	case Literal:
		s += printLiteral(v)
	default:
		panic(fmt.Sprintf("unsupported operand type: %T", v))
	}

	if e.function.name != "" {
		if e.operand != nil { // literal or variable
			s += " "
		}

		s += e.function.name

		for _, o := range e.function.options {
			s += " " + o.key + spacing + "=" + spacing

			switch v := o.operand.(type) {
			default:
				s += printLiteral(o.operand)
			case Var:
				s += v.String()
			}

		}
	}

	return s + spacing + "}"
}

func (e *Expression) Literal(v any) *Expression {
	e.operand = v
	return e
}

func (e *Expression) Var(v string) *Expression {
	if len(v) == 0 {
		panic("variable name cannot be empty")
	}

	e.operand = Var(v)

	return e
}

type FuncOption struct {
	operand any // literal or variable
	key     string
}

func Option(key string, operand any) FuncOption { return FuncOption{key: key, operand: operand} }

func Func(name string, option ...FuncOption) *Expression {
	return Expr().Func(name, option...)
}

func (e *Expression) Func(name string, option ...FuncOption) *Expression {
	if len(name) == 0 {
		panic("function name cannot be empty")
	}

	e.function = function{name: name, options: option}

	return e
}

func printLiteral(l any) string {
	switch v := l.(type) { // TODO: more liberal
	case string:
		for i, r := range v {
			if i == 0 && !isNameStart(r) || i > 0 && !isName(r) {
				return printQuoted(v)
			}
		}

		return v
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64,
		float32, float64:
		b, err := json.Marshal(v)
		if err != nil {
			panic(err)
		}

		return string(b)
	default:
		panic(fmt.Sprintf("unsupported literal type: %T", v))
	}
}

// helpers

/*
	printQuoted escapes special characters in quoted name literal.

ABNF:
quoted-escape   = backslash ( backslash / "|" )
.
*/
func printQuoted(s string) string {
	return "|" + strings.NewReplacer("\\", "\\\\", "|", "\\|").Replace(s) + "|"
}

/*
	textEscape escapes special characters in text.

ABNF:
text-escape     = backslash ( backslash / "{" / "}" )
.
*/
func textEscape(s string) string {
	return strings.NewReplacer("\\", "\\\\", "{", "\\{", "}", "\\}").Replace(s)
}

// hasSimpleStart returns true if the string has a simple start.
// ABNF:
// simple-start = simple-start-char / text-escape / placeholder
// .
func hasSimpleStart(s string) bool {
	if len(s) > 0 {
		c := []rune(s)[0]

		if isSimpleStart(c) ||
			c == '{' || c == '}' || c == '\\' { // text-escape     = backslash ( backslash / "{" / "}" )
			return true
		}
	}

	return false
}

// isSimpleStart returns true if r is simple start character.
//
// ABNF:
//
//	simple-start-char = %x0-2D         ; omit .
//	                  / %x2F-5B        ; omit \
//	                  / %x5D-7A        ; omit {
//	                  / %x7C           ; omit }
//	                  / %x7E-D7FF      ; omit surrogates
//	                  / %xE000-10FFFF
func isSimpleStart(r rune) bool {
	return 0x0 <= r && r <= 0x2D || // omit .
		0x2F <= r && r <= 0x5B || // omit \
		0x5D <= r && r <= 0x7A || // omit {}
		r == 0x7C || // omit }
		0x7E <= r && r <= 0xD7FF || // omit surrogates
		0xE000 <= r && r <= 0x10FFFF
}

// hasCatchAllVariant() checks if at least variant has catch-all keys.
func hasCatchAllVariant(variants []variant) bool {
	for _, v := range variants {
		var catchAllCount int

		for _, key := range v.keys {
			if key == "*" {
				catchAllCount++
			}

			if catchAllCount == len(v.keys) {
				return true
			}
		}
	}

	return false
}

// isName returns true if r is name character.
//
// ABNF:
//
//	name-char = name-start / DIGIT / "-" / "." / %xB7 / %x0300-036F / %x203F-2040.
func isName(v rune) bool {
	return isAlpha(v) ||
		'0' <= v && v <= '9' ||
		v == '-' ||
		v == '.' ||
		v == 0xB7 ||
		0x0300 <= v && v <= 0x036F ||
		0x203F <= v && v <= 2040
}

// isNameStart returns true if r is name start character.
//
// ABNF:
//
//	name-start = ALPHA / "_"
//	           / %xC0-D6 / %xD8-F6 / %xF8-2FF
//	           / %x370-37D / %x37F-1FFF / %x200C-200D
//	           / %x2070-218F / %x2C00-2FEF / %x3001-D7FF
//	           / %xF900-FDCF / %xFDF0-FFFD / %x10000-EFFFF
func isNameStart(r rune) bool {
	return isAlpha(r) ||
		r == '_' ||
		0xC0 <= r && r <= 0xD6 ||
		0xD8 <= r && r <= 0xF6 ||
		0xF8 <= r && r <= 0x2FF ||
		0x370 <= r && r <= 0x37D ||
		0x37F <= r && r <= 0x1FFF ||
		0x200C <= r && r <= 0x200D ||
		0x2070 <= r && r <= 0x218F ||
		0x2C00 <= r && r <= 0x2FEF ||
		0x3001 <= r && r <= 0xD7FF ||
		0xF900 <= r && r <= 0xFDCF ||
		0xFDF0 <= r && r <= 0xFFFD ||
		0x10000 <= r && r <= 0xEFFFF
}

// isAlpha returns true if r is alphabetic character.
func isAlpha(r rune) bool {
	return ('a' <= r && r <= 'z') || ('A' <= r && r <= 'Z')
}

func coalesce[T comparable](l ...T) T {
	var c T

	for _, v := range l {
		if v != c {
			return v
		}
	}

	return c
}
