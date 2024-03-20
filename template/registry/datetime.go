package registry

import (
	"fmt"
	"strings"
	"time"
)

// https://github.com/unicode-org/message-format-wg/blob/122e64c2482b54b6eff4563120915e0f86de8e4d/spec/registry.xml#L13

var datetimeRegistryF = &Func{
	Name:           "datetime",
	Description:    "Locale-sensitive date and time formatting",
	Fn:             datetimeF,
	MatchSignature: nil, // Not allowed to use in matching context
	FormatSignature: &Signature{
		IsInputRequired: true,
		ValidateInput: func(a any) error {
			if _, ok := a.(time.Time); !ok {
				return fmt.Errorf("unsupported type: %T", a)
			}

			return nil
		},
		Options: Options{
			{
				Name:           "dateStyle",
				Description:    "The predefined date formatting style to use.",
				PossibleValues: []any{"full", "long", "medium", "short"},
			},
			{
				Name:           "timeStyle",
				Description:    "The predefined time formatting style to use.",
				PossibleValues: []any{"full", "long", "medium", "short"},
			},
			{
				Name:        "calendar",
				Description: "Calendar to use.",
				PossibleValues: []any{
					"buddhist", "chinese", "coptic", "dangi", "ethioaa", "ethiopic", "gregory",
					"hebrew", "indian", "islamic", "islamic-umalqura", "islamic-tbla",
					"islamic-civil", "islamic-rgsa", "iso8601", "japanese", "persian", "roc",
				},
			},
			{
				Name:        "numberingSystem",
				Description: "Numbering system to use.",
				PossibleValues: []any{
					"arab", "arabext", "bali", "beng", "deva", "fullwide", "gujr", "guru", "hanidec",
					"khmr", "knda", "laoo", "latn", "limb", "mlym", "mong", "mymr", "orya", "tamldec",
					"telu", "thai", "tibt",
				},
			},
			{
				Name: "timeZone",
				Description: `The time zone to use.
The only value implementations must recognize is "UTC";
the default is the runtime's default time zone.
Implementations may also recognize the time zone names of the IANA time zone database,
such as "Asia/Shanghai", "Asia/Kolkata", "America/New_York".`,
				Default: "UTC",
			},
			{
				Name:           "hourCycle",
				Description:    "The hour cycle to use.",
				PossibleValues: []any{"h11", "h12", "h23", "h24"},
			},
			{
				Name:           "dayPeriod",
				Description:    `The formatting style used for day periods like "in the morning", "am", "noon", "n" etc.`,
				PossibleValues: []any{"narrow", "short", "long"},
			},
			{
				Name:           "weekday",
				Description:    "The representation of the weekday.",
				PossibleValues: []any{"narrow", "short", "long"},
			},
			{
				Name:           "era",
				Description:    "The representation of the era.",
				PossibleValues: []any{"narrow", "short", "long"},
			},
			{
				Name:           "year",
				Description:    "The representation of the year.",
				PossibleValues: []any{"numeric", "2-digit"},
			},
			{
				Name:           "month",
				Description:    "The representation of the month.",
				PossibleValues: []any{"numeric", "2-digit", "narrow", "short", "long"},
			},
			{
				Name:           "day",
				Description:    "The representation of the day.",
				PossibleValues: []any{"numeric", "2-digit"},
			},
			{
				Name:           "hour",
				Description:    "The representation of the hour.",
				PossibleValues: []any{"numeric", "2-digit"},
			},
			{
				Name:           "minute",
				Description:    "The representation of the minute.",
				PossibleValues: []any{"numeric", "2-digit"},
			},
			{
				Name:           "second",
				Description:    "The representation of the second.",
				PossibleValues: []any{"numeric", "2-digit"},
			},
			{
				Name: "fractionalSecondDigits",
				Description: `The number of digits used to represent fractions of a second
(any additional digits are truncated).`,
				PossibleValues: []any{1, 2, 3},
			},
			{
				Name:           "timeZoneName",
				Description:    "The localized representation of the time zone name.",
				PossibleValues: []any{"long", "short", "shortOffset", "longOffset", "shortGeneric", "longGeneric"},
			},
		},
	},
}

func datetimeF(a any, options map[string]any) (any, error) {
	if len(options) == 0 {
		return fmt.Sprint(a), nil
	}

	tim := a.(time.Time) //nolint:forcetypeassert // we already validated the input type

	// TODO: implement all options
	for optName := range options {
		switch optName {
		case "calendar", "numberingSystem", "hourCycle", "dayPeriod", "weekday", "era",
			"year", "month", "day", "hour", "minute", "second", "fractionalSecondDigits":
			return nil, fmt.Errorf("option '%s' is not implemented", optName)
		}
	}

	var layout string

	if dateStyle, ok := options["dateStyle"].(string); ok {
		switch dateStyle {
		case "full":
			layout = "Monday, 02 January 2006"
		case "long":
			layout = "02 January 2006"
		case "medium":
			layout = "02 Jan 2006"
		case "short":
			layout = "02/01/06"
		}
	}

	if timeStyle, ok := options["timeStyle"].(string); ok {
		switch timeStyle {
		case "full", "long":
			layout += " 15:04:05"
		case "medium":
			layout += " 15:04"
		case "short":
			layout += " 15"
		}
	}

	layout = strings.TrimSpace(layout)

	if timeZone, ok := options["timeZone"].(string); ok {
		loc, err := time.LoadLocation(timeZone)
		if err != nil {
			return nil, fmt.Errorf("load location: %w", err)
		}

		tim = tim.In(loc)
	}

	return tim.Format(layout), nil
}
