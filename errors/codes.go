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

	// P2P
	ValidationError    Code = "VALIDATION_FAILED"
	NotFoundError      Code = "NOT_FOUND"
	InvalidOrExpired   Code = "SESSION_TOKEN_FAILED"
	Forbidden          Code = "FORBIDDEN"
	ResourceNotFound   Code = "RESOURCE_NOT_FOUND"
	UnhandledMethodErr Code = "UNHANDLED_METHOD"

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
	InvalidFormat: {10, "the data format is invalid or corrupt."},

	AmbiguousValue:        {10, "ambigous value."},
	IncompleteParty:       {10, "party [buyer or seller] is incomplete."},
	PdfContainerError:     {10, "container for PDF generation not running."},
	ZugferdContainerError: {10, "container for ZUGFeRD generation not running."},
	UnsupportedWriterKind: {10, "writer-Kind for renderengine not supported."},

	// Master Validate
	NormalizationFailed: {20, "normalization failed."},
	RuleViolation:       {20, "ruleviolation in input."},
	TotalMismatch:       {20, "the expected total differs from input total ."},
	Inconsistent:        {20, "data is inconsistent."},

	ValidationError:    {32, "validation failed error"},
	NotFoundError:      {32, "not found error"},
	InvalidOrExpired:   {32, "session/token invalid or expired"},
	Forbidden:          {32, "permission denied"},
	ResourceNotFound:   {32, "template / profile / attachment not found"},
	UnhandledMethodErr: {32, "unhandled method"},

	// Render : infrastructure = 40
	EmptyInput:       {40, "input expected."},
	WriteError:       {40, "write error."},
	LockError:        {40, "lock file kwrite error."},
	ReadError:        {40, "read error."},
	LoEngineError:    {40, "LO engine [loEngine.odt] error."},
	JavaEngineError:  {40, "java engine [mustang-cli,jar] error."},
	ContainerTimeout: {40, "container [nexgate_lomanager] timeout error."},
	JobStatusError:   {40, "jobstatus signalisiert error."},

	// common : unknow = 99
	NotYetImplemented: {90, "not yet implemented, maybe later"},

	InternalError:     {99, "internal error"},
	Unknown:           {99, "unknown error"},
	UnauthorizedError: {99, "not authorized error"},
	ExecutionError:    {99, "execution failed"},
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
   | 20   | MasterInvoice            | Validation, Normlization       |
   | 32   | nexutils P2P             | node, delegate, peer           |
   | 40   | Infrastruktur            | Timeout, IO Crash              |
   | 90   | Implementierung          | NotYetImplemented              |
   | 99   | Unbekannt                | Panic                          |
*/
