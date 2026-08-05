// Copyright 2026 Georg Hagn
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"sync"
	"time"

	"codeberg.org/tiny-frameworks/nexutils/logger"
)

type JsonRPChandler func(p *Peer, req JsonRPCrequest) (any, *JsonRPCerror)

type clientRequest struct {
	peer *Peer
	req  JsonRPCrequest
}

type SessionData struct {
	Username string
	Expires  time.Time
}

type Router struct {
	handlers map[string]JsonRPChandler

	// Session store at the node level (for reconnects)
	sessionMu sync.RWMutex
	sessions  map[string]SessionData

	// the abstracted interface
	authenticator UserAuthenticator
}

func newRouter() *Router {
	r := &Router{
		handlers:      make(map[string]JsonRPChandler),
		sessions:      make(map[string]SessionData),
		authenticator: &DummyAuthenticator{}, // Standard-Fallback
	}
	r.registerStandardHandlers()
	return r
}

// SetAuthenticator allows the injection of any user management system.
func (r *Router) SetAuthenticator(auth UserAuthenticator) {
	if auth != nil {
		r.authenticator = auth
	}
}

func (r *Router) RegisterHandler(method string, h JsonRPChandler) {
	r.handlers[method] = h
}

func (r *Router) dispatchLoop(nodeCtx contextContext, requests <-chan clientRequest) {
	for {
		select {
		case <-nodeCtx.Done():
			return
		case req, ok := <-requests:
			if !ok {
				return
			}

			handler, found := r.handlers[req.req.Method]
			if !found {
				sendError(req.peer, req.req.ID, MethodNotFound, StdError[MethodNotFound])
				continue
			}

			go func(h JsonRPChandler, cr clientRequest) {
				result, rpcErr := h(cr.peer, cr.req)
				if cr.req.ID != nil {
					resp := JsonRPCresponse{
						JSONRPC: jsonRPCversion,
						ID:      cr.req.ID,
					}
					if rpcErr != nil {
						resp.Error = rpcErr
					} else {
						resp.Result = result
					}
					cr.peer.Send(resp)
				}
			}(handler, req)
		}
	}
}

// --- Built-in Standard Handlers ---

func (r *Router) registerStandardHandlers() {
	// 1. Heartbeat handler with integrated time synchronization
	r.RegisterHandler("heartbeat", func(p *Peer, req JsonRPCrequest) (any, *JsonRPCerror) {
		return map[string]any{
			"status": "pong",
			"time":   time.Now().Format(time.RFC3339Nano),
		}, nil
	})

	// 2. Auth & Reconnect Handler
	r.RegisterHandler("auth", r.handleAuth)
}

type AuthParams struct {
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	Token    string `json:"token,omitempty"`
}

type AuthResult struct {
	Status   string `json:"status"`
	Token    string `json:"token,omitempty"`
	Username string `json:"username,omitempty"`
}

func (r *Router) handleAuth(p *Peer, req JsonRPCrequest) (any, *JsonRPCerror) {
	var params AuthParams
	if err := req.UnmarshalParams(&params); err != nil {
		return nil, &JsonRPCerror{Code: InvalidParams, Message: StdError[InvalidParams]}
	}

	// 1. Reconnect via Token
	if params.Token != "" {
		r.sessionMu.RLock()
		sess, found := r.sessions[params.Token]
		r.sessionMu.RUnlock()

		if found && sess.Expires.After(time.Now()) {
			ttl := time.Until(sess.Expires)
			p.SetAuth(sess.Username, params.Token, ttl)

			logger.Logger.Info("Session restored via token", "peer", p.ID, "user", sess.Username)
			return AuthResult{
				Status:   "session restored",
				Username: sess.Username,
				Token:    params.Token,
			}, nil
		}

		return nil, &JsonRPCerror{Code: InvalidOrExpired, Message: StdError[InvalidOrExpired]}
	}

	// 2. Login via Username/Password
	if params.Username != "" && params.Password != "" {

		// Access the connected provider.
		if r.authenticator.Authenticate(p.ctx, params.Username, params.Password) {
			token := generateToken()
			ttl := 24 * time.Hour
			expires := time.Now().Add(ttl)

			r.sessionMu.Lock()
			r.sessions[token] = SessionData{
				Username: params.Username,
				Expires:  expires,
			}
			r.sessionMu.Unlock()

			p.SetAuth(params.Username, token, ttl)

			logger.Logger.Info("User authenticated successfully", "peer", p.ID, "user", params.Username)
			return AuthResult{
				Status:   "authenticated",
				Token:    token,
				Username: params.Username,
			}, nil
		}
	}

	return nil, &JsonRPCerror{Code: UnAuthorized, Message: StdError[UnAuthorized]}
}

func sendError(p *Peer, id json.RawMessage, code int, msg string) {
	p.Send(JsonRPCresponse{
		JSONRPC: jsonRPCversion,
		Error:   &JsonRPCerror{Code: code, Message: msg},
		ID:      id,
	})
}

func generateToken() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

type contextContext = interface {
	Done() <-chan struct{}
}
