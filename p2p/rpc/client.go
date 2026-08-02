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
	MaxRetries      int // 0 = unendlich
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

// ManagedClient verwaltet eine P2P-Verbindung inklusive Auto-Reconnect und Session.
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

// SetCredentials hinterlegt Zugangsdaten für automatische Login-/Re-Auth-Vorgänge.
func (mc *ManagedClient) SetCredentials(username, password string) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.username = username
	mc.password = password
}

// Start startet die Verbindung und den Überwachungs-Loop im Hintergrund.
func (mc *ManagedClient) Start() {
	go mc.lifecycleLoop()
}

// Close beendet den Client dauerhaft.
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

// Call leitet Aufrufe nur durch, wenn die Verbindung vollständig bereit (Ready) ist.
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

// GetPeer liefert den aktuellen Peer (oder nil).
func (mc *ManagedClient) GetPeer() *Peer {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	return mc.currentPeer
}

// --- Der zentrale Lifecycle-Loop (EINZIGE Stelle für Connect & Reconnect) ---

func (mc *ManagedClient) lifecycleLoop() {
	interval := mc.config.InitialInterval
	attempts := 0

	for {
		// 0. Prüfen, ob der Client oder Node beendet wurde
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

		// PHASE 1: Verbinden & Authentifizieren
		peer, err := mc.node.ConnectToPeer(mc.targetURL)
		if err == nil {
			// Verbindung steht, jetzt Authentifizieren
			if err := mc.authenticatePeer(peer); err == nil {
				// SUCCESS! Zähler zurücksetzen
				attempts = 0
				interval = mc.config.InitialInterval

				mc.mu.Lock()
				mc.currentPeer = peer
				mc.isReady = true
				mc.mu.Unlock()

				logger.Logger.Info("Successfully connected and authenticated!", "target", mc.targetURL, "peer", peer.ID)

				// PHASE 2: WARTEN auf Verbindungsabbruch
				// Die Schleife BLOCKIERT hier sauber, solange die Verbindung lebt!
				select {
				case <-mc.ctx.Done():
					return // Client wurde manuell beendet
				case <-peer.ctx.Done():
					// Verbindung wurde getrennt! Status sofort zurücksetzen
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

		// PHASE 3: BACKOFF WARTEZEIT (Wird IMMER ausgeführt, wenn Phase 1 oder 2 fehlschlagen/abbrechen)
		jitter := time.Duration(rand.Float64() * 0.4 * float64(interval))
		waitDuration := interval - (20 * interval / 100) + jitter

		logger.Logger.Info(fmt.Sprintf("Waiting %v before next reconnect attempt...", waitDuration.Round(time.Millisecond)), "target", mc.targetURL)

		select {
		case <-mc.ctx.Done():
			return
		case <-time.After(waitDuration):
		}

		// Backoff für den nächsten Fehlversuch verdoppeln
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

	// 1. Wenn überhaupt keine Auth-Daten vorliegen -> Sofort durchwinken
	if user == "" && pass == "" && token == "" {
		return nil
	}

	ctx, cancel := context.WithTimeout(mc.ctx, 3*time.Second)
	defer cancel()

	var authRes AuthResult

	// --- SCHRITT A: Versuch mit bestehendem Token ---
	if token != "" {
		rpcErr := p.Call(ctx, "auth", AuthParams{Token: token}, &authRes)
		if rpcErr == nil && authRes.Token != "" {
			// Token war noch gültig!
			mc.mu.Lock()
			mc.token = authRes.Token
			mc.mu.Unlock()
			logger.Logger.Info("Token re-authentication successful!", "peer", p.ID)
			return nil
		}

		// Token war ungültig oder abgelaufen!
		logger.Logger.Warn("Token invalid or expired. Resetting token and retrying with credentials...", "peer", p.ID)

		// Token löschen, damit zukünftige Versuche sauber sind
		mc.mu.Lock()
		mc.token = ""
		mc.mu.Unlock()
	}

	// --- SCHRITT B: Erst-Auth oder Fallback mit Username & Passwort ---
	if user != "" && pass != "" {
		rpcErr := p.Call(ctx, "auth", AuthParams{Username: user, Password: pass}, &authRes)
		if rpcErr != nil {
			return fmt.Errorf("credential auth failed: %s", rpcErr.Message)
		}

		// Neues Token für zukünftige Reconnects hinterlegen
		mc.mu.Lock()
		mc.token = authRes.Token
		mc.mu.Unlock()
		logger.Logger.Info("Credential authentication successful! Saved new token.", "peer", p.ID)
		return nil
	}

	return fmt.Errorf("authentication failed: no valid token or credentials available")
}
