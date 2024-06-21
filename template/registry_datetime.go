package template

import (
	"errors"
	"fmt"
	"time"

	"golang.org/x/text/language"
)

// See ".message-format-wg/spec/registry.xml".

// datetimeFunc is the implementation of the datetime function. Locale-sensitive date and time formatting.
var datetimeRegistryFunc = RegistryFunc{
	Format: datetimeFunc,
}

type datetimeOptions struct {
	TimeZone               *time.Location
	DateStyle              string
	TimeStyle              string
	Calendar               string
	NumberingSystem        string
	HourCycle              string
	DayPeriod              string
	Weekday                string
	Era                    string
	Year                   string
	Month                  string
	Day                    string
	Hour                   string
	Minute                 string
	Second                 string
	TimeZoneName           string
	FractionalSecondDigits int
}

func parseDatetimeInput(input any) (time.Time, error) {
	if input == nil {
		return time.Time{}, fmt.Errorf("input is required: %w", ErrOperandMismatch)
	}

	switch v := input.(type) {
	default:
		return time.Time{}, fmt.Errorf("unsupported datetime type %T: %w", input, ErrOperandMismatch)
	case string:
		// layout is quick and dirty, does not conform with ISO 8601 fully as required
		t, err := time.Parse(time.RFC3339[:len(v)], v)
		if err != nil {
			return time.Time{}, fmt.Errorf("parse datetime %s: %w", v, ErrOperandMismatch)
		}

		return t, nil
	case time.Time:
		return v, nil
	}
}

func parseDatetimeOptions(options Options) (*datetimeOptions, error) {
	var (
		opts datetimeOptions
		err  error
	)

	// The predefined date formatting style to use.
	dateStyles := oneOf("full", "long", "medium", "short")
	if opts.DateStyle, err = options.GetString("dateStyle", "", dateStyles); err != nil {
		return nil, err
	}

	// The predefined time formatting style to use.
	timeStyles := oneOf("full", "long", "medium", "short")
	if opts.TimeStyle, err = options.GetString("timeStyle", "", timeStyles); err != nil {
		return nil, err
	}

	//  Calendar to use.
	calendars := oneOf(
		"buddhist", "chinese", "coptic", "dangi", "ethioaa", "ethiopic", "gregory",
		"hebrew", "indian", "islamic", "islamic-umalqura", "islamic-tbla",
		"islamic-civil", "islamic-rgsa", "iso8601", "japanese", "persian", "roc",
	)
	if opts.Calendar, err = options.GetString("calendar", "", calendars); err != nil {
		return nil, err
	}

	// Numbering system to use.
	numberingSystems := oneOf(
		"arab", "arabext", "bali", "beng", "deva", "fullwide", "gujr", "guru", "hanidec",
		"khmr", "knda", "laoo", "latn", "limb", "mlym", "mong", "mymr", "orya", "tamldec",
		"telu", "thai", "tibt",
	)
	if opts.NumberingSystem, err = options.GetString("numberingSystem", "", numberingSystems); err != nil {
		return nil, err
	}

	opts.TimeZone = time.UTC

	if v, ok := options["timeZone"]; ok {
		switch tz := v.(type) {
		default:
			return nil, fmt.Errorf("unsupported timeZone type: %T", v)
		case *time.Location:
			opts.TimeZone = tz
		case string:
			if opts.TimeZone, err = time.LoadLocation(tz); err != nil {
				return nil, fmt.Errorf("load tz %s: %w", tz, err)
			}
		}
	}

	// The hour cycle to use.
	hourCycles := oneOf("h11", "h12", "h23", "h24")
	if opts.HourCycle, err = options.GetString("hourCycle", "", hourCycles); err != nil {
		return nil, err
	}

	// The formatting style used for day periods like "in the morning", "am", "noon", "n" etc.
	dayPeriods := oneOf("short", "long")
	if opts.DayPeriod, err = options.GetString("dayPeriod", "", dayPeriods); err != nil {
		return nil, err
	}

	// The representation of the weekday.
	weekdays := oneOf("narrow", "short", "long")
	if opts.Weekday, err = options.GetString("weekday", "", weekdays); err != nil {
		return nil, err
	}

	// The representation of the era.
	eras := oneOf("narrow", "short", "long")
	if opts.Era, err = options.GetString("era", "", eras); err != nil {
		return nil, err
	}

	// The representation of the year.
	years := oneOf("numeric", "2-digit")
	if opts.Year, err = options.GetString("year", "", years); err != nil {
		return nil, err
	}

	// The representation of the month.
	months := oneOf("numeric", "2-digit", "narrow", "short", "long")
	if opts.Month, err = options.GetString("month", "", months); err != nil {
		return nil, err
	}

	// The representation of the day.
	days := oneOf("numeric", "2-digit")
	if opts.Day, err = options.GetString("day", "", days); err != nil {
		return nil, err
	}

	// The representation of the hour.
	hours := oneOf("numeric", "2-digit")
	if opts.Hour, err = options.GetString("hour", "", hours); err != nil {
		return nil, err
	}

	// The representation of the minute.
	minutes := oneOf("numeric", "2-digit")
	if opts.Minute, err = options.GetString("minute", "", minutes); err != nil {
		return nil, err
	}

	// The representation of the second.
	seconds := oneOf("numeric", "2-digit")
	if opts.Second, err = options.GetString("second", "", seconds); err != nil {
		return nil, err
	}

	// The number of fractional seconds to display.
	//nolint:mnd
	if opts.FractionalSecondDigits, err = options.GetInt("fractionalSecondDigits", 0, oneOf(1, 2, 3)); err != nil {
		return nil, err
	}

	// The localized representation of the time zone name.
	timeZoneNames := oneOf("long", "short", "shortOffset", "longOffset", "shortGeneric", "longGeneric")
	if opts.TimeZoneName, err = options.GetString("timeZoneName", "", timeZoneNames); err != nil {
		return nil, err
	}

	return &opts, nil
}

