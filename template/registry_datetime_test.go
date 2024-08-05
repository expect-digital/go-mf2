package template

import (
	"testing"
	"time"

	"golang.org/x/text/language"
)

var testDate = time.Date(2021, 1, 2, 3, 4, 5, 6, time.UTC)

func Test_Datetime(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input   any
		options map[string]any
		name    string
		want    string
		wantErr bool
	}{
		// positive tests
		{
			// {$d :datetime} is the same as {$d :datetime dateStyle=medium timeStyle=short}
			name:  "no options",
			input: testDate,
			want:  "02 Jan 2021 03:04",
		},
		{
			name:    "dateStyle",
			input:   testDate,
			options: map[string]any{"dateStyle": "full"},
			want:    "Saturday, 02 January 2021",
		},
		{
			name:    "timeStyle",
			input:   testDate,
			options: map[string]any{"timeStyle": "medium"},
			want:    "03:04:05",
		},
		{
			name:    "dateStyle and timeStyle",
			input:   testDate,
			options: map[string]any{"dateStyle": "short", "timeStyle": "long"},
			want:    "02/01/21 03:04:05 +0000",
		},
		{
			name:    "timeZone",
			input:   testDate,
			options: map[string]any{"timeStyle": "long", "dateStyle": "medium", "timeZone": "EET"},
			want:    "02 Jan 2021 05:04:05 +0200",
		},
		// negative tests
		{
			name:    "not implemented",
			input:   testDate,
			options: map[string]any{"calendar": "buddhist"},
			wantErr: true,
		},
		{
			name:    "illegal type",
			input:   struct{}{},
			options: nil,
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			v, err := datetimeFunc(test.input, test.options, language.AmericanEnglish)

			if test.wantErr {
				if err == nil {
					t.Error("want error, got nil")
				}

				return
			}

			if err != nil {
				t.Error(err)
			}

			got := v.format()
			if test.want != got {
				t.Errorf("want '%s', got '%s'", test.want, got)
			}
		})
	}
}
