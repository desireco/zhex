package lang

import (
	"fmt"
	"time"
)

// resolveCivilTime chooses the earlier occurrence of an overlap and advances
// a nonexistent local time by the transition's gap.
func resolveCivilTime(year int, month time.Month, day, hour, minute, second, nanosecond int, zone *time.Location) (time.Time, error) {
	if year < 1 || year > 9999 || month < 1 || month > 12 || day < 1 || hour < 0 || hour > 23 || minute < 0 || minute > 59 || second < 0 || second > 59 || nanosecond < 0 || nanosecond >= 1e9 || zone == nil {
		return time.Time{}, fmt.Errorf("invalid civil time")
	}
	wall := time.Date(year, month, day, hour, minute, second, nanosecond, time.UTC)
	if wall.Year() != year || wall.Month() != month || wall.Day() != day {
		return time.Time{}, fmt.Errorf("invalid civil date")
	}
	// Enumerating transitions, instead of sampling offsets, handles historical
	// transitions that are not whole hours as well as date-line changes.
	start, end := wall.Add(-72*time.Hour), wall.Add(72*time.Hour)
	offsets := map[int]bool{}
	for cursor := start; !cursor.After(end); {
		local := cursor.In(zone)
		_, offset := local.Zone()
		offsets[offset] = true
		_, next := local.ZoneBounds()
		if next.IsZero() || next.After(end) || !next.After(cursor) {
			break
		}
		cursor = next
	}
	var exact, shifted time.Time
	var shift time.Duration
	var hasExact, hasShifted bool
	for offset := range offsets {
		candidate := wall.Add(-time.Duration(offset) * time.Second)
		local := candidate.In(zone)
		localWall := time.Date(local.Year(), local.Month(), local.Day(), local.Hour(), local.Minute(), local.Second(), local.Nanosecond(), time.UTC)
		difference := localWall.Sub(wall)
		if difference == 0 && (!hasExact || candidate.Before(exact)) {
			exact = candidate
			hasExact = true
		}
		if difference > 0 && (!hasShifted || difference < shift || (difference == shift && candidate.Before(shifted))) {
			shifted, shift = candidate, difference
			hasShifted = true
		}
	}
	if hasExact {
		return exact.In(zone), nil
	}
	if hasShifted {
		return shifted.In(zone), nil
	}
	return time.Time{}, fmt.Errorf("cannot resolve civil time in zone")
}
