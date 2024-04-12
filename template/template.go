package template

import (
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"golang.org/x/exp/slices"
	"golang.org/x/text/language"

	ast "go.expect.digital/mf2/parse"
	"go.expect.digital/mf2/template/registry"
)

// MessageFormat2 Errors as defined in the specification.
//
// https://github.com/unicode-org/message-format-wg/blob/20a61b4af534acb7ecb68a3812ca0143b34dfc76/spec/errors.md
var (
	ErrSyntax                    = errors.New("syntax error")
	ErrUnresolvedVariable        = errors.New("unresolved variable")
	ErrUnknownFunction           = errors.New("unknown function reference")
	ErrDuplicateOptionName       = errors.New("duplicate option name")
	ErrUnsupportedExpression     = errors.New("unsupported expression")
	ErrFormatting                = errors.New("formatting error")
	ErrUnsupportedStatement      = errors.New("unsupported statement")
	ErrDuplicateDeclaration      = errors.New("duplicate declaration")
	ErrMissingSelectorAnnotation = errors.New("missing selector annotation")
	ErrSelection                 = errors.New("selection error")
)

// Func is a function, that will be called when a function is encountered in the template.
type Func func(operand any, options map[string]any) (string, error)

// Template represents a MessageFormat2 template.
type Template struct {
	// TODO: locale field. Can change the output of some functions.
	// e.g. number formatting, given example { $num :number }:
	//  - "en-US" -> 1,234.56
	//  - "lv-LV" -> 1234,56
	// e.g. date formatting, given example { $date :datetime }:
	//  - "en-US" -> 1/2/2023
	//  - "lv-LV" -> 2.1.2023
	ast          *ast.AST
	funcRegistry registry.Registry
	locale       language.Tag
}

// New returns a new Template.
func New(options ...Option) *Template {
	t := &Template{
		funcRegistry: registry.New(),
		locale:       language.AmericanEnglish,
	}

	for _, o := range options {
		o(t)
	}

	return t
}

// Option is a template option.
type Option func(t *Template)

// WithFunc adds a single function to function registry.
func WithFunc(name string, f registry.Func) Option {
	return func(t *Template) {
		t.funcRegistry[name] = registry.F{Format: f.Format, Match: f.Match}
	}
}

func WithFuncs(reg registry.Registry) Option {
	return func(t *Template) {
		for k, f := range reg {
			t.funcRegistry[k] = f
		}
	}
}

func WithLocale(locale language.Tag) Option {
	return func(t *Template) {
		t.locale = locale
	}
}

// Parse parses the MessageFormat2 string and returns the template.
func (t *Template) Parse(input string) (*Template, error) {
	ast, err := ast.Parse(input)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrSyntax, err.Error())
	}

	t.ast = &ast

	return t, nil
}

// Execute writes the result of the template to the given writer.
func (t *Template) Execute(w io.Writer, input map[string]any) error {
	if t.ast == nil {
		return errors.New("AST is nil")
	}

	executer := &executer{template: t, w: w, variables: make(map[string]any, len(input))}

	for k, v := range input {
		executer.variables[k] = v
	}

	return executer.execute()
}

// Sprint wraps Execute and returns the result as a string.
func (t *Template) Sprint(input map[string]any) (string, error) {
	sb := new(strings.Builder)

	if err := t.Execute(sb, input); err != nil {
		return sb.String(), fmt.Errorf("execute: %w", err)
	}

	return sb.String(), nil
}

type executer struct {
	template  *Template
	w         io.Writer
	variables map[string]any
}

func (e *executer) write(s string) error {
	if _, err := e.w.Write([]byte(s)); err != nil {
		return fmt.Errorf("write: %w", err)
	}

	return nil
}

func (e *executer) execute() error {
	switch message := e.template.ast.Message.(type) {
	default:
		return fmt.Errorf("unknown message type: '%T'", message)
	case nil:
		return nil
	case ast.SimpleMessage:
		if err := e.resolvePattern(message); err != nil {
			return fmt.Errorf("resolve pattern: %w", err)
		}
	case ast.ComplexMessage:
		return e.resolveComplexMessage(message)
	}

	return nil
}

