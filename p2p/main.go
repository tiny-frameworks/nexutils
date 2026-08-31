package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"codeberg.org/tiny-frameworks/nexutils/logger"
	"codeberg.org/tiny-frameworks/nexutils/p2p/rpc"
)

// -------------------------------------------------------------------------
// DELEGATES
// -------------------------------------------------------------------------

type AddParams struct {
	A int `json:"a"`
	B int `json:"b"`
}

// NodeADelegate steuert das Anwendungsverhalten von Node A (Server)
type AuthParams struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type NodeADelegate struct {
	rpc.DefaultNexDelegate
}

func (d *NodeADelegate) OnRequest(ctx context.Context, peer *rpc.Peer, method string, params []byte) (any, error) {
	switch method {
	case "auth":
		var auth AuthParams
		_ = json.Unmarshal(params, &auth)

		// Authentifizierung bestätigen und Peer-State auf authorized setzen
		peer.SetAuth(auth.Username, "demo-token", 24*time.Hour)

		return map[string]any{
			"status":   "authenticated",
			"token":    "demo-token",
			"username": auth.Username,
		}, nil

	case "add":
		var p AddParams
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, &rpc.JsonRPCerror{Code: rpc.InvalidParams, Message: rpc.StdError[rpc.InvalidParams]}
		}

		logger.Logger.Info("Node A processes computer request", "peer", peer.ID, "user", peer.Username(), "a", p.A, "b", p.B)
		return p.A + p.B, nil

	default:
		return d.DefaultNexDelegate.OnRequest(ctx, peer, method, params)
	}
}

// NodeBDelegate steuert das Anwendungsverhalten von Node B (Client)
type NodeBDelegate struct {
	rpc.DefaultNexDelegate
}

func (d *NodeBDelegate) OnNotification(ctx context.Context, peer *rpc.Peer, method string, params []byte) {
	if method == "systemAlert" {
		var msg string
		_ = json.Unmarshal(params, &msg)
		logger.Logger.Info(">>> Node B received broadcast!", "Message", msg)
	}
}

// -------------------------------------------------------------------------
// MAIN
// -------------------------------------------------------------------------

func main() {
	// 1. Setting up the logger (console)
	if err := logger.SetupLogging(nil); err != nil {
		log.Fatalf("Logger-Error: %v", err)
	}

	logger.Logger.Info("=== Start nexutils/rpc P2P Demo ===")

	// -------------------------------------------------------------------------
	// KNOTEN A (Server-Role mit NodeADelegate)
	// -------------------------------------------------------------------------
	nodeA := rpc.NewNode(rpc.Options{
		Addr:              "127.0.0.1:8080",
		HeartbeatInterval: 5 * time.Second,
		Delegate:          &NodeADelegate{},
	})

	// Start Node A in background
	go func() {
		if err := nodeA.Start(); err != nil {
			logger.Logger.Error("Node A stopped", "err", err)
		}
	}()
	defer nodeA.Stop()

	// Wait until Listener A is ready.
	time.Sleep(100 * time.Millisecond)

	// -------------------------------------------------------------------------
	// KNOTEN B (Client-Role mit NodeBDelegate)
	// -------------------------------------------------------------------------
	nodeB := rpc.NewNode(rpc.Options{
		Addr:              "127.0.0.1:8081",
		HeartbeatInterval: 5 * time.Second,
		Delegate:          &NodeBDelegate{},
	})

	go func() {
		if err := nodeB.Start(); err != nil {
			logger.Logger.Error("Node B stopped", "err", err)
		}
	}()
	defer nodeB.Stop()

	time.Sleep(100 * time.Millisecond)

	// -------------------------------------------------------------------------
	// 1. P2P CONNECTION ESTABLISHMENT & AUTHENTICATION (Node B -> Node A)
	// -------------------------------------------------------------------------
	logger.Logger.Info("--> Connect Node B autonomous with Node A...")

	client := nodeB.ConnectWithAutoReconnect("ws://127.0.0.1:8080/ws", rpc.ReconnectConfig{
		InitialInterval: 1 * time.Second,
		MaxInterval:     10 * time.Second,
		MaxRetries:      5,
	})

	client.SetCredentials("georg", "secret")
	defer client.Close()

	time.Sleep(200 * time.Millisecond)

	// -------------------------------------------------------------------------
	// 2. SYNCHRONIZED RPC-CALL ("add") VIA MANAGED CLIENT
	// -------------------------------------------------------------------------
	logger.Logger.Info("--> Execute synchronous client.Call('add')...")

	callCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	var sum int

	rpcErr := client.Call(callCtx, "add", AddParams{A: 25, B: 17}, &sum)
	cancel()

	if rpcErr != nil {
		logger.Logger.Error("RPC Call 'add' failed", "err", rpcErr.Message)
	} else {
		fmt.Printf("\n========================================")
		fmt.Printf("\n  Result of Remote Call: 25 + 17 = %d", sum)
		fmt.Printf("\n========================================\n\n")
	}

	// -------------------------------------------------------------------------
	// 3. BROADCAST FROM SERVER TO LOGGED-IN CLIENTS
	// -------------------------------------------------------------------------
	logger.Logger.Info("--> Node A sends a broadcast to all registered peers....")
	nodeA.BroadcastAuthorized("systemAlert", "Maintenance work in 10 minutes!")

	// -------------------------------------------------------------------------
	// 4. RUN THE DEMO & MONITOR THE HEARTBEAT
	// -------------------------------------------------------------------------
	logger.Logger.Info("--> Demo running... Ending in 6 seconds (watch logs)")
	time.Sleep(6 * time.Second)

	logger.Logger.Info("=== Demo finished. ===")
}
