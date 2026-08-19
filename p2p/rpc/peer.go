// Copyright 2026 Georg Hagn
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"codeberg.org/tiny-frameworks/nexutils/logger"
)

type PeerRole int

const (
	RoleInbound PeerRole = iota
	RoleOutbound
)

// pendingRequest holds the return channel for a synchronous call.
type pendingRequest struct {
	done chan JsonRPCresponse
}

type Peer struct {
	ID         string
	Role       PeerRole
	conn       *websocket.Conn
	remoteAddr string
	node       *Node
	send       chan any
	ctx        context.Context
	cancel     context.CancelFunc

	// Request ID Generator
	nextID uint64

	// Sync Call Matching
	pendingMu sync.Mutex
	pending   map[string]*pendingRequest

	// Auth & State
	mu         sync.RWMutex
	username   string
	token      string
	expires    time.Time
	authorized bool
}

func NewPeer(conn *websocket.Conn, remoteAddr string) *Peer {
	return &Peer{
		conn:       conn,
		remoteAddr: remoteAddr,
	}
}

func newPeer(ctx context.Context, conn *websocket.Conn, role PeerRole, node *Node, remoteAddr string) *Peer {
	pCtx, cancel := context.WithCancel(ctx)
	return &Peer{
		ID:         fmt.Sprintf("%p", conn),
		Role:       role,
		conn:       conn,
		remoteAddr: remoteAddr,
		node:       node,
		send:       make(chan any, 32),
		pending:    make(map[string]*pendingRequest),
		ctx:        pCtx,
		cancel:     cancel,
	}
}

func (p *Peer) RemoteAddr() string {
	if p == nil || p.remoteAddr == "" {
		return "unknown"
	}
	return p.remoteAddr
}

func (p *Peer) Start() {
	go p.readLoop()
	go p.writeLoop()

	if p.Role == RoleOutbound {
		go p.heartbeatLoop()
	}
}

func (p *Peer) Close() {
	// 1. Cancel context (signals to all goroutines: peer is shutting down)
	if p.cancel != nil {
		p.cancel()
	}

	// 2. Cleanly close the WebSocket connection
	if p.conn != nil {
		_ = p.conn.Close(websocket.StatusNormalClosure, "closing peer")
	}

	// 3. Immediately cancel all pending RPC calls (resource cleanup)
	p.pendingMu.Lock()
	for id, req := range p.pending {
		close(req.done)       // Unblocks waiting select blocks in peer.Call()
		delete(p.pending, id) // Clear up the map.
	}
	p.pendingMu.Unlock()

}

// --- Synchrone Call Methode ---

// Call sends a request to the peer and waits synchronously for the response.
func (p *Peer) Call(ctx context.Context, method string, params any, resultTarget any) *JsonRPCerror {

	// 1. Preliminary check: Is the connection actually still active?
	select {
	case <-p.ctx.Done():
		return &JsonRPCerror{Code: InternalError, Message: "Peer connection is closed"}
	default:
	}

	// 2. Create unique ID
	reqID := atomic.AddUint64(&p.nextID, 1)
	idStr := fmt.Sprintf("%d", reqID)
	idRaw := json.RawMessage(idStr)

	// 3. Marshal params (if present)
	var rawParams json.RawMessage
	if params != nil {
		var err error
		rawParams, err = json.Marshal(params)
		if err != nil {
			return &JsonRPCerror{Code: InvalidParams, Message: "Failed to marshal params: " + err.Error()}
		}
	}

	req := JsonRPCrequest{
		JSONRPC: jsonRPCversion,
		Method:  method,
		Params:  rawParams,
		ID:      idRaw,
	}

	// 4. register Pending-Channel
	done := make(chan JsonRPCresponse, 1)
	p.pendingMu.Lock()
	p.pending[idStr] = &pendingRequest{done: done}
	p.pendingMu.Unlock()

	// 5. Ensure that the registration is cleaned up upon cancellation or termination.
	defer func() {
		p.pendingMu.Lock()
		delete(p.pending, idStr)
		p.pendingMu.Unlock()
	}()

	// 6. sent Request
	p.Send(req)

	// 7. Wait for answer or Context-Timeout
	select {
	case <-ctx.Done():
		return &JsonRPCerror{Code: InternalError, Message: "RPC call timed out or canceled: " + ctx.Err().Error()}
	case <-p.ctx.Done():
		return &JsonRPCerror{Code: InternalError, Message: "Peer connection closed during call"}
	case resp, ok := <-done:
		if !ok {
			return &JsonRPCerror{Code: InternalError, Message: "Pending response channel closed unexpectedly"}
		}
		if resp.Error != nil {
			return resp.Error
		}

		// 8. Unmarshal the result into the target object.
		if resultTarget != nil && resp.Result != nil {
			// resp.Result Typ 'any' :=> first convert to JSON bytes...
			rawResult, err := json.Marshal(resp.Result)
			if err != nil {
				return &JsonRPCerror{Code: ParseError, Message: "Failed to marshal result: " + err.Error()}
			}
			// ...and then unmarshal
			if err := json.Unmarshal(rawResult, resultTarget); err != nil {
				return &JsonRPCerror{Code: ParseError, Message: "Failed to unmarshal result target: " + err.Error()}
			}
		}
		return nil
	}
}

