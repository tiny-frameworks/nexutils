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
	// PreFlight / SetUp
	// Graceful Shutdown: Haupt-Context für die gesamte Anwendungslebensdauer
	mainCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// initialize logger
	lgrCfg := &logger.LoggerConfig{
		Filename:   "./secret.log",
		UseLocking: false,
		Level:      logger.LevelInfo,
	}
	if err := logger.SetupLogging(lgrCfg); err != nil {
		log.Fatalf("Logger-Fehler: %v", err)
	}

	// Logger mit Kontext anreichern via .With()
	sysLog := logger.Logger.With("component", "secret-service", "role", "client")
	sysLog.Info("=== Starte nexutils/p2p secret-service Demo ===")

	// Take Off / Application
	secretService := rpc.NewNode(rpc.Options{
		Addr:              "127.0.0.1:8082",
		HeartbeatInterval: 5 * time.Second,
	})

	// Register handler: Broadcast-Empfänger
	secretService.RegisterHandler("systemAlert", func(p *rpc.Peer, req rpc.JsonRPCrequest) (any, *rpc.JsonRPCerror) {
		var msg string
		_ = req.UnmarshalParams(&msg)
		sysLog.Info("📢 >>> secretService hat Broadcast empfangen!", "nachricht", msg)
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

	// Kurze Pause für die Initialisierung
	time.Sleep(100 * time.Millisecond)

	// 3.) ManagedClient erzeugen & starten (Login & Reconnect geschehen autonom im Hintergrund)
	// Dynamische Client-Anbindung mit Auto-Reconnect & Session-Management
	// Im Gegensatz zu ConnectToPeer (order-service) liefert diese Methode einen
	// 'ManagedClient' zurück. Dieser verwaltet den Peer-Zeiger dynamisch bei Verbindungsabbrüchen,
	// führt automatische Reconnects durch und erlaubt re-authentication (Session-Persistenz).
	// Aufrufe erfolgen entkoppelt über client.Call(...), nicht direkt über ein statisches Peer-Objekt.
	// Siehe auch: order-service.go (statische Einmalverbindung)
	sysLog.Info("--> Starte ManagedClient für paymentService...")
	client := secretService.ConnectWithAutoReconnect(paymentAddr, rpc.ReconnectConfig{
		InitialInterval: 1 * time.Second,
		MaxInterval:     10 * time.Second,
		MaxRetries:      10,
	})
	client.SetCredentials("georg", "secret") // Credentials einmalig hinterlegen
	defer client.Close()

	// 4.) Aktive Logik (periodischer RPC-Call)
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

				// 1. Timeout für DIESEN EINZELNEN Aufruf definieren (3s Limit)
				callCtx, cancel := context.WithTimeout(mainCtx, 3*time.Second)
				var result string
				rpcErr := client.Call(callCtx, "payment.process", orderID, &result)
				cancel() // Ressourcen sofort wieder freigeben

				if rpcErr != nil {
					clientLog.Error("[Secret] Payment failed", "rpcErr", rpcErr)

					// Falls der Client nach 10 Fehlversuchen aufgeben hat:
					if client.IsClosed() {
						clientLog.Error("ManagedClient hat endgültig aufgegeben. Beende Worker.")
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
	sysLog.Info("Warte auf SIGINT/SIGTERM (mainCtx)...")
	<-mainCtx.Done()
	sysLog.Info("Signal empfangen! Grund:", "err", mainCtx.Err())
}
