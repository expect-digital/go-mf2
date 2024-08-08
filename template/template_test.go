package template

import (
	"errors"
	"testing"

	"go.expect.digital/mf2"
	"golang.org/x/text/language"
)

func Test_ExecuteSimpleMessage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input      map[string]any
		funcs      Registry // format functions to be added before executing
		name, text string
		want       string
	}{
		{
			name: "empty message",
		},
		{
			name: "plain message",
			text: "Hello, World!",
			want: "Hello, World!",
		},
		{
			name:  "variables and literals",
			text:  "Hello, { $name } { unquoted } { |quoted| } { 42 }!",
			input: map[string]any{"name": "World"},
			want:  "Hello, World unquoted quoted 42!",
		},
		{
			name: "functions with operand",
			text: "Hello, { $firstName :string } your age is { $age :number style=decimal }!",
			input: map[string]any{
				"firstName": "John",
				"age":       23,
			},
			want: "Hello, John your age is 23!",
		},
		{
			name: "function without operand",
			text: "Hello, { :randName }",
			funcs: Registry{
				"randName": func(*ResolvedValue, Options, language.Tag) (*ResolvedValue, error) {
					return NewResolvedValue("John", WithFormat(func() string { return "John" })), nil
				},
			},
			want: "Hello, John",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			template, err := New(WithFuncs(test.funcs)).Parse(test.text)
			if err != nil {
				t.Error(err)
			}

			got, err := template.Sprint(test.input)
			if err != nil {
				t.Error(err)
			}

			if test.want != got {
				t.Errorf("want '%s', got '%s'", test.want, got)
			}
		})
	}
}

func Test_ExecuteComplexMessage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		inputs     map[string]any
		funcs      Registry // format functions to be added before executing
		name, text string
		want       string
	}{
		{
			name: "complex message without declaration",
			text: "{{Hello, {|literal|} World!}}",
			want: "Hello, literal World!",
		},
		{
			name: "local declarations",
			text: `.local $var1 = { literalExpression }
		.local $var2 = { $anotherVar }
		.local $var3 = { :randNum }
		{{Hello, {$var1} {$var2} {$var3}!}}`,
			inputs: map[string]any{"anotherVar": "World"},
			funcs: Registry{
				"randNum": func(*ResolvedValue, Options, language.Tag) (*ResolvedValue, error) {
					return NewResolvedValue(0, WithFormat(func() string { return "0" })), nil
				},
			},
			want: "Hello, literalExpression World 0!",
		},
		{
			name:   "input declaration",
			text:   ".input { $name :string } {{Hello, {$name}!}}",
			inputs: map[string]any{"name": 999},
			want:   "Hello, 999!",
		},
		{
			name: "markup",
			text: "Click {#link href=$url}here{/link} standalone {#foo/}",
			want: "Click here standalone ",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			template, err := New(WithFuncs(test.funcs)).Parse(test.text)
			if err != nil {
				t.Error(err)
			}

			got, err := template.Sprint(test.inputs)
			if err != nil {
				t.Error(err)
			}

			if test.want != got {
				t.Errorf("want '%s', got '%s'", test.want, got)
			}
		})
	}
}

func Test_Matcher(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		text   string
		inputs []map[string]any
		want   []string
	}{
		{
			name: "matcher string",
			text: `.match { $n :string } no {{no apples}} one {{{ $n } apple}} * {{{ $n } apples}}`,
			inputs: []map[string]any{
				{"n": "no"},
				{"n": "one"},
				{"n": "many"},
			},
			want: []string{"no apples", "one apple", "many apples"},
		},
		{
			name: "Pattern Selection with string annotation",
			//nolint:dupword
			text: ".match {$foo :string} {$bar :string} bar bar {{All bar}} foo foo {{All foo}} * * {{Otherwise}}",
			inputs: []map[string]any{
				{"foo": "foo", "bar": "bar"},
			},
			want: []string{"Otherwise"},
		},
		{
			name:   "Pattern Selection with Multiple Variants",
			text:   ".match {$foo :string} {$bar :string} * bar {{Any and bar}}foo * {{Foo and any}} foo bar {{Foo and bar}} * * {{Otherwise}}", //nolint:lll
			inputs: []map[string]any{{"foo": "foo", "bar": "bar"}},
			want:   []string{"Foo and bar"},
		},
		{
			name:   "Plural Format Selection",
			text:   ".match {$count :string} one {{Category match}} 1 {{Exact match}} *   {{Other match}}",
			inputs: []map[string]any{{"count": "1"}},
			want:   []string{"Exact match"},
		},
	}

	for _, test := range tests {
		if len(test.want) != len(test.inputs) {
			t.Errorf("want len %d, got %d", len(test.want), len(test.inputs))
		}

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			template, err := New().Parse(test.text)
			if err != nil {
				t.Error(err)
			}

			for i, inputMap := range test.inputs {
				t.Run(test.want[i], func(t *testing.T) {
					t.Parallel()

					got, err := template.Sprint(inputMap)
					if err != nil {
						t.Error(err)
					}

					if test.want[i] != got {
						t.Errorf("want '%s' at %d, got '%s'", test.want[i], i, got)
					}
				})
			}
		})
	}
}

