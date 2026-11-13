package lang

import (
	"fmt"
	"io"
	"sync/atomic"
	"time"
)

// RunOptions configures optional tracing, arguments, and reporting for one run.
type RunOptions struct {
	Trace            io.Writer
	Diagnostics      io.Writer // console.fail output; nil discards it
	Args             []string
	Reporter         func(ErrorReport) error
	IncludeSensitive bool
	Correlation      string
}

// SourceLocation identifies a Zhex location. File is omitted from default reports.
type SourceLocation struct {
	File   string `json:"file,omitempty"`
	Line   int    `json:"line"`
	Column int    `json:"column"`
}

// ErrorSnapshot contains a report-safe copy of an error and its causes.
type ErrorSnapshot struct {
	Code           string            `json:"code"`
	Operation      string            `json:"operation"`
	Origin         SourceLocation    `json:"origin"`
	Message        string            `json:"message,omitempty"`
	Details        map[string]string `json:"details,omitempty"`
	Cause          *ErrorSnapshot    `json:"cause,omitempty"`
	CauseTruncated bool              `json:"cause_truncated,omitempty"`
}

// ErrorReport is the versioned envelope delivered to an optional host reporter.
type ErrorReport struct {
	SchemaVersion int            `json:"schema_version"`
	OccurrenceID  string         `json:"occurrence_id"`
	Timestamp     time.Time      `json:"timestamp"`
	Outcome       string         `json:"outcome"`
	Error         ErrorSnapshot  `json:"error"`
	Boundary      SourceLocation `json:"boundary"`
	Correlation   string         `json:"correlation,omitempty"`
}

var occurrenceCounter atomic.Uint64

func location(pos position, sensitive bool) SourceLocation {
	result := SourceLocation{Line: pos.line, Column: pos.column}
	if sensitive {
		result.File = pos.file
	}
	return result
}

func snapshot(d *diagnostic, sensitive bool, depth int) ErrorSnapshot {
	result := ErrorSnapshot{Code: string(d.code), Operation: d.operation, Origin: location(d.pos, sensitive)}
	if sensitive {
		result.Message = d.message
		if len(d.details) != 0 {
			result.Details = make(map[string]string, len(d.details))
			for key, value := range d.details {
				result.Details[key] = value
			}
		}
	}
	if d.cause != nil {
		if depth >= 32 {
			result.CauseTruncated = true
		} else {
			cause := snapshot(d.cause, sensitive, depth+1)
			result.Cause = &cause
		}
	}
	return result
}

func newOccurrence() *errorOccurrence {
	return &errorOccurrence{id: fmt.Sprintf("%x-%x", time.Now().UnixNano(), occurrenceCounter.Add(1))}
}
