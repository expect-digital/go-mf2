package registry

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_Number(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input       any
		options     map[string]any
		expected    any
		name        string
		expectedErr bool
	}{
		// positive
		{
			name:     "int",
			input:    53,
			expected: float64(53),
		},
		{
			name:     "style",
			input:    0.23,
			options:  map[string]any{"style": "percent"},
			expected: "23.00%",
		},
		{
			name:     "signDisplay and percent style",
			input:    0.23,
			options:  map[string]any{"style": "percent", "signDisplay": "always"},
			expected: "+23.00%",
		},
		// negative
		{
			name:        "not implemented",
			input:       0.23,
			options:     map[string]any{"compactDisplay": "short"},
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

			actual, err := numberRegistryF.Format(tt.input, tt.options)

			if tt.expectedErr {
				require.Error(t, err)
				require.Empty(t, actual)

				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.expected, actual)
		})
	}
}
