package template

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"
)

var testDate = time.Date(2021, 1, 2, 3, 4, 5, 6, time.UTC)

func Test_Datetime(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input       any
		options     map[string]any
		expected    any
		name        string
		expectedErr bool
	}{
		// positive tests
		{
			name:     "no options",
			input:    testDate,
			expected: "2021-01-02 03:04:05.000000006 +0000 UTC",
		},
		{
			name:     "dateStyle",
			input:    testDate,
			options:  map[string]any{"dateStyle": "full"},
			expected: "Saturday, 02 January 2021",
		},
		{
			name:     "timeStyle",
			input:    testDate,
			options:  map[string]any{"timeStyle": "medium"},
			expected: "03:04",
		},
		{
			name:     "dateStyle and timeStyle",
			input:    testDate,
			options:  map[string]any{"dateStyle": "short", "timeStyle": "long"},
			expected: "02/01/21 03:04:05",
		},
		{
			name:     "timeZone",
			input:    testDate,
			options:  map[string]any{"timeStyle": "long", "dateStyle": "medium", "timeZone": "EET"},
			expected: "02 Jan 2021 05:04:05",
		},
		{
			name:     "year",
			input:    testDate,
			options:  map[string]any{"year": "2-digit"},
			expected: "21",
		},
		// negative tests
		{
			name:        "not implemented",
			input:       testDate,
			options:     map[string]any{"calendar": "buddhist"},
			expectedErr: true,
		},
		{
			name:        "illegal type",
			input:       struct{}{},
			options:     nil,
			expectedErr: true,
		},
		{
			name:        "has style options AND field options",
			input:       testDate,
			options:     map[string]any{"dateStyle": "short", "year": "2-digit"},
			expectedErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			actual, err := datetimeRegistryFunc.Format(test.input, test.options, language.AmericanEnglish)

			if test.expectedErr {
				require.Error(t, err)

				return
			}

			require.NoError(t, err)
			require.Equal(t, test.expected, actual)
		})
	}
}
