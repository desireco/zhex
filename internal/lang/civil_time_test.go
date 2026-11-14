package lang

import (
	"testing"
	"time"
)

func TestCivilTimeResolvesOverlapAndGap(t *testing.T) {
	zone, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		year              int
		month             time.Month
		day, hour, minute int
		want              string
	}{
		{2024, time.November, 3, 1, 30, "2024-11-03T05:30:00Z"},
		{2024, time.March, 10, 2, 30, "2024-03-10T07:30:00Z"},
		{2024, time.January, 12, 10, 15, "2024-01-12T15:15:00Z"},
	} {
		actual, err := resolveCivilTime(tc.year, tc.month, tc.day, tc.hour, tc.minute, 0, 0, zone)
		if err != nil || actual.UTC().Format(time.RFC3339) != tc.want {
			t.Errorf("%+v: %v, %v", tc, actual, err)
		}
	}
}

func TestCivilTimeRejectsInvalidCalendarValues(t *testing.T) {
	for _, tc := range []struct {
		y    int
		m    time.Month
		d, h int
	}{{0, 1, 1, 0}, {2024, 2, 30, 0}, {2024, 1, 1, 24}, {10000, 1, 1, 0}} {
		if _, err := resolveCivilTime(tc.y, tc.m, tc.d, tc.h, 0, 0, 0, time.UTC); err == nil {
			t.Errorf("accepted %+v", tc)
		}
	}
}

func TestCivilTimeNonHourAndDateLineGaps(t *testing.T) {
	for _, tc := range []struct {
		zone              string
		year              int
		month             time.Month
		day, hour, minute int
		want              string
	}{
		{"Australia/Lord_Howe", 2024, time.October, 6, 2, 15, "2024-10-05T15:45:00Z"},
		{"Pacific/Apia", 2011, time.December, 30, 12, 0, "2011-12-30T22:00:00Z"},
	} {
		zone, err := time.LoadLocation(tc.zone)
		if err != nil {
			t.Fatal(err)
		}
		got, err := resolveCivilTime(tc.year, tc.month, tc.day, tc.hour, tc.minute, 0, 0, zone)
		if err != nil || got.UTC().Format(time.RFC3339) != tc.want {
			t.Errorf("%s: %v, %v", tc.zone, got, err)
		}
	}
	got, err := resolveCivilTime(1, 1, 1, 0, 0, 0, 0, time.UTC)
	if err != nil || got.Year() != 1 {
		t.Fatalf("minimum year: %v, %v", got, err)
	}
}