func (e *executer) resolveComplexMessage(message ast.ComplexMessage) error {
	var resolutionErr error

	err := e.resolveDeclarations(message.Declarations)

	switch {
	case errors.Is(err, ErrUnsupportedStatement), errors.Is(err, ErrUnresolvedVariable):
		resolutionErr = fmt.Errorf("resolve declarations: %w", err)
	case err != nil:
		return fmt.Errorf("resolve declarations: %w", err)
	}

	err = e.resolveComplexBody(message.ComplexBody)

	switch {
	case errors.Is(err, ErrUnresolvedVariable):
		resolutionErr = fmt.Errorf("resolve complex body: %w", err)
	case err != nil:
		return fmt.Errorf("resolve complex body: %w", err)
	}

	return resolutionErr
}

func (e *executer) resolveDeclarations(declarations []ast.Declaration) error {
	m := make(map[ast.Value]struct{}, len(declarations))

	for _, decl := range declarations {
		switch d := decl.(type) {
		case ast.ReservedStatement:
			return fmt.Errorf("%w: '%s'", ErrUnsupportedStatement, "reserved statement")
		case ast.LocalDeclaration:
			if _, ok := m[d.Variable]; ok {
				return fmt.Errorf("%w '%s'", ErrDuplicateDeclaration, d)
			}

			m[d.Variable] = struct{}{}

			resolved, err := e.resolveExpression(d.Expression)
			if err != nil {
				return fmt.Errorf("resolve expression: %w", err)
			}

			e.variables[string(d.Variable)] = resolved
		case ast.InputDeclaration:
			if _, ok := m[d.Operand]; ok {
				return fmt.Errorf("%w '%s'", ErrDuplicateDeclaration, d)
			}

			m[d.Operand] = struct{}{}

			resolved, err := e.resolveExpression(ast.Expression(d))
			if err != nil {
				return fmt.Errorf("resolve expression: %w", err)
			}

			e.variables[string(d.Operand.(ast.Variable))] = resolved //nolint: forcetypeassert // Will always be a variable.
		}
	}

	return nil
}

func (e *executer) resolveComplexBody(body ast.ComplexBody) error {
	switch b := body.(type) {
	case ast.Matcher:
		if err := e.resolveMatcher(b); err != nil {
			return fmt.Errorf("resolve matcher: %w", err)
		}
	case ast.QuotedPattern:
		if err := e.resolvePattern(b); err != nil {
			return fmt.Errorf("resolve pattern: %w", err)
		}
	}

	return nil
}

func (e *executer) resolvePattern(pattern []ast.PatternPart) error {
	var resolutionErr error

	for _, part := range pattern {
		switch v := part.(type) {
		case ast.Text:
			if err := e.write(string(v)); err != nil {
				return errors.Join(resolutionErr, fmt.Errorf("write text: %w", err))
			}
		case ast.Expression:
			resolved, err := e.resolveExpression(v)
			if err != nil {
				resolutionErr = errors.Join(resolutionErr, fmt.Errorf("resolve expression: %w", err))
			}

			if err := e.write(resolved); err != nil {
				return errors.Join(resolutionErr, fmt.Errorf("write expression: %w", err))
			}
		//nolint:lll
		// When formatting to a string, markup placeholders format to an empty string by default.
		// https://github.com/unicode-org/message-format-wg/blob/main/exploration/open-close-placeholders.md#formatting-to-a-string
		case ast.Markup:
		}
	}

	return resolutionErr
}

func (e *executer) resolveExpression(expr ast.Expression) (string, error) {
	value, err := e.resolveValue(expr.Operand)
	if err != nil {
		return fmt.Sprint(value), fmt.Errorf("resolve value: %w", err)
	}

	resolved, err := e.resolveAnnotation(value, expr.Annotation)
	if err != nil {
		return resolved, fmt.Errorf("resolve annotation: %w", err)
	}

	return resolved, nil
}

// resolveValue resolves the value of an expression's operand.
//
//   - If the operand is a literal, it returns the literal's value.
//   - If the operand is a variable, it returns the value of the variable from the input map.
func (e *executer) resolveValue(v ast.Value) (any, error) {
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
		val, ok := e.variables[string(v)]
		if !ok {
			return fmt.Sprintf("{%s}", v), fmt.Errorf("%w '%s'", ErrUnresolvedVariable, v)
		}

		return val, nil
	}
}

