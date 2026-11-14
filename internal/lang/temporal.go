package lang

import (
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
		if y < 1 || y > 9999 || m < 1 || m > 12 || d < 1 || d > 31 {
			return value{}, errorAt(x.pos, CodeTemporalInvalid, "invalid date components")
		}
		v := time.Date(int(y), time.Month(m), int(d), 0, 0, 0, 0, time.UTC)
		if v.Month() != time.Month(m) {
			return value{}, errorAt(x.pos, CodeTemporalInvalid, "invalid date components")
		}
		return value{moment: v}, nil
	case "time.new":
		h, m, s := args[0].number, args[1].number, args[2].number
		if h < 0 || h > 23 || m < 0 || m > 59 || s < 0 || s > 59 {
			return value{}, errorAt(x.pos, CodeTemporalInvalid, "invalid time components")
		}
		return value{moment: time.Date(1, 1, 1, int(h), int(m), int(s), 0, time.UTC)}, nil
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
		return value{result: &inputResult{value: value{number: int64(v)}}}, nil
	case "duration.new":
		return value{number: args[0].number}, nil
	case "date.to-string":
		return value{text: args[0].moment.Format("2006-01-02")}, nil
	case "time.to-string":
		return value{text: args[0].moment.Format("15:04:05.999999999")}, nil
	case "duration.to-string":
		return value{text: time.Duration(args[0].number).String()}, nil
	case "date.add-days":
		days := args[1].number
		if days < -3652059 || days > 3652059 {
			return value{}, errorAt(x.pos, CodeTemporalRange, "date arithmetic is outside the supported range")
		}
		v := args[0].moment.AddDate(0, 0, int(days))
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
		v := args[0].moment.Add(time.Duration(args[1].number))
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
		v := args[0].moment.Add(time.Duration(args[1].number))
		if !validTemporal(v) {
			return value{}, errorAt(x.pos, CodeTemporalRange, "zoned time is outside the supported range")
		}
		return value{moment: v}, nil
	case "zoned.add-days":
		days := args[1].number
		if days < -3652059 || days > 3652059 {
			return value{}, errorAt(x.pos, CodeTemporalRange, "zoned arithmetic is outside the supported range")
		}
		old := args[0].moment
		civil := time.Date(old.Year(), old.Month(), old.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, int(days))
		v, err := resolveCivilTime(civil.Year(), civil.Month(), civil.Day(), old.Hour(), old.Minute(), old.Second(), old.Nanosecond(), old.Location())
		if err != nil || !validTemporal(v) {
			return value{}, errorAt(x.pos, CodeTemporalRange, "zoned time is outside the supported range")
		}
		return value{moment: v}, nil
	}
	panic("unknown temporal builtin")
}

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
