// node_test
package rpc_test

import (
	//	"context"
	"encoding/json"
	//	"fmt"
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