// --- Loops ---

func (p *Peer) readLoop() {
	defer func() {
		p.node.unregisterPeer(p)
		p.Close()
	}()

	for {
		var rawMsg json.RawMessage
		err := wsjson.Read(p.ctx, p.conn, &rawMsg)
		if err != nil {
			if websocket.CloseStatus(err) != -1 {
				logger.Logger.Debug("Peer disconnected normally", "peer", p.ID)
			} else {
				logger.Logger.Error("Read error in peer readLoop", "peer", p.ID, "err", err)
			}
			return
		}

		// First, attempt to parse it as a response (a reply to your own call).
		var resp JsonRPCresponse
		if err := json.Unmarshal(rawMsg, &resp); err == nil && resp.ID != nil && resp.Method == "" {
			idStr := string(resp.ID)

			p.pendingMu.Lock()
			pr, found := p.pending[idStr]
			p.pendingMu.Unlock()

			if found {
				pr.done <- resp
				continue // Successfully matched; do not process as an inbound request!
			}
		}

		// If it was not a response -> process as an incoming request.
		var req JsonRPCrequest
		if err := json.Unmarshal(rawMsg, &req); err == nil {
			p.node.requests <- clientRequest{peer: p, req: req}
		}
	}
}

func (p *Peer) writeLoop() {
	for {
		select {
		case <-p.ctx.Done():
			return
		case msg, ok := <-p.send:
			if !ok {
				return
			}
			if err := wsjson.Write(p.ctx, p.conn, msg); err != nil {
				logger.Logger.Error("Write error in peer writeLoop", "peer", p.ID, "err", err)
				return
			}
		}
	}
}

func (p *Peer) heartbeatLoop() {
	ticker := time.NewTicker(p.node.opts.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-p.ctx.Done():
			return
		case <-ticker.C:
			req := JsonRPCrequest{
				JSONRPC: jsonRPCversion,
				Method:  "heartbeat",
				ID:      rawJSONID(time.Now().UnixNano()),
			}
			p.Send(req)
		}
	}
}

// --- Helper & State ---

func (p *Peer) Send(msg any) {
	select {
	case p.send <- msg:
	default:
		logger.Logger.Warn("Peer send buffer full — message dropped", "peer", p.ID)
	}
}

func (p *Peer) SetAuth(username, token string, ttl time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.username = username
	p.token = token
	p.expires = time.Now().Add(ttl)
	p.authorized = true
}

func (p *Peer) IsAuthorized() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if !p.authorized || p.token == "" {
		return false
	}
	return p.expires.After(time.Now())
}

func (p *Peer) Username() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.username
}

// Notify sends a message without an ID to the peer (fire-and-forget; no response expected).
func (p *Peer) Notify(method string, params any) error {
	var rawParams json.RawMessage
	if params != nil {
		var err error
		rawParams, err = json.Marshal(params)
		if err != nil {
			return err
		}
	}

	// A message without an ID is a notification signal according to the JSON-RPC 2.0 specification.
	req := JsonRPCrequest{
		JSONRPC: jsonRPCversion,
		Method:  method,
		Params:  rawParams,
	}

	p.Send(req)
	return nil
}
