package parse

import (
	"strings"
	"testing"
)

func TestParseSimpleMessage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		want  Message
		input string
	}{
		{
			name:  "text only",
			input: "Hello, World!",
			want: SimpleMessage{
				Text("Hello, World!"),
			},
		},
		{
			name:  "text only with escaped chars",
			input: "Hello, \\{World!\\}",
			want: SimpleMessage{
				Text("Hello, {World!}"),
			},
		},
		{
			name:  "variable expression in the middle",
			input: "Hello, { $variable } World!",
			want: SimpleMessage{
				Text("Hello, "),
				Expression{Operand: Variable("variable")},
				Text(" World!"),
			},
		},
		{
			name:  "variable expression at the start",
			input: "{ $variable } Hello, World!",
			want: SimpleMessage{
				Expression{Operand: Variable("variable")},
				Text(" Hello, World!"),
			},
		},
		{
			name:  "variable expression at the end",
			input: "Hello, World! { $variable }",
			want: SimpleMessage{
				Text("Hello, World! "),
				Expression{Operand: Variable("variable")},
			},
		},
		{
			name:  "variable expression with annotation",
			input: "Hello, { $variable :function }  World!",
			want: SimpleMessage{
				Text("Hello, "),
				Expression{
					Operand: Variable("variable"),
					Annotation: Function{
						Identifier: Identifier{
							Namespace: "",
							Name:      "function",
						},
					},
				},
				Text("  World!"),
			},
		},
		{
			name:  "variable expression with annotation options and attributes",
			input: "Hello, { $variable :function option1 = -3.14 ns:option2 = |value2| option3 = $variable2 @attr1 = attr1} World!", //nolint:lll
			want: SimpleMessage{
				Text("Hello, "),
				Expression{
					Operand: Variable("variable"),
					Annotation: Function{
						Identifier: Identifier{
							Namespace: "",
							Name:      "function",
						},
						Options: []Option{
							{
								Value: NumberLiteral(-3.14),
								Identifier: Identifier{
									Namespace: "",
									Name:      "option1",
								},
							},
							{
								Value: QuotedLiteral("value2"),
								Identifier: Identifier{
									Namespace: "ns",
									Name:      "option2",
								},
							},
							{
								Value: Variable("variable2"),
								Identifier: Identifier{
									Namespace: "",
									Name:      "option3",
								},
							},
						},
					},
					Attributes: []Attribute{
						{
							Value:      NameLiteral("attr1"),
							Identifier: Identifier{Name: "attr1"},
						},
					},
				},
				Text(" World!"),
			},
		},
		{
			name:  "quoted literal expression",
			input: "Hello, { |literal| }  World!",
			want: SimpleMessage{
				Text("Hello, "),
				Expression{Operand: QuotedLiteral("literal")},
				Text("  World!"),
			},
		},
		{
			name:  "unquoted scientific notation number literal expression",
			input: "Hello, { 1e3 }  World!",
			want: SimpleMessage{
				Text("Hello, "),
				Expression{Operand: NumberLiteral(1e3)},
				Text("  World!"),
			},
		},
		{
			name:  "unquoted name literal expression",
			input: "Hello, { name } World!",
			want: SimpleMessage{
				Text("Hello, "),
				Expression{Operand: NameLiteral("name")},
				Text(" World!"),
			},
		},
		{
			name:  "quoted name literal expression with annotation",
			input: "Hello, { |name| :function } World!",
			want: SimpleMessage{
				Text("Hello, "),
				Expression{
					Operand: QuotedLiteral("name"),
					Annotation: Function{
						Identifier: Identifier{
							Namespace: "",
							Name:      "function",
						},
					},
				},
				Text(" World!"),
			},
		},
		{
			name:  "quoted name literal expression with annotation and options",
			input: "Hello, { |name| :function ns1:option1 = -1 ns2:option2 = 1 option3 = |value3| } World!",
			want: SimpleMessage{
				Text("Hello, "),
				Expression{
					Operand: QuotedLiteral("name"),
					Annotation: Function{
						Identifier: Identifier{
							Namespace: "",
							Name:      "function",
						},
						Options: []Option{
							{
								Value: NumberLiteral(-1),
								Identifier: Identifier{
									Namespace: "ns1",
									Name:      "option1",
								},
							},
							{
								Value: NumberLiteral(+1),
								Identifier: Identifier{
									Namespace: "ns2",
									Name:      "option2",
								},
							},
							{
								Value: QuotedLiteral("value3"),
								Identifier: Identifier{
									Namespace: "",
									Name:      "option3",
								},
							},
						},
					},
				},
				Text(" World!"),
			},
		},
		{
			name:  "function expression",
			input: "Hello { :function } World!",
			want: SimpleMessage{
				Text("Hello "),
				Expression{
					Annotation: Function{
						Identifier: Identifier{
							Namespace: "",
							Name:      "function",
						},
					},
				},
				Text(" World!"),
			},
		},
		{
			name:  "function expression with options and namespace",
			input: "Hello { :namespace:function namespace:option999 = 999 } World!",
			want: SimpleMessage{
				Text("Hello "),
				Expression{
					Annotation: Function{
						Identifier: Identifier{
							Namespace: "namespace",
							Name:      "function",
						},
						Options: []Option{
							{
								Value: NumberLiteral(999),
								Identifier: Identifier{
									Namespace: "namespace",
									Name:      "option999",
								},
							},
						},
					},
				},
				Text(" World!"),
			},
		},
		{
			name:  "private use and reserved annotation",
			input: `Hello { $hey ^private }{ !|reserved| \|hey\| \{ @v @k=2 @l:l=$s} World!`,
			want: SimpleMessage{
				Text("Hello "),
				Expression{
					Operand: Variable("hey"),
					Annotation: PrivateUseAnnotation{
						Start: '^',
						ReservedBody: []ReservedBody{
							ReservedText("private"),
						},
					},
				},
				Expression{
					Annotation: ReservedAnnotation{
						Start: '!',
						ReservedBody: []ReservedBody{
							QuotedLiteral("reserved"),
							ReservedText("|hey|"),
							ReservedText("{"),
						},
					},
					Attributes: []Attribute{
						{
							Identifier: Identifier{Name: "v"},
						},
						{
							Identifier: Identifier{Name: "k"},
							Value:      NumberLiteral(2),
						},
						{
							Identifier: Identifier{Namespace: "l", Name: "l"},
							Value:      Variable("s"),
						},
					},
				},
				Text(" World!"),
			},
		},
		{
			name:  "markup",
			input: `It is a {#button opt1=val1 @attr1=val1 } button { /button } this is a { #br /} something else, {#ns:tag1}{#tag2}text{ #img /}{/tag2}{/ns:tag1}`, //nolint:lll
			want: SimpleMessage{
				// 1. Open-Close markup
				Text("It is a "),
				Markup{
					Typ: Open,
					Identifier: Identifier{
						Namespace: "",
						Name:      "button",
					},
					Options: []Option{
						{
							Value: NameLiteral("val1"),
							Identifier: Identifier{
								Name: "opt1",
							},
						},
					},
					Attributes: []Attribute{
						{
							Value:      NameLiteral("val1"),
							Identifier: Identifier{Name: "attr1"},
						},
					},
				},
				Text(" button "),
				Markup{Typ: Close, Identifier: Identifier{Name: "button"}},
				// 2. Self-close markup
				Text(" this is a "),
				Markup{Typ: SelfClose, Identifier: Identifier{Name: "br"}},
				Text(" something else, "),
				// 3. Nested markup
				Markup{Typ: Open, Identifier: Identifier{Namespace: "ns", Name: "tag1"}},
				Markup{Typ: Open, Identifier: Identifier{Name: "tag2"}},
				Text("text"),
				Markup{Typ: SelfClose, Identifier: Identifier{Name: "img"}},
				Markup{Typ: Close, Identifier: Identifier{Name: "tag2"}},
				Markup{Typ: Close, Identifier: Identifier{Namespace: "ns", Name: "tag1"}},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, err := Parse(test.input)
			if err != nil {
				t.Error(err)
			}

			// Check that AST message is equal to expected one.
			if test.want.String() != got.Message.String() {
				t.Errorf("want %v, got %v", test.want, got.Message)
			}

			// Check that AST message converted back to string is equal to input.

			// Edge case: scientific notation number is converted to normal notation, hence comparison is bound to fail.
			// I.E. input string has 1e3, output string has 1000.
			if test.name == "unquoted scientific notation number literal expression" {
				return
			}

			// If strings already match, we're done.
			// Otherwise check both sanitized strings.
			if got := got.String(); got != test.input {
				requireEqualMF2String(t, test.input, got)
			}
		})
	}
}

