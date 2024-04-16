package registry

import (
	"errors"
	"fmt"

	"golang.org/x/text/language"
)

// https://github.com/unicode-org/message-format-wg/blob/20a61b4af534acb7ecb68a3812ca0143b34dfc76/spec/registry.xml#L259

// stringRegistryFunc is the implementation of the string function.
// Formatting of strings as a literal and selection based on string equality.
var stringRegistryFunc = Func{
	Format: stringFunc,
	Match:  stringFunc,
}

func stringFunc(input any, options Options, locale language.Tag) (any, error) {
	if input == nil {
		return "", errors.New("string function requires input, got nil")
	}

	if len(options) > 0 {
		return "", errors.New("string function takes no options")
	}

	switch value := input.(type) {
	default:
		s, err := castAs[string](input) // if underlying type is not string, return error
		if err != nil {
			return nil, fmt.Errorf("unsupported input type in string function: %T: %w", input, err)
		}

		return s, nil
	case fmt.Stringer:
		return value.String(), nil
	case string, []byte, []rune, int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64, float32, float64, bool,
		complex64, complex128, error:
		return fmt.Sprint(value), nil
	}
}
