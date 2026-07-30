// node.go
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

	peer := newPeer(n.ctx, conn, RoleOutbound, n)
	n.registerPeer(peer)
	peer.Start()

	logger.Logger.Info("Connected to remote RPC peer", "target", targetURL)
	return peer, nil
}

func (n *Node) handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
	})
	if err != nil {
		logger.Logger.Error("Failed to accept websocket connection", "err", err)
		return
	}

	peer := newPeer(n.ctx, conn, RoleInbound, n)
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