func Test_ExecuteErrors(t *testing.T) {
	t.Parallel()

	type want struct {
		parseErr, execErr error
		text              string
	}

	tests := []struct {
		input      map[string]any
		funcs      Registry
		name, text string
		want       want
	}{
		{
			name: "syntax error",
			text: "Hello { $name",
			want: want{parseErr: mf2.ErrSyntax},
		},
		{
			name: "unresolved variable",
			text: "Hello, { $name }!",
			want: want{execErr: mf2.ErrUnresolvedVariable, text: "Hello, {$name}!"},
		},
		{
			name: "unknown function",
			text: "Hello, { :f }!",
			want: want{execErr: mf2.ErrUnknownFunction, text: "Hello, {:f}!"},
		},
		{
			name: "duplicate option name",
			text: "Hello, { :number style=decimal style=percent }!",
			want: want{execErr: mf2.ErrDuplicateOptionName, text: "Hello, {:number}!"},
		},
		{
			name: "unsupported expression",
			text: "Hello, { 12 ^private }!",
			want: want{execErr: mf2.ErrUnsupportedExpression, text: "Hello, 12!"},
		},
		{
			name: "unsupported declaration",
			text: ".reserved { name } {{Hello!}}",
			want: want{execErr: mf2.ErrUnsupportedStatement, text: "Hello!"},
		},
		{
			name:  "duplicate input declaration",
			text:  ".input {$var} .input {$var} {{Redeclaration of the same variable}}",
			input: map[string]any{"var": "22"},
			want:  want{parseErr: mf2.ErrDuplicateDeclaration},
		},
		{
			name:  "duplicate input and local declaration",
			text:  ".local $var = {$ext} .input {$var} {{Redeclaration of a local variable}}",
			input: map[string]any{"ext": "22"},
			want:  want{parseErr: mf2.ErrDuplicateDeclaration},
		},
		{
			name:  "Selection Error No Annotation",
			text:  ".match {$n} 0 {{no apples}} 1 {{apple}} * {{apples}}",
			input: map[string]any{"n": "1"},
			want:  want{execErr: mf2.ErrMissingSelectorAnnotation},
		},
		{
			name:  "Selection with Reversed Annotation",
			text:  ".match {$count ^string} one {{Category match}} 1 {{Exact match}} *   {{Other match}}",
			input: map[string]any{"count": "1"},
			want:  want{execErr: mf2.ErrUnsupportedExpression},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			template, err := New(WithFuncs(test.funcs)).Parse(test.text)
			if test.want.parseErr != nil {
				if !errors.Is(err, test.want.parseErr) {
					t.Errorf("want '%s', got '%s'", test.want.parseErr, err)
				}

				return
			}

			text, err := template.Sprint(test.input)
			if !errors.Is(err, test.want.execErr) {
				t.Errorf("want '%s', got '%s'", test.want.execErr, err)
			}

			if test.want.text != text {
				t.Errorf("want '%s', got '%s'", test.want.text, text)
			}
		})
	}
}

func BenchmarkTemplate_Sprint(b *testing.B) {
	//nolint:dupword
	tmpl, err := New().Parse(".match {$foo :string} {$bar :number} one one {{one one}} one * {{one other}} * * {{other}}")
	if err != nil {
		b.Error(err)
	}

	_, err = tmpl.Sprint(map[string]any{"foo": "foo", "bar": 1})
	if err != nil {
		b.Error(err)
	}

	var result string

	for range b.N {
		result, _ = tmpl.Sprint(map[string]any{"foo": "foo", "bar": 1})
	}

	_ = result
}
