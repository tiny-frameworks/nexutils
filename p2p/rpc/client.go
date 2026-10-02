// Copyright 2026 Georg Hagn
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"codeberg.org/tiny-frameworks/nexutils/logger"
)

type ReconnectConfig struct {
	InitialInterval time.Duration
	MaxInterval     time.Duration
	Multiplier      float64
	MaxRetries      int // 0 = infinite
}

func (c *ReconnectConfig) setDefaults() {
	if c.InitialInterval == 0 {
		c.InitialInterval = 1 * time.Second
	}
	if c.MaxInterval == 0 {
		c.MaxInterval = 10 * time.Second
	}
	if c.Multiplier == 0 {
		c.Multiplier = 2.0
	}
}

// ManagedClient manages a P2P connection, including auto-reconnect and session handling.
type ManagedClient struct {
	node      *Node
	targetURL string
	config    ReconnectConfig

	mu          sync.RWMutex
	currentPeer *Peer
	username    string
	password    string
	token       string
	isReady     bool
	closed      bool

	ctx    context.Context
	cancel context.CancelFunc
}

func NewManagedClient(node *Node, targetURL string, cfg ReconnectConfig) *ManagedClient {
	cfg.setDefaults()
	ctx, cancel := context.WithCancel(node.ctx)

	return &ManagedClient{
		node:      node,
		targetURL: targetURL,
		config:    cfg,
		ctx:       ctx,
		cancel:    cancel,
	}
}

// SetCredentials stores access credentials for automatic login and re-authentication processes.
func (mc *ManagedClient) SetCredentials(username, password string) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.username = username
	mc.password = password
}

// "Start" initiates the connection and the monitoring loop in the background.
func (mc *ManagedClient) Start() {
	go mc.lifecycleLoop()
}

// Close closes permanently terminates the client.
func (mc *ManagedClient) Close() {
	mc.mu.Lock()
	if mc.closed {
		mc.mu.Unlock()
		return
	}
	mc.closed = true
	peer := mc.currentPeer
	mc.isReady = false
	mc.mu.Unlock()

	mc.cancel()
	if peer != nil {
		peer.Close()
	}
}

func (mc *ManagedClient) IsClosed() bool {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	return mc.closed
}

func (mc *ManagedClient) IsReady() bool {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	return mc.isReady && !mc.closed
}

// Call forwards calls only when the connection is fully ready.
func (mc *ManagedClient) Call(ctx context.Context, method string, params any, resultTarget any) *JsonRPCerror {
	mc.mu.RLock()
	peer := mc.currentPeer
	ready := mc.isReady
	closed := mc.closed
	mc.mu.RUnlock()

	if closed {
		return &JsonRPCerror{Code: -32000, Message: "client is closed"}
	}
	if !ready || peer == nil {
		return &JsonRPCerror{Code: -32000, Message: "client connection is not ready (reconnecting...)"}
	}

	return peer.Call(ctx, method, params, resultTarget)
}

// GetPeer returns the current peer (or nil).
func (mc *ManagedClient) GetPeer() *Peer {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	return mc.currentPeer
}

// --- The central lifecycle loop (the SINGLE point for Connect & Reconnect) ---

