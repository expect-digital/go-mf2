package template

import (
	"fmt"

	"golang.org/x/text/language"
)

// integerFunc is the implementation of the integer function. Locale-sensitive integer formatting.
func integerFunc(operand *ResolvedValue, options Options, locale language.Tag) (*ResolvedValue, error) {
	if options == nil {
		options = Options{"maximumFractionDigits": NewResolvedValue(0)}
	} else {
		options["maximumFractionDigits"] = NewResolvedValue(0)
	}

	value, err := numberFunc(operand, options, locale)
	if err != nil {
		return nil, fmt.Errorf("exec integer func: %w", err)
	}

	return value, nil
}
