// Copyright 2026 Georg Hagn
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"context"
	"encoding/json"

	"codeberg.org/tiny-frameworks/nexutils/errors"
)

const jsonRPCversion = "2.0"

// Standard JSON-RPC 2.0 Error Codes
const (
	ParseError     = -32700
	InvalidRequest = -32600
	MethodNotFound = -32601
	InvalidParams  = -32602
	InternalError  = -32603

	// Custom Business / Auth Errors
	UnAuthorized     = -32001
	NotAuthorized    = -32002
	InvalidOrExpired = -32003
)

var StdError = map[int]string{
	ParseError:       "Parse error",
	InvalidRequest:   "Invalid Request",
	MethodNotFound:   "Method not found",
	InvalidParams:    "Invalid params",
	InternalError:    "Internal error",
	UnAuthorized:     "Unauthorized",
	NotAuthorized:    "Not authorized",
	InvalidOrExpired: "Invalid or expired token",
}

type JsonRPCrequest struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
	ID      json.RawMessage `json:"id,omitempty"`
}

type JsonRPCerror struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

type JsonRPCresponse struct {
	JSONRPC string          `json:"jsonrpc"`
	Result  any             `json:"result,omitempty"`
	Error   *JsonRPCerror   `json:"error,omitempty"`
	Method  string          `json:"method,omitempty"`
	ID      json.RawMessage `json:"id,omitempty"`
}

// NewRPCErrorFromNexError konvertiert ein nexutils/errors.Error in einen JsonRPCerror.
// Das nexutils Error-Objekt wird dabei sauber im 'Data'-Feld mitgeliefert.
func NewRPCErrorFromNexError(code int, err *errors.Error) *JsonRPCerror {
	msg := StdError[code]
	if msg == "" && err != nil {
		msg = err.Message
	}
	return &JsonRPCerror{
		Code:    code,
		Message: msg,
		Data:    err,
	}
}

// rawJSONID Hilfsfunktion für IDs
func rawJSONID(id int64) json.RawMessage {
	b, _ := json.Marshal(id)
	return b
}

// UserAuthenticator muss von jeder User-Verwaltung implementiert werden.
type UserAuthenticator interface {
	// Authenticate prüft Username & Password.
	// Gibt true zurück, wenn die Credentials korrekt sind.
	Authenticate(ctx context.Context, username, password string) bool
}

// Default/Fallback Provider (für Demos oder Tests)
type DummyAuthenticator struct{}

func (d *DummyAuthenticator) Authenticate(ctx context.Context, username, password string) bool {
	// Standard-Verhalten beibehalten, solange kein echter Provider gesetzt ist
	return username == "georg" && password == "secret"
}

// IsResponse prüft, ob es sich um eine JSON-RPC Antwort auf einen eigenen Request handelt
func (req *JsonRPCrequest) IsResponse() bool {
	return req.Method == "" && req.ID != nil
}

// UnmarshalParams ist eine bequeme Hilfsmethode, um die JSON-RPC Parameter
// direkt in ein Ziel-Struct oder eine Variable zu parsen.
func (req *JsonRPCrequest) UnmarshalParams(v any) error {
	if len(req.Params) == 0 {
		return nil
	}
	return json.Unmarshal(req.Params, v)
}
