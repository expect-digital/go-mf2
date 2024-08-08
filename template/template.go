package template

import (
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"golang.org/x/exp/slices"
	"golang.org/x/text/language"

	"go.expect.digital/mf2"
	ast "go.expect.digital/mf2/parse"
)

// Template represents a MessageFormat2 template.
type Template struct {
	// TODO: locale field. Can change the output of some functions.
	// e.g. number formatting, given example { $num :number }:
	//  - "en-US" -> 1,234.56
	//  - "lv-LV" -> 1234,56
	// e.g. date formatting, given example { $date :datetime }:
	//  - "en-US" -> 1/2/2023
	//  - "lv-LV" -> 2.1.2023
	ast      *ast.AST
	registry Registry
	locale   language.Tag
}

// ResolvedValue keeps the result of the Expression resolution with optionally
// defined format() and selectKey() functions for Format and Select contexts.
type ResolvedValue struct {
	value     any
	selectKey func(keys []string) string
	format    func() string
	err       error
}

func defaultFormat(value any) string {
	switch v := value.(type) {
	default:
		// TODO(jhorsts): if underlying type is not string, return errorf("unsupported value type: %T: %w", r.value, err)
		s, _ := v.(string)
		return s
	case fmt.Stringer:
		return v.String()
	case string, []byte, []rune, int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64, float32, float64, bool,
		complex64, complex128, error:
		return fmt.Sprint(v)
	}
}

func defaultSelectKey(value any, keys []string) string {
	v := defaultFormat(value)

	for _, k := range keys {
		if v == k {
			return k
		}
	}

	return ast.CatchAllKey{}.String()
}

// String makes the ResolvedValue implement the fmt.Stringer interface.
func (r *ResolvedValue) String() string {
	if r.format != nil {
		return r.format()
	}

	return defaultFormat(r.value)
}

// ResolvedValueOpt is a function to apply to the ResolvedValue.
type ResolvedValueOpt func(*ResolvedValue)

// WithFormat applies a custom format() function to the ResolvedValue.
func WithFormat(format func() string) ResolvedValueOpt {
	return func(r *ResolvedValue) {
		r.format = format
	}
}

// WithSelectKey applies a custom format() function to the ResolvedValue.
func WithSelectKey(selectKey func(keys []string) string) ResolvedValueOpt {
	return func(r *ResolvedValue) {
		r.selectKey = selectKey
	}
}

// NewResolvedValue creates a new variable of type *ResolvedValue.
// If value is already *ResolvedValue, the optional format() and selectKey() are applied to it.
func NewResolvedValue(value any, options ...ResolvedValueOpt) *ResolvedValue {
	r, ok := value.(*ResolvedValue)
	if !ok {
		r = &ResolvedValue{
			value:     value,
			format:    func() string { return defaultFormat(value) },
			selectKey: func(keys []string) string { return defaultSelectKey(value, keys) },
		}
	}

	for _, f := range options {
		f(r)
	}

	return r
}

func newFallbackValue(value any) *ResolvedValue {
	var s string

	switch v := value.(type) {
	default:
		return NewResolvedValue("")
	case ast.ReservedAnnotation:
		s = string(v.Start)
	case ast.PrivateUseAnnotation:
		s = string(v.Start)
	case ast.Function:
		s = ":" + v.Identifier.String()
	case ast.Variable:
		s = v.String()
	case ast.NameLiteral:
		s = ast.QuotedLiteral(v).String()
	case ast.NumberLiteral:
		s = ast.QuotedLiteral(v.String()).String()
	case ast.QuotedLiteral:
		s = v.String()
	case ast.Expression:
		if v.Operand != nil {
			return newFallbackValue(v.Operand)
		}

		return newFallbackValue(v.Annotation)
	case *ResolvedValue:
		return newFallbackValue(v.value)
	}

	return NewResolvedValue("{" + s + "}")
}

// New returns a new Template.
func New(options ...Option) *Template {
	t := &Template{
		registry: NewRegistry(),
		locale:   language.AmericanEnglish,
	}

	for _, o := range options {
		o(t)
	}

	return t
}

// Option is a template option.
type Option func(t *Template)

// WithFunc adds a single function to function registry.
func WithFunc(name string, f Func) Option {
	return func(t *Template) {
		t.registry[name] = f
	}
}

// WithFuncs adds functions to function registry.
func WithFuncs(reg Registry) Option {
	return func(t *Template) {
		for k, f := range reg {
			t.registry[k] = f
		}
	}
}

// WithLocale adds locale information to the template.
func WithLocale(locale language.Tag) Option {
	return func(t *Template) {
		t.locale = locale
	}
}

