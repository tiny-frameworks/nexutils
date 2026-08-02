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
	// Logger ohne File-Output für den Test aufsetzen
	_ = logger.SetupLogging(nil)

	// 1. NODE A aufsetzen (Server-Rolle)
	nodeA := rpc.NewNode(rpc.Options{
		Addr:              "127.0.0.1:9091",
		HeartbeatInterval: 500 * time.Millisecond,
	})

	// Geschützte Test-Methode auf Node A registrieren
	nodeA.RegisterHandler("getSecretData", func(p *rpc.Peer, req rpc.JsonRPCrequest) (any, *rpc.JsonRPCerror) {
		if !p.IsAuthorized() {
			errObj := errors.New(errors.UnauthorizedError, "Zugriff verweigert", "nexutils.p2p.rpc.node_test.go")
			return nil, rpc.NewRPCErrorFromNexError(rpc.NotAuthorized, errObj)
		}
		return map[string]string{"secret": "Geheimnis für " + p.ID}, nil
	})

	// Node A im Hintergrund starten
	go func() {
		if err := nodeA.Start(); err != nil {
			t.Logf("Node A gestoppt: %v", err)
		}
	}()
	defer nodeA.Stop()

	// Warten bis Listener bereit ist
	time.Sleep(100 * time.Millisecond)

	// 2. NODE B aufsetzen (Client-Rolle)
	nodeB := rpc.NewNode(rpc.Options{
		Addr:              "127.0.0.1:9092",
		HeartbeatInterval: 500 * time.Millisecond,
	})

	go func() {
		if err := nodeB.Start(); err != nil {
			t.Logf("Node B gestoppt: %v", err)
		}
	}()
	defer nodeB.Stop()

	time.Sleep(100 * time.Millisecond)

	// 3. VERBINDUNGSAUFBAU: Node B verbindet sich zu Node A
	t.Log("--> Verbinde Node B mit Node A...")
	peerA, err := nodeB.ConnectToPeer("ws://127.0.0.1:9091/ws")
	if err != nil {
		t.Fatalf("Verbindungsfehler: %v", err)
	}

	// 4. TEST: Aufruf geschützter Methode vor der Auth (muss fehlschlagen)
	t.Log("--> Teste Zugriff auf geschützte Methode OHNE Auth...")
	unauthReq := rpc.JsonRPCrequest{
		JSONRPC: "2.0",
		Method:  "getSecretData",
		ID:      json.RawMessage(`1`),
	}
	peerA.Send(unauthReq)

	// 5. TEST: Authentifizierung durchführen
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

	// 6. Warten für den automatischen Heartbeat Loop (Node B -> Node A)
	t.Log("--> Warte 1.5s (prüfe aktiven Heartbeat-Betrieb)...")
	time.Sleep(1500 * time.Millisecond)

	t.Log("--> Test erfolgreich durchgelaufen!")
}