// A conflictingOptions checks if options conform to WG registry spec:
//
//	https://github.com/unicode-org/message-format-wg/blob/main/spec/registry.md#options-3
//
// The :datetime function can use either the appropriate style options or can use a collection
// of field options (but not both) to control the formatted output.
// If both are specified, a Bad Option error MUST be emitted and a fallback value used as the
// resolved value of the expression.
func conflictingOptions(options datetimeOptions) error {
	hasStyleOptions := options.TimeStyle != "" || options.DateStyle != ""
	if !hasStyleOptions {
		return nil
	}

	hasFieldOptions := options.Weekday != "" ||
		options.Era != "" ||
		options.Year != "" ||
		options.Month != "" ||
		options.Day != "" ||
		options.Hour != "" ||
		options.Minute != "" ||
		options.Second != "" ||
		options.FractionalSecondDigits != 0 ||
		options.HourCycle != "" ||
		options.TimeZoneName != ""
	if hasFieldOptions {
		return errors.New("bad option")
	}

	return nil
}

func datetimeFunc(input any, options Options, locale language.Tag) (any, error) {
	value, err := parseDatetimeInput(input)
	if err != nil {
		return "", err
	}

	opts, err := parseDatetimeOptions(options)
	if err != nil {
		return "", err
	}

	if err := conflictingOptions(*opts); err != nil {
		return "", err
	}

	if len(options) == 0 {
		return fmt.Sprint(input), nil
	}

	for optName := range options {
		switch optName {
		case "calendar", "numberingSystem", "hourCycle", "dayPeriod", "weekday", "era",
			"month", "day", "hour", "minute", "second", "fractionalSecondDigits":
			return nil, fmt.Errorf("option '%s' is not implemented", optName)
		}
	}

	var layout string

	switch opts.DateStyle {
	case "full":
		layout = "Monday, 02 January 2006"
	case "long":
		layout = "02 January 2006"
	case "medium":
		layout = "02 Jan 2006"
	case "short":
		layout = "02/01/06"
	}

	if len(opts.TimeStyle) > 0 {
		if len(layout) > 0 {
			layout += " "
		}

		switch opts.TimeStyle {
		case "full", "long":
			layout += "15:04:05"
		case "medium":
			layout += "15:04"
		case "short":
			layout += "15"
		}
	}

	value = value.In(opts.TimeZone)

	if opts.Year != "" {
		switch opts.Year {
		case "numeric":
			layout = "2006"
		case "2-digit":
			layout = "06"
		}
	}

	return value.Format(layout), nil
}
