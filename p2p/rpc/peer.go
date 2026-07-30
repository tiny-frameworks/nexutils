package rpc

import (
	"context"
	"fmt"
	"sync"
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

type Peer struct {
	ID     string
	Role   PeerRole
	conn   *websocket.Conn
	node   *Node
	send   chan any
	ctx    context.Context
	cancel context.CancelFunc

	// Auth & State
	mu         sync.RWMutex
	username   string
	token      string
	expires    time.Time
	authorized bool
}

func newPeer(ctx context.Context, conn *websocket.Conn, role PeerRole, node *Node) *Peer {
	pCtx, cancel := context.WithCancel(ctx)
	return &Peer{
		ID:     fmt.Sprintf("%p", conn),
		Role:   role,
		conn:   conn,
		node:   node,
		send:   make(chan any, 32),
		ctx:    pCtx,
		cancel: cancel,
	}
}

func (p *Peer) Start() {
	go p.readLoop()
	go p.writeLoop()

	if p.Role == RoleOutbound {
		go p.heartbeatLoop()
	}
}

func (p *Peer) Close() {
	p.cancel()
	p.conn.Close(websocket.StatusNormalClosure, "closing peer")
}

// --- Loops ---

func (p *Peer) readLoop() {
	defer func() {
		p.node.unregisterPeer(p)
		p.Close()
	}()

	for {
		var req JsonRPCrequest
		// wsjson übernimmt das Lesen & Parsing
		err := wsjson.Read(p.ctx, p.conn, &req)
		if err != nil {
			if websocket.CloseStatus(err) != -1 {
				logger.Logger.Debug("Peer disconnected normally", "peer", p.ID)
			} else {
				logger.Logger.Error("Read error in peer readLoop", "peer", p.ID, "err", err)
			}
			return
		}

		p.node.requests <- clientRequest{peer: p, req: req}
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
			// wsjson übernimmt das Serialisieren & Schreiben
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
