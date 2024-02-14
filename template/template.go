package template

import (
	"errors"
	"fmt"
	"io"
	"strings"

	ast "go.expect.digital/mf2/parse"
)

// MessageFormat2 Errors as defined in the specification.
//
// https://github.com/unicode-org/message-format-wg/blob/122e64c2482b54b6eff4563120915e0f86de8e4d/spec/errors.md
var (
	ErrSyntax                = errors.New("syntax error")
	ErrUnresolvedVariable    = errors.New("unresolved variable")
	ErrUnknownFunction       = errors.New("unknown function reference")
	ErrDuplicateOptionName   = errors.New("duplicate option name")
	ErrUnsupportedExpression = errors.New("unsupported expression")
	ErrFormatting            = errors.New("formatting error")
)

type ExecFn func(operand any, opts map[string]any) (string, error)

type Template struct {
	ast       *ast.AST
	execFuncs map[string]ExecFn
	executer  *executer
}

type executer struct {
	wr    io.Writer
	input map[string]any
}

func (e *executer) writeString(s string) error {
	if _, err := e.wr.Write([]byte(s)); err != nil {
		return fmt.Errorf("write: %w", err)
	}

	return nil
}

func newExecuter(wr io.Writer, input map[string]any) *executer {
	return &executer{wr: wr, input: input}
}

func New() *Template {
	return &Template{execFuncs: make(map[string]ExecFn)}
}

func Must(t *Template, err error) *Template {
	if err != nil {
		panic(err)
	}

	return t
}

// AddFunc adds a function to the template's function map.
func (t *Template) AddFunc(name string, f ExecFn) {
	t.execFuncs[name] = f
}

func (t *Template) Parse(input string) (*Template, error) {
	ast, err := ast.Parse(input)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrSyntax, err.Error())
	}

	t.ast = &ast

	return t, nil
}

func (t *Template) Execute(wr io.Writer, input map[string]any) error {
	if t.ast == nil {
		return errors.New("AST is nil")
	}

	t.executer = newExecuter(wr, input)

	switch message := t.ast.Message.(type) {
	default:
		return fmt.Errorf("unknown message type: '%T'", message)
	case nil:
		return nil
	case ast.SimpleMessage:
		return t.resolveSimpleMessage(message)
	case ast.ComplexMessage:
		return errors.New("complex message not implemented") // TODO: Implement.
	}
}

// Sprint wraps Execute and returns the result as a string.
func (t *Template) Sprint(input map[string]any) (string, error) {
	sb := new(strings.Builder)

	if err := t.Execute(sb, input); err != nil {
		return "", fmt.Errorf("execute: %w", err)
	}

	return sb.String(), nil
}

// ------------------------------------Resolvers------------------------------------

func (t *Template) resolveSimpleMessage(message ast.SimpleMessage) error {
	for _, pattern := range message {
		switch pattern := pattern.(type) {
		case ast.TextPattern:
			if err := t.executer.writeString(string(pattern)); err != nil {
				return err
			}
		case ast.Expression:
			if err := t.resolveExpression(pattern); err != nil {
				return fmt.Errorf("resolve expression: %w", err)
			}
		case ast.Markup: // TODO: Implement.
			return fmt.Errorf("'%T' not implemented", pattern)
		}
	}

	return nil
}

func (t *Template) resolveExpression(expr ast.Expression) error {
	value, err := t.resolveValue(expr.Operand)
	if err != nil {
		return fmt.Errorf("resolve value: %w", err)
	}

	if expr.Annotation == nil {
		// NOTE: Parser won't allow value to be nil if annotation is nil.
		valueStr := fmt.Sprint(value) // TODO: If value does not implement fmt.Stringer, what then ?
		return t.executer.writeString(valueStr)
	}

	if err := t.resolveAnnotation(value, expr.Annotation); err != nil {
		return fmt.Errorf("resolve annotation: %w", err)
	}

	return nil
}

// resolveValue resolves the value of an expression's operand.
//
//   - If the operand is a literal, it returns the literal's value.
//   - If the operand is a variable, it returns the value of the variable from the input map.
func (t *Template) resolveValue(v ast.Value) (any, error) {
	switch v := v.(type) {
	default:
		return nil, fmt.Errorf("unknown value type: '%T'", v)
	case nil:
		return v, nil // nil is also a valid value.
	case ast.QuotedLiteral:
		return string(v), nil
	case ast.NameLiteral:
		return string(v), nil
	case ast.NumberLiteral:
		return float64(v), nil
	case ast.Variable:
		val, ok := t.executer.input[string(v)]
		if !ok {
			return nil, fmt.Errorf("%w '%s'", ErrUnresolvedVariable, v)
		}

		return val, nil
	}
}

func (t *Template) resolveAnnotation(operand any, annotation ast.Annotation) error {
	annoFn, ok := annotation.(ast.Function)
	if !ok {
		return fmt.Errorf("%w with %T annotation: '%s'", ErrUnsupportedExpression, annotation, annotation)
	}

	execF, ok := t.execFuncs[annoFn.Identifier.Name]
	if !ok {
		return fmt.Errorf("%w '%s'", ErrUnknownFunction, annoFn.Identifier.Name)
	}

	opts, err := t.resolveOptions(annoFn.Options)
	if err != nil {
		return fmt.Errorf("resolve options: %w", err)
	}

	result, err := execF(operand, opts)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrFormatting, err.Error())
	}

	return t.executer.writeString(result)
}

func (t *Template) resolveOptions(options []ast.Option) (map[string]any, error) {
	m := make(map[string]any, len(options))

	for _, opt := range options {
		name := opt.Identifier.Name
		if _, ok := m[name]; ok {
			return nil, fmt.Errorf("%w '%s'", ErrDuplicateOptionName, name)
		}

		value, err := t.resolveValue(opt.Value)
		if err != nil {
			return nil, fmt.Errorf("resolve value: %w", err)
		}

		m[name] = value
	}

	return m, nil
}
