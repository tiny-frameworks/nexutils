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

// NewRPCErrorFromNexError converts a nexutils/errors.Error into a JsonRPCerror.
// The nexutils error object is cleanly included in the 'Data' field.
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

// rawJSONID Helperfunction for IDs
func rawJSONID(id int64) json.RawMessage {
	b, _ := json.Marshal(id)
	return b
}

// UserAuthenticator must be implemented by every user management system.
type UserAuthenticator interface {
	// Authenticate checks the username and password.
	// Returns true if the credentials are correct.
	Authenticate(ctx context.Context, username, password string) bool
}

// Default/Fallback Provider (for Demos or Tests)
type DummyAuthenticator struct{}

func (d *DummyAuthenticator) Authenticate(ctx context.Context, username, password string) bool {
	// Maintain standard behavior as long as no actual provider is set.
	return username == "georg" && password == "secret"
}

// IsResponse checks whether it is a JSON-RPC response to a request of its own.
func (req *JsonRPCrequest) IsResponse() bool {
	return req.Method == "" && req.ID != nil
}

// UnmarshalParams is a convenient helper method for parsing JSON-RPC parameters
// directly into a target struct or variable.
func (req *JsonRPCrequest) UnmarshalParams(v any) error {
	if len(req.Params) == 0 {
		return nil
	}
	return json.Unmarshal(req.Params, v)
}
