// Copyright 2026 Georg Hagn
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"codeberg.org/tiny-frameworks/nexutils/logger"
	"codeberg.org/tiny-frameworks/nexutils/p2p/rpc"
)

// -------------------------------------------------------------------------
// PaymentServiceDelegate
// -------------------------------------------------------------------------
type AuthParams struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type PaymentServiceDelegate struct {
	rpc.DefaultNexDelegate
	log slog.Logger
}

func (d *PaymentServiceDelegate) OnRequest(ctx context.Context, peer *rpc.Peer, method string, params []byte) (any, error) {
	switch method {
	case "auth":
		var auth AuthParams
		_ = json.Unmarshal(params, &auth)

		// Peer authentifizieren und Token zuweisen
		peer.SetAuth(auth.Username, "payment-token", 24*time.Hour)

		return map[string]any{
			"status":   "authenticated",
			"token":    "payment-token",
			"username": auth.Username,
		}, nil

	case "payment.process":
		var orderID string
		_ = json.Unmarshal(params, &orderID)

		successID := fmt.Sprintf("Payment_Success_ID: %s", orderID)
		logger.Logger.Info("💳 Process payment", "orderID", orderID, "peer", peer.RemoteAddr())

		return successID, nil

	default:
		return d.DefaultNexDelegate.OnRequest(ctx, peer, method, params)
	}
}

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
	// Node payment-service (Server-Role mit Delegate)
	// -------------------------------------------------------------------------
	paymentService := rpc.NewNode(rpc.Options{
		Addr:              "127.0.0.1:8080",
		HeartbeatInterval: 5 * time.Second,
		Delegate:          &PaymentServiceDelegate{},
		//log:               logger.Logger,
	})

	// Node in einem Hintergrund-Thread starten
	go func() {
		log.Info("Listen for incoming P2P connections...", "addr", "127.0.0.1:8080")
		if err := paymentService.Start(); err != nil {
			log.Error("Node 'paymentService' gestoppt", "err", err)
		}
	}()
	defer paymentService.Stop()

	// -------------------------------------------------------------------------
	// Broadcast-Loop im paymentService
	// -------------------------------------------------------------------------
	go func() {
		bcastLog := log.With("task", "broadcast-emitter")

		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()

		var counter int

		for {
			select {
			case <-ctx.Done():
				return

			case <-ticker.C:
				counter++
				alertMsg := fmt.Sprintf("System maintenance in %d minutes! (Broadcast #%d)", 60-counter, counter)

				bcastLog.Info("📢 Send BroadcastAuthorized to all authenticated peers....", "msg", alertMsg)
				paymentService.BroadcastAuthorized("systemAlert", alertMsg)
			}
		}
	}()

	// Block until Ctrl+C (SIGINT) or SIGTERM is sent
	log.Info("Wait for SIGINT/SIGTERM (mainCtx)...")
	<-ctx.Done()
	log.Info("Signal received! Reason:", "err", ctx.Err())

}