// Parse parses the MessageFormat2 string and returns the template.
func (t *Template) Parse(input string) (*Template, error) {
	ast, err := ast.Parse(input)
	if err != nil {
		return nil, err //nolint:wrapcheck
	}

	t.ast = &ast

	return t, nil
}

// Execute writes the result of the template to the given writer.
func (t *Template) Execute(w io.Writer, input map[string]any) error {
	if t.ast == nil {
		return errors.New("execute template: AST is nil")
	}

	executer := &executer{template: t, w: w, variables: make(map[string]*ResolvedValue, len(input))}

	for k, v := range input {
		var f Func

		switch v.(type) {
		default:
			executer.variables[k] = NewResolvedValue(v, WithFormat(func() string { return defaultFormat(v) }))
			continue
		case string:
			f = stringFunc
		case float64, int:
			f = numberFunc
		}

		r, err := f(NewResolvedValue(v), nil, t.locale)
		if err != nil {
			return fmt.Errorf("execute template: %w", err)
		}

		executer.variables[k] = r
	}

	if err := executer.execute(); err != nil {
		return fmt.Errorf("execute template: %w", err)
	}

	return nil
}

// Sprint wraps Execute and returns the result as a string.
func (t *Template) Sprint(input map[string]any) (string, error) {
	sb := new(strings.Builder)
	err := t.Execute(sb, input)

	return sb.String(), err
}

type executer struct {
	template  *Template
	w         io.Writer
	variables map[string]*ResolvedValue
}

func (e *executer) execute() error {
	switch message := e.template.ast.Message.(type) {
	default: // this should never happen, AST must be valid.
		return fmt.Errorf("unexpected message type: '%T'", message)
	case nil:
		return nil
	case ast.SimpleMessage:
		return e.resolvePattern(message)
	case ast.ComplexMessage:
		return e.resolveComplexMessage(message)
	}
}

func (e *executer) resolveComplexMessage(message ast.ComplexMessage) error {
	var resolutionErr error

	err := e.resolveDeclarations(message.Declarations)

	switch {
	case errors.Is(err, mf2.ErrUnsupportedStatement),
		errors.Is(err, mf2.ErrUnresolvedVariable),
		errors.Is(err, mf2.ErrBadOperand),
		errors.Is(err, mf2.ErrBadOption):
		resolutionErr = fmt.Errorf("complex message: %w", err)
	case err != nil:
		return fmt.Errorf("complex message: %w", err)
	}

	switch b := message.ComplexBody.(type) {
	case ast.Matcher:
		err = e.resolveMatcher(b)
	case ast.QuotedPattern:
		err = e.resolvePattern(b)
	}

	if err != nil {
		return errors.Join(resolutionErr, fmt.Errorf("complex message: %w", err))
	}

	return resolutionErr
}

func (e *executer) resolveDeclarations(declarations []ast.Declaration) error {
	for _, decl := range declarations {
		switch d := decl.(type) {
		case ast.ReservedStatement:
			return fmt.Errorf("%w", mf2.ErrUnsupportedStatement)
		case ast.LocalDeclaration:
			r, err := e.resolveExpression(d.Expression)
			if err != nil {
				r.err = errors.Join(r.err, fmt.Errorf("resolve local %s: %w", d.Variable, err))
			}

			e.variables[string(d.Variable)] = r // newVariable(d.Expression, nil)
		case ast.InputDeclaration:
			r, err := e.resolveExpression(ast.Expression(d))
			if err != nil {
				r.err = errors.Join(r.err, fmt.Errorf("resolve input %s: %w", d.Operand, err))
			}

			e.variables[string(d.Operand.(ast.Variable))] = r //nolint: forcetypeassert // always ast.Variable
		}
	}

	return nil
}

func (e *executer) resolvePattern(pattern []ast.PatternPart) error {
	var resolutionErr error

	errorf := func(format string, args ...any) error {
		return errors.Join(resolutionErr, fmt.Errorf("pattern: "+format, args...))
	}

	for _, part := range pattern {
		switch v := part.(type) {
		case ast.Text:
			if _, err := e.w.Write([]byte(v)); err != nil {
				return errorf("write text: %w", err)
			}
		case ast.Expression:
			resolved, err := e.resolveExpression(v)
			if err != nil {
				resolutionErr = errors.Join(resolutionErr, err)
			}

			if _, err := e.w.Write([]byte(resolved.String())); err != nil {
				return errorf("write resolved expression: %w", err)
			}
		// When formatting to a string, markup placeholders format to an empty string by default.
		// See ".message-format-wg/exploration/open-close-placeholders.md#formatting-to-a-string"
		case ast.Markup:
		}
	}

	return resolutionErr
}

