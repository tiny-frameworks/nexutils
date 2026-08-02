// Copyright 2026 Georg Hagn
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"codeberg.org/tiny-frameworks/nexutils/logger"
	"codeberg.org/tiny-frameworks/nexutils/p2p/rpc"
)

const paymentAddr = "ws://127.0.0.1:8080/ws"

func main() {

	// PreFlight / SetUp
	// Graceful Shutdown: activate signal-handling
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// initialize logger
	if err := logger.SetupLogging(nil); err != nil {
		log.Fatalf("Logger-Fehler: %v", err)
	}

	// Logger mit Kontext anreichern via .With()
	sysLog := logger.Logger.With("component", "order-service", "role", "client")
	sysLog.Info("=== Starte nexutils/p2p order-service Demo ===")

	// Take Off / Application
	// -------------------------------------------------------------------------
	// Knoten order-service (Client & Server zugleich in P2P)
	// -------------------------------------------------------------------------
	orderService := rpc.NewNode(rpc.Options{
		Addr:              "127.0.0.1:8081",
		HeartbeatInterval: 5 * time.Second,
	})

	// Register handler (What should happen if the payment service calls us?)
	orderService.RegisterHandler("order.process", func(p *rpc.Peer, req rpc.JsonRPCrequest) (any, *rpc.JsonRPCerror) {
		var params json.RawMessage
		req.UnmarshalParams(&params)
		sysLog.Info("[Order] Received status update from partner", "params", string(params), "peer", p.RemoteAddr())
		return "OK", nil
	})

	go func() {
		if err := orderService.Start(); err != nil {
			sysLog.Error("orderService gestoppt", "err", err)
		}
	}()
	defer orderService.Stop()

	// Kurze Pause, damit der eigene Node initialisieren kann
	time.Sleep(100 * time.Millisecond)

	// 3.) Statische Einmalverbindung (Direct Peer Connection)
	// Stellt eine direkte WebSocket-Verbindung zu einem spezifischen Peer her und liefert
	// ein festes *rpc.Peer-Handle zurück. Bei einem Verbindungsabbruch findet KEIN
	// automatischer Wiederaufbau statt; nachfolgende Calls schlagen fehl.
	// Ideal für einfache, kurzlebige Interaktionen oder kontrollierte Ad-hoc-Verbindungen.
	// Siehe auch: secret-service.go (resilienter ManagedClient mit Auto-Reconnect)
	paymentPeer, err := orderService.ConnectToPeer(paymentAddr)
	if err != nil {
		sysLog.Error("Verbindungsfehler zum Payment-Service", "err", err)
		return
	}

	// Aktive Logik (periodischer RPC-Call)
	go func() {
		clientLog := logger.Logger.With("component", "order-service", "role", "client")
		var cnt int = 0
		// Ticker ist oft sauberer als time.After in einer for-Schleife (verhindert Memory-Leaks)
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				cnt++
				orderID := fmt.Sprintf("Order_%d", cnt)
				clientLog.Info("[Order] Attempting payment", "orderID", orderID)

				var result string

				// 1. Lokalen Context erzeugen, der nach exakt 3 Sekunden abbricht
				callCtx, cancel := context.WithTimeout(ctx, 3*time.Second)

				// 2. callCtx (statt mainCtx) an den Peer übergeben
				rpcErr := paymentPeer.Call(callCtx, "payment.process", orderID, &result)

				// 3. Timer-Ressourcen des Contexts sofort wieder freigeben
				cancel()

				// Call ruft remote "payment.process" auf
				if rpcErr != nil {
					clientLog.Error("[Order] Payment failed – stop worker loop", "rpcErr", rpcErr)
					stop()
					return // beendet die Goroutine
				}

				clientLog.Info("💳 [Order] Confirmation received", "result", result)
			}
		}
	}()

	// Landing / Tear Down
	// Blockieren, bis Strg+C (SIGINT) oder SIGTERM gesendet wird
	sysLog.Info("Warte auf SIGINT/SIGTERM (mainCtx)...")
	<-ctx.Done()
	sysLog.Info("Signal empfangen! Grund:", "err", ctx.Err())
	orderService.Stop()
}
