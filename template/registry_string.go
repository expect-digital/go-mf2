package template

import (
	"fmt"

	"golang.org/x/text/language"
)

// stringFunc is the implementation of the string function.
// Formatting of strings as a literal and selection based on string equality.
func stringFunc(operand *ResolvedValue, options Options, _ language.Tag) (*ResolvedValue, error) {
	errorf := func(format string, args ...any) (*ResolvedValue, error) {
		return nil, fmt.Errorf("exec string function: "+format, args...)
	}

	if operand.value == nil {
		return newFallbackValue(""), nil
	}

	if len(options) > 0 {
		return errorf("want no options")
	}

	return operand, nil
}
