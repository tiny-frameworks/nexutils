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

	// SetUp
	// Graceful Shutdown: activate signal-handling
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// initialize logger
	if err := logger.SetupLogging(nil); err != nil {
		log.Fatalf("Logger-Error: %v", err)
	}

	// Enriching the logger with context via .With()
	sysLog := logger.Logger.With("component", "order-service", "role", "client")
	sysLog.Info("=== Starte nexutils/p2p order-service Demo ===")

	// Application
	// -------------------------------------------------------------------------
	// Node order-service (Client & Server at the same time in P2P)
	// -------------------------------------------------------------------------
	orderService := rpc.NewNode(rpc.Options{
		Addr:              "127.0.0.1:8081",
		HeartbeatInterval: 5 * time.Second,
	})

	// Send a system alert message to everyone every 15 seconds.
	orderService.RegisterHandler("order.process", func(p *rpc.Peer, req rpc.JsonRPCrequest) (any, *rpc.JsonRPCerror) {
		var params json.RawMessage
		req.UnmarshalParams(&params)
		sysLog.Info("[Order] Received status update from partner", "params", string(params), "peer", p.RemoteAddr())
		return "OK", nil
	})

	go func() {
		if err := orderService.Start(); err != nil {
			sysLog.Error("orderService stoppt", "err", err)
		}
	}()
	defer orderService.Stop()

	// Short delay to allow your own node to initialize.
	time.Sleep(100 * time.Millisecond)

	// 3.) Static one-off connection (Direct Peer Connection)
	// Establishes a direct WebSocket connection to a specific peer and returns
	// a fixed *rpc.Peer handle. If the connection drops, NO automatic
	// reconnection takes place; subsequent calls will fail.
	// Ideal for simple, short-lived interactions or controlled ad-hoc connections.
	// See also: secret-service.go (resilient ManagedClient with auto-reconnect)
	paymentPeer, err := orderService.ConnectToPeer(paymentAddr)
	if err != nil {
		sysLog.Error("Connection error to the payment service", "err", err)
		return
	}

	// Active Logic (periodic RPC-Call)
	go func() {
		clientLog := logger.Logger.With("component", "order-service", "role", "client")
		var cnt int = 0
		// `Ticker` is often cleaner than `time.After` inside a `for` loop (it prevents memory leaks).
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

				// 1. Create a local context that terminates after exactly 3 seconds.
				callCtx, cancel := context.WithTimeout(ctx, 3*time.Second)

				// 2. Pass callCtx (instead of mainCtx) to the peer.
				rpcErr := paymentPeer.Call(callCtx, "payment.process", orderID, &result)

				// 3. Immediately release the context's timer resources.
				cancel()

				// Call calls remote "payment.process"
				if rpcErr != nil {
					clientLog.Error("[Order] Payment failed – stop worker loop", "rpcErr", rpcErr)
					stop()
					return
				}

				clientLog.Info("💳 [Order] Confirmation received", "result", result)
			}
		}
	}()

	// Tear Down
	// Block until Ctrl+C (SIGINT) or SIGTERM is sent
	sysLog.Info("Wait for SIGINT/SIGTERM (mainCtx)...")
	<-ctx.Done()
	sysLog.Info("Signal received! Reason:", "err", ctx.Err())
	orderService.Stop()
}
