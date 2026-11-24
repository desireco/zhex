package lang

import (
	_ "embed"
	"sort"
	"strings"
	"zhex/internal/zhexrt"
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
	CodeXMLSyntax           Code = "xml-syntax"
	CodeXMLType             Code = "xml-type"
	CodeXMLField            Code = "xml-field"
	CodeXMLRange            Code = "xml-range"
	CodeXMLDepth            Code = "xml-depth"
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
	CodeUnclosedDelimiter   Code = "unclosed-delimiter"
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
	CodeWithLocation        Code = "with-location"
	CodeWithSeed            Code = "with-seed"
	CodeWithStep            Code = "with-step"

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
	CodeMainParameters  Code = "main-parameters"

	CodeUseMissing  Code = "use-missing"
	CodeUseCycle    Code = "use-cycle"
	CodeUseReserved Code = "use-reserved"
	CodeUseLocation Code = "use-location"
	CodeModuleBody  Code = "module-body"
	CodePrivateUse  Code = "private-use"

	CodeIntegerOverflow  Code = "integer-overflow"
	CodeDivisionZero     Code = "division-zero"
	CodeIntShiftCount    Code = "int-shift-count"
	CodeIntShiftWidth    Code = "int-shift-width"
	CodeDepthLimit       Code = "depth-limit"
	CodeUnsupportedCode  Code = "unsupported-code"
	CodeConsoleOutput    Code = "console-output"
	CodeConsoleInput     Code = "console-input"
	CodeConsoleUTF8      Code = "console-utf8"
	CodeTextToInt        Code = "text-to-int"
	CodeReplaceEmpty     Code = "replace-empty"
	CodeTextCodePoint    Code = "text-code-point"
	CodeTextPadWidth     Code = "text-pad-width"
	CodeTextPadFill      Code = "text-pad-fill"
	CodeTextRepeatCount  Code = "text-repeat-count"
	CodeBufferRange      Code = "buffer-range"
	CodeFileOpen         Code = "file-open"
	CodeFileRead         Code = "file-read"
	CodeFileWrite        Code = "file-write"
	CodeFileBytesRange   Code = "file-bytes-range"
	CodeFileUTF8         Code = "file-utf8"
	CodeFileExists       Code = "file-exists"
	CodeFileList         Code = "file-list"
	CodeFileSize         Code = "file-size"
	CodeFileFreeSpace    Code = "file-free-space"
	CodeFileMakeFolder   Code = "file-make-folder"
	CodeFileDelete       Code = "file-delete"
	CodeFileDeleteFolder Code = "file-delete-folder"
	CodeFileMove         Code = "file-move"
	CodeEnvRead          Code = "env-read"
	CodeEnvParse         Code = "env-parse"
	CodeArgumentMissing  Code = "argument-missing"

	CodeDoctestMismatch Code = "doctest-mismatch"
	CodeDoctestCheck    Code = "doctest-check"
	CodeTestFailed      Code = "test-failed"

	CodeNotAvailableInBuild Code = "not-available-in-build"
)

var diagnosticCodes = []Code{
	CodeDateParse, CodeTimeParse, CodeDurationParse, CodeTemporalInvalid, CodeTemporalRange, CodeInstantParse, CodeTimezoneLoad,
	CodeErrorNoCause, CodeExitStatus, CodeJSONSyntax, CodeJSONType, CodeJSONField, CodeJSONRange, CodeJSONDepth, CodeXMLSyntax, CodeXMLType, CodeXMLField, CodeXMLRange, CodeXMLDepth, CodeArrayRange, CodeArrayRank, CodeArrayShape, CodeListRange, CodeCollectionLimit, CodeCollectionIterating,
	CodeSourceUTF8, CodeTabCharacter, CodeIndentation, CodeUnterminatedString, CodeUnclosedDelimiter, CodeStringEscape, CodeUnexpectedCharacter,
	CodeExpectedToken, CodeExpectedName, CodeUnknownType, CodeExpectedIndent, CodeEmptyBlock, CodeNestingLimit,
	CodeIntegerLiteralRange, CodeExpectedExpression, CodeFlowStep,
	CodeWithLocation, CodeWithSeed, CodeWithStep,
	CodeUnknownName, CodeDuplicateName, CodeDuplicateWord, CodeWordResult, CodeImmutableName, CodeAssignmentType,
	CodeConditionType, CodeBranchType, CodeRangeType, CodeTryInput, CodeUnhandledResult, CodeUnaryType, CodeOperandTypes, CodeBoolOperand,
	CodeCompareType, CodeOrderingType, CodeAdditionType, CodeArithmeticType, CodeArgumentCount, CodeArgumentType,
	CodeUnknownWord, CodePrintType, CodeRangeContext, CodeMainParameters,
	CodeUseMissing, CodeUseCycle, CodeUseReserved, CodeUseLocation, CodeModuleBody, CodePrivateUse,
	CodeIntegerOverflow, CodeDivisionZero, CodeIntShiftCount, CodeIntShiftWidth, CodeDepthLimit, CodeUnsupportedCode,
	CodeConsoleOutput, CodeConsoleInput, CodeConsoleUTF8, CodeTextToInt, CodeReplaceEmpty, CodeTextCodePoint, CodeTextPadWidth, CodeTextPadFill, CodeTextRepeatCount, CodeBufferRange,
	CodeFileOpen, CodeFileRead, CodeFileWrite, CodeFileBytesRange, CodeFileUTF8, CodeFileExists, CodeFileList, CodeFileSize, CodeFileFreeSpace, CodeFileMakeFolder, CodeFileDelete, CodeFileDeleteFolder, CodeFileMove, CodeEnvRead, CodeEnvParse, CodeArgumentMissing,
	CodeDoctestMismatch, CodeDoctestCheck, CodeTestFailed,
	CodeNotAvailableInBuild,
}

//go:embed catalog.md
var catalogMarkdown string

// catalogCodes distinguishes built-in codes from application error.new
// codes: only the former carry a learn URL.
var catalogCodes = func() map[Code]bool {
	set := make(map[Code]bool, len(diagnosticCodes))
	for _, code := range diagnosticCodes {
		set[code] = true
	}
	return set
}()

// DocsURL returns the errors-page anchor for the code — the catalog's
// section the learn line and the JSON report's url field point at — or ""
// for codes outside the catalog, which own their own namespaces.
func (c Code) DocsURL() string {
	if !catalogCodes[c] {
		return ""
	}
	return zhexrt.LearnURL(string(c))
}

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

// CatalogEntry is one cataloged code with its catalog prose — the same
// entry the site's errors page renders from CatalogMarkdown.
type CatalogEntry struct {
	Code, Text string
}

// CatalogEntries splits the embedded catalog into one entry per
// "## code" section, in sorted DiagnosticCodes order: the data source
// for `zhex errors`, so the shipped guide cannot drift from the codes
// the tool raises.
func CatalogEntries() []CatalogEntry {
	var entries []CatalogEntry
	for i, section := range strings.Split(strings.TrimSpace(catalogMarkdown), "\n## ") {
		if i == 0 {
			continue // the intro before the first section
		}
		code, text, _ := strings.Cut(section, "\n")
		entries = append(entries, CatalogEntry{strings.TrimSpace(code), strings.TrimSpace(text)})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Code < entries[j].Code })
	return entries
}

// CodeHint returns the did-you-mean line for an unknown diagnostic code
// over the catalog's code names — the same closest-name machinery name
// and word hints use.
func CodeHint(code string) string { return suggestHint(code, DiagnosticCodes()) }
