// Copyright 2026 Georg Hagn
// SPDX-License-Identifier: Apache-2.0

package rpc_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"codeberg.org/tiny-frameworks/nexutils/p2p/rpc"
)

// TestDelegate speichert Ereignisse für Assertionen
type TestDelegate struct {
	rpc.DefaultNexDelegate

	connectedPeers    []*rpc.Peer
	disconnectedPeers []*rpc.Peer
	lastMethod        string
	lastParams        []byte
}

func (d *TestDelegate) OnPeerConnected(peer *rpc.Peer) {
	d.connectedPeers = append(d.connectedPeers, peer)
}

func (d *TestDelegate) OnPeerDisconnected(peer *rpc.Peer, err error) {
	d.disconnectedPeers = append(d.disconnectedPeers, peer)
}

func (d *TestDelegate) OnRequest(ctx context.Context, peer *rpc.Peer, method string, params []byte) (any, error) {
	d.lastMethod = method
	d.lastParams = params

	switch method {
	case "ping":
		return "pong", nil
	case "fail":
		return nil, errors.New("custom error")
	default:
		return d.DefaultNexDelegate.OnRequest(ctx, peer, method, params)
	}
}

func TestNodeWithDelegate(t *testing.T) {
	serverDelegate := &TestDelegate{}

	// 1. Server-Node starten
	serverOpts := rpc.Options{
		Addr:     "127.0.0.1:8999",
		Delegate: serverDelegate,
	}
	serverNode := rpc.NewNode(serverOpts)

	go func() {
		_ = serverNode.Start()
	}()
	defer serverNode.Stop()

	// Kurz warten, bis Server lauscht
	time.Sleep(50 * time.Millisecond)

	// 2. Client-Node erstellen und verbinden
	clientNode := rpc.NewNode(rpc.Options{})
	defer clientNode.Stop()

	peer, err := clientNode.ConnectToPeer("ws://127.0.0.1:8999/ws")
	if err != nil {
		t.Fatalf("Failed to connect to peer: %v", err)
	}

	// Warten bis OnPeerConnected auf dem Server aufgerufen wurde
	time.Sleep(50 * time.Millisecond)
	if len(serverDelegate.connectedPeers) != 1 {
		t.Errorf("Expected 1 connected peer on server, got %d", len(serverDelegate.connectedPeers))
	}

	// 3. RPC Call testen ("ping" -> "pong")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var response string
	rpcErr := peer.Call(ctx, "ping", nil, &response)
	if rpcErr != nil {
		t.Fatalf("Unexpected RPC error: %v", rpcErr)
	}
	if response != "pong" {
		t.Errorf("Expected response 'pong', got '%s'", response)
	}

	// 4. Disconnect testen
	peer.Close()
	time.Sleep(50 * time.Millisecond)

	if len(serverDelegate.disconnectedPeers) != 1 {
		t.Errorf("Expected 1 disconnected peer event on server, got %d", len(serverDelegate.disconnectedPeers))
	}
}
