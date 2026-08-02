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

// ConnectWithAutoReconnect erstellt einen ManagedClient, der sich im Hintergrund
// autonom verbindet, authentifiziert und Reconnects verwaltet.
func (n *Node) ConnectWithAutoReconnect(targetURL string, cfg ReconnectConfig) *ManagedClient {
	client := NewManagedClient(n, targetURL, cfg)
	client.Start() // Startet den synchronisierenden lifecycleLoop im Hintergrund
	return client
}

// Broadcast Funktionalität
// PeerFilter ist eine Funktion, die entscheidet, ob ein Peer ein Signal erhalten soll.
type PeerFilter func(p *Peer) bool

// BroadcastFilter schickt ein Notification-Signal an alle Peers, auf die der Filter zutrifft.
func (n *Node) BroadcastFilter(method string, params any, filter PeerFilter) {
	n.mu.RLock()
	// Wir machen einen Schnappschuss der aktuellen Peers, um den Mutex schnell freizugeben
	activePeers := make([]*Peer, 0, len(n.peers))
	for p := range n.peers {
		activePeers = append(activePeers, p)
	}
	n.mu.RUnlock()

	// Über die Liste iterieren und filtern
	for _, p := range activePeers {
		// Falls ein Filter übergeben wurde und er 'false' liefert -> überspringen
		if filter != nil && !filter(p) {
			continue
		}

		if err := p.Notify(method, params); err != nil {
			logger.Logger.Warn("Failed to send broadcast notify to peer", "peer", p.ID, "err", err)
		}
	}
}

// Broadcast schickt ein Notification-Signal an ALLE verbundenen Peers.
func (n *Node) Broadcast(method string, params any) {
	n.BroadcastFilter(method, params, nil)
}

// BroadcastAuthorized schickt ein Notification-Signal NUR an authentifizierte Peers.
func (n *Node) BroadcastAuthorized(method string, params any) {
	n.BroadcastFilter(method, params, func(p *Peer) bool {
		return p.IsAuthorized()
	})
}

// BroadcastToUsers schickt ein Notification-Signal an eine spezifische Liste von Usernamen.
func (n *Node) BroadcastToUsers(method string, params any, usernames []string) {
	userMap := make(map[string]bool, len(usernames))
	for _, u := range usernames {
		userMap[u] = true
	}

	n.BroadcastFilter(method, params, func(p *Peer) bool {
		return p.IsAuthorized() && userMap[p.Username()]
	})
}
