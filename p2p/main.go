// Copyright 2026 Georg Hagn
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"codeberg.org/tiny-frameworks/nexutils/logger"
	"codeberg.org/tiny-frameworks/nexutils/p2p/rpc"
)

type AddParams struct {
	A int `json:"a"`
	B int `json:"b"`
}

func main() {
	// 1. Setting up the logger (console)
	if err := logger.SetupLogging(nil); err != nil {
		log.Fatalf("Logger-Error: %v", err)
	}

	logger.Logger.Info("=== Start nexutils/rpc P2P Demo ===")

	// -------------------------------------------------------------------------
	// KNOTEN A (Server-Role)
	// -------------------------------------------------------------------------
	nodeA := rpc.NewNode(rpc.Options{
		Addr:              "127.0.0.1:8080",
		HeartbeatInterval: 5 * time.Second,
	})

	// Register application handler on Node A
	nodeA.RegisterHandler("add", func(p *rpc.Peer, req rpc.JsonRPCrequest) (any, *rpc.JsonRPCerror) {
		// parse Parameter
		var params AddParams
		if err := req.UnmarshalParams(&params); err != nil {
			return nil, &rpc.JsonRPCerror{Code: rpc.InvalidParams, Message: rpc.StdError[rpc.InvalidParams]}
		}

		logger.Logger.Info("Node A processes computer request", "peer", p.ID, "user", p.Username(), "a", params.A, "b", params.B)
		return params.A + params.B, nil
	})

	// Start Node A in the background thread
	go func() {
		if err := nodeA.Start(); err != nil {
			logger.Logger.Error("Node A stopped", "err", err)
		}
	}()
	defer nodeA.Stop()

	// Wait until Listener A is ready.
	time.Sleep(100 * time.Millisecond)

	// -------------------------------------------------------------------------
	// KNOTEN B (Client-Role with Auto-Reconnect Manager)
	// -------------------------------------------------------------------------
	nodeB := rpc.NewNode(rpc.Options{
		Addr:              "127.0.0.1:8081",
		HeartbeatInterval: 5 * time.Second,
	})

	// Node B registers a handler to receive broadcasts.
	nodeB.RegisterHandler("systemAlert", func(p *rpc.Peer, req rpc.JsonRPCrequest) (any, *rpc.JsonRPCerror) {
		var msg string
		_ = req.UnmarshalParams(&msg)
		logger.Logger.Info(">>> Node B received broadcast!", "Message", msg)
		return nil, nil //Notifications do not require a response.
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
	logger.Logger.Info("--> Conncect Node B autonomous with Node A...")

	// Creates the ManagedClient and starts the lifecycle in the background.
	client := nodeB.ConnectWithAutoReconnect("ws://127.0.0.1:8080/ws", rpc.ReconnectConfig{
		InitialInterval: 1 * time.Second,
		MaxInterval:     10 * time.Second,
		MaxRetries:      5,
	})

	// Store credentials for initial login and automatic re-authentication
	client.SetCredentials("georg", "secret")
	defer client.Close()

	// Short wait time for initial connection & automatic background authentication
	time.Sleep(200 * time.Millisecond)

	// -------------------------------------------------------------------------
	// 2. SYNCHRONICED RPC-CALL ("add") VIA MANAGED CLIENT
	// -------------------------------------------------------------------------
	logger.Logger.Info("--> Execute synchronous client.Call('add')...")

	//Dedicated context for the RPC call (request timeout)
	callCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	var sum int

	rpcErr := client.Call(callCtx, "add", AddParams{A: 25, B: 17}, &sum)
	cancel() // Release the context's timer resources.

	if rpcErr != nil {
		logger.Logger.Error("RPC Call 'add' failed", "err", rpcErr.Message)
	} else {
		fmt.Printf("\n============== ========================")
		fmt.Printf("\n  Ressult of Remote Call: 25 + 17 = %d", sum)
		fmt.Printf("\n============= =========================\n\n")
	}

	// -------------------------------------------------------------------------
	// 3. BROADCAST FROM SERVER TO LOGGED-IN CLIENTS
	// -------------------------------------------------------------------------
	logger.Logger.Info("--> Node A sends a broadcast to all registered peers....")
	nodeA.BroadcastAuthorized("systemAlert", "Maintenance work in 10 minutes!")

	// -------------------------------------------------------------------------
	// 4.RUN THE DEMO & MONITOR THE HEARTBEAT
	// -------------------------------------------------------------------------
	logger.Logger.Info("--> Demo running... Ending in 6 seconds (watch logs)")
	time.Sleep(6 * time.Second)

	logger.Logger.Info("=== Demo finished. ===")
}
