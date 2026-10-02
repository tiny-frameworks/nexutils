package rpc_test

import (
	"testing"
	"time"

	"codeberg.org/tiny-frameworks/nexutils/p2p/rpc"
)

func TestManagedClient_SessionStore_Reconnect(t *testing.T) {
	// 1. Server-Node mit DefaultNexDelegate aufsetzen (nutzt intern den SessionStore)
	serverDelegate := &rpc.DefaultNexDelegate{}
	serverNode := rpc.NewNode(rpc.Options{
		Addr:     "127.0.0.1:9095",
		Delegate: serverDelegate,
	})

	go serverNode.Start()
	defer serverNode.Stop()

	// Warten bis der Server-Listener bereit ist
	time.Sleep(100 * time.Millisecond)

	// 2. Client-Node aufsetzen
	clientNode := rpc.NewNode(rpc.Options{
		Addr: "127.0.0.1:9096",
	})
	go clientNode.Start()
	defer clientNode.Stop()

	time.Sleep(100 * time.Millisecond)

	// 3. ManagedClient konfigurieren (schnelles Reconnect-Intervall für den Test)
	reconnectCfg := rpc.ReconnectConfig{
		InitialInterval: 500 * time.Millisecond,
		MaxInterval:     2 * time.Second,
		Multiplier:      1.5,
		MaxRetries:      5,
	}

	client := clientNode.ConnectWithAutoReconnect("ws://127.0.0.1:9095/ws", reconnectCfg)
	client.SetCredentials("georg", "secret")
	defer client.Close()

	// -------------------------------------------------------------------------
	// PHASE 1: Erstmaliger Verbindungsaufbau & Login via Credentials
	// -------------------------------------------------------------------------
	assertClientReady(t, client, 1*time.Second, "Client sollte nach Erst-Login bereit sein")

	firstPeer := client.GetPeer()
	if firstPeer == nil {
		t.Fatal("Client sollte eine valide Peer-Referenz besitzen")
	}

	t.Logf("Erstverbindung erfolgreich! Peer-ID: %s, Username: %s", firstPeer.ID, firstPeer.Username())

	// -------------------------------------------------------------------------
	// PHASE 2: Unterbrechung erzwingen (Verbindung serverseitig kappen)
	// -------------------------------------------------------------------------
	t.Log("Kappe die Verbindung serverseitig...")
	firstPeer.Close()

	// Kurze Pause, damit die lifecycleLoop den Verlust registriert
	time.Sleep(10 * time.Millisecond)

	if client.IsReady() {
		t.Error("Client sollte direkt nach dem Verbindungsabbruch NICHT mehr ready sein")
	}

	// -------------------------------------------------------------------------
	// PHASE 3: Autonomer Reconnect & Re-Auth über SessionStore-Token
	// -------------------------------------------------------------------------
	assertClientReady(t, client, 3*time.Second, "Client sollte sich via Token-Session wieder erfolgreich verbinden")

	reconnectedPeer := client.GetPeer()
	if reconnectedPeer == nil {
		t.Fatal("Reconnected Client sollte eine neue Peer-Referenz besitzen")
	}

	if reconnectedPeer.ID == firstPeer.ID {
		t.Error("Nach dem Reconnect sollte eine NEUE Peer-Instanz erzeugt worden sein")
	}

	if !reconnectedPeer.IsAuthorized() {
		t.Error("Der neue Peer muss nach der Token-Restaurierung als autorisiert markiert sein")
	}

	if reconnectedPeer.Username() != "georg" {
		t.Errorf("Erwarteter Username 'georg', erhalten: %s", reconnectedPeer.Username())
	}

	t.Logf("Reconnect erfolgreich via SessionStore! Neue Peer-ID: %s", reconnectedPeer.ID)
}

// Hilfsfunktion zum Warten auf den Ready-Zustand des ManagedClients
func assertClientReady(t *testing.T, mc *rpc.ManagedClient, timeout time.Duration, errorMsg string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if mc.IsReady() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("Timeout (%v): %s", timeout, errorMsg)
}