func (e *executer) resolveAnnotation(operand any, annotation ast.Annotation) (string, error) {
	var (
		funcName      string
		options       map[string]any
		resolutionErr error
	)

	switch v := annotation.(type) {
	default:
		return "", fmt.Errorf("%w with %T annotation: '%s'", ErrUnsupportedExpression, v, v)
	case ast.Function:
		var err error

		funcName = v.Identifier.Name

		if options, err = e.resolveOptions(v.Options); err != nil {
			return "", fmt.Errorf("resolve options: %w", err)
		}
	case ast.PrivateUseAnnotation:
		// https://github.com/unicode-org/message-format-wg/blob/20a61b4af534acb7ecb68a3812ca0143b34dfc76/spec/formatting.md
		//
		// Supported private-use annotation with no operand: the annotation starting sigil, optionally followed by
		// implementation-defined details conforming with patterns in the other cases (such as quoting literals).
		// If details are provided, they SHOULD NOT leak potentially private information.
		resolutionErr = fmt.Errorf("%w with %T private use annotation: '%s'", ErrUnsupportedExpression, v, v)

		if operand == nil {
			return "{" + string(v.Start) + "}", resolutionErr
		}
	case ast.ReservedAnnotation:
		resolutionErr = fmt.Errorf("%w with %T reserved annotation: '%s'", ErrUnsupportedExpression, v, v)

		if operand == nil {
			return "{" + string(v.Start) + "}", resolutionErr
		}
	case nil: // noop, no annotation
	}

	// return empty string if operand is nil
	fmtOperand := func() string {
		if operand == nil {
			return ""
		}

		return fmt.Sprint(operand)
	}

	if funcName == "" {
		switch operand.(type) {
		default: // TODO(jhorsts): how is unknown type formatted?
			return fmtOperand(), resolutionErr
		case string:
			funcName = "string"
		case float64:
			funcName = "number"
		}
	}

	f, ok := e.template.funcRegistry[funcName] // TODO(jhorsts): lookup by namespace and name
	if !ok {
		return fmtOperand(), errors.Join(resolutionErr, fmt.Errorf("%w '%s'", ErrUnknownFunction, funcName))
	}

	if f.Format == nil {
		return "", fmt.Errorf("function '%s' not allowed in formatting context", funcName)
	}

	result, err := f.Format(operand, options, e.template.locale)
	if err != nil {
		return "", errors.Join(resolutionErr, ErrFormatting, err)
	}

	return fmt.Sprint(result), resolutionErr
}

func (e *executer) resolveOptions(options []ast.Option) (map[string]any, error) {
	m := make(map[string]any, len(options))

	for _, opt := range options {
		name := opt.Identifier.Name
		if _, ok := m[name]; ok {
			return nil, fmt.Errorf("%w '%s'", ErrDuplicateOptionName, name)
		}

		value, err := e.resolveValue(opt.Value)
		if err != nil {
			return nil, fmt.Errorf("resolve value: %w", err)
		}

		m[name] = value
	}

	return m, nil
}

func (e *executer) resolveMatcher(m ast.Matcher) error {
	res, err := e.resolveSelector(m)
	if err != nil {
		return fmt.Errorf("resolve selector: %w", err)
	}

	pref := e.resolvePreferences(m, res)

	filteredVariants := e.filterVariants(m, pref)

	sortable := e.sortVariants(filteredVariants, pref)

	err = e.selectBestVariant(sortable)
	if err != nil {
		return err
	}

	return nil
}

