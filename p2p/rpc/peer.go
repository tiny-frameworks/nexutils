// Copyright 2026 Georg Hagn
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
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
	pendingMu    sync.Mutex
	pending      map[string]*pendingRequest
	pendingBatch map[string]chan []JsonRPCresponse

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
		ID:           fmt.Sprintf("%p", conn),
		Role:         role,
		conn:         conn,
		remoteAddr:   remoteAddr,
		node:         node,
		send:         make(chan any, 32),
		pending:      make(map[string]*pendingRequest),
		pendingBatch: make(map[string]chan []JsonRPCresponse),
		ctx:          pCtx,
		cancel:       cancel,
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
	var readErr error
	defer func() {
		p.node.unregisterPeer(p, readErr)
		p.Close()
	}()

	for {
		var rawMsg json.RawMessage
		err := wsjson.Read(p.ctx, p.conn, &rawMsg)
		if err != nil {
			readErr = err
			if websocket.CloseStatus(err) != -1 {
				logger.Logger.Debug("Peer disconnected normally", "peer", p.ID)
			} else {
				logger.Logger.Error("Read error in peer readLoop", "peer", p.ID, "err", err)
			}
			return
		}

		// ---------------------------------------------------------------------
		// FALL 1: Eingehende BATCH-Antwort (JSON-Array, beginnt mit '[')
		// ---------------------------------------------------------------------
		if len(rawMsg) > 0 && rawMsg[0] == '[' {
			var batchResponses []JsonRPCresponse
			if err := json.Unmarshal(rawMsg, &batchResponses); err == nil && len(batchResponses) > 0 {
				// Aus einer Sub-ID wie "batch_1_0" die Haupt-ID "batch_1" extrahieren
				firstID := string(batchResponses[0].ID)
				// Anführungszeichen entfernen, falls vorhanden
				if len(firstID) >= 2 && firstID[0] == '"' && firstID[len(firstID)-1] == '"' {
					firstID = firstID[1 : len(firstID)-1]
				}

				// Haupt-Batch-ID ermitteln (alles vor dem letzten '_')
				if idx := strings.LastIndex(firstID, "_"); idx != -1 {
					batchID := firstID[:idx] // z.B. "batch_1"

					p.pendingMu.Lock()
					ch, found := p.pendingBatch[batchID]
					p.pendingMu.Unlock()

					if found {
						ch <- batchResponses
						continue // Batch-Antwort erfolgreich verarbeitet
					}
				}
			}
		}

		// ---------------------------------------------------------------------
		// FALL 2: Eingehende EINZEL-Antwort (wie bisher)
		// ---------------------------------------------------------------------
		var resp JsonRPCresponse
		if err := json.Unmarshal(rawMsg, &resp); err == nil && resp.ID != nil && resp.Method == "" {
			idStr := string(resp.ID)

			p.pendingMu.Lock()
			pr, found := p.pending[idStr]
			p.pendingMu.Unlock()

			if found {
				pr.done <- resp
				continue // Einzelne Antwort erfolgreich verarbeitet
			}
		}

		// ---------------------------------------------------------------------
		// FALL 3: Eingehende Anforderung / Request an diesen Node (Einzeln oder Batch)
		// ---------------------------------------------------------------------
		p.node.handleIncomingRawMessage(p.ctx, p, rawMsg)
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
	ticker := time.NewTicker(p.node.heartbeatInterval)
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

// BatchItem repräsentiert einen einzelnen Aufruf innerhalb eines Batches
type BatchItem struct {
	Method string
	Params any
	Result any // Pointer auf Ziel-Struct für das Ergebnis
}

// CallBatch sendet mehrere RPC-Aufrufe als ein einziges JSON-Array.
func (p *Peer) CallBatch(ctx context.Context, items []BatchItem) []*JsonRPCerror {
	if len(items) == 0 {
		return nil
	}

	// 1. Vorab-Prüfung: Verbindung aktiv?
	select {
	case <-p.ctx.Done():
		errs := make([]*JsonRPCerror, len(items))
		for i := range errs {
			errs[i] = &JsonRPCerror{Code: InternalError, Message: "Peer connection is closed"}
		}
		return errs
	default:
	}

	// 2. Batch ID generieren (für das Dispatching des gesamten Batch-Response-Arrays)
	reqID := atomic.AddUint64(&p.nextID, 1)
	batchIDStr := fmt.Sprintf("batch_%d", reqID)

	// 3. Requests-Array zusammenbauen
	requests := make([]JsonRPCrequest, 0, len(items))
	for i, item := range items {
		var rawParams json.RawMessage
		if item.Params != nil {
			var err error
			rawParams, err = json.Marshal(item.Params)
			if err != nil {
				errs := make([]*JsonRPCerror, len(items))
				errs[i] = &JsonRPCerror{Code: InvalidParams, Message: "Failed to marshal params: " + err.Error()}
				return errs
			}
		}

		// Unter-ID für Zuordnung der einzelnen Antworten, z.B. "batch_1_0"
		subIDStr := fmt.Sprintf("%s_%d", batchIDStr, i)
		subIDRaw := json.RawMessage(fmt.Sprintf("%q", subIDStr))

		requests = append(requests, JsonRPCrequest{
			JSONRPC: jsonRPCversion,
			Method:  item.Method,
			Params:  rawParams,
			ID:      subIDRaw,
		})
	}

	// 4. Pending-Channel registrieren (über die Haupt-Batch-ID)
	done := make(chan []JsonRPCresponse, 1)
	p.pendingMu.Lock()
	// Hinweis: pendingBatch hält den Channel für Batch-Antworten
	p.pendingBatch[batchIDStr] = done
	p.pendingMu.Unlock()

	defer func() {
		p.pendingMu.Lock()
		delete(p.pendingBatch, batchIDStr)
		p.pendingMu.Unlock()
	}()

	// 5. Batch-Array als eine Frame-Message senden
	p.Send(requests)

	// 6. Auf Antwort warten oder Timeout
	select {
	case <-ctx.Done():
		errs := make([]*JsonRPCerror, len(items))
		for i := range errs {
			errs[i] = &JsonRPCerror{Code: InternalError, Message: "RPC batch call timed out: " + ctx.Err().Error()}
		}
		return errs

	case <-p.ctx.Done():
		errs := make([]*JsonRPCerror, len(items))
		for i := range errs {
			errs[i] = &JsonRPCerror{Code: InternalError, Message: "Peer connection closed during batch call"}
		}
		return errs

	case responses, ok := <-done:
		if !ok {
			errs := make([]*JsonRPCerror, len(items))
			for i := range errs {
				errs[i] = &JsonRPCerror{Code: InternalError, Message: "Batch response channel closed unexpectedly"}
			}
			return errs
		}

		// 7. Antworten den einzelnen BatchItems zuordnen
		errorsResult := make([]*JsonRPCerror, len(items))

		// Map zur schnellen Zuordnung via Sub-ID ("batch_1_0" -> Response)
		respMap := make(map[string]JsonRPCresponse, len(responses))
		for _, resp := range responses {
			if resp.ID != nil {
				// ID-String bereinigen (Anführungszeichen entfernen)
				idStr := string(resp.ID)
				if len(idStr) >= 2 && idStr[0] == '"' && idStr[len(idStr)-1] == '"' {
					idStr = idStr[1 : len(idStr)-1]
				}
				respMap[idStr] = resp
			}
		}

		for i, item := range items {
			subIDStr := fmt.Sprintf("%s_%d", batchIDStr, i)
			resp, found := respMap[subIDStr]
			if !found {
				errorsResult[i] = &JsonRPCerror{Code: InternalError, Message: "Missing response for batch item"}
				continue
			}

			if resp.Error != nil {
				errorsResult[i] = resp.Error
				continue
			}

			// Ergebnis in das Ziel-Objekt unmarshaln
			if item.Result != nil && resp.Result != nil {
				rawResult, err := json.Marshal(resp.Result)
				if err != nil {
					errorsResult[i] = &JsonRPCerror{Code: ParseError, Message: "Failed to marshal result: " + err.Error()}
					continue
				}
				if err := json.Unmarshal(rawResult, item.Result); err != nil {
					errorsResult[i] = &JsonRPCerror{Code: ParseError, Message: "Failed to unmarshal result target: " + err.Error()}
					continue
				}
			}
		}

		return errorsResult
	}
}
