// Copyright 2026 Georg Hagn
// SPDX-License-Identifier: Apache-2.0

package transport

import (
	"context"
	stdErrors "errors"
	"log/slog"
	"net/http"

	"github.com/coder/websocket"
)

// WSProvider encapsulates the logic for establishing the connection.
type WSProvider struct {
	server *http.Server
	logger *slog.Logger
}

func NewWSProvider(logger *slog.Logger) *WSProvider {
	if logger == nil {
		logger = slog.Default() // Kein Panic, vernünftiger Fallback
	}
	return &WSProvider{logger: logger.With("component", "transport.ws")}
}

// Server is waiting for a connection (server-side)
// We use a channel to reconnect after the upgrade
func (p *WSProvider) Listen(ctx context.Context, addr string, found chan<- Connection) error {
	p.logger.Info("WebSocket Server startet...", "addres", addr)

	mux := http.NewServeMux()

	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			InsecureSkipVerify: true,
		})
		if err != nil {
			return
		}
		// Send new connection to the main inbox
		found <- &WSConnection{Conn: c}
	})

	p.server = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	// A goroutine that waits for the context to be terminated.
	go func() {
		<-ctx.Done()
		p.logger.Info("HTTP-Server fährt herunter...")
		p.server.Shutdown(context.Background())
	}()

	//ListenAndServe blocks here until Shutdown() is called.
	err := p.server.ListenAndServe()
	if stdErrors.Is(err, http.ErrServerClosed) {
		return nil // Ganz normales Herunterfahren
	}
	return err

}

// Dial connects to a server (client side)
func (p *WSProvider) Dial(ctx context.Context, url string) (Connection, error) {
	p.logger.Info("Dial...", "url", url)
	c, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		return nil, err
	}
	return &WSConnection{Conn: c}, nil
}
