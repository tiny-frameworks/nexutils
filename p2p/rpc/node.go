// Copyright 2026 Georg Hagn
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"context"
	"encoding/json"
	stdErrors "errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"codeberg.org/tiny-frameworks/nexutils/errors"
	"codeberg.org/tiny-frameworks/nexutils/p2p/transport"
)

type HandlerFunc func(ctx context.Context, params json.RawMessage) (any, error)

type Node struct {
	connMu sync.RWMutex
	conn   transport.Connection

	handlers map[string]HandlerFunc
	mu       sync.RWMutex

	pending   map[string]pendingRequest
	pendingMu sync.Mutex
	nextID    uint64

	dialAddr string
	provider *transport.WSProvider

	logger *slog.Logger
}

type pendingRequest struct {
	done chan Response
}

func NewNode(
	conn transport.Connection,
	provider *transport.WSProvider,
	dialAddr string,
	loggerInstance *slog.Logger,
) *Node {
	if loggerInstance == nil {
		loggerInstance = slog.Default()
	}

	return &Node{
		conn:     conn,
		handlers: make(map[string]HandlerFunc),
		pending:  make(map[string]pendingRequest),
		provider: provider,
		dialAddr: dialAddr,
		logger:   loggerInstance.With("component", "p2p.node"),
	}
}

func (node *Node) Register(method string, h HandlerFunc) {
	node.mu.Lock()
	defer node.mu.Unlock()
	node.handlers[method] = h
}

// Call sendet einen Request und blockiert, bis die Antwort eintrifft.
func (node *Node) Call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	node.connMu.RLock()
	currentConn := node.conn
	node.connMu.RUnlock()

	if currentConn == nil {
		return nil, NewRPCError(ErrCodeInternalError, "The connection is currently being re-established.")
	}

	node.pendingMu.Lock()
	node.nextID++
	idVal := node.nextID
	idJSON, _ := json.Marshal(idVal)

	ch := make(chan Response, 1)
	idStr := fmt.Sprintf("%d", idVal)
	node.pending[idStr] = pendingRequest{done: ch}
	node.pendingMu.Unlock()

	defer func() {
		node.pendingMu.Lock()
		delete(node.pending, idStr)
		node.pendingMu.Unlock()
	}()

	pBytes, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}

	req := Request{
		JSONRPC: JRPCVERSION,
		Method:  method,
		Params:  pBytes,
		ID:      idJSON,
	}

	data, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	if err := currentConn.Send(ctx, data); err != nil {
		return nil, err
	}

	select {
	case resp := <-ch:
		if resp.Error != nil {
			return nil, resp.Error
		}
		return resp.Result, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Notify sendet eine Benachrichtigung ohne erwartete Antwort.
func (node *Node) Notify(ctx context.Context, method string, params any) error {
	node.connMu.RLock()
	currentConn := node.conn
	node.connMu.RUnlock()

	if currentConn == nil {
		return NewRPCError(ErrCodeInternalError, "Notification failed: Reconnecting")
	}

	pBytes, err := json.Marshal(params)
	if err != nil {
		return err
	}

	req := Request{
		JSONRPC: JRPCVERSION,
		Method:  method,
		Params:  pBytes,
	}

	data, err := json.Marshal(req)
	if err != nil {
		return err
	}

	return currentConn.Send(ctx, data)
}

func (node *Node) Listen(ctx context.Context) error {
	for {
		node.connMu.RLock()
		currentConn := node.conn
		node.connMu.RUnlock()

		if currentConn == nil {
			if node.dialAddr == "" {
				return stdErrors.New("Connection lost and no reconnect address available")
			}

			node.logger.Info("Connection lost. Trying to reconnect...")
			if err := node.attemptReconnect(ctx); err != nil {
				return err
			}
			continue
		}

		data, err := currentConn.Receive(ctx)
		if err != nil {
			node.logger.Error("Network error during receive", "error", err)

			node.connMu.Lock()
			node.conn = nil
			node.connMu.Unlock()

			node.cleanupPendingRequests("Connection lost")
			continue
		}

		go node.handleIncoming(ctx, data)
	}
}

func (node *Node) attemptReconnect(ctx context.Context) error {
	backoff := 1 * time.Second
	maxBackoff := 30 * time.Second

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			node.logger.Info("Attempting reconnect", "dialAddr", node.dialAddr)
			newConn, err := node.provider.Dial(ctx, node.dialAddr)
			if err == nil {
				node.logger.Info("Reconnect successful!")
				node.connMu.Lock()
				node.conn = newConn
				node.connMu.Unlock()
				return nil
			}

			node.logger.Error("Reconnect failed, retrying...", "error", err, "backoff", backoff)

			time.Sleep(backoff)
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}
	}
}

type incomingMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method,omitempty"`
	ID      json.RawMessage `json:"id,omitempty"`
}

func (node *Node) handleIncoming(ctx context.Context, data []byte) {
	var msg incomingMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		node.logger.Error("Failed to parse incoming message", "error", err)
		return
	}

	if msg.Method != "" {
		var req Request
		if err := json.Unmarshal(data, &req); err == nil {
			node.processRequest(ctx, req)
		} else {
			node.logger.Error("Failed to unmarshal RPC request", "error", err)
		}
		return
	}

	var resp Response
	if err := json.Unmarshal(data, &resp); err == nil {
		node.processResponse(resp)
	} else {
		node.logger.Error("Failed to unmarshal RPC response", "error", err)
	}
}

func (node *Node) processRequest(ctx context.Context, req Request) {
	node.mu.RLock()
	handler, ok := node.handlers[req.Method]
	node.mu.RUnlock()

	var resp Response
	resp.JSONRPC = JRPCVERSION
	resp.ID = req.ID

	if !ok {
		resp.Error = NewRPCError(ErrCodeMethodNotFound, req.Method)
		node.logger.Warn("Method not found", "method", req.Method)
	} else {
		result, err := handler(ctx, req.Params)
		if err != nil {
			var nexErr *errors.Error
			if stdErrors.As(err, &nexErr) {
				// 1. JSON-RPC-Protokoll-Ebene: Standard-Code (-32603) und Nachricht
				rpcErr := NewRPCError(ErrCodeInternalError, nexErr.Message)

				// 2. Domänen-Ebene: Der GESAMTE nexutils.Error (inkl. String-Code, Path etc.) wandert in 'data'
				if dataBytes, marshalErr := json.Marshal(nexErr); marshalErr == nil {
					rpcErr.Data = dataBytes
				}

				resp.Error = rpcErr
			} else {
				// Fallback für einfache Go-Standardfehler
				resp.Error = NewRPCError(ErrCodeInternalError, err.Error())
			}
			node.logger.Error("Handler execution failed", "method", req.Method, "error", err)
		} else {
			resBytes, _ := json.Marshal(result)
			resp.Result = resBytes
		}
	}

	if req.ID != nil && string(req.ID) != "null" {
		respBytes, _ := json.Marshal(resp)
		_ = node.conn.Send(ctx, respBytes)
	} else {
		node.logger.Info("Notification processed", "method", req.Method)
	}
}

func (node *Node) processResponse(resp Response) {
	idStr := strings.Trim(string(resp.ID), `"`)

	node.pendingMu.Lock()
	req, ok := node.pending[idStr]
	node.pendingMu.Unlock()

	if ok {
		req.done <- resp
	}
}

func (node *Node) cleanupPendingRequests(reason string) {
	node.pendingMu.Lock()
	defer node.pendingMu.Unlock()
	for id, req := range node.pending {
		req.done <- Response{
			ID:    json.RawMessage(id),
			Error: NewRPCError(ErrCodeInternalError, reason),
		}
		delete(node.pending, id)
	}
}
