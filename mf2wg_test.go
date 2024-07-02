package mf2_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"testing"

	"go.expect.digital/mf2"
	"go.expect.digital/mf2/template"
	"golang.org/x/text/language"
)

var failing []string

func init() {
	//nolint:lll
	failing = []string{
		"TestMF2WG/.message-format-wg/test/tests/data-model-errors.json/.input_{$foo}_.input_{$foo}_{{_}}",
		"TestMF2WG/.message-format-wg/test/tests/data-model-errors.json/.input_{$foo}_.local_$foo_=_{42}_{{_}}",
		"TestMF2WG/.message-format-wg/test/tests/data-model-errors.json/.local_$foo_=_{:unknown}_.local_$foo_=_{42}_{{_}}",
		"TestMF2WG/.message-format-wg/test/tests/data-model-errors.json/.local_$foo_=_{$foo}_{{_}}",
		"TestMF2WG/.message-format-wg/test/tests/data-model-errors.json/.local_$foo_=_{42_:func_opt=$foo}_{{_}}",
		"TestMF2WG/.message-format-wg/test/tests/data-model-errors.json/.local_$foo_=_{$bar}_.local_$bar_=_{42}_{{_}}",
		"TestMF2WG/.message-format-wg/test/tests/data-model-errors.json/.local_$foo_=_{$bar}_.local_$bar_=_{$baz}_{{_}}",
		"TestMF2WG/.message-format-wg/test/tests/data-model-errors.json/.local_$foo_=_{42_:func_opt=$bar}_.local_$bar_=_{42}_{{_}}",
		"TestMF2WG/.message-format-wg/test/tests/data-model-errors.json/.local_$foo_=_{$bar_:func}_.local_$bar_=_{$baz}_{{_}}",
		"TestMF2WG/.message-format-wg/test/tests/data-model-errors.json/.match_{$foo_:x}_*_*_{{foo}}",
		"TestMF2WG/.message-format-wg/test/tests/data-model-errors.json/.match_{:foo}_{:bar}_*_1_{{_}}_1_*_{{_}}",
		"TestMF2WG/.message-format-wg/test/tests/data-model-errors.json/.match_{:foo}_1_{{_}}",
		"TestMF2WG/.message-format-wg/test/tests/data-model-errors.json/.match_{:foo}_other_{{_}}",
		"TestMF2WG/.message-format-wg/test/tests/data-model-errors.json/.match_{$foo_:x}_{$bar_:x}_*_{{foo}}",

		"TestMF2WG/.message-format-wg/test/tests/functions/date.json/{horse_:date}",
		"TestMF2WG/.message-format-wg/test/tests/functions/date.json/{:date}",
		"TestMF2WG/.message-format-wg/test/tests/functions/date.json/{|2006-01-02T15:04:06|_:date}",
		"TestMF2WG/.message-format-wg/test/tests/functions/date.json/{|2006-01-02|_:date}",
		"TestMF2WG/.message-format-wg/test/tests/functions/date.json/.local_$t_=_{|2006-01-02T15:04:06|_:time}_{{{$t_:date}}}",
		"TestMF2WG/.message-format-wg/test/tests/functions/date.json/.local_$d_=_{|2006-01-02|_:date_style=long}_{{{$d_:date}}}",
		"TestMF2WG/.message-format-wg/test/tests/functions/date.json/{|2006-01-02|_:date_style=long}",

		"TestMF2WG/.message-format-wg/test/tests/functions/datetime.json/{:datetime}",
		"TestMF2WG/.message-format-wg/test/tests/functions/datetime.json/{horse_:datetime}",
		"TestMF2WG/.message-format-wg/test/tests/functions/datetime.json/{|2006-01-02T15:04:06|_:datetime_year=numeric_month=|2-digit|}",
		"TestMF2WG/.message-format-wg/test/tests/functions/datetime.json/{$x_:datetime}",

		"TestMF2WG/.message-format-wg/test/tests/functions/integer.json/.match_{$foo_:integer}_one_{{one}}_*_{{other}}",
		"TestMF2WG/.message-format-wg/test/tests/functions/integer.json/hello_{0.42e+1_:integer}",
		"TestMF2WG/.message-format-wg/test/tests/functions/integer.json/hello_{4.2_:integer}",
		"TestMF2WG/.message-format-wg/test/tests/functions/integer.json/hello_{-4.20_:integer}",

		"TestMF2WG/.message-format-wg/test/tests/functions/string.json/.match_{$foo_:string}_1_{{one}}_*_{{other}}#02",

		"TestMF2WG/.message-format-wg/test/tests/functions/time.json/.local_$d_=_{|2006-01-02T15:04:06|_:date}_{{{$d_:time}}}",
		"TestMF2WG/.message-format-wg/test/tests/functions/time.json/{|2006-01-02T15:04:06|_:time}",
		"TestMF2WG/.message-format-wg/test/tests/functions/time.json/{:time}",
		"TestMF2WG/.message-format-wg/test/tests/functions/time.json/{horse_:time}",
		"TestMF2WG/.message-format-wg/test/tests/functions/time.json/.local_$t_=_{|2006-01-02T15:04:06|_:time_style=medium}_{{{$t_:time}}}",
		"TestMF2WG/.message-format-wg/test/tests/functions/time.json/{|2006-01-02T15:04:06|_:time_style=medium}",

		"TestMF2WG/.message-format-wg/test/tests/functions/number.json/invalid_number_literal_{01_:number}",
		"TestMF2WG/.message-format-wg/test/tests/functions/number.json/.input_{$bar}_.match_{$bar_:number}_one_{{one}}_*_{{other}}#03",
		"TestMF2WG/.message-format-wg/test/tests/functions/number.json/invalid_number_literal_{0x1_:number}",
		"TestMF2WG/.message-format-wg/test/tests/functions/number.json/.input_{$foo_:number_minimumFractionDigits=foo}_{{bar_{$foo}}}",
		"TestMF2WG/.message-format-wg/test/tests/functions/number.json/.input_{$bar}_.match_{$bar_:number}_one_{{one}}_*_{{other}}#02",
		"TestMF2WG/.message-format-wg/test/tests/functions/number.json/.local_$bar_=_{$none}_.match_{$foo_:number}_one_{{one}}_*_{{{$bar}}}#01",
		"TestMF2WG/.message-format-wg/test/tests/functions/number.json/.match_{$foo_:number}_{$bar_:number}_one_one_{{one_one}}_one_*_{{one_other}}_*_*_{{other}}",
		"TestMF2WG/.message-format-wg/test/tests/functions/number.json/.local_$bar_=_{$none}_.match_{$foo_:number}_one_{{one}}_*_{{{$bar}}}",
		"TestMF2WG/.message-format-wg/test/tests/functions/number.json/.input_{$foo_:number}_{{bar_{$foo}}}#01",
		"TestMF2WG/.message-format-wg/test/tests/functions/number.json/hello_{:number}",
		"TestMF2WG/.message-format-wg/test/tests/functions/number.json/hello_{|4.2|_:number_minimumFractionDigits=|2|}",
		"TestMF2WG/.message-format-wg/test/tests/functions/number.json/.match_{$foo_:number}_one_{{one}}_*_{{other}}",
		"TestMF2WG/.message-format-wg/test/tests/functions/number.json/.input_{$bar}_.match_{$bar_:number}_one_{{one}}_*_{{other}}",
		"TestMF2WG/.message-format-wg/test/tests/functions/number.json/.input_{$none}_.match_{$foo_:number}_one_{{one}}_*_{{{$none}}}",
		"TestMF2WG/.message-format-wg/test/tests/functions/number.json/invalid_number_literal_{.1_:number}",
		"TestMF2WG/.message-format-wg/test/tests/functions/number.json/invalid_number_literal_{|+1|_:number}",
		"TestMF2WG/.message-format-wg/test/tests/functions/number.json/hello_{foo_:number}",
		"TestMF2WG/.message-format-wg/test/tests/functions/number.json/.input_{$foo_:number}_.local_$bar_=_{$foo}_.match_{$bar}_one_{{one}}_*_{{other}}",
		"TestMF2WG/.message-format-wg/test/tests/functions/number.json/invalid_number_literal_{1._:number}",
		"TestMF2WG/.message-format-wg/test/tests/functions/number.json/.match_{$foo_:number}_{$bar_:number}_one_one_{{one_one}}_one_*_{{one_other}}_*_*_{{other}}#02",
		"TestMF2WG/.message-format-wg/test/tests/functions/number.json/.match_{$foo_:number}_one_{{one}}_1_{{=1}}_*_{{other}}",
		"TestMF2WG/.message-format-wg/test/tests/functions/number.json/.input_{$bar_:number}_.match_{$bar}_one_{{one}}_*_{{other}}",
		"TestMF2WG/.message-format-wg/test/tests/functions/number.json/.input_{$foo_:number}_.match_{$foo}_one_{{one}}_*_{{other}}",
		"TestMF2WG/.message-format-wg/test/tests/functions/number.json/.input_{$bar}_.match_{$bar_:number}_one_{{one}}_*_{{other}}#01",
		"TestMF2WG/.message-format-wg/test/tests/functions/number.json/.local_$foo_=_{$bar_:number}_{{bar_{$foo}}}#01",
		"TestMF2WG/.message-format-wg/test/tests/functions/number.json/.local_$foo_=_{$bar_:number_minimumFractionDigits=foo}_{{bar_{$foo}}}",
		"TestMF2WG/.message-format-wg/test/tests/functions/number.json/.local_$foo_=_{$bar_:number}_.match_{$foo}_one_{{one}}_*_{{other}}",
		"TestMF2WG/.message-format-wg/test/tests/functions/number.json/.match_{$foo_:number}_{$bar_:number}_one_one_{{one_one}}_one_*_{{one_other}}_*_*_{{other}}#01",
		"TestMF2WG/.message-format-wg/test/tests/functions/number.json/.match_{$foo_:number}_1_{{=1}}_one_{{one}}_*_{{other}}",
		"TestMF2WG/.message-format-wg/test/tests/functions/number.json/hello_{|4.2|_:number_minimumFractionDigits=$foo}",
	}
}

