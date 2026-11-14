package lang

import (
	_ "embed"
	"sort"
)

// Code is a diagnostic's stable identity: kebab-case, never renamed or
// reused, one anchored section per code in catalog.md.
type Code string

// Every diagnostic raised by this package carries one of these codes. Keep
// this list, the constructors' usage, and catalog.md in sync — the test in
// codes_test.go fails when they drift.
const (
	CodeDateParse           Code = "date-parse"
	CodeTimeParse           Code = "time-parse"
	CodeDurationParse       Code = "duration-parse"
	CodeTemporalInvalid     Code = "temporal-invalid"
	CodeTemporalRange       Code = "temporal-range"
	CodeInstantParse        Code = "instant-parse"
	CodeTimezoneLoad        Code = "timezone-load"
	CodeJSONSyntax          Code = "json-syntax"
	CodeJSONType            Code = "json-type"
	CodeJSONField           Code = "json-field"
	CodeJSONRange           Code = "json-range"
	CodeJSONDepth           Code = "json-depth"
	CodeArrayRange          Code = "array-range"
	CodeArrayRank           Code = "array-rank"
	CodeArrayShape          Code = "array-shape"
	CodeListRange           Code = "list-range"
	CodeCollectionLimit     Code = "collection-limit"
	CodeCollectionIterating Code = "collection-iterating"
	CodeExitStatus          Code = "console-exit-status"
	CodeErrorNoCause        Code = "error-no-cause"
	CodeSourceUTF8          Code = "source-utf8"
	CodeTabCharacter        Code = "tab-character"
	CodeIndentation         Code = "indentation"
	CodeUnterminatedString  Code = "unterminated-string"
	CodeStringEscape        Code = "string-escape"
	CodeUnexpectedCharacter Code = "unexpected-character"

	CodeExpectedToken       Code = "expected-token"
	CodeExpectedName        Code = "expected-name"
	CodeUnknownType         Code = "unknown-type"
	CodeExpectedIndent      Code = "expected-indent"
	CodeEmptyBlock          Code = "empty-block"
	CodeNestingLimit        Code = "nesting-limit"
	CodeIntegerLiteralRange Code = "integer-literal-range"
	CodeExpectedExpression  Code = "expected-expression"
	CodeFlowStep            Code = "flow-step"

	CodeUnknownName     Code = "unknown-name"
	CodeDuplicateName   Code = "duplicate-name"
	CodeDuplicateWord   Code = "duplicate-word"
	CodeWordResult      Code = "word-result"
	CodeImmutableName   Code = "immutable-name"
	CodeAssignmentType  Code = "assignment-type"
	CodeConditionType   Code = "condition-type"
	CodeBranchType      Code = "branch-type"
	CodeRangeType       Code = "range-type"
	CodeTryInput        Code = "try-input"
	CodeUnhandledResult Code = "unhandled-result"
	CodeUnaryType       Code = "unary-type"
	CodeOperandTypes    Code = "operand-types"
	CodeBoolOperand     Code = "bool-operand"
	CodeCompareType     Code = "compare-type"
	CodeOrderingType    Code = "ordering-type"
	CodeAdditionType    Code = "addition-type"
	CodeArithmeticType  Code = "arithmetic-type"
	CodeArgumentCount   Code = "argument-count"
	CodeArgumentType    Code = "argument-type"
	CodeUnknownWord     Code = "unknown-word"
	CodePrintType       Code = "print-type"
	CodeRangeContext    Code = "range-context"

	CodeUseMissing  Code = "use-missing"
	CodeUseCycle    Code = "use-cycle"
	CodeUseReserved Code = "use-reserved"
	CodeUseLocation Code = "use-location"
	CodeModuleBody  Code = "module-body"
	CodePrivateUse  Code = "private-use"

	CodeIntegerOverflow Code = "integer-overflow"
	CodeDivisionZero    Code = "division-zero"
	CodeDepthLimit      Code = "depth-limit"
	CodeUnsupportedCode Code = "unsupported-code"
	CodeConsoleOutput   Code = "console-output"
	CodeConsoleInput    Code = "console-input"
	CodeConsoleUTF8     Code = "console-utf8"
	CodeTextToInt       Code = "text-to-int"
	CodeReplaceEmpty    Code = "replace-empty"
	CodeBufferRange     Code = "buffer-range"
	CodeFileOpen        Code = "file-open"
	CodeFileRead        Code = "file-read"
	CodeFileUTF8        Code = "file-utf8"
	CodeArgumentMissing Code = "argument-missing"
)

var diagnosticCodes = []Code{
	CodeDateParse, CodeTimeParse, CodeDurationParse, CodeTemporalInvalid, CodeTemporalRange, CodeInstantParse, CodeTimezoneLoad,
	CodeErrorNoCause, CodeExitStatus, CodeJSONSyntax, CodeJSONType, CodeJSONField, CodeJSONRange, CodeJSONDepth, CodeArrayRange, CodeArrayRank, CodeArrayShape, CodeListRange, CodeCollectionLimit, CodeCollectionIterating,
	CodeSourceUTF8, CodeTabCharacter, CodeIndentation, CodeUnterminatedString, CodeStringEscape, CodeUnexpectedCharacter,
	CodeExpectedToken, CodeExpectedName, CodeUnknownType, CodeExpectedIndent, CodeEmptyBlock, CodeNestingLimit,
	CodeIntegerLiteralRange, CodeExpectedExpression, CodeFlowStep,
	CodeUnknownName, CodeDuplicateName, CodeDuplicateWord, CodeWordResult, CodeImmutableName, CodeAssignmentType,
	CodeConditionType, CodeBranchType, CodeRangeType, CodeTryInput, CodeUnhandledResult, CodeUnaryType, CodeOperandTypes, CodeBoolOperand,
	CodeCompareType, CodeOrderingType, CodeAdditionType, CodeArithmeticType, CodeArgumentCount, CodeArgumentType,
	CodeUnknownWord, CodePrintType, CodeRangeContext,
	CodeUseMissing, CodeUseCycle, CodeUseReserved, CodeUseLocation, CodeModuleBody, CodePrivateUse,
	CodeIntegerOverflow, CodeDivisionZero, CodeDepthLimit, CodeUnsupportedCode,
	CodeConsoleOutput, CodeConsoleInput, CodeConsoleUTF8, CodeTextToInt, CodeReplaceEmpty, CodeBufferRange,
	CodeFileOpen, CodeFileRead, CodeFileUTF8, CodeArgumentMissing,
}

//go:embed catalog.md
var catalogMarkdown string

// DiagnosticCodes lists every code the language can raise, sorted.
func DiagnosticCodes() []string {
	codes := make([]string, len(diagnosticCodes))
	for i, code := range diagnosticCodes {
		codes[i] = string(code)
	}
	sort.Strings(codes)
	return codes
}

// CatalogMarkdown returns the full catalog: one "## code" section per code.
func CatalogMarkdown() string { return catalogMarkdown }
