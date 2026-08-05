// Copyright 2026 Georg Hagn
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"context"
	"net/http"
	"sync"

	"github.com/coder/websocket"

	"codeberg.org/tiny-frameworks/nexutils/logger"
)

type Node struct {
	opts       Options
	router     *Router
	httpServer *http.Server

	mu       sync.RWMutex
	peers    map[*Peer]bool
	requests chan clientRequest

	ctx    context.Context
	cancel context.CancelFunc
}

func NewNode(opts Options) *Node {
	opts.setDefaults()

	ctx, cancel := context.WithCancel(context.Background())

	n := &Node{
		opts:     opts,
		router:   newRouter(),
		peers:    make(map[*Peer]bool),
		requests: make(chan clientRequest, 64),
		ctx:      ctx,
		cancel:   cancel,
	}

	return n
}

func (n *Node) RegisterHandler(method string, h JsonRPChandler) {
	n.router.RegisterHandler(method, h)
}

func (n *Node) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", n.handleWS)

	n.httpServer = &http.Server{
		Addr:    n.opts.Addr,
		Handler: mux,
	}

	go n.router.dispatchLoop(n.ctx, n.requests)

	logger.Logger.Info("RPC Node listening for WebSocket connections", "addr", n.opts.Addr)
	return n.httpServer.ListenAndServe()
}

func (n *Node) Stop() error {
	n.cancel()

	n.mu.Lock()
	for p := range n.peers {
		p.Close()
	}
	n.mu.Unlock()

	ctxShutdown, cancel := context.WithTimeout(context.Background(), n.opts.ShutdownDelay)
	defer cancel()

	return n.httpServer.Shutdown(ctxShutdown)
}

func (n *Node) ConnectToPeer(targetURL string) (*Peer, error) {
	conn, _, err := websocket.Dial(n.ctx, targetURL, nil)
	if err != nil {
		return nil, err
	}

	peer := newPeer(n.ctx, conn, RoleOutbound, n, targetURL)
	n.registerPeer(peer)
	peer.Start()

	logger.Logger.Info("Connected to remote RPC peer", "target", targetURL)
	return peer, nil
}

func (n *Node) handleWS(w http.ResponseWriter, r *http.Request) {
	// RemoteAddr aus dem HTTP Request merken
	clientAddr := r.RemoteAddr

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
	})
	if err != nil {
		logger.Logger.Error("Failed to accept websocket connection", "err", err)
		return
	}

	peer := newPeer(n.ctx, conn, RoleInbound, n, clientAddr)
	n.registerPeer(peer)
	peer.Start()
}

func (n *Node) registerPeer(p *Peer) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.peers[p] = true
}

func (n *Node) unregisterPeer(p *Peer) {
	n.mu.Lock()
	defer n.mu.Unlock()
	delete(n.peers, p)
}

func (n *Node) SetAuthenticator(auth UserAuthenticator) {
	n.router.SetAuthenticator(auth)
}

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
