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

// -------------------------------------------------------------------------
// OrderServiceDelegate
// -------------------------------------------------------------------------
type OrderServiceDelegate struct {
	rpc.DefaultNexDelegate
}

func (d *OrderServiceDelegate) OnRequest(ctx context.Context, peer *rpc.Peer, method string, params []byte) (any, error) {
	switch method {
	case "order.process":
		logger.Logger.Info("[Order] Received status update from partner", "params", string(params), "peer", peer.RemoteAddr())
		return "OK", nil

	default:
		return d.DefaultNexDelegate.OnRequest(ctx, peer, method, params)
	}
}

func (d *OrderServiceDelegate) OnNotification(ctx context.Context, peer *rpc.Peer, method string, params []byte) {
	if method == "systemAlert" {
		var msg string
		_ = json.Unmarshal(params, &msg)
		logger.Logger.Info("📢 [Order] Broadcast alert received", "msg", msg, "peer", peer.RemoteAddr())
	}
}

func main() {
	// Graceful Shutdown: Signal-Handling aktivieren
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Logger initialisieren
	if err := logger.SetupLogging(nil); err != nil {
		log.Fatalf("Logger-Error: %v", err)
	}

	sysLog := logger.Logger.With("component", "order-service", "role", "client")
	sysLog.Info("=== Starte nexutils/p2p order-service Demo ===")

	// -------------------------------------------------------------------------
	// Node order-service (Client & Server zeitgleich mit Delegate)
	// -------------------------------------------------------------------------
	orderService := rpc.NewNode(rpc.Options{
		Addr:              "127.0.0.1:8081",
		HeartbeatInterval: 5 * time.Second,
		Delegate:          &OrderServiceDelegate{},
	})

	go func() {
		if err := orderService.Start(); err != nil {
			sysLog.Error("orderService gestoppt", "err", err)
		}
	}()
	defer orderService.Stop()

	// Kurze Pause zur Initialisierung des eigenen Listeners
	time.Sleep(100 * time.Millisecond)

	// Direkte Peer-Verbindung herstellen
	paymentPeer, err := orderService.ConnectToPeer(paymentAddr)
	if err != nil {
		sysLog.Error("Connection error to the payment service", "err", err)
		return
	}

	// -------------------------------------------------------------------------
	// Periodische RPC-Call Schleife
	// -------------------------------------------------------------------------
	go func() {
		clientLog := logger.Logger.With("component", "order-service", "role", "client")
		var cnt int
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
				callCtx, cancel := context.WithTimeout(ctx, 3*time.Second)

				rpcErr := paymentPeer.Call(callCtx, "payment.process", orderID, &result)
				cancel()

				if rpcErr != nil {
					clientLog.Error("[Order] Payment failed – stop worker loop", "rpcErr", rpcErr)
					stop()
					return
				}

				clientLog.Info("💳 [Order] Confirmation received", "result", result)
			}
		}
	}()

	// Warten auf Beendigungssignal
	sysLog.Info("Wait for SIGINT/SIGTERM (mainCtx)...")
	<-ctx.Done()
	sysLog.Info("Signal received! Reason:", "err", ctx.Err())
}