func (e *executer) resolveExpression(expr ast.Expression) (*ResolvedValue, error) {
	var (
		funcName      string
		options       Options
		resolutionErr error
	)

	value, err := e.resolveValue(expr.Operand)
	if err != nil {
		resolutionErr = errors.Join(resolutionErr, fmt.Errorf("expression: %w", err))
	}

	switch v := expr.Annotation.(type) {
	default:
		return newFallbackValue(v), fmt.Errorf(`expression: %T annotation "%s": %w`, v, v, mf2.ErrUnsupportedExpression)
	case ast.Function:
		funcName = v.Identifier.Name

		if options, err = e.resolveOptions(v.Options); err != nil {
			return newFallbackValue(v), fmt.Errorf("expression: %w", err)
		}
	case ast.PrivateUseAnnotation:
		// See ".message-format-wg/spec/formatting.md".
		//
		// Supported private-use annotation with no operand: the annotation starting sigil, optionally followed by
		// implementation-defined details conforming with patterns in the other cases (such as quoting literals).
		// If details are provided, they SHOULD NOT leak potentially private information.
		resolutionErr = fmt.Errorf(`expression: private use annotation "%s": %w`, v, mf2.ErrUnsupportedExpression)

		if value == nil {
			return newFallbackValue(v), resolutionErr
		}
	case ast.ReservedAnnotation:
		resolutionErr = fmt.Errorf(`expression: reserved annotation "%s": %w`, v, mf2.ErrUnsupportedExpression)

		if value == nil {
			return newFallbackValue(v), resolutionErr
		}
	case nil: // noop, no annotation
	}

	if funcName == "" {
		switch t := value.(type) {
		default: // TODO(jhorsts): how is unknown type formatted?
			return newFallbackValue(value), resolutionErr
		case *ResolvedValue:
			// the expression has already been resolved before
			return t, resolutionErr
		case string:
			funcName = "string"
		case float64:
			funcName = "number"
		}
	}

	f, ok := e.template.registry[funcName] // TODO(jhorsts): lookup by namespace and name
	if !ok {
		err = fmt.Errorf(`expression: %w "%s"`, mf2.ErrUnknownFunction, funcName)
		return newFallbackValue(expr), errors.Join(resolutionErr, err)
	}

	result, err := f(NewResolvedValue(value), options, e.template.locale)
	if err != nil {
		return newFallbackValue(expr), errors.Join(resolutionErr, fmt.Errorf("expression: %w", err))
	}

	return result, resolutionErr
}

// resolveValue resolves the value of an expression's operand.
//
//   - If the operand is a literal, it returns the literal's value.
//   - If the operand is a variable, it returns the value of the variable from the input map.
func (e *executer) resolveValue(v ast.Value) (any, error) {
	switch v := v.(type) {
	default: // this should never happen, should be cought by lexer/parser.
		return nil, fmt.Errorf(`unknown value type "%T"`, v)
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
			return newFallbackValue(v), fmt.Errorf(`%w "%s"`, mf2.ErrUnresolvedVariable, v)
		}

		return val, val.err
	}
}

func (e *executer) resolveOptions(options []ast.Option) (Options, error) {
	m := make(Options, len(options))

	for _, opt := range options {
		name := opt.Identifier.Name
		if _, ok := m[name]; ok {
			return nil, fmt.Errorf(`%w "%s"`, mf2.ErrDuplicateOptionName, name)
		}

		value, err := e.resolveValue(opt.Value)
		if err != nil {
			return nil, fmt.Errorf("option: %w", err)
		}

		m[name] = NewResolvedValue(value)
	}

	return m, nil
}

func (e *executer) resolveMatcher(m ast.Matcher) error {
	res, matcherErr := e.resolveSelector(m)

	switch {
	case errors.Is(matcherErr, mf2.ErrUnknownFunction),
		errors.Is(matcherErr, mf2.ErrUnresolvedVariable): // noop
	case matcherErr != nil:
		return fmt.Errorf("matcher: %w", matcherErr)
	}

	pref := e.resolvePreferences(m, res)

	filteredVariants := e.filterVariants(m, pref)

	err := e.resolvePattern(e.bestMatchedPattern(filteredVariants, pref))
	if err != nil {
		return errors.Join(matcherErr, fmt.Errorf("matcher: %w", err))
	}

	return matcherErr
}

func (e *executer) hasAnnotation(operand ast.Value) bool {
	m, ok := e.template.ast.Message.(ast.ComplexMessage)
	if !ok {
		return false
	}

	for _, decl := range m.Declarations {
		switch v := decl.(type) {
		default:
			return false
		case ast.LocalDeclaration:
			if v.Variable.String() != operand.String() {
				continue
			}

			if v.Expression.Annotation != nil {
				return true
			}

			return e.hasAnnotation(v.Expression.Operand)
		case ast.InputDeclaration:
			if v.Operand.String() != operand.String() {
				continue
			}

			return v.Annotation != nil
		}
	}

	return false
}

