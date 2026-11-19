package lang

import (
	"zhex/internal/zhexrt"
	"fmt"
	"regexp"
	"strconv"
	"time"
	_ "time/tzdata"
)

var dateText = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}$`)
var instantText = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(\.[0-9]{1,9})?(Z|[+-][0-9]{2}:[0-9]{2})$`)
var timeText = regexp.MustCompile(`^[0-9]{2}:[0-9]{2}:[0-9]{2}(\.[0-9]{1,9})?$`)

func temporalBuiltin(x *expression, args []value) (value, error) {
	switch x.text {
	case "date.new":
		y, m, d := args[0].number, args[1].number, args[2].number
		if !componentFits(y) || !componentFits(m) || !componentFits(d) ||
			y.AsInt64() < 1 || y.AsInt64() > 9999 || m.AsInt64() < 1 || m.AsInt64() > 12 || d.AsInt64() < 1 || d.AsInt64() > 31 {
			return value{}, temporalComponents(errorAt(x.pos, CodeTemporalInvalid, "invalid date components"), "year", y, "month", m, "day", d)
		}
		v := time.Date(int(y.AsInt64()), time.Month(m.AsInt64()), int(d.AsInt64()), 0, 0, 0, 0, time.UTC)
		if v.Month() != time.Month(m.AsInt64()) {
			return value{}, temporalComponents(errorAt(x.pos, CodeTemporalInvalid, "invalid date components"), "year", y, "month", m, "day", d)
		}
		return value{moment: v}, nil
	case "time.new":
		h, m, s := args[0].number, args[1].number, args[2].number
		if !componentFits(h) || !componentFits(m) || !componentFits(s) ||
			h.AsInt64() < 0 || h.AsInt64() > 23 || m.AsInt64() < 0 || m.AsInt64() > 59 || s.AsInt64() < 0 || s.AsInt64() > 59 {
			return value{}, temporalComponents(errorAt(x.pos, CodeTemporalInvalid, "invalid time components"), "hour", h, "minute", m, "second", s)
		}
		return value{moment: time.Date(1, 1, 1, int(h.AsInt64()), int(m.AsInt64()), int(s.AsInt64()), 0, time.UTC)}, nil
	case "date.parse":
		v, err := time.Parse("2006-01-02", args[0].text)
		if err != nil || !dateText.MatchString(args[0].text) || v.Year() < 1 {
			return value{result: &inputResult{err: errorAt(x.pos, CodeDateParse, "invalid date")}}, nil
		}
		return value{result: &inputResult{value: value{moment: v}}}, nil
	case "time.parse":
		v, err := time.Parse("15:04:05", args[0].text)
		if err != nil || !timeText.MatchString(args[0].text) {
			return value{result: &inputResult{err: errorAt(x.pos, CodeTimeParse, "invalid time")}}, nil
		}
		return value{result: &inputResult{value: value{moment: v}}}, nil
	case "duration.parse":
		v, err := time.ParseDuration(args[0].text)
		if err != nil {
			return value{result: &inputResult{err: errorAt(x.pos, CodeDurationParse, "invalid duration")}}, nil
		}
		return value{result: &inputResult{value: value{number: zhexrt.I64(int64(v))}}}, nil
	case "duration.new":
		ns := args[0].number
		if !ns.Fits64() {
			return value{}, errorAt(x.pos, CodeTemporalRange, "duration is outside the supported range")
		}
		return value{number: ns}, nil
	case "date.to-string":
		return value{text: args[0].moment.Format("2006-01-02")}, nil
	case "time.to-string":
		return value{text: args[0].moment.Format("15:04:05.999999999")}, nil
	case "duration.to-string":
		return value{text: time.Duration(args[0].number.AsInt64()).String()}, nil
	case "date.add-days":
		days := args[1].number
		if !componentFits(days) || days.AsInt64() < -3652059 || days.AsInt64() > 3652059 {
			return value{}, errorAt(x.pos, CodeTemporalRange, "date arithmetic is outside the supported range")
		}
		v := args[0].moment.AddDate(0, 0, int(days.AsInt64()))
		if v.Year() < 1 || v.Year() > 9999 {
			return value{}, errorAt(x.pos, CodeTemporalRange, "date arithmetic is outside the supported range")
		}
		return value{moment: v}, nil
	case "instant.now":
		return value{moment: time.Now().UTC()}, nil
	case "instant.parse":
		text := args[0].text
		valid := instantText.MatchString(text)
		if valid && text[len(text)-1] != 'Z' {
			h, _ := strconv.Atoi(text[len(text)-5 : len(text)-3])
			m, _ := strconv.Atoi(text[len(text)-2:])
			valid = h <= 23 && m <= 59
		}
		v, err := time.Parse(time.RFC3339Nano, text)
		if !valid || err != nil || !validTemporal(v) {
			return value{result: &inputResult{err: errorAt(x.pos, CodeInstantParse, "invalid instant")}}, nil
		}
		return value{result: &inputResult{value: value{moment: v.UTC()}}}, nil
	case "instant.to-string":
		return value{text: args[0].moment.UTC().Format(time.RFC3339Nano)}, nil
	case "instant.add-duration":
		v := args[0].moment.Add(time.Duration(args[1].number.AsInt64()))
		if !validTemporal(v) {
			return value{}, errorAt(x.pos, CodeTemporalRange, "instant is outside the supported range")
		}
		return value{moment: v.UTC()}, nil
	case "timezone.load":
		name := args[0].text
		if name == "" || name == "Local" {
			return value{result: &inputResult{err: errorAt(x.pos, CodeTimezoneLoad, "invalid time zone")}}, nil
		}
		zone, err := time.LoadLocation(name)
		if err != nil {
			return value{result: &inputResult{err: errorAt(x.pos, CodeTimezoneLoad, "unknown time zone")}}, nil
		}
		return value{result: &inputResult{value: value{zone: zone}}}, nil
	case "timezone.name":
		return value{text: args[0].zone.String()}, nil
	case "zoned.new":
		date, clock := args[0].moment, args[1].moment
		v, err := resolveCivilTime(date.Year(), date.Month(), date.Day(), clock.Hour(), clock.Minute(), clock.Second(), clock.Nanosecond(), args[2].zone)
		if err != nil || !validTemporal(v) {
			return value{}, errorAt(x.pos, CodeTemporalRange, "zoned time is outside the supported range")
		}
		return value{moment: v}, nil
	case "zoned.to-string":
		return value{text: formatZoned(args[0].moment)}, nil
	case "zoned.instant":
		return value{moment: args[0].moment.UTC()}, nil
	case "zoned.in-zone":
		v := args[0].moment.In(args[1].zone)
		if !validTemporal(v) {
			return value{}, errorAt(x.pos, CodeTemporalRange, "zoned time is outside the supported range")
		}
		return value{moment: v}, nil
	case "zoned.add-duration":
		v := args[0].moment.Add(time.Duration(args[1].number.AsInt64()))
		if !validTemporal(v) {
			return value{}, errorAt(x.pos, CodeTemporalRange, "zoned time is outside the supported range")
		}
		return value{moment: v}, nil
	case "zoned.add-days":
		days := args[1].number
		if days.IsZero() {
			return args[0], nil
		}
		if !componentFits(days) || days.AsInt64() < -3652059 || days.AsInt64() > 3652059 {
			return value{}, errorAt(x.pos, CodeTemporalRange, "zoned arithmetic is outside the supported range")
		}
		old := args[0].moment
		civil := time.Date(old.Year(), old.Month(), old.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, int(days.AsInt64()))
		v, err := resolveCivilTime(civil.Year(), civil.Month(), civil.Day(), old.Hour(), old.Minute(), old.Second(), old.Nanosecond(), old.Location())
		if err != nil || !validTemporal(v) {
			return value{}, errorAt(x.pos, CodeTemporalRange, "zoned time is outside the supported range")
		}
		return value{moment: v}, nil
	}
	panic("unknown temporal builtin")
}

func temporalComponents(err error, a string, av i128, b string, bv i128, c string, cv i128) error {
	d := err.(*diagnostic)
	d.details = map[string]string{a: zhexrt.Str(av), b: zhexrt.Str(bv), c: zhexrt.Str(cv)}
	return d
}

// componentFits reports whether an Int argument narrows to the int64
// component ranges the temporal constructors accept.
func componentFits(v i128) bool { return v.Fits64() }

// Both representations must remain in the supported Gregorian range.
func validTemporal(t time.Time) bool {
	return t.Year() >= 1 && t.Year() <= 9999 && t.UTC().Year() >= 1 && t.UTC().Year() <= 9999
}

// Historical zone offsets may contain seconds; preserve them instead of rounding.
func formatZoned(t time.Time) string {
	_, offset := t.Zone()
	if offset%60 == 0 {
		return t.Format(time.RFC3339Nano) + "[" + t.Location().String() + "]"
	}
	sign := "+"
	if offset < 0 {
		sign = "-"
		offset = -offset
	}
	return t.Format("2006-01-02T15:04:05.999999999") + fmt.Sprintf("%s%02d:%02d:%02d[%s]", sign, offset/3600, offset%3600/60, offset%60, t.Location())
}
