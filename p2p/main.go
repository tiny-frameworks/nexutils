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
	// 1. Logger aufsetzen (Konsole)
	if err := logger.SetupLogging(nil); err != nil {
		log.Fatalf("Logger-Fehler: %v", err)
	}

	logger.Logger.Info("=== Starte nexutils/rpc P2P Demo ===")

	// -------------------------------------------------------------------------
	// KNOTEN A (Server-Rolle)
	// -------------------------------------------------------------------------
	nodeA := rpc.NewNode(rpc.Options{
		Addr:              "127.0.0.1:8080",
		HeartbeatInterval: 5 * time.Second,
	})

	// Anwendungs-Handler auf Node A registrieren
	nodeA.RegisterHandler("add", func(p *rpc.Peer, req rpc.JsonRPCrequest) (any, *rpc.JsonRPCerror) {
		// Parameter parsen
		var params AddParams
		if err := req.UnmarshalParams(&params); err != nil {
			return nil, &rpc.JsonRPCerror{Code: rpc.InvalidParams, Message: rpc.StdError[rpc.InvalidParams]}
		}

		logger.Logger.Info("Node A verarbeitet Rechner-Anfrage", "peer", p.ID, "user", p.Username(), "a", params.A, "b", params.B)
		return params.A + params.B, nil
	})

	// Node A im Hintergrund-Thread starten
	go func() {
		if err := nodeA.Start(); err != nil {
			logger.Logger.Error("Node A gestoppt", "err", err)
		}
	}()
	defer nodeA.Stop()

	// Warten bis Listener A bereit ist
	time.Sleep(100 * time.Millisecond)

	// -------------------------------------------------------------------------
	// KNOTEN B (Client-Rolle mit Auto-Reconnect Manager)
	// -------------------------------------------------------------------------
	nodeB := rpc.NewNode(rpc.Options{
		Addr:              "127.0.0.1:8081",
		HeartbeatInterval: 5 * time.Second,
	})

	// Node B registriert einen Handler, um Broadcasts zu empfangen
	nodeB.RegisterHandler("systemAlert", func(p *rpc.Peer, req rpc.JsonRPCrequest) (any, *rpc.JsonRPCerror) {
		var msg string
		_ = req.UnmarshalParams(&msg)
		logger.Logger.Info(">>> Node B hat Broadcast empfangen!", "nachricht", msg)
		return nil, nil // Notifications erwarten keine Antwort
	})

	go func() {
		if err := nodeB.Start(); err != nil {
			logger.Logger.Error("Node B gestoppt", "err", err)
		}
	}()
	defer nodeB.Stop()

	time.Sleep(100 * time.Millisecond)

	// -------------------------------------------------------------------------
	// 1. P2P VERBINDUNGSAUFBAU & AUTHENTIFIZIERUNG (Node B -> Node A)
	// -------------------------------------------------------------------------
	logger.Logger.Info("--> Verbinde Node B autonom mit Node A...")

	// Erzeugt den ManagedClient und startet den Lifecycle im Hintergrund
	client := nodeB.ConnectWithAutoReconnect("ws://127.0.0.1:8080/ws", rpc.ReconnectConfig{
		InitialInterval: 1 * time.Second,
		MaxInterval:     10 * time.Second,
		MaxRetries:      5,
	})

	// Credentials für Erst-Login & automatischen Re-Auth hinterlegen
	client.SetCredentials("georg", "secret")
	defer client.Close()

	// Kurze Wartezeit für Erst-Connect & automatische Authentifizierung im Hintergrund
	time.Sleep(200 * time.Millisecond)

	// -------------------------------------------------------------------------
	// 2. SYNCHRONER RPC-CALL ("add") VIA MANAGED CLIENT
	// -------------------------------------------------------------------------
	logger.Logger.Info("--> Führe synchronen client.Call('add') aus...")

	// Eigener Context für den RPC-Call (Request-Timeout)
	callCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	var sum int

	rpcErr := client.Call(callCtx, "add", AddParams{A: 25, B: 17}, &sum)
	cancel() // Timer-Ressourcen des Contexts freigeben

	if rpcErr != nil {
		logger.Logger.Error("RPC Call 'add' fehlgeschlagen", "err", rpcErr.Message)
	} else {
		fmt.Printf("\n==========================================")
		fmt.Printf("\n  Ergebnis vom Remote Call: 25 + 17 = %d", sum)
		fmt.Printf("\n==========================================\n\n")
	}

	// -------------------------------------------------------------------------
	// 3. BROADCAST VOM SERVER AN EINGELOGGTE CLIENTS
	// -------------------------------------------------------------------------
	logger.Logger.Info("--> Node A sendet Broadcast an alle angemeldeten Peers...")
	nodeA.BroadcastAuthorized("systemAlert", "Wartungsarbeiten in 10 Minuten!")

	// -------------------------------------------------------------------------
	// 4. DEMO LAUFEN LASSEN & HEARTBEAT BEOBACHTEN
	// -------------------------------------------------------------------------
	logger.Logger.Info("--> Demo läuft... Beende in 6 Sekunden (beobachte Logs)")
	time.Sleep(6 * time.Second)

	logger.Logger.Info("=== Demo beendet. ===")
}
