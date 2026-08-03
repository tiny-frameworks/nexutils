// Copyright 2026 Georg Hagn
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	//"log/slog"
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
	mainCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// initialize logger
	lgrCfg := &logger.LoggerConfig{
		Filename:   "./secret.log",
		UseLocking: false,
		Level:      logger.LevelInfo,
	}
	if err := logger.SetupLogging(lgrCfg); err != nil {
		log.Fatalf("Logger-Error: %v", err)
	}

	// Enriching the logger with context via .With()
	sysLog := logger.Logger.With("component", "secret-service", "role", "client")
	sysLog.Info("=== Starte nexutils/p2p secret-service Demo ===")

	// Application
	secretService := rpc.NewNode(rpc.Options{
		Addr:              "127.0.0.1:8082",
		HeartbeatInterval: 5 * time.Second,
	})

	// Register handler: Broadcast-Receiver
	secretService.RegisterHandler("systemAlert", func(p *rpc.Peer, req rpc.JsonRPCrequest) (any, *rpc.JsonRPCerror) {
		var msg string
		_ = req.UnmarshalParams(&msg)
		sysLog.Info("📢 >>> secretService has received Broadcast!", "message", msg)
		return nil, nil
	})

	// Register handler: Incoming RPC Calls
	secretService.RegisterHandler("secret.process", func(p *rpc.Peer, req rpc.JsonRPCrequest) (any, *rpc.JsonRPCerror) {
		var params json.RawMessage
		req.UnmarshalParams(&params)
		sysLog.Info("[Secret] Received status update from partner", "params", string(params), "peer", p.RemoteAddr())
		return "OK", nil
	})

	// 2.) Start Listening
	go func() {
		if err := secretService.Start(); err != nil {
			sysLog.Error("secretService gestoppt", "err", err)
		}
	}()
	defer secretService.Stop()

	// short delay for initialization
	time.Sleep(100 * time.Millisecond)

	// 3.) Create & start ManagedClient (login & reconnect happen autonomously in the background)
	// Dynamic client connection with auto-reconnect & session management
	// Unlike ConnectToPeer (order-service), this method returns a
	// 'ManagedClient'. This manages the peer pointer dynamically during connection drops,
	// performs automatic reconnects, and allows re-authentication (session persistence).
	// Calls are made in a decoupled manner via client.Call(...), not directly via a static peer object.
	// See also: order-service.go (static one-time connection)
	sysLog.Info("--> Start ManagedClient for paymentService...")
	client := secretService.ConnectWithAutoReconnect(paymentAddr, rpc.ReconnectConfig{
		InitialInterval: 1 * time.Second,
		MaxInterval:     10 * time.Second,
		MaxRetries:      10,
	})
	client.SetCredentials("georg", "secret") // Enter credentials once
	defer client.Close()

	// 4.) Active Logic (periodicr RPC-Call)
	go func() {
		clientLog := logger.Logger.With("component", "secret-service", "role", "client")
		cnt := 0
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-mainCtx.Done():
				return
			case <-ticker.C:
				cnt++
				orderID := fmt.Sprintf("Order_%d", cnt)

				// 1. Define a timeout for THIS SPECIFIC call (3s limit)
				callCtx, cancel := context.WithTimeout(mainCtx, 3*time.Second)
				var result string
				rpcErr := client.Call(callCtx, "payment.process", orderID, &result)
				cancel() // Release resources immediately

				if rpcErr != nil {
					clientLog.Error("[Secret] Payment failed", "rpcErr", rpcErr)

					// If the client has given up after 10 failed attempts:
					if client.IsClosed() {
						clientLog.Error("ManagedClient has given up for good. Shutting down worker.")
						stop()
						return
					}
					continue
				}

				clientLog.Info("💳 [Secret] Confirmation received", "result", result)
			}
		}
	}()

	// Landing / Tear Down
	sysLog.Info("Wait for SIGINT/SIGTERM (mainCtx)...")
	<-mainCtx.Done()
	sysLog.Info("Signal received! Reason:", "err", mainCtx.Err())
}
