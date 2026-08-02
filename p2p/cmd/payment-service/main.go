// Copyright 2026 Georg Hagn
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"codeberg.org/tiny-frameworks/nexutils/logger"
	"codeberg.org/tiny-frameworks/nexutils/p2p/rpc"
)

func main() {
	// Graceful Shutdown Signal-Handling aktivieren
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Logger initialisieren
	if err := logger.SetupLogging(nil); err != nil {
		log.Fatalf("Logger-Fehler: %v", err)
	}

	// Logger mit Kontext anreichern via .With()
	log := logger.Logger.With("component", "payment-service", "role", "server")
	log.Info("=== Starte nexutils/p2p payment-service Demo ===")

	// -------------------------------------------------------------------------
	// Knoten payment-service (Server-Rolle)
	// -------------------------------------------------------------------------
	paymentService := rpc.NewNode(rpc.Options{
		Addr:              "127.0.0.1:8080",
		HeartbeatInterval: 5 * time.Second,
	})

	// Anwendungs-Handler auf Node paymentService registrieren
	paymentService.RegisterHandler("payment.process", func(p *rpc.Peer, req rpc.JsonRPCrequest) (any, *rpc.JsonRPCerror) {
		var orderID string

		req.UnmarshalParams(&orderID)
		successID := fmt.Sprintf("Payment_Success_ID: %s", orderID)
		log.Info("💳 Process payment", "orderID", orderID, "peer", p.RemoteAddr())

		// simuliere success
		return successID, nil
	})

	// Node paymentService im Hintergrund-Thread starten
	go func() {
		log.Info("Listen for incoming P2P connections...", "addr", "127.0.0.1:8080")
		if err := paymentService.Start(); err != nil {
			log.Error("Node 'paymentService' gestoppt", "err", err)
		}
	}()

	// -------------------------------------------------------------------------
	// Broadcast-Schleife im paymentService (Server-Rolle)
	// -------------------------------------------------------------------------
	go func() {
		// Logger für die Hintergrund-Aufgabe
		bcastLog := log.With("task", "broadcast-emitter")

		// Alle 15 Sekunden eine System-Alert-Nachricht an alle schicken
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()

		var counter int

		for {
			select {
			case <-ctx.Done():
				// Stoppt die Schleife, wenn der paymentService herunterfährt
				return

			case <-ticker.C:
				counter++
				alertMsg := fmt.Sprintf("System-Wartung in %d Minuten! (Broadcast #%d)", 60-counter, counter)

				bcastLog.Info("📢 Sende BroadcastAuthorized an alle authentifizierten Peers...", "msg", alertMsg)

				// Nur an Peers senden, die den Auth-Handshake ("georg") erfolgreich absolviert haben!
				paymentService.BroadcastAuthorized("systemAlert", alertMsg)
			}
		}
	}()

	// Blockieren, bis Strg+C (SIGINT) oder SIGTERM gesendet wird
	log.Info("Warte auf SIGINT/SIGTERM (mainCtx)...")
	<-ctx.Done()
	log.Info("Signal empfangen! Grund:", "err", ctx.Err())

}