// TestMF2WG runs tests by Message Format Working Group.
func TestMF2WG(t *testing.T) {
	t.Parallel()

	err := filepath.Walk(".message-format-wg/test/tests", func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		t.Run(path, func(t *testing.T) {
			t.Parallel()

			f, err := os.Open(path)
			if err != nil {
				t.Error(err)
			}

			defer f.Close()

			var tests Tests

			if err := json.NewDecoder(f).Decode(&tests); err != nil {
				t.Error(err)
			}

			for _, test := range tests.Tests {
				t.Run(test.Src, func(t *testing.T) {
					t.Parallel()

					if slices.Contains(failing, t.Name()) {
						t.Skip()
					}

					run(t, test.Apply(tests.DefaultTestProperties))
				})
			}
		})

		return nil
	})
	if err != nil {
		t.Error(err)
	}
}

// run runs single test by Message Format Working Group.
func run(t *testing.T, test Test) {
	var options []template.Option
	if test.Locale != nil {
		options = append(options, template.WithLocale(*test.Locale))
	}

	templ, err := template.New(options...).Parse(test.Src)
	// The implementation returns error in two places:
	// - when parsing the template
	// - when executing the template
	//
	// This affects asserting the error. We do not know if expected error is parsing
	// or executing the template.
	//
	// If the test expects parse error but parsing does not return error,
	// then the test tries to assert when executing error.
	if err != nil {
		assertErr(t, test.ExpErrors, err)

		return
	}

	input := make(map[string]interface{}, len(test.Params))

	for _, v := range test.Params {
		input[v.Name] = v.Value
	}

	got, err := templ.Sprint(input)

	// Look at the description of first assertWgErr() in this func.
	assertErr(t, test.ExpErrors, err)

	// Expected is optional. The built-in formatters is implementation
	// specific across programming languages and libraries.
	if test.Exp != nil && *test.Exp != got {
		t.Errorf("want %s, got %s", *test.Exp, got)
	}
}