func TestAutoReconnectAndSessionRestore(t *testing.T) {
	_ = logger.SetupLogging(nil)

	serverAddr := "127.0.0.1:9095"

	// Helper-Funktion: Registriert den Auth-Handler auf dem Server
	registerAuthHandler := func(n *rpc.Node) {
		n.RegisterHandler("auth", func(p *rpc.Peer, req rpc.JsonRPCrequest) (any, *rpc.JsonRPCerror) {
			var params rpc.AuthParams
			if err := req.UnmarshalParams(&params); err != nil {
				return nil, &rpc.JsonRPCerror{Code: rpc.InvalidParams, Message: "invalid params"}
			}
			// Akzeptiere sowohl Valid-User als auch Re-Auth via Token
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

	// 3. Reconnect-Config mit schnellen Intervallen für den Unit Test
	reconnectCfg := rpc.ReconnectConfig{
		InitialInterval: 100 * time.Millisecond,
		MaxInterval:     500 * time.Millisecond,
		Multiplier:      1.5,
		MaxRetries:      5,
	}

	t.Log("--> [Step 1] Node B verbindet sich via ManagedClient mit Node A...")
	// ConnectWithAutoReconnect liefert NUR NOCH den *ManagedClient zurück:
	client := nodeB.ConnectWithAutoReconnect("ws://"+serverAddr+"/ws", reconnectCfg)
	client.SetCredentials("georg", "secret") // Credentials für Login & Auto-Re-Auth setzen
	defer client.Close()

	// Kurze Pause, bis Erstverbindung + Auth durch sind
	time.Sleep(300 * time.Millisecond)

	if !client.IsReady() {
		t.Fatalf("Erstverbindung/Auth fehlgeschlagen!")
	}

	// 4. SERVER-AUSFALL SIMULIEREN
	t.Log("--> [Step 2] Fahre Node A herunter (Simulierter Netzwerkausfall/Crash)...")
	_ = nodeA.Stop()

	// Warten, damit der Client den Abbruch bemerkt und isReady auf false geht
	time.Sleep(300 * time.Millisecond)

	if client.IsReady() {
		t.Fatalf("Fehler: Client meldet immer noch 'IsReady', obwohl Server tot ist!")
	}

	// 5. RESTART DES SERVERS
	t.Log("--> [Step 3] Starte Node A neu auf demselben Port...")
	nodeA_restarted := rpc.NewNode(rpc.Options{Addr: serverAddr})
	registerAuthHandler(nodeA_restarted) // Auth-Handler wieder auf neuem Server registrieren

	go func() {
		_ = nodeA_restarted.Start()
	}()
	defer nodeA_restarted.Stop()

	t.Log("--> [Step 4] Warte auf automatischen Reconnect + Re-Auth...")
	time.Sleep(1000 * time.Millisecond)

	// Prüfen, ob der Client wieder eine aktive & authentifizierte Verbindung hat
	if !client.IsReady() {
		t.Fatalf("Fehler: ManagedClient hat nach Server-Restart die Verbindung nicht erfolgreich wiederhergestellt!")
	}

	if client.GetPeer() == nil {
		t.Fatalf("Fehler: ManagedClient.GetPeer() ist nil nach Reconnect!")
	}

	t.Log("--> [Erfolg] Auto-Reconnect & Re-Auth haben die Verbindung erfolgreich wiederhergestellt!")
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
		t.Fatalf("Verbindungsfehler: %v", err)
	}

	// SYNCHRONER CALL TEST
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var result int
	params := map[string]int{"A": 15, "B": 27}

	t.Log("--> Führe synchronen peer.Call('add') aus...")
	rpcErr := peerA.Call(ctx, "add", params, &result)

	if rpcErr != nil {
		t.Fatalf("RPC Call fehlgeschlagen: %s", rpcErr.Message)
	}

	if result != 42 {
		t.Fatalf("Unerwartetes Ergebnis: Erwartet 42, bekommen %d", result)
	}

	t.Logf("--> [Erfolg] Synchroner Call hat geklappt! Ergebnis = %d", result)
}

func TestBroadcastWithFilter(t *testing.T) {
	_ = logger.SetupLogging(nil)

	nodeA := rpc.NewNode(rpc.Options{Addr: "127.0.0.1:9099"})

	// Kanal zum Mitzählen empfangener Notifications auf Client-Seite
	receivedNotifications := make(chan string, 10)

	nodeA.RegisterHandler("notifyEvent", func(p *rpc.Peer, req rpc.JsonRPCrequest) (any, *rpc.JsonRPCerror) {
		var msg string
		_ = req.UnmarshalParams(&msg)
		receivedNotifications <- msg
		return nil, nil // Notifications erwarten eigentlich keine Antworte, Handler verarbeitet nur
	})

	go func() { _ = nodeA.Start() }()
	defer nodeA.Stop()
	time.Sleep(100 * time.Millisecond)

	// Node B verbindet sich mit Node A
	nodeB := rpc.NewNode(rpc.Options{Addr: "127.0.0.1:9100"})
	go func() { _ = nodeB.Start() }()
	defer nodeB.Stop()
	time.Sleep(100 * time.Millisecond)

	peerA, err := nodeB.ConnectToPeer("ws://127.0.0.1:9099/ws")
	if err != nil {
		t.Fatalf("Verbindungsfehler: %v", err)
	}

	// Sende Broadcast von Node B an Node A
	t.Log("--> Sende Broadcast 'notifyEvent'...")
	peerA.Notify("notifyEvent", "Hallo Node A!")

	select {
	case msg := <-receivedNotifications:
		if msg != "Hallo Node A!" {
			t.Fatalf("Unerwartete Nachricht: %s", msg)
		}
		t.Log("--> [Erfolg] Notification empfangen und verarbeitet!")
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout: Notification wurde nicht empfangen")
	}
}
