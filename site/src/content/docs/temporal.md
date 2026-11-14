---
title: Dates, times, and time zones
description: Work with calendar dates, elapsed durations, instants, and named time zones.
section: Standard library
order: 115
---

Zhex separates calendar dates and local times from instants. A `Duration` measures elapsed nanoseconds; a `TimeZone` names a zone, and a `ZonedDateTime` combines a zone with an instant. Format these values explicitly with `to-string`.

```zhex
let zone = timezone.load "America/New_York" try
let start = zoned.new(date.new(2024, 3, 9), time.new(12, 0, 0), zone)
let elapsed = duration.parse "24h" try
print(zoned.to-string(zoned.add-days(start, 1)))
print(zoned.to-string(zoned.add-duration(start, elapsed)))
```

The first result is noon the next day; the second is 1 p.m. because the daylight-saving change skips an hour. See the complete [temporal example](../examples.html#temporal).

| Operation | Result | Use |
| --- | --- | --- |
| `date.new(year, month, day)` | `Date` | Construct a Gregorian date in years 1–9999. |
| `date.parse(text)` | `Result<Date, Error>` | Parse `YYYY-MM-DD`. |
| `date.to-string(date)` | `String` | Format a date. |
| `date.add-days(date, days)` | `Date` | Move by calendar days. |
| `time.new(hour, minute, second)` | `Time` | Construct a local time; leap seconds are not supported. |
| `time.parse(text)` | `Result<Time, Error>` | Parse `HH:MM:SS` with optional fractional seconds up to nine digits. |
| `time.to-string(time)` | `String` | Format local time. |
| `duration.new(nanoseconds)` | `Duration` | Construct from signed 64-bit nanoseconds. |
| `duration.parse(text)` | `Result<Duration, Error>` | Parse elapsed units such as `ns`, `us`, `µs`, `ms`, `s`, `m`, and `h`. |
| `duration.to-string(duration)` | `String` | Format elapsed duration. |
| `instant.now()` | `Instant` | Read the current instant. |
| `instant.parse(text)` | `Result<Instant, Error>` | Parse RFC 3339 with an explicit offset. |
| `instant.to-string(instant)` | `String` | Format in UTC with nanosecond precision as needed. |
| `instant.add-duration(instant, duration)` | `Instant` | Move by elapsed time. |
| `timezone.load(name)` | `Result<TimeZone, Error>` | Load `UTC` or a named IANA zone. Empty and `Local` are rejected. |
| `timezone.name(zone)` | `String` | Return the zone name. |
| `zoned.new(date, time, zone)` | `ZonedDateTime` | Resolve a local date and time in a zone. |
| `zoned.to-string(value)` | `String` | Format local date, time, offset, and zone name. |
| `zoned.instant(value)` | `Instant` | Extract the instant. |
| `zoned.in-zone(value, zone)` | `ZonedDateTime` | Preserve the instant in another zone. |
| `zoned.add-days(value, days)` | `ZonedDateTime` | Move by local calendar days. |
| `zoned.add-duration(value, duration)` | `ZonedDateTime` | Move by elapsed time. |

When clocks repeat a local time, construction selects the earlier instant. When a local time falls in a gap, it shifts forward by the actual gap, including non-hour transitions. Adding zero calendar days preserves the original instant, including the later occurrence of a repeated time. Named-zone data is bundled as a fallback when system zone data is unavailable.

Ordinary numeric construction and arithmetic validate their inputs and raise source-located runtime diagnostics for invalid components or out-of-range results. Parsing and zone lookup are recoverable Results: use `try` or `catch`. Time and instant parsing reject leap seconds and excess fractional precision. Instant inputs need an explicit offset; output uses UTC. The supported range is years 1 through 9999 for both local and UTC representations.

Zoned display uses an RFC 3339-style local timestamp followed by `[zone name]`. Some historical zones used offsets containing seconds; their display includes `±HH:MM:SS` to preserve the actual offset. This extended zoned display is not an `instant.parse` input. Temporal values have no implicit conversion, direct print or equality, or JSON encoding; use explicit formatting.
