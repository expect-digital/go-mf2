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
		"TestMF2WG/.message-format-wg/test/tests/functions/datetime.json/{|2006-01-02T15:04:06|_:datetime_year=numeric_month=|2-digit|}",

		"TestMF2WG/.message-format-wg/test/tests/syntax.json/{0E-1}#01",
		"TestMF2WG/.message-format-wg/test/tests/syntax.json/{0E1}",
		"TestMF2WG/.message-format-wg/test/tests/syntax.json/{0E-1}",
		"TestMF2WG/.message-format-wg/test/tests/syntax.json/{0e1}",
		"TestMF2WG/.message-format-wg/test/tests/syntax.json/{-0}",
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
		t.Errorf("want '%s', got '%s'", *test.Exp, got)
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
		t.Errorf("want no error, got '%s'", err)
	}

	wantErr := func(want error) {
		if !errors.Is(err, want) {
			t.Errorf("want error '%s', got '%s'", want, err)
		}
	}

	for _, v := range want.Errors {
		switch v.Type {
		default:
			t.Errorf("asserting error '%s' is not implemented", v)
		case "bad-operand":
			wantErr(mf2.ErrBadOperand)
		case "bad-option":
			wantErr(mf2.ErrBadOption)
		case "duplicate-declaration":
			wantErr(mf2.ErrDuplicateDeclaration)
		case "duplicate-option-name":
			wantErr(mf2.ErrDuplicateOptionName)
		case "missing-fallback-variant":
			wantErr(mf2.ErrMissingFallbackVariant)
		case "missing-selector-annotation":
			wantErr(mf2.ErrMissingSelectorAnnotation)
		case "syntax-error":
			wantErr(mf2.ErrSyntax)
		case "unknown-function":
			wantErr(mf2.ErrUnknownFunction)
		case "unresolved-variable":
			wantErr(mf2.ErrUnresolvedVariable)
		case "unsupported-expression":
			wantErr(mf2.ErrUnsupportedExpression)
		case "unsupported-statement":
			wantErr(mf2.ErrUnsupportedStatement)
		case "variant-key-mismatch":
			wantErr(mf2.ErrVariantKeyMismatch)
		}
	}
}
