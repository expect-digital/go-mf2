package registry

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
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
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			actual, err := datetimeRegistryF.Format(tt.input, tt.options)

			if tt.expectedErr {
				require.Error(t, err)

				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.expected, actual)
		})
	}
}
