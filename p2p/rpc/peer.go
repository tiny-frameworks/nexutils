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

// pendingRequest hält den Rückgabekanal für einen synchronen Call
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
	// 1. Context abbrechen (signalisiert allen Goroutinen: Peer fährt herunter)
	if p.cancel != nil {
		p.cancel()
	}

	// 2. WebSocket-Verbindung sauber schließen
	if p.conn != nil {
		_ = p.conn.Close(websocket.StatusNormalClosure, "closing peer")
	}

	// 3. Alle schwebenden RPC-Aufrufe sofort abbrechen (Resource Cleanup)
	p.pendingMu.Lock()
	for id, req := range p.pending {
		close(req.done)       // Entsperrt wartende select-Blöcke in peer.Call()
		delete(p.pending, id) // Räumt die Map auf
	}
	p.pendingMu.Unlock()

}

// --- Synchrone Call Methode ---

// Call sendet eine Anfrage an den Peer und wartet synchron auf die Antwort.
func (p *Peer) Call(ctx context.Context, method string, params any, resultTarget any) *JsonRPCerror {

	// 1. Vorab-Check: Steht die Verbindung überhaupt noch?
	select {
	case <-p.ctx.Done():
		return &JsonRPCerror{Code: InternalError, Message: "Peer connection is closed"}
	default:
	}

	// 2. Eindeutige ID erzeugen
	reqID := atomic.AddUint64(&p.nextID, 1)
	idStr := fmt.Sprintf("%d", reqID)
	idRaw := json.RawMessage(idStr)

	// 3. Params marshalfizieren (falls vorhanden)
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

	// 4. Pending-Channel registrieren
	done := make(chan JsonRPCresponse, 1)
	p.pendingMu.Lock()
	p.pending[idStr] = &pendingRequest{done: done}
	p.pendingMu.Unlock()

	// 5. Sicherstellen, dass die Registrierung bei Abbruch/Ende aufgeräumt wird
	defer func() {
		p.pendingMu.Lock()
		delete(p.pending, idStr)
		p.pendingMu.Unlock()
	}()

	// 6. Request absenden
	p.Send(req)

	// 7. Warten auf Antwort oder Context-Timeout
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

		// 8. Resultat in das Ziel-Objekt unmarshalfizieren
		if resultTarget != nil && resp.Result != nil {
			// resp.Result Typ 'any' :=> erst in JSON-Bytes wandeln...
			rawResult, err := json.Marshal(resp.Result)
			if err != nil {
				return &JsonRPCerror{Code: ParseError, Message: "Failed to marshal result: " + err.Error()}
			}
			// ...und dann unmarshaln
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

		// Zuerst versuchen als Response (Antwort auf eigenen Call) zu parsen
		var resp JsonRPCresponse
		if err := json.Unmarshal(rawMsg, &resp); err == nil && resp.ID != nil && resp.Method == "" {
			idStr := string(resp.ID)

			p.pendingMu.Lock()
			pr, found := p.pending[idStr]
			p.pendingMu.Unlock()

			if found {
				pr.done <- resp
				continue // Erfolgreich gematcht, nicht als Inbound Request verarbeiten!
			}
		}

		// Falls es keine Response war -> als eingehende Request verarbeiten
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

// Notify sendet eine Nachricht ohne ID an den Peer (Fire-and-Forget, keine Antwort erwartet).
func (p *Peer) Notify(method string, params any) error {
	var rawParams json.RawMessage
	if params != nil {
		var err error
		rawParams, err = json.Marshal(params)
		if err != nil {
			return err
		}
	}

	// Nachricht OHNE ID ist per JSON-RPC 2.0 Spezifikation ein Notification-Signal
	req := JsonRPCrequest{
		JSONRPC: jsonRPCversion,
		Method:  method,
		Params:  rawParams,
	}

	p.Send(req)
	return nil
}
