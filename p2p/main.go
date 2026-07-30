package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	stdErrors "errors"

	"github.com/coder/websocket"

	"codeberg.org/tiny-frameworks/nexutils/errors"
	"codeberg.org/tiny-frameworks/nexutils/p2p/rpc"
	"codeberg.org/tiny-frameworks/nexutils/p2p/transport"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	provider := transport.NewWSProvider(logger)

	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}

	// --- 1. SERVER SETUP (Node A) ---
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		// WebSocket-Handshake durchführen
		wsConn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			logger.Error("Upgrade failed", "err", err)
			return
		}

		// Server-Node für diese eingehende Verbindung erstellen
		serverConn := &transport.WSConnection{
			Conn: wsConn,
		}
		//serverConn := transport.NewWSConnection(wsConn)
		serverNode := rpc.NewNode(serverConn, provider, "", logger)

		// Handler auf Server-Seite registrieren
		serverNode.Register("echo", func(ctx context.Context, params json.RawMessage) (any, error) {
			var input string
			_ = json.Unmarshal(params, &input)

			if input == "fail" {
				// Hier werfen wir deinen nexutils.Error
				return nil, errors.New("INVALID_INPUT", "Das Wort 'fail' ist nicht erlaubt", "main.handler")
			}
			return "OK: " + input, nil
		})

		// Auf eingehende Anfragen von Node B lauschen
		go serverNode.Listen(r.Context())
	})

	go func() {
		logger.Info("Starting WebSocket Server on :8080")
		if err := http.ListenAndServe(":8080", nil); err != nil {
			logger.Error("Server failed", "err", err)
		}
	}()

	time.Sleep(200 * time.Millisecond) // Kurz warten, bis Server gelauscht hat

	// --- 2. CLIENT SETUP (Node B) ---
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := provider.Dial(ctx, "ws://localhost:8080/ws")
	if err != nil {
		logger.Error("Dial failed", "err", err)
		return
	}

	clientNode := rpc.NewNode(conn, provider, "ws://localhost:8080/ws", logger)
	go clientNode.Listen(ctx)

	// --- 3. TEST CALL ---
	logger.Info("Executing RPC Call with error trigger...")
	_, callErr := clientNode.Call(ctx, "echo", "fail")
	if callErr != nil {
		var rpcErr *rpc.RPCError
		if stdErrors.As(callErr, &rpcErr) {
			if nexErr, ok := rpcErr.AsNexError(); ok {
				fmt.Printf("\n--> ERFOLG! Domänen-Fehler korrekt empfangen:\n")
				fmt.Printf("    Code:    %s\n", nexErr.Code)
				fmt.Printf("    Message: %s\n", nexErr.Message)
				return
			}
		}
		fmt.Printf("Normaler Fehler: %v\n", callErr)
	}
}