func (mc *ManagedClient) lifecycleLoop() {
	interval := mc.config.InitialInterval
	attempts := 0

	for {
		// 1. Check whether the client or node has terminated.
		select {
		case <-mc.ctx.Done():
			return
		default:
		}

		attempts++
		if mc.config.MaxRetries > 0 && attempts > mc.config.MaxRetries {
			logger.Logger.Error("Max reconnect attempts reached. Client stopping permanently.", "target", mc.targetURL, "attempts", attempts)
			mc.Close()
			return
		}

		logger.Logger.Info(fmt.Sprintf("Connecting to peer (Attempt %d)...", attempts), "target", mc.targetURL)

		//PHASE 1: Connect & Authenticate
		peer, err := mc.node.ConnectToPeer(mc.targetURL)
		if err == nil {
			// Connection established; authenticate now.
			if err := mc.authenticatePeer(peer); err == nil {
				// SUCCESS! reset counter
				attempts = 0
				interval = mc.config.InitialInterval

				mc.mu.Lock()
				mc.currentPeer = peer
				mc.isReady = true
				mc.mu.Unlock()

				logger.Logger.Info("Successfully connected and authenticated!", "target", mc.targetURL, "peer", peer.ID)

				// PHASE 2: WAIT for connection termination
				// The loop cleanly blocks here as long as the connection is active!
				select {
				case <-mc.ctx.Done():
					return // Client was manually terminated.
				case <-peer.ctx.Done():
					// Connection lost! Reset status immediately.
					logger.Logger.Warn("Peer connection lost! Initiating reconnect...", "target", mc.targetURL)

					mc.mu.Lock()
					mc.currentPeer = nil
					mc.isReady = false
					mc.mu.Unlock()
				}
			} else {
				logger.Logger.Error("Authentication failed during connect", "target", mc.targetURL, "err", err)
				peer.Close()
			}
		} else {
			logger.Logger.Warn("Connection failed", "target", mc.targetURL, "err", err)
		}

		// PHASE 3: BACKOFF WAIT TIME (Always executed if Phase 1 or 2 fails or aborts)
		jitter := time.Duration(rand.Float64() * 0.4 * float64(interval))
		waitDuration := interval - (20 * interval / 100) + jitter

		logger.Logger.Info(fmt.Sprintf("Waiting %v before next reconnect attempt...", waitDuration.Round(time.Millisecond)), "target", mc.targetURL)

		select {
		case <-mc.ctx.Done():
			return
		case <-time.After(waitDuration):
		}

		// Double the backoff for the next failed attempt.
		interval = time.Duration(float64(interval) * mc.config.Multiplier)
		if interval > mc.config.MaxInterval {
			interval = mc.config.MaxInterval
		}
	}
}

func (mc *ManagedClient) authenticatePeer(p *Peer) error {
	mc.mu.RLock()
	user, pass, token := mc.username, mc.password, mc.token
	mc.mu.RUnlock()

	if user == "" && pass == "" && token == "" {
		return nil
	}

	ctx, cancel := context.WithTimeout(mc.ctx, 3*time.Second)
	defer cancel()

	var authRes AuthResult

	// --- SCHRITT A: Re-Authentifizierung mit vorhandenem Token ---
	if token != "" {
		rpcErr := p.Call(ctx, "system.auth", AuthParams{Token: token}, &authRes)
		if rpcErr == nil && authRes.Token != "" {
			// Erfolgreich via Token!
			mc.mu.Lock()
			mc.token = authRes.Token
			mc.mu.Unlock()

			// Fallback to mc.username, if authRes.Username was empty
			resolvedUser := authRes.Username
			if resolvedUser == "" {
				resolvedUser = user
			}

			// Auth-State am clientseitigen Peer nachziehen
			p.SetAuth(authRes.Username, authRes.Token, 24*time.Hour)

			logger.Logger.Info("Token re-authentication successful!", "peer", p.ID)
			return nil
		}

		logger.Logger.Warn("Token invalid or expired. Resetting token and retrying with credentials...", "peer", p.ID)

		mc.mu.Lock()
		mc.token = ""
		mc.mu.Unlock()
	}

	// --- SCHRITT B: Erst-Anmeldung via Username & Passwort ---
	if user != "" && pass != "" {
		rpcErr := p.Call(ctx, "system.auth", AuthParams{Username: user, Password: pass}, &authRes)
		if rpcErr != nil {
			return fmt.Errorf("credential auth failed: %s", rpcErr.Message)
		}

		mc.mu.Lock()
		mc.token = authRes.Token
		mc.mu.Unlock()

		// Auth-State am clientseitigen Peer setzen
		p.SetAuth(authRes.Username, authRes.Token, 24*time.Hour)

		logger.Logger.Info("Credential authentication successful! Saved new token.", "peer", p.ID)
		return nil
	}

	return fmt.Errorf("authentication failed: no valid token or credentials available")
}

// CallBatch führt mehrere RPC-Aufrufe als gebündelten Batch über den ManagedClient aus.
func (mc *ManagedClient) CallBatch(ctx context.Context, items []BatchItem) []*JsonRPCerror {
	mc.mu.RLock()
	peer := mc.currentPeer
	mc.mu.RUnlock()

	if peer == nil {
		return []*JsonRPCerror{{Code: InternalError, Message: "client connection is not ready"}}
	}

	return peer.CallBatch(ctx, items)
}
