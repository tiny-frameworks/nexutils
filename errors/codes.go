// Copyright 2026 Georg Hagn (tiny-frameworks)
// SPDX-License-Identifier: Apache-2.0

package errors

// exitInfo bundles the numeric code with a description.
type exitInfo struct {
	ExitCode int
	Desc     string
}

const (
	// Config
	UnsupportedWriterKind Code = "UNSUPPORTED_WRITER"
	ZugferdContainerError Code = "ZF_CONTAINER_FAILED"
	PdfContainerError     Code = "PDF_CONTAINER_FAILED"

	// Input
	MissingField  Code = "MISSING_FIELD"
	InvalidFormat Code = "INVALID_FORMAT"
	InvalidValue  Code = "INVALID_VALUE"

	// Normalize
	AmbiguousValue      Code = "AMBIGUOUS_VALUE"
	NormalizationFailed Code = "NORMALIZATION_FAILED"
	IncompleteParty     Code = "INCOMPLETE_PARTY"

	// Master Validate
	RuleViolation Code = "RULE_VIOLATION"
	TotalMismatch Code = "TOTAL_MISMATCH"
	Inconsistent  Code = "INCONSISTENT_DATA"

	// Render
	EmptyInput       Code = "EMPTY_INPUT"
	WriteError       Code = "WRITE_ERROR"
	ReadError        Code = "READ_ERROR"
	LoEngineError    Code = "LO_ENGINE_ERROR"
	JavaEngineError  Code = "JAVA_ENGINE_ERROR"
	ContainerTimeout Code = "TIME_OUT"
	JobStatusError   Code = "JOB_STATUS_ERROR"
	LockError        Code = "LOCK_ERROR"

	// misc errors and warnings
	NotYetImplemented Code = "NOT_YET_IMPLEMENTED"
	InternalError     Code = "INTERNAL_ERROR"
	Unknown           Code = "UNKNOWN_ERROR"
	ExecutionError    Code = "EXECUTION_ERROR"

	UnauthorizedError Code = "UNAUTHORIZED_ERROR"
	//OK            Code = "SUCCESS"
)

// The central registry (private)
var exitCodeMap = map[Code]exitInfo{
	MissingField:  {10, "a mandatory field is missing."},
	InvalidFormat: {10, "The data format is invalid or corrupt."},

	AmbiguousValue:        {10, "ambigous value."},
	NormalizationFailed:   {30, "normalization failed."},
	IncompleteParty:       {10, "Party [buyer or seller] is incomplete."},
	PdfContainerError:     {10, "Container for PDF generation not running."},
	ZugferdContainerError: {10, "Container for ZUGFeRD generation not running."},
	UnsupportedWriterKind: {10, "Writer-Kind for renderengine not supported."},

	// Master Validate
	RuleViolation: {30, "ruleviolation in input."},
	TotalMismatch: {30, "the expected total differs from input total ."},
	Inconsistent:  {30, "data is inconsistent."},

	// Render : infrastructure = 40
	EmptyInput:       {40, "input expected."},
	WriteError:       {40, "write error."},
	LockError:        {40, "lock file kwrite error."},
	ReadError:        {40, "read error."},
	LoEngineError:    {40, "LO engine [loEngine.odt] error."},
	JavaEngineError:  {40, "Java Engine [mustang-cli,jar] error."},
	ContainerTimeout: {40, "container [nexgate_lomanager] timeout error."},
	JobStatusError:   {40, "Jobstatus signalisiert error."},

	// common : unknow = 99
	NotYetImplemented: {90, "Not yet implemented, maybe later"},
	InternalError:     {99, "internal error"},
	Unknown:           {99, "unknown error"},
	UnauthorizedError: {99, "not authorized error"},
	ExecutionError:    {99, "os execution error"},
}

var AllCodes []Code // AllCodes serves as a reference list for tests and documentation.

func init() {
	AllCodes = make([]Code, 0, len(exitCodeMap))
	for code := range exitCodeMap {
		AllCodes = append(AllCodes, code)
	}
}

/*
   | Code | Meaning                  | Example                        |
   | ---- | ------------------------ | ------------------------------ |
   | 0    | OK                       | alles valid                    |
   | 10   | Config fehlerhaft        | YAML falsch, Pflichtfeld fehlt |
   | 20   | Preflight fehlgeschlagen | Java fehlt, Heartbeat stale    |
   | 30   | Laufzeitfehler           | Merge fehlgeschlagen           |
   | 40   | Infrastruktur            | Timeout, IO Crash              |
   | 90   | Implementierung          | NotYetImplemented              |
   | 99   | Unbekannt                | Panic                          |
*/