func TestParseComplexMessage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		want  Message
		input string
	}{
		{
			name:  "no declarations",
			input: "{{Hello, { |literal| } World!}}",
			want: ComplexMessage{
				Declarations: nil,
				ComplexBody: QuotedPattern{
					Text("Hello, "),
					Expression{Operand: QuotedLiteral("literal")},
					Text(" World!"),
				},
			},
		},
		{
			name:  "trailing whitespace",
			input: "{{Hello, World!}}\n\t ",
			want: ComplexMessage{
				Declarations: nil,
				ComplexBody: QuotedPattern{
					Text("Hello, World!"),
				},
			},
		},
		//nolint:dupword
		{
			name: "all declarations",
			input: `.input{$input :number @a}
.input { $input2 ^|quot| @b=c}
.input { $input3 ! hey hey @c=1 @d=2}
.local $local1={1}
.local $local2={|2| ^private @a @b=2}
.local $local3 = { > reserved}
.reserved1 {$reserved1}
.reserved2 hey |quot| hey { |reserved| :func }
.reserved3 |body| |body2| {$expr1} {|expr2|} { :expr3 } { $expr4 ^hey @beep @boop}
{{Text}}`,
			want: ComplexMessage{
				ComplexBody: QuotedPattern{Text("Text")},
				Declarations: []Declaration{
					// .input{$input :number @a}
					InputDeclaration{
						Operand:    Variable("input"),
						Annotation: Function{Identifier: Identifier{Name: "number"}},
						Attributes: []Attribute{{Identifier: Identifier{Name: "a"}}},
					},
					// .input { $input2 ^|quot| @b=c}
					InputDeclaration{
						Operand: Variable("input2"),
						Annotation: PrivateUseAnnotation{
							Start:        '^',
							ReservedBody: []ReservedBody{QuotedLiteral("quot")},
						},
						Attributes: []Attribute{{Identifier: Identifier{Name: "b"}, Value: NameLiteral("c")}},
					},
					// .input { $input3 ! hey hey @c=1 @d=2}
					InputDeclaration{
						Operand: Variable("input3"),
						Annotation: ReservedAnnotation{
							Start: '!',
							ReservedBody: []ReservedBody{
								ReservedText("hey"),
								ReservedText("hey"),
							},
						},
						Attributes: []Attribute{
							{Identifier: Identifier{Name: "c"}, Value: NumberLiteral(1)},
							{Identifier: Identifier{Name: "d"}, Value: NumberLiteral(2)},
						},
					},
					// .local $local1={1}
					LocalDeclaration{
						Variable:   Variable("local1"),
						Expression: Expression{Operand: NumberLiteral(1)},
					},
					// .local $local2={|2| ^private @a @b=2}
					LocalDeclaration{
						Variable: Variable("local2"),
						Expression: Expression{
							Operand: QuotedLiteral("2"),
							Annotation: PrivateUseAnnotation{
								Start:        '^',
								ReservedBody: []ReservedBody{ReservedText("private")},
							},
							Attributes: []Attribute{
								{Identifier: Identifier{Name: "a"}},
								{Identifier: Identifier{Name: "b"}, Value: NumberLiteral(2)},
							},
						},
					},
					// .local $local3 = { > reserved}
					LocalDeclaration{
						Variable: Variable("local3"),
						Expression: Expression{
							Annotation: ReservedAnnotation{
								Start:        '>',
								ReservedBody: []ReservedBody{ReservedText("reserved")},
							},
						},
					},
					// .reserved1 {$reserved1}
					ReservedStatement{
						Keyword: "reserved1",
						Expressions: []Expression{
							{Operand: Variable("reserved1")},
						},
					},
					// .reserved2 hey |quot| hey { |reserved| :func }
					ReservedStatement{
						Keyword: "reserved2",
						ReservedBody: []ReservedBody{
							ReservedText("hey"),
							QuotedLiteral("quot"),
							ReservedText("hey"),
						},
						Expressions: []Expression{
							{
								Operand:    QuotedLiteral("reserved"),
								Annotation: Function{Identifier: Identifier{Name: "func"}},
							},
						},
					},
					// .reserved3 |body| |body2| {$expr1} {|expr2|} { :expr3 } { $expr4 ^hey @beep @boop}
					ReservedStatement{
						Keyword: "reserved3",
						ReservedBody: []ReservedBody{
							QuotedLiteral("body"),
							QuotedLiteral("body2"),
						},
						Expressions: []Expression{
							{Operand: Variable("expr1")},
							{Operand: QuotedLiteral("expr2")},
							{Annotation: Function{Identifier: Identifier{Name: "expr3"}}},
							{
								Operand: Variable("expr4"),
								Annotation: PrivateUseAnnotation{
									Start:        '^',
									ReservedBody: []ReservedBody{ReservedText("hey")},
								},
								Attributes: []Attribute{
									{Identifier: Identifier{Name: "beep"}},
									{Identifier: Identifier{Name: "boop"}},
								},
							},
						},
					},
				},
			},
		},
		// Matcher
		{
			name:  "simple matcher one line",
			input: ".match { $variable :number } 1 {{Hello { $variable} world}} * {{Hello { $variable } worlds}}",
			want: ComplexMessage{
				Declarations: nil,
				ComplexBody: Matcher{
					MatchStatements: []Expression{
						{
							Operand: Variable("variable"),
							Annotation: Function{
								Identifier: Identifier{Namespace: "", Name: "number"},
							},
						},
					},
					Variants: []Variant{
						{
							Keys: []VariantKey{NumberLiteral(1)},
							QuotedPattern: QuotedPattern{
								Text("Hello "),
								Expression{Operand: Variable("variable")},
								Text(" world"),
							},
						},
						{
							Keys: []VariantKey{CatchAllKey{}},
							QuotedPattern: QuotedPattern{
								Text("Hello "),
								Expression{Operand: Variable("variable")},
								Text(" worlds"),
							},
						},
					},
				},
			},
		},
		{
			name: "simple matcher with newline variants",
			input: `.match { $variable :number }
1 {{Hello { $variable } world}}
* {{Hello { $variable } worlds}}`,
			want: ComplexMessage{
				Declarations: nil,
				ComplexBody: Matcher{
					MatchStatements: []Expression{
						{
							Operand: Variable("variable"),
							Annotation: Function{
								Identifier: Identifier{Namespace: "", Name: "number"},
							},
						},
					},
					Variants: []Variant{
						{
							Keys: []VariantKey{NumberLiteral(1)},
							QuotedPattern: QuotedPattern{
								Text("Hello "),
								Expression{Operand: Variable("variable")},
								Text(" world"),
							},
						},
						{
							Keys: []VariantKey{CatchAllKey{}},
							QuotedPattern: QuotedPattern{
								Text("Hello "),
								Expression{Operand: Variable("variable")},
								Text(" worlds"),
							},
						},
					},
				},
			},
		},
		{
			name: "simple matcher with newline variants in one line",
			input: `.match { $variable :number }

1 {{Hello { $variable} world}}* {{Hello { $variable } worlds}}`,
			want: ComplexMessage{
				Declarations: nil,
				ComplexBody: Matcher{
					MatchStatements: []Expression{
						{
							Operand: Variable("variable"),
							Annotation: Function{
								Identifier: Identifier{Namespace: "", Name: "number"},
							},
						},
					},
					Variants: []Variant{
						{
							Keys: []VariantKey{NumberLiteral(1)},
							QuotedPattern: QuotedPattern{
								Text("Hello "),
								Expression{Operand: Variable("variable")},
								Text(" world"),
							},
						},
						{
							Keys: []VariantKey{CatchAllKey{}},
							QuotedPattern: QuotedPattern{
								Text("Hello "),
								Expression{Operand: Variable("variable")},
								Text(" worlds"),
							},
						},
					},
				},
			},
		},
		{
			name: "matcher with declarations",
			input: `.local $var1 = { male }
.local $var2 = { |female| }
.match { :gender }
male {{Hello sir!}}
|female| {{Hello madam!}}
* {{Hello { $var1 } or { $var2 }!}}`,
			want: ComplexMessage{
				Declarations: []Declaration{
					LocalDeclaration{
						Variable:   Variable("var1"),
						Expression: Expression{Operand: NameLiteral("male")},
					},
					LocalDeclaration{
						Variable:   Variable("var2"),
						Expression: Expression{Operand: QuotedLiteral("female")},
					},
				},
				ComplexBody: Matcher{
					MatchStatements: []Expression{
						{
							Annotation: Function{
								Identifier: Identifier{
									Namespace: "",
									Name:      "gender",
								},
							},
						},
					},
					Variants: []Variant{
						{
							Keys: []VariantKey{NameLiteral("male")},
							QuotedPattern: QuotedPattern{
								Text("Hello sir!"),
							},
						},
						{
							Keys: []VariantKey{QuotedLiteral("female")},
							QuotedPattern: QuotedPattern{
								Text("Hello madam!"),
							},
						},
						{
							Keys: []VariantKey{CatchAllKey{}},
							QuotedPattern: QuotedPattern{
								Text("Hello "),
								Expression{Operand: Variable("var1")},
								Text(" or "),
								Expression{Operand: Variable("var2")},
								Text("!"),
							},
						},
					},
				},
			},
		},
		{
			name: "double matcher",
			//nolint:dupword
			input: `.match { $var1 } { $var2 }
yes yes {{Hello beautiful world!}}
yes no {{Hello beautiful!}}
no yes {{Hello world!}}
no no {{Hello!}}`,
			want: ComplexMessage{
				Declarations: nil,
				ComplexBody: Matcher{
					MatchStatements: []Expression{
						{Operand: Variable("var1")},
						{Operand: Variable("var2")},
					},
					Variants: []Variant{
						{
							Keys: []VariantKey{
								NameLiteral("yes"),
								NameLiteral("yes"),
							},
							QuotedPattern: QuotedPattern{
								Text("Hello beautiful world!"),
							},
						},
						{
							Keys: []VariantKey{
								NameLiteral("yes"),
								NameLiteral("no"),
							},
							QuotedPattern: QuotedPattern{
								Text("Hello beautiful!"),
							},
						},
						{
							Keys: []VariantKey{
								NameLiteral("no"),
								NameLiteral("yes"),
							},
							QuotedPattern: QuotedPattern{
								Text("Hello world!"),
							},
						},
						{
							Keys: []VariantKey{
								NameLiteral("no"),
								NameLiteral("no"),
							},
							QuotedPattern: QuotedPattern{
								Text("Hello!"),
							},
						},
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, err := Parse(test.input)
			if err != nil {
				t.Error(err)
			}

			// Check that AST message is equal to expected one.
			if test.want.String() != got.Message.String() {
				t.Errorf("want %s, got %s", test.want, got.Message)
			}

			// Check that AST message converted back to string is equal to input.

			// If strings already match, we're done.
			// Otherwise check both sanitized strings.
			if got := got.String(); got != test.input {
				requireEqualMF2String(t, test.input, got)
			}
		})
	}
}

func TestParseErrors(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		in, wantErr string
	}{
		{
			in:      ".match {|foo| :x} {|bar| :x} ** {{foo}}",
			wantErr: "parse MF2: complex message: matcher: variant keys: missing space between keys * and *",
		},
		{
			in:      ".match {|foo| :x} {|bar| :x} *1 {{foo}}",
			wantErr: "parse MF2: complex message: matcher: variant keys: missing space between keys * and 1",
		},
	} {
		t.Run(test.in, func(t *testing.T) {
			t.Parallel()

			if _, err := Parse(test.in); err == nil || err.Error() != test.wantErr {
				t.Errorf("want %s, got %s", test.wantErr, err)
			}
		})
	}
}

