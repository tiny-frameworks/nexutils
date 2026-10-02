// Copyright 2026 Georg Hagn
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"context"
	"encoding/json"

	//"errors"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"

	"codeberg.org/tiny-frameworks/nexutils/errors"
	"codeberg.org/tiny-frameworks/nexutils/logger"
)

type Node struct {
	// Konfiguration (aus Options übernommen)
	addr              string
	heartbeatInterval time.Duration
	shutdownDelay     time.Duration
	writeReadLimit    int64
	delegate          NexDelegate

	// Laufzeit-Status
	httpServer *http.Server

	mu    sync.RWMutex
	peers map[*Peer]bool

	ctx    context.Context
	cancel context.CancelFunc
}

// NewNode erstellt einen Node mit den angegebenen Options.
func NewNode(opts Options) *Node {
	opts.setDefaults()

	ctx, cancel := context.WithCancel(context.Background())

	return &Node{
		addr:              opts.Addr,
		heartbeatInterval: opts.HeartbeatInterval,
		shutdownDelay:     opts.ShutdownDelay,
		writeReadLimit:    opts.WriteReadLimit,
		delegate:          opts.Delegate,

		peers:  make(map[*Peer]bool),
		ctx:    ctx,
		cancel: cancel,
	}
}

func (n *Node) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", n.handleWS)

	n.httpServer = &http.Server{
		Addr:    n.addr,
		Handler: mux,
	}

	logger.Logger.Info("RPC Node listening for WebSocket connections", "addr", n.addr)
	return n.httpServer.ListenAndServe()
}

func (n *Node) Stop() error {
	n.cancel()

	n.mu.Lock()
	for p := range n.peers {
		p.Close()
	}
	n.mu.Unlock()

	ctxShutdown, cancel := context.WithTimeout(context.Background(), n.shutdownDelay)
	defer cancel()

	if n.httpServer != nil {
		return n.httpServer.Shutdown(ctxShutdown)
	}
	return nil
}

func (n *Node) ConnectToPeer(targetURL string) (*Peer, error) {
	conn, _, err := websocket.Dial(n.ctx, targetURL, nil)
	if err != nil {
		return nil, err
	}
	conn.SetReadLimit(n.writeReadLimit)

	peer := newPeer(n.ctx, conn, RoleOutbound, n, targetURL)

	// Delegate Validierung (z.B. Auth / Outbound Check)
	if !n.delegate.ValidatePeer(peer) {
		peer.Close()
		nexErr := errors.New(
			errors.InternalError,
			"peer validation failed by delegate",
			"p2p.rpc.ConnectToPeer",
		)
		rpcErr := NewRPCErrorFromNexError(InternalError, nexErr)
		return nil, rpcErr
	}

	n.registerPeer(peer)
	peer.Start()

	logger.Logger.Info("Connected to remote RPC peer", "target", targetURL)
	return peer, nil
}

func (n *Node) handleWS(w http.ResponseWriter, r *http.Request) {
	clientAddr := r.RemoteAddr

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
	})
	if err != nil {
		logger.Logger.Error("Failed to accept websocket connection", "err", err)
		return
	}
	conn.SetReadLimit(n.writeReadLimit)

	peer := newPeer(n.ctx, conn, RoleInbound, n, clientAddr)

	// Delegate Validierung für Inbound Verbindungen
	if !n.delegate.ValidatePeer(peer) {
		peer.Close()
		return
	}

	n.registerPeer(peer)
	peer.Start()
}

func (n *Node) registerPeer(p *Peer) {
	n.mu.Lock()
	n.peers[p] = true
	n.mu.Unlock()

	// Hook bei erfolgreicher Registrierung aufrufen
	n.delegate.OnPeerConnected(p)
}

func (n *Node) unregisterPeer(p *Peer, err error) {
	n.mu.Lock()
	_, exists := n.peers[p]
	if exists {
		delete(n.peers, p)
	}
	n.mu.Unlock()

	if exists {
		n.delegate.OnPeerDisconnected(p, err)
	}
}

// handleIncomingRequest wird vom Peer aufgerufen, wenn ein Frame eintrifft
func (n *Node) handleIncomingRequest(ctx context.Context, peer *Peer, req JsonRPCrequest) {
	// Fall 1: Notification (keine ID vorhanden)
	if req.ID == nil && req.Method != "" {
		n.delegate.OnNotification(ctx, peer, req.Method, req.Params)
		return
	}

	// Fall 2: Request (ID und Methode vorhanden)
	if req.Method != "" {
		result, err := n.delegate.OnRequest(ctx, peer, req.Method, req.Params)
		if err != nil {
			var rpcErr *JsonRPCerror
			if !errors.As(err, &rpcErr) {
				var nexErr *errors.Error
				if errors.As(err, &nexErr) {
					rpcErr = NewRPCErrorFromNexError(InternalError, nexErr)
				} else {
					rpcErr = &JsonRPCerror{
						Code:    InternalError,
						Message: err.Error(),
					}
				}
			}

			n.delegate.OnError(peer, rpcErr)

			peer.Send(JsonRPCresponse{
				JSONRPC: jsonRPCversion,
				Error:   rpcErr,
				ID:      req.ID,
			})
			return
		}

		peer.Send(JsonRPCresponse{
			JSONRPC: jsonRPCversion,
			Result:  result,
			ID:      req.ID,
		})
	}
}

