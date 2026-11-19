package lang

import "zhex/internal/zhexrt"

// The adapters between the interpreter's positions/diagnostics and the
// zhexrt runtime's, for the builtin implementations the two share (the
// M3 extraction): the moved bodies live in zhexrt, the interpreter calls
// them through these, and the differential corpus gates keep the
// behavior identical. Dynamic-over-value operations (collections, files)
// stay per-engine — the interpreter's injectable os/io seams are its
// failure tests' territory.

func rtPos(pos position) zhexrt.Pos {
	return zhexrt.Pos{File: pos.file, Line: pos.line, Col: pos.column}
}

// fromRT adapts a runtime diagnostic into the interpreter's diagnostic
// shape, keeping every rendered field.
func fromRT(err *zhexrt.Error) error {
	if err == nil {
		return nil
	}
	d := errorAt(position{file: err.Pos.File, line: err.Pos.Line, column: err.Pos.Col}, Code(err.Code), "%s", err.Message).(*diagnostic)
	d.operation, d.details, d.hint = err.Operation, err.Details, err.Hint
	d.cause = (*diagnostic)(nil)
	if err.Cause != nil {
		d.cause = fromRT(err.Cause).(*diagnostic)
	}
	return d
}