// TestValidate tests negative cases for AST validation. Positive cases are covered by TestParse* tests.
func TestValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		ast       AST
		name      string
		errorPath string // path to the failing field (simplified to last n fields)
	}{
		{
			name:      "No message",
			ast:       AST{Message: nil},
			errorPath: "ast",
		},
		{
			// Hello, { $ } World!
			name: "Variable expression empty variable name",
			ast: AST{
				Message: SimpleMessage{
					Text("Hello, "),
					Expression{Operand: Variable("")},
				},
			},
			errorPath: "expression.variable",
		},
		{
			// Hello, { $variable : } World!
			name: "Variable expression with annotation empty function name",
			ast: AST{
				Message: SimpleMessage{
					Text("Hello, "),
					Expression{
						Operand: Variable("variable"),
						Annotation: Function{
							Identifier: Identifier{
								Namespace: "",
								Name:      "",
							},
						},
					},
				},
			},
			errorPath: "function.identifier",
		},
		{
			// Hello, { } World!
			name: "Empty annotation expression",
			ast: AST{
				Message: SimpleMessage{
					Text("Hello, "),
					Expression{},
					Text(" World!"),
				},
			},
			errorPath: "simpleMessage.expression",
		},
		{
			// .input { $ } {{Hello, World!}}
			name: "Empty variable in input declaration",
			ast: AST{
				Message: ComplexMessage{
					Declarations: []Declaration{
						InputDeclaration{Operand: Variable("")},
					},
					ComplexBody: QuotedPattern{
						Text("Hello, World!"),
					},
				},
			},
			errorPath: "inputDeclaration.expression",
		},
		{
			// .local $var = {  } {{Hello, World!}}
			name: "Empty expression in local declaration",
			ast: AST{
				Message: ComplexMessage{
					Declarations: []Declaration{
						LocalDeclaration{
							Variable:   Variable("var"),
							Expression: Expression{},
						},
					},
					ComplexBody: QuotedPattern{
						Text("Hello, World!"),
					},
				},
			},
			errorPath: "complexMessage.localDeclaration",
		},
		{
			// .match { } 1 {{Hello, World!}}
			name: "Empty expression in matcher",
			ast: AST{
				Message: ComplexMessage{
					Declarations: nil,
					ComplexBody: Matcher{
						MatchStatements: nil,
						Variants: []Variant{
							{
								Keys: []VariantKey{NumberLiteral(1)},
								QuotedPattern: QuotedPattern{
									Text("Hello, World!"),
								},
							},
						},
					},
				},
			},
			errorPath: "complexMessage.matcher",
		},
		{
			// .match { $variable }
			name: "Matcher without variants",
			ast: AST{
				Message: ComplexMessage{
					Declarations: nil,
					ComplexBody: Matcher{
						MatchStatements: []Expression{
							{Operand: Variable("variable")},
						},
						Variants: nil,
					},
				},
			},
			errorPath: "complexMessage.matcher",
		},
		{
			// .match { $variable } {{Hello world}}
			name: "Matcher without variant key",
			ast: AST{
				Message: ComplexMessage{
					ComplexBody: Matcher{
						MatchStatements: []Expression{
							{Operand: Variable("variable")},
						},
						Variants: []Variant{
							{
								Keys:          []VariantKey{},
								QuotedPattern: QuotedPattern{Text("Hello world")},
							},
						},
					},
				},
			},
			errorPath: "matcher.variant",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if test.errorPath == "" {
				t.Error("test.errorPath is not set")
			}

			err := test.ast.validate()
			if !strings.Contains(err.Error(), test.errorPath) {
				t.Errorf("want %s, got %s", test.errorPath, err)
			}
		})
	}
}

// helpers

// requireEqualMF2String compares two strings, but ignores whitespace, tabs, and newlines.
func requireEqualMF2String(t *testing.T, want, got string) {
	t.Helper()

	r := strings.NewReplacer(
		"\n", "",
		"\t", "",
		" ", "",
	)

	if r.Replace(want) != r.Replace(got) {
		t.Errorf("want %s, got %s", want, got)
	}
}
