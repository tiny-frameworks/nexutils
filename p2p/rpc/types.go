package rpc

import (
	"encoding/json"

	"codeberg.org/tiny-frameworks/nexutils/errors"
)

const (
	JRPCVERSION = "2.0"

	// Standard JSON-RPC Fehlercodes
	ErrCodeParseError      = -32700
	ErrConnectionLostError = -32701
	ErrCodeJSONError       = -32702
	ErrCodeInvalidRequest  = -32600
	ErrCodeMethodNotFound  = -32601
	ErrCodeInvalidParams   = -32602
	ErrCodeInternalError   = -32603

	// Custom App Error Codes (Example)
	ErrCodeUnauthorized = 401
	ErrCodeForbidden    = 403
)

// --- Error Messages Map ---
// Here we define the default text for each code.
var stdErrorMessages = map[int]string{
	ErrCodeParseError:      "Parse error",
	ErrConnectionLostError: "Connection lost during request",
	ErrCodeJSONError:       "JSON could not be created",
	ErrCodeInvalidRequest:  "Invalid Request",
	ErrCodeMethodNotFound:  "Method not found",
	ErrCodeInvalidParams:   "Invalid params",
	ErrCodeInternalError:   "Internal error",
	ErrCodeUnauthorized:    "Unauthorized",
	ErrCodeForbidden:       "Forbidden",
}

type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
	ID      json.RawMessage `json:"id,omitempty"`
}

type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
	ID      json.RawMessage `json:"id,omitempty"`
}

type RPCError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"` // Hier wandern nexutils/errors rein!
}

func NewRPCError(code int, message string) *RPCError {
	return &RPCError{
		Code:    code,
		Message: message,
	}
}

func (e *RPCError) Error() string {
	return e.Message
}

// AsNexError versucht, das 'data'-Feld des RPCError in einen nexutils.Error zu entpacken.
func (e *RPCError) AsNexError() (*errors.Error, bool) {
	if len(e.Data) == 0 {
		return nil, false
	}

	var nexErr errors.Error
	if err := json.Unmarshal(e.Data, &nexErr); err == nil && nexErr.Code != "" {
		return &nexErr, true
	}

	return nil, false
}