// Tests contains harness tests by MF2 WG, schema defined in
// ".message-format-wg/spec/schemas/v0/tests.schema.json".
type Tests struct {
	Tests                 []Test                `json:"tests"`
	DefaultTestProperties DefaultTestProperties `json:"defaultTestProperties"`
}

type Test struct {
	// The MF2 message to be tested.
	Src string `json:"src"`
	// The locale to use for formatting. Defaults to 'en-US'.
	Locale *language.Tag `json:"locale"`
	// Parameters to pass in to the formatter for resolving external variables.
	Params []Var `json:"params"`
	// The expected result of formatting the message to a string.
	Exp *string `json:"exp"`
	// The expected result of formatting the message to parts.
	Parts []any `json:"parts"`
	// A normalixed form of `src`, for testing stringifiers.
	CleanSrc string `json:"cleanSrc"`
	// The runtime errors expected to be emitted when formatting the message.
	ExpErrors Errors `json:"expErrors"`
}

// Apply applies default properties to the test.
func (t Test) Apply(defaultProperties DefaultTestProperties) Test {
	if t.ExpErrors.Expected == nil {
		t.ExpErrors.Expected = defaultProperties.ExpErrors.Expected
	}

	if len(t.ExpErrors.Errors) == 0 {
		t.ExpErrors.Errors = defaultProperties.ExpErrors.Errors
	}

	if t.Locale == nil {
		t.Locale = defaultProperties.Locale
	}

	return t
}

