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
	// SetUp
	// Graceful Shutdown: activate signal-handling
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// initialize logger
	if err := logger.SetupLogging(nil); err != nil {
		log.Fatalf("Logger-Error: %v", err)
	}

	// Enriching the logger with context via .With()
	log := logger.Logger.With("component", "payment-service", "role", "server")
	log.Info("=== Starte nexutils/p2p payment-service Demo ===")

	// -------------------------------------------------------------------------
	// Node payment-service (Server-Role)
	// -------------------------------------------------------------------------
	paymentService := rpc.NewNode(rpc.Options{
		Addr:              "127.0.0.1:8080",
		HeartbeatInterval: 5 * time.Second,
	})

	// Register application handler on Node paymentService
	paymentService.RegisterHandler("payment.process", func(p *rpc.Peer, req rpc.JsonRPCrequest) (any, *rpc.JsonRPCerror) {
		var orderID string

		req.UnmarshalParams(&orderID)
		successID := fmt.Sprintf("Payment_Success_ID: %s", orderID)
		log.Info("💳 Process payment", "orderID", orderID, "peer", p.RemoteAddr())

		// simulate success
		return successID, nil
	})

	// Start the paymentService node in a background thread.
	go func() {
		log.Info("Listen for incoming P2P connections...", "addr", "127.0.0.1:8080")
		if err := paymentService.Start(); err != nil {
			log.Error("Node 'paymentService' stoppt", "err", err)
		}
	}()

	// -------------------------------------------------------------------------
	// Broadcast-Loop in paymentService (Server-Role)
	// -------------------------------------------------------------------------
	go func() {
		//Logger for the background task
		bcastLog := log.With("task", "broadcast-emitter")

		// Send a system alert message to everyone every 15 seconds.
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()

		var counter int

		for {
			select {
			case <-ctx.Done():
				// Stop the loop when the paymentService shuts down.
				return

			case <-ticker.C:
				counter++
				alertMsg := fmt.Sprintf("System maintenance in %d minutes! (Broadcast #%d)", 60-counter, counter)

				bcastLog.Info("📢 Send BroadcastAuthorized to all authenticated peers....", "msg", alertMsg)

				// Send only to peers that have successfully completed the auth handshake ("georg")!
				paymentService.BroadcastAuthorized("systemAlert", alertMsg)
			}
		}
	}()

	// Block until Ctrl+C (SIGINT) or SIGTERM is sent
	log.Info("Wait for SIGINT/SIGTERM (mainCtx)...")
	<-ctx.Done()
	log.Info("Signal received! Reason:", "err", ctx.Err())

}