func (e *executer) resolveSelector(matcher ast.Matcher) ([]any, error) {
	res := make([]any, 0, len(matcher.MatchStatements))

	for _, selectExpr := range matcher.MatchStatements {
		var function ast.Function

		switch annotation := selectExpr.Annotation.(type) {
		case nil:
			return nil, ErrMissingSelectorAnnotation
		case ast.ReservedAnnotation, ast.PrivateUseAnnotation:
			return nil, ErrUnsupportedExpression
		case ast.Function:
			function = annotation
		}

		f, ok := e.template.funcRegistry[function.Identifier.Name]
		if !ok {
			return nil, fmt.Errorf("%w '%s'", ErrUnknownFunction, function.Identifier.Name)
		}

		if f.Match == nil {
			return nil, fmt.Errorf("function '%s' not allowed in selector context", function.Identifier.Name)
		}

		opts, err := e.resolveOptions(function.Options)
		if err != nil {
			return nil, fmt.Errorf("resolve options: %w", err)
		}

		input, err := e.resolveValue(selectExpr.Operand)
		if err != nil {
			return nil, fmt.Errorf("resolve value: %w", err)
		}

		rslt, err := f.Match(input, opts, e.template.locale)
		if err != nil {
			return nil, fmt.Errorf("%w: %s", ErrSelection, err.Error())
		}

		res = append(res, rslt)
	}

	return res, nil
}

func (e *executer) resolvePreferences(m ast.Matcher, res []any) [][]string {
	// Step 2: Resolve Preferences
	pref := make([][]string, 0, len(res))

	for i := range res {
		var keys []string

		for _, variant := range m.Variants {
			for _, vKey := range variant.Keys {
				switch key := vKey.(type) {
				case ast.CatchAllKey:
					continue
				case ast.NameLiteral, ast.QuotedLiteral:
					keys = append(keys, key.String())
				case ast.NumberLiteral:
					keys = append(keys, fmt.Sprint(key))
				}
			}
		}

		rv := res[i]

		matches := matchSelectorKeys(rv, keys)
		pref = append(pref, matches)
	}

	return pref
}

func (e *executer) filterVariants(m ast.Matcher, pref [][]string) []ast.Variant {
	// Step 3: Filter Variants
	var filteredVariants []ast.Variant

	for _, variant := range m.Variants {
		matchesAllSelectors := true

		for i, keyOrder := range pref {
			key := variant.Keys[i]

			var ks string

			switch key := key.(type) {
			case ast.CatchAllKey:
				continue
			case ast.NameLiteral, ast.QuotedLiteral:
				ks = key.String()
			case ast.NumberLiteral:
				ks = fmt.Sprint(key)
			}

			if !slices.Contains(keyOrder, ks) {
				matchesAllSelectors = false
				break
			}
		}

		if matchesAllSelectors {
			filteredVariants = append(filteredVariants, variant)
		}
	}

	return filteredVariants
}

func (e *executer) sortVariants(filteredVariants []ast.Variant, pref [][]string) []SortableVariant {
	// Step 4: Sort Variants
	sortable := make([]SortableVariant, 0, len(filteredVariants))

	for _, variant := range filteredVariants {
		sortable = append(sortable, SortableVariant{Score: -1, Variant: variant})
	}

	for i := len(pref) - 1; i >= 0; i-- {
		matches := pref[i]

		for tupleIndex, tuple := range sortable {
			key := tuple.Variant.Keys[i]
			currentScore := len(matches)

			var ks string

			switch key := key.(type) {
			case ast.CatchAllKey:
				sortable[tupleIndex].Score = currentScore
				continue
			case ast.NameLiteral, ast.QuotedLiteral:
				ks = key.String()
			case ast.NumberLiteral:
				ks = fmt.Sprint(key)
			}

			currentScore = slices.Index(matches, ks)

			sortable[tupleIndex].Score = currentScore
		}

		sort.Sort(SortableVariants(sortable))
	}

	return sortable
}

func (e *executer) selectBestVariant(sortable []SortableVariant) error {
	// Select the best variant
	if err := e.resolvePattern(sortable[0].Variant.QuotedPattern); err != nil {
		return fmt.Errorf("resolve pattern: %w", err)
	}

	return nil
}

func matchSelectorKeys(rv any, keys []string) []string {
	value, ok := rv.(string)
	if !ok {
		return nil
	}

	var matches []string

	for _, key := range keys {
		if key == value {
			matches = append(matches, key)
		}
	}

	return matches
}

type SortableVariant struct {
	Variant ast.Variant
	Score   int
}

type SortableVariants []SortableVariant

func (s SortableVariants) Len() int {
	return len(s)
}

func (s SortableVariants) Less(i, j int) bool {
	return s[i].Score < s[j].Score
}

func (s SortableVariants) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