func (e *executer) resolveSelector(matcher ast.Matcher) ([]any, error) {
	var selectorErr error

	selectors := make([]any, 0, len(matcher.Selectors))

	addErr := func(err error) {
		selectorErr = errors.Join(selectorErr, fmt.Errorf("selector: %w", err))

		selectors = append(selectors, ast.CatchAllKey{})
	}

	for _, selector := range matcher.Selectors {
		var function ast.Function

		switch annotation := selector.Annotation.(type) {
		case nil:
			if !e.hasAnnotation(selector.Operand) {
				return nil, mf2.ErrMissingSelectorAnnotation
			}

			input, err := e.resolveValue(selector.Operand)
			if err != nil {
				addErr(err)
				continue
			}

			v, ok := input.(*ResolvedValue)
			if !ok {
				addErr(mf2.ErrBadOperand)
			}

			selectors = append(selectors, v.selectKey([]string{}))

			continue
		case ast.ReservedAnnotation, ast.PrivateUseAnnotation:
			return nil, mf2.ErrUnsupportedExpression
		case ast.Function:
			function = annotation
		}

		f, ok := e.template.registry[function.Identifier.Name]
		if !ok {
			addErr(fmt.Errorf(`%w "%s"`, mf2.ErrUnknownFunction, function.Identifier.Name))
			continue
		}

		opts, err := e.resolveOptions(function.Options)
		if err != nil {
			addErr(err)
			continue
		}

		input, err := e.resolveValue(selector.Operand)
		if err != nil {
			addErr(err)
			continue
		}

		rslt, err := f(NewResolvedValue(input), opts, e.template.locale)
		if err != nil {
			addErr(err)
			continue
		}

		selectors = append(selectors, rslt)
	}

	return selectors, selectorErr
}

func (e *executer) resolvePreferences(m ast.Matcher, res []any) [][]string {
	// Step 2: Resolve Preferences
	pref := make([][]string, 0, len(res))

	for i := range res {
		var keys []string

		for _, variant := range m.Variants {
			for _, vKey := range variant.Keys {
				// NOTE(mvilks): since collected keys will be compared to the selector,
				//	we need the keys's raw string value, not the representation of it
				//  e.g. the `1` should be equal to `|1|`
				switch key := vKey.(type) {
				case ast.CatchAllKey:
					continue
				case ast.QuotedLiteral:
					keys = append(keys, string(key))
				case ast.NameLiteral:
					keys = append(keys, string(key))
				case ast.NumberLiteral:
					keys = append(keys, key.String())
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

			// NOTE(mvilks): since collected keys will be compared to the selector,
			//	we need the keys's raw string value, not the representation of it
			//  e.g. the `1` should be equal to `|1|`
			switch key := key.(type) {
			case ast.CatchAllKey:
				continue
			case ast.QuotedLiteral:
				ks = string(key)
			case ast.NameLiteral:
				ks = string(key)
			case ast.NumberLiteral:
				ks = key.String()
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

func (e *executer) bestMatchedPattern(filteredVariants []ast.Variant, pref [][]string) ast.QuotedPattern {
	// Step 4: Sort Variants
	sortable := make([]sortableVariant, 0, len(filteredVariants))

	for _, variant := range filteredVariants {
		sortable = append(sortable, sortableVariant{Score: -1, Variant: variant})
	}

	for i := len(pref) - 1; i >= 0; i-- {
		matches := pref[i]

		for tupleIndex, tuple := range sortable {
			key := tuple.Variant.Keys[i]
			currentScore := len(matches)

			var ks string

			// NOTE(mvilks): since collected keys will be compared to the selector,
			//	we need the keys's raw string value, not the representation of it
			//  e.g. the `1` should be equal to `|1|`
			switch key := key.(type) {
			case ast.CatchAllKey:
				sortable[tupleIndex].Score = currentScore
				continue
			case ast.QuotedLiteral:
				ks = string(key)
			case ast.NameLiteral:
				ks = string(key)
			case ast.NumberLiteral:
				ks = key.String()
			}

			currentScore = slices.Index(matches, ks)

			sortable[tupleIndex].Score = currentScore
		}

		sort.Sort(sortableVariants(sortable))
	}

	return sortable[0].Variant.QuotedPattern
}

func matchSelectorKeys(rv any, keys []string) []string {
	if v, ok := rv.(*ResolvedValue); ok {
		rv = v.selectKey(keys)
	}

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

type sortableVariant struct {
	Variant ast.Variant
	Score   int
}

type sortableVariants []sortableVariant

func (s sortableVariants) Len() int {
	return len(s)
}

func (s sortableVariants) Less(i, j int) bool {
	return s[i].Score < s[j].Score
}

func (s sortableVariants) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
