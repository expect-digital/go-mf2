package registry

import (
	"errors"
	"fmt"
	"strings"
)

type Registry map[string]Func

// Func is a function that can be used in formatting and matching contexts.
type Func struct {
	FormatSignature *Signature                             // Signature of the function when called in formatting context
	MatchSignature  *Signature                             // Signature of the function when called in matching context
	Fn              func(any, map[string]any) (any, error) // Function itself
	Name            string
	Description     string
}

// Signature is a signature of the function, i.e. what input and options are allowed.
type Signature struct {
	ValidateInput   func(any) error
	Options         Options
	IsInputRequired bool
}

// Option is a possible options for the function.
type Option struct {
	Name        string
	Description string
	Default     any

	ValidateValue  func(any) error // If option value is not restricted by a set of values.
	PossibleValues []any           // If option value is restricted by a set of values.
}

type Options []Option

// New returns a new registry with default functions.
func New() Registry {
	return Registry{
		"string":   *stringRegistryF,
		"number":   *numberRegistryF,
		"datetime": *datetimeRegistryF,
	}
}

func (f *Func) Format(input any, options map[string]any) (any, error) {
	if f.FormatSignature == nil {
		return "", fmt.Errorf("function '%s' is not allowed to use in formatting context", f.Name)
	}

	if err := f.FormatSignature.check(input, options); err != nil {
		return "", fmt.Errorf("check input: %w", err)
	}

	return f.Fn(input, options)
}

func (f *Func) Match(input any, options map[string]any) (any, error) {
	if f.MatchSignature == nil {
		return "", fmt.Errorf("function '%s' is not allowed to use in selector context", f.Name)
	}

	if err := f.MatchSignature.check(input, options); err != nil {
		return "", fmt.Errorf("check input: %w", err)
	}

	return f.Fn(input, options)
}

func (s *Signature) check(input any, options map[string]any) error {
	if s.IsInputRequired && input == nil {
		return errors.New("input is required, got nil")
	}

	if !s.IsInputRequired && input != nil {
		return errors.New("input is not allowed")
	}

	if len(s.Options) == 0 && len(options) > 0 {
		return errors.New("options are not allowed")
	}

	if s.ValidateInput != nil {
		if err := s.ValidateInput(input); err != nil {
			return fmt.Errorf("validate input: %w", err)
		}
	}

	if err := s.Options.check(options); err != nil {
		return fmt.Errorf("check options: %w", err)
	}

	return nil
}

func (o Options) check(got map[string]any) error {
	for name, val := range got {
		opt, ok := o.find(name)
		if !ok {
			validOptions := make([]string, 0, len(o))
			for _, opt := range o {
				validOptions = append(validOptions, opt.Name)
			}

			return fmt.Errorf("expected one of options - %s, got %s", strings.Join(validOptions, ", "), name)
		}

		if opt.ValidateValue != nil {
			if err := opt.ValidateValue(val); err != nil {
				return fmt.Errorf("validate option value '%s': %w", name, err)
			}
		}

		if len(opt.PossibleValues) > 0 {
			var found bool

			for _, possible := range opt.PossibleValues {
				if possible == val {
					found = true
					break
				}
			}

			if !found {
				return fmt.Errorf("invalid value '%s' for option '%s'. Valid values: %s  ", val, name, opt.PossibleValues)
			}
		}
	}

	// TODO: Append option to the got map, when option is not provided, but it has a default value.

	return nil
}

func (o Options) find(name string) (Option, bool) {
	for _, opt := range o {
		if opt.Name == name {
			return opt, true
		}
	}

	return Option{}, false
}
