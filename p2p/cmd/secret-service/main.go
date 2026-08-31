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
// SecretServiceDelegate
// -------------------------------------------------------------------------

type SecretServiceDelegate struct {
	rpc.DefaultNexDelegate
}

func (d *SecretServiceDelegate) OnRequest(ctx context.Context, peer *rpc.Peer, method string, params []byte) (any, error) {
	switch method {
	case "secret.process":
		logger.Logger.Info("[Secret] Received status update from partner", "params", string(params), "peer", peer.RemoteAddr())
		return "OK", nil

	default:
		return d.DefaultNexDelegate.OnRequest(ctx, peer, method, params)
	}
}

func (d *SecretServiceDelegate) OnNotification(ctx context.Context, peer *rpc.Peer, method string, params []byte) {
	if method == "systemAlert" {
		var msg string
		_ = json.Unmarshal(params, &msg)
		logger.Logger.Info("📢 >>> secretService has received Broadcast!", "message", msg)
	}
}

// -------------------------------------------------------------------------
// MAIN
// -------------------------------------------------------------------------
func main() {
	// Graceful Shutdown: Signal-Handling aktivieren
	mainCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Logger initialisieren
	lgrCfg := &logger.LoggerConfig{
		Filename:   "./secret.log",
		UseLocking: false,
		Level:      logger.LevelInfo,
	}
	if err := logger.SetupLogging(lgrCfg); err != nil {
		log.Fatalf("Logger-Error: %v", err)
	}

	sysLog := logger.Logger.With("component", "secret-service", "role", "client")
	sysLog.Info("=== Starte nexutils/p2p secret-service Demo ===")

	// -------------------------------------------------------------------------
	// Node secret-service (mit Delegate)
	// -------------------------------------------------------------------------
	secretService := rpc.NewNode(rpc.Options{
		Addr:              "127.0.0.1:8082",
		HeartbeatInterval: 5 * time.Second,
		Delegate:          &SecretServiceDelegate{},
	})

	go func() {
		if err := secretService.Start(); err != nil {
			sysLog.Error("secretService gestoppt", "err", err)
		}
	}()
	defer secretService.Stop()

	time.Sleep(100 * time.Millisecond)

	// -------------------------------------------------------------------------
	// ManagedClient mit Auto-Reconnect & Auth
	// -------------------------------------------------------------------------
	sysLog.Info("--> Start ManagedClient for paymentService...")
	client := secretService.ConnectWithAutoReconnect(paymentAddr, rpc.ReconnectConfig{
		InitialInterval: 1 * time.Second,
		MaxInterval:     10 * time.Second,
		MaxRetries:      10,
	})
	client.SetCredentials("georg", "secret")
	defer client.Close()

	// -------------------------------------------------------------------------
	// Periodische RPC-Call Schleife
	// -------------------------------------------------------------------------
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

				callCtx, cancel := context.WithTimeout(mainCtx, 3*time.Second)
				var result string
				rpcErr := client.Call(callCtx, "payment.process", orderID, &result)
				cancel()

				if rpcErr != nil {
					clientLog.Error("[Secret] Payment failed", "rpcErr", rpcErr)

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

	// Warten auf Beendigungssignal
	sysLog.Info("Wait for SIGINT/SIGTERM (mainCtx)...")
	<-mainCtx.Done()
	sysLog.Info("Signal received! Reason:", "err", mainCtx.Err())
}
