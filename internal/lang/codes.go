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

	CodeUnknownName    Code = "unknown-name"
	CodeDuplicateName  Code = "duplicate-name"
	CodeDuplicateWord  Code = "duplicate-word"
	CodeWordResult     Code = "word-result"
	CodeImmutableName  Code = "immutable-name"
	CodeAssignmentType Code = "assignment-type"
	CodeConditionType  Code = "condition-type"
	CodeBranchType     Code = "branch-type"
	CodeRangeType      Code = "range-type"
	CodeTryInput       Code = "try-input"
	CodeUnaryType      Code = "unary-type"
	CodeOperandTypes   Code = "operand-types"
	CodeBoolOperand    Code = "bool-operand"
	CodeCompareType    Code = "compare-type"
	CodeOrderingType   Code = "ordering-type"
	CodeAdditionType   Code = "addition-type"
	CodeArithmeticType Code = "arithmetic-type"
	CodeArgumentCount  Code = "argument-count"
	CodeArgumentType   Code = "argument-type"
	CodeUnknownWord    Code = "unknown-word"
	CodePrintType      Code = "print-type"
	CodeRangeContext   Code = "range-context"

	CodeIntegerOverflow Code = "integer-overflow"
	CodeDivisionZero    Code = "division-zero"
	CodeDepthLimit      Code = "depth-limit"
	CodeUnsupportedCode Code = "unsupported-code"
	CodeConsoleOutput   Code = "console-output"
	CodeConsoleInput    Code = "console-input"
	CodeConsoleUTF8     Code = "console-utf8"
	CodeConsoleInt      Code = "console-int"
)

var diagnosticCodes = []Code{
	CodeSourceUTF8, CodeTabCharacter, CodeIndentation, CodeUnterminatedString, CodeStringEscape, CodeUnexpectedCharacter,
	CodeExpectedToken, CodeExpectedName, CodeUnknownType, CodeExpectedIndent, CodeEmptyBlock, CodeNestingLimit,
	CodeIntegerLiteralRange, CodeExpectedExpression, CodeFlowStep,
	CodeUnknownName, CodeDuplicateName, CodeDuplicateWord, CodeWordResult, CodeImmutableName, CodeAssignmentType,
	CodeConditionType, CodeBranchType, CodeRangeType, CodeTryInput, CodeUnaryType, CodeOperandTypes, CodeBoolOperand,
	CodeCompareType, CodeOrderingType, CodeAdditionType, CodeArithmeticType, CodeArgumentCount, CodeArgumentType,
	CodeUnknownWord, CodePrintType, CodeRangeContext,
	CodeIntegerOverflow, CodeDivisionZero, CodeDepthLimit, CodeUnsupportedCode,
	CodeConsoleOutput, CodeConsoleInput, CodeConsoleUTF8, CodeConsoleInt,
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