/*
func (n *Node) SetAuthenticator(auth UserAuthenticator) {
	n.router.SetAuthenticator(auth)
}
*/

// ConnectWithAutoReconnect creates a ManagedClient that autonomously connects,
// authenticates, and manages reconnections in the background.
func (n *Node) ConnectWithAutoReconnect(targetURL string, cfg ReconnectConfig) *ManagedClient {
	client := NewManagedClient(n, targetURL, cfg)
	client.Start() // Starts the synchronizing lifecycleLoop in the background.
	return client
}

// Broadcast functionality
// PeerFilter is a function that determines whether a peer should receiv e a signal.
type PeerFilter func(p *Peer) bool

// BroadcastFilter sends a notification signal to all peers to which the filter applies.
func (n *Node) BroadcastFilter(method string, params any, filter PeerFilter) {
	n.mu.RLock()
	// We take a snapshot of the current peers in order to quickly release the mutex.
	activePeers := make([]*Peer, 0, len(n.peers))
	for p := range n.peers {
		activePeers = append(activePeers, p)
	}
	n.mu.RUnlock()

	// Iterate over and filter the list
	for _, p := range activePeers {
		// If a filter was passed and it returns 'false' -> skip
		if filter != nil && !filter(p) {
			continue
		}

		if err := p.Notify(method, params); err != nil {
			logger.Logger.Warn("Failed to send broadcast notify to peer", "peer", p.ID, "err", err)
		}
	}
}

// Broadcast sends a notification signal to ALL connected peers.
func (n *Node) Broadcast(method string, params any) {
	n.BroadcastFilter(method, params, nil)
}

// BroadcastAuthorized sends a notification signal ONLY to authenticated peers.
func (n *Node) BroadcastAuthorized(method string, params any) {
	n.BroadcastFilter(method, params, func(p *Peer) bool {
		return p.IsAuthorized()
	})
}

// BroadcastToUsers sends a notification signal to a specific list of usernames.
func (n *Node) BroadcastToUsers(method string, params any, usernames []string) {
	userMap := make(map[string]bool, len(usernames))
	for _, u := range usernames {
		userMap[u] = true
	}

	n.BroadcastFilter(method, params, func(p *Peer) bool {
		return p.IsAuthorized() && userMap[p.Username()]
	})
}

// handleIncomingRawMessage wird aufgerufen, wenn vom Peer ein Frame empfangen wird
func (n *Node) handleIncomingRawMessage(ctx context.Context, peer *Peer, rawMsg []byte) {
	// -------------------------------------------------------------------------
	// FALL 1: Eingehender BATCH-Request (JSON-Array, beginnt mit '[')
	// -------------------------------------------------------------------------
	if len(rawMsg) > 0 && rawMsg[0] == '[' {
		var requests []JsonRPCrequest
		if err := json.Unmarshal(rawMsg, &requests); err != nil {
			peer.Send(JsonRPCresponse{
				JSONRPC: jsonRPCversion,
				Error:   &JsonRPCerror{Code: ParseError, Message: "Invalid Batch JSON: " + err.Error()},
			})
			return
		}

		responses := make([]JsonRPCresponse, 0, len(requests))

		for _, req := range requests {
			// Sub-Fall A: Notification im Batch (keine Antwort erzeugen)
			if req.ID == nil && req.Method != "" {
				n.delegate.OnNotification(ctx, peer, req.Method, req.Params)
				continue
			}

			// Sub-Fall B: Synchroner Request im Batch
			if req.Method != "" {
				result, err := n.delegate.OnRequest(ctx, peer, req.Method, req.Params)
				if err != nil {
					var rpcErr *JsonRPCerror
					if !errors.As(err, &rpcErr) {
						var nexErr *errors.Error
						if errors.As(err, &nexErr) {
							rpcErr = NewRPCErrorFromNexError(InternalError, nexErr)
						} else {
							rpcErr = &JsonRPCerror{
								Code:    InternalError,
								Message: err.Error(),
							}
						}
					}

					n.delegate.OnError(peer, rpcErr)

					responses = append(responses, JsonRPCresponse{
						JSONRPC: jsonRPCversion,
						Error:   rpcErr,
						ID:      req.ID,
					})
					continue
				}

				responses = append(responses, JsonRPCresponse{
					JSONRPC: jsonRPCversion,
					Result:  result,
					ID:      req.ID,
				})
			}
		}

		// Antwort-Array nur senden, wenn mind. ein Request eine Response erfordert
		if len(responses) > 0 {
			peer.Send(responses)
		}
		return
	}

	// -------------------------------------------------------------------------
	// FALL 2: Einzelner Request
	// -------------------------------------------------------------------------
	var singleReq JsonRPCrequest
	if err := json.Unmarshal(rawMsg, &singleReq); err == nil {
		n.handleIncomingRequest(ctx, peer, singleReq)
	}
}
