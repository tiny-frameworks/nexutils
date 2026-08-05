// Copyright 2026 Georg Hagn
// SPDX-License-Identifier: Apache-2.0

package rpc_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"codeberg.org/tiny-frameworks/nexutils/errors"
	"codeberg.org/tiny-frameworks/nexutils/logger"
	"codeberg.org/tiny-frameworks/nexutils/p2p/rpc"
)

func TestP2PCommunicationAndAuth(t *testing.T) {
	// Set up a logger without file output for testing.
	_ = logger.SetupLogging(nil)

	// 1. Set up NODE A (server role)
	nodeA := rpc.NewNode(rpc.Options{
		Addr:              "127.0.0.1:9091",
		HeartbeatInterval: 500 * time.Millisecond,
	})

	// Register protected test method on Node A
	nodeA.RegisterHandler("getSecretData", func(p *rpc.Peer, req rpc.JsonRPCrequest) (any, *rpc.JsonRPCerror) {
		if !p.IsAuthorized() {
			errObj := errors.New(errors.UnauthorizedError, "Access denied", "nexutils.p2p.rpc.node_test.go")
			return nil, rpc.NewRPCErrorFromNexError(rpc.NotAuthorized, errObj)
		}
		return map[string]string{"secret": "Secret to " + p.ID}, nil
	})

	// Start Node A in the background
	go func() {
		if err := nodeA.Start(); err != nil {
			t.Logf("Node A stopped: %v", err)
		}
	}()
	defer nodeA.Stop()

	// Wait until the listener is ready
	time.Sleep(100 * time.Millisecond)

	// 2. Set up NODE B (client role)
	nodeB := rpc.NewNode(rpc.Options{
		Addr:              "127.0.0.1:9092",
		HeartbeatInterval: 500 * time.Millisecond,
	})

	go func() {
		if err := nodeB.Start(); err != nil {
			t.Logf("Node B stopped: %v", err)
		}
	}()
	defer nodeB.Stop()

	time.Sleep(100 * time.Millisecond)

	// 3. CONNECTION ESTABLISHMENT: Node B connects to Node A.
	t.Log("--> Connect Node B to Node A....")
	peerA, err := nodeB.ConnectToPeer("ws://127.0.0.1:9091/ws")
	if err != nil {
		t.Fatalf("Connection error: %v", err)
	}

	// 4. TEST: Call to protected method before authentication (must fail)
	t.Log("--> Test access to protected method WITHOUT auth...")
	unauthReq := rpc.JsonRPCrequest{
		JSONRPC: "2.0",
		Method:  "getSecretData",
		ID:      json.RawMessage(`1`),
	}
	peerA.Send(unauthReq)

	// 5. TEST: Perform authentication
	t.Log("--> Sende Auth-Anfrage an Node A...")
	authParams, _ := json.Marshal(map[string]string{
		"username": "georg",
		"password": "secret",
	})
	authReq := rpc.JsonRPCrequest{
		JSONRPC: "2.0",
		Method:  "auth",
		Params:  authParams,
		ID:      json.RawMessage(`2`),
	}
	peerA.Send(authReq)

	// 6. Waiting for the automatic heartbeat loop (Node B -> Node A)
	t.Log("--> Wait 1.5s (check active heartbeat operation)...")
	time.Sleep(1500 * time.Millisecond)

	t.Log("--> Test completed successfully!")
}