type Errors struct {
	Expected *bool
	Errors   []Error
}

func (e *Errors) UnmarshalJSON(data []byte) error {
	switch {
	default: // parse bool
		v, err := strconv.ParseBool(string(data))
		if err != nil {
			return fmt.Errorf("want bool: %w", err)
		}

		e.Expected = &v

		return nil
	case len(data) == 0:
		return nil
	case data[0] == '[': // parse errors slice
		if err := json.Unmarshal(data, &e.Errors); err != nil {
			return fmt.Errorf("want slice: %w", err)
		}

		return nil
	}
}

type DefaultTestProperties struct {
	Locale    *language.Tag `json:"locale"`
	ExpErrors Errors        `json:"expErrors"`
}

type Error struct {
	Type string `json:"type"`
}

type Var struct {
	Name  string `json:"name"`
	Value any    `json:"value"`
	Type  string `json:"type"`
}

func assertErr(t *testing.T, want Errors, err error) {
	// we expect error but we don't know the exact error
	if want.Expected != nil && *want.Expected && err == nil {
		t.Error("want error, got nil")
	}

	if len(want.Errors) == 0 && err != nil {
		t.Errorf("want no error, got %s", err)
	}

	wantErr := func(want error) {
		if !errors.Is(err, want) {
			t.Errorf("want %s, got %s", want, err)
		}
	}

	for _, v := range want.Errors {
		switch v.Type {
		default:
			t.Errorf("asserting error %s is not implemented", v)
		case "bad-operand":
			wantErr(mf2.ErrBadOperand)
		case "duplicate-declaration":
			wantErr(mf2.ErrDuplicateDeclaration)
		case "duplicate-option-name":
			wantErr(mf2.ErrDuplicateOptionName)
		case "missing-fallback-variant":
			wantErr(mf2.ErrMissingFallbackVariant)
		case "missing-selector-annotation":
			wantErr(mf2.ErrMissingSelectorAnnotation)
		case "unsupported-expression":
			wantErr(mf2.ErrUnsupportedExpression)
		case "unsupported-statement":
			wantErr(mf2.ErrUnsupportedStatement)
		case "unresolved-variable":
			wantErr(mf2.ErrUnresolvedVariable)
		case "syntax-error":
			wantErr(mf2.ErrSyntax)
		case "variant-key-mismatch":
			wantErr(mf2.ErrVariantKeyMismatch)
		}
	}
}