func TestAutoReconnectAndSessionRestore(t *testing.T) {
	_ = logger.SetupLogging(nil)

	serverAddr := "127.0.0.1:9095"

	// Helper-Funkcion: Registers the auth handler on the server.
	registerAuthHandler := func(n *rpc.Node) {
		n.RegisterHandler("auth", func(p *rpc.Peer, req rpc.JsonRPCrequest) (any, *rpc.JsonRPCerror) {
			var params rpc.AuthParams
			if err := req.UnmarshalParams(&params); err != nil {
				return nil, &rpc.JsonRPCerror{Code: rpc.InvalidParams, Message: "invalid params"}
			}
			// Accept both valid users and re-authentication via token.
			if params.Username == "georg" || params.Token != "" {
				return rpc.AuthResult{Token: "test-session-token-12345", Status: "OK"}, nil
			}
			return nil, &rpc.JsonRPCerror{Code: rpc.InternalError, Message: "auth failed"}
		})
	}

	// 1. NODE A (Server) initial starten
	nodeA := rpc.NewNode(rpc.Options{Addr: serverAddr})
	registerAuthHandler(nodeA)

	go func() {
		_ = nodeA.Start()
	}()
	time.Sleep(100 * time.Millisecond) // Listener-Time-to-Ready

	// 2. NODE B (Client) aufbauen
	nodeB := rpc.NewNode(rpc.Options{Addr: "127.0.0.1:9096"})
	go func() {
		_ = nodeB.Start()
	}()
	defer nodeB.Stop()
	time.Sleep(100 * time.Millisecond)

	// 3. Reconnect configuration with rapid intervals for the unit test
	reconnectCfg := rpc.ReconnectConfig{
		InitialInterval: 100 * time.Millisecond,
		MaxInterval:     500 * time.Millisecond,
		Multiplier:      1.5,
		MaxRetries:      5,
	}

	t.Log("--> [Step 1] Node B verbindet sich via ManagedClient mit Node A...")
	// ConnectWithAutoReconnect now returns ONLY the *ManagedClient:
	client := nodeB.ConnectWithAutoReconnect("ws://"+serverAddr+"/ws", reconnectCfg)
	client.SetCredentials("georg", "secret") // Credentials for Login & Auto-Re-Auth
	defer client.Close()

	// Short delay until the initial connection and authentication are complete.
	time.Sleep(300 * time.Millisecond)

	if !client.IsReady() {
		t.Fatalf("Initial connection/authentication failed!")
	}

	// 4. Simulate server failure
	t.Log("--> [Step 2] Shut down Node A (simulated network failure/crash)...")
	_ = nodeA.Stop()

	// Wait for the client to detect the cancellation and for isReady to switch to false.
	time.Sleep(300 * time.Millisecond)

	if client.IsReady() {
		t.Fatalf("Error: Client still reports 'IsReady' even though the server is down!")
	}

	// 5. RESTART DES SERVERS
	t.Log("--> [Step 3] Restart Node A on the same port....")
	nodeA_restarted := rpc.NewNode(rpc.Options{Addr: serverAddr})
	registerAuthHandler(nodeA_restarted) // again register Auth-Handler to new Server

	go func() {
		_ = nodeA_restarted.Start()
	}()
	defer nodeA_restarted.Stop()

	t.Log("--> [Step 4] Waiting for automatic reconnect+ Re-Auth...")
	time.Sleep(1000 * time.Millisecond)

	// Check whether the client has an active and authenticated connection again.
	if !client.IsReady() {
		t.Fatalf("Error: ManagedClient failed to successfully re-establish the connection after the server restart!")
	}

	if client.GetPeer() == nil {
		t.Fatalf("Error: ManagedClient.GetPeer() is nil after reconnect!")
	}

	t.Log("--> [Success] Auto-reconnect and re-auth have successfully restored the connection!")
}

func TestSynchronousCall(t *testing.T) {
	_ = logger.SetupLogging(nil)

	// Node A starten mit Add-Handler
	nodeA := rpc.NewNode(rpc.Options{Addr: "127.0.0.1:9097"})
	nodeA.RegisterHandler("add", func(p *rpc.Peer, req rpc.JsonRPCrequest) (any, *rpc.JsonRPCerror) {
		var pStruct struct{ A, B int }
		_ = req.UnmarshalParams(&pStruct)
		return pStruct.A + pStruct.B, nil
	})

	go func() { _ = nodeA.Start() }()
	defer nodeA.Stop()
	time.Sleep(100 * time.Millisecond)

	// Node B verbindet sich zu Node A
	nodeB := rpc.NewNode(rpc.Options{Addr: "127.0.0.1:9098"})
	go func() { _ = nodeB.Start() }()
	defer nodeB.Stop()
	time.Sleep(100 * time.Millisecond)

	peerA, err := nodeB.ConnectToPeer("ws://127.0.0.1:9097/ws")
	if err != nil {
		t.Fatalf("Connection error: %v", err)
	}

	// SYNCHRONER CALL TEST
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var result int
	params := map[string]int{"A": 15, "B": 27}

	t.Log("--> Execute synchronous peer.Call('add')...")
	rpcErr := peerA.Call(ctx, "add", params, &result)

	if rpcErr != nil {
		t.Fatalf("RPC Call failed: %s", rpcErr.Message)
	}

	if result != 42 {
		t.Fatalf("Unexpected result: Expected 42, got %d", result)
	}

	t.Logf("--> [Success] Synchronous call succeeded! Result = %d", result)
}

func TestBroadcastWithFilter(t *testing.T) {
	_ = logger.SetupLogging(nil)

	nodeA := rpc.NewNode(rpc.Options{Addr: "127.0.0.1:9099"})

	//Channel for counting received notifications on the client side
	receivedNotifications := make(chan string, 10)

	nodeA.RegisterHandler("notifyEvent", func(p *rpc.Peer, req rpc.JsonRPCrequest) (any, *rpc.JsonRPCerror) {
		var msg string
		_ = req.UnmarshalParams(&msg)
		receivedNotifications <- msg
		return nil, nil // Notifications do not actually expect a response; the handler simply processes them.
	})

	go func() { _ = nodeA.Start() }()
	defer nodeA.Stop()
	time.Sleep(100 * time.Millisecond)

	// Node B connects to Node A.
	nodeB := rpc.NewNode(rpc.Options{Addr: "127.0.0.1:9100"})
	go func() { _ = nodeB.Start() }()
	defer nodeB.Stop()
	time.Sleep(100 * time.Millisecond)

	peerA, err := nodeB.ConnectToPeer("ws://127.0.0.1:9099/ws")
	if err != nil {
		t.Fatalf("Connection error: %v", err)
	}

	// Sende Broadcast von Node B an Node A
	t.Log("--> Send Broadcast 'notifyEvent'...")
	peerA.Notify("notifyEvent", "Hallo Node A!")

	select {
	case msg := <-receivedNotifications:
		if msg != "Hallo Node A!" {
			t.Fatalf("Unespected Message: %s", msg)
		}
		t.Log("--> [Success] Notification received and processed!")
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout: Notification was not received.")
	}
}
